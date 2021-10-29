package db

import (
	"context"
	"fmt"
	"io"
	"sort"
	"strings"

	vcutils "tierceron/trcconfig/utils"
	"tierceron/trcx/extract"
	eUtils "tierceron/utils"
	"tierceron/vaulthelper/kv"

	sqle "github.com/dolthub/go-mysql-server"
	"github.com/dolthub/go-mysql-server/memory"
	"github.com/dolthub/go-mysql-server/sql"
)

func writeToTable(te *TierceronEngine, envEnterprise string, version string, project string, service string, templateResult *extract.TemplateResultData) {

	//
	// What we need is in ValueSection and SecretSection...
	//
	if templateResult.InterfaceTemplateSection == nil {
		// No templates, no configs, no tables.
		return
	}

	// Create tables with naming convention: Service.configFileName  Column names should be template variable names.
	configTableMap := templateResult.InterfaceTemplateSection.(map[string]interface{})["templates"].(map[string]interface{})[project].(map[string]interface{})[service].(map[string]interface{})
	for configName, _ := range configTableMap {
		tableName := project + "_" + service + "_" + configName
		tierceronTable := te.TableCache[tableName]
		valueColumns := templateResult.ValueSection["values"][service]
		secretColumns := templateResult.SecretSection["super-secrets"][service]

		if strings.Contains(envEnterprise, ".") {
			envEnterpriseParts := strings.Split(envEnterprise, ".")
			valueColumns["_Env_"] = envEnterpriseParts[0]
			valueColumns["_EnterpriseId_"] = envEnterpriseParts[1]
		} else {
			valueColumns["_Env_"] = envEnterprise
		}
		valueColumns["_Version_"] = version

		if tierceronTable == nil {
			// This is cacheable...
			tierceronTable = &TierceronTable{Table: nil, Schema: []*sql.Column{}}

			columnKeys := []string{}

			for valueKeyColumn, _ := range valueColumns {
				columnKeys = append(columnKeys, valueKeyColumn)
			}

			for secretKeyColumn, _ := range secretColumns {
				columnKeys = append(columnKeys, secretKeyColumn)
			}

			// Alpha sort -- yay...?
			sort.Strings(columnKeys)

			for _, columnKey := range columnKeys {
				column := sql.Column{Name: columnKey, Type: sql.Text, Source: tableName}
				tierceronTable.Schema = append(tierceronTable.Schema, &column)
			}

			table := memory.NewTable(tableName, tierceronTable.Schema)
			te.Database.AddTable(tableName, table)
			tierceronTable.Table = table
			te.TableCache[tableName] = tierceronTable
		}

		row := []interface{}{}

		// TODO: Add Enterprise, Environment, and Version....

		for _, column := range tierceronTable.Schema {
			if value, ok := valueColumns[column.Name]; ok {
				row = append(row, value)
			} else if secretValue, svOk := secretColumns[column.Name]; svOk {
				row = append(row, secretValue)
			}
		}

		tierceronTable.Table.Insert(te.Context, sql.NewRow(row...))
	}
}

func TransformConfig(goMod *kv.Modifier, te *TierceronEngine, envEnterprise string, version string, project string, service string, config eUtils.DriverConfig) error {
	listPath := "templates/" + project + "/" + service
	secret, err := goMod.List(listPath)
	if err != nil {
		return nil
	}
	templatePaths := []string{}
	for _, fileName := range secret.Data["keys"].([]interface{}) {
		if strFile, ok := fileName.(string); ok {
			if strFile[len(strFile)-1] != '/' { // Skip subdirectories where template files are stored
				templatePaths = append(templatePaths, listPath+"/"+strFile)
			} else {
				templatePaths = append(templatePaths, listPath+"/"+strings.ReplaceAll(strFile, "/", ""))
			}
		}
	}

	// TODO: Make this async for performance...
	for _, templatePath := range templatePaths {

		var templateResult extract.TemplateResultData
		templateResult.ValueSection = map[string]map[string]map[string]string{}
		templateResult.ValueSection["values"] = map[string]map[string]string{}

		templateResult.SecretSection = map[string]map[string]map[string]string{}
		templateResult.SecretSection["super-secrets"] = map[string]map[string]string{}

		var cds *vcutils.ConfigDataStore
		if goMod != nil {
			cds = new(vcutils.ConfigDataStore)
			goMod.Env = envEnterprise
			goMod.Version = version
			cds.Init(goMod, config.SecretMode, true, project, service)
		}

		_, _, _, templateResult.TemplateDepth = extract.ToSeed(goMod,
			cds,
			templatePath,
			config.Log,
			project,
			service,
			true,
			&(templateResult.InterfaceTemplateSection),
			&(templateResult.ValueSection),
			&(templateResult.SecretSection),
		)

		writeToTable(te, envEnterprise, version, project, service, &templateResult)
	}

	te.Engine = sqle.NewDefault(sql.NewDatabaseProvider(te.Database))
	return nil
}

// CreateEngine - creates a Tierceron query engine for query of configurations.
func CreateEngine(config eUtils.DriverConfig,
	templatePaths []string) (*TierceronEngine, error) {

	te := &TierceronEngine{Database: memory.NewDatabase("TierceronDB"), Engine: nil, TableCache: map[string]*TierceronTable{}, Context: sql.NewEmptyContext()}

	var goMod *kv.Modifier
	goMod, errModInit := kv.NewModifier(config.Insecure, config.Token, config.VaultAddress, "", config.Regions)
	if errModInit != nil {
		return nil, errModInit
	}
	projectServiceMap, err := goMod.GetProjectServicesMap()
	if err != nil {
		return nil, err
	}

	var envEnterprises []string
	goMod.Env = ""
	tempEnterprises, err := goMod.List("values")
	if err != nil {
		fmt.Println(err)
		return nil, err
	}
	for _, enterprise := range tempEnterprises.Data["keys"].([]interface{}) {
		envEnterprises = append(envEnterprises, strings.Replace(enterprise.(string), "/", "", 1))
	}

	// Fun stuff here....
	var versionMetadata []string
	for _, envEnterprise := range envEnterprises {
		goMod.Env = ""
		versionMetadata = versionMetadata[:0]
		fileMetadata, err := goMod.GetVersionValues(goMod, "values/"+envEnterprise)
		if err != nil {
			fmt.Println(err)
			return nil, err
		}

		var first map[string]interface{}
		for _, file := range fileMetadata {
			if first == nil {
				first = file
				break
			}
		}

		for versionNumber, _ := range first {
			versionMetadata = append(versionMetadata, versionNumber)
		}

		for _, versionNo := range versionMetadata {
			for project, services := range projectServiceMap {
				for _, service := range services {
					TransformConfig(goMod, te, envEnterprise, versionNo, project, service, config)
				}
			}
		}
	}

	return te, nil
}

// Query - queries configurations using standard ANSI SQL syntax.
// Example: select * from ServiceTechMobileAPI.configfile
func Query(te *TierceronEngine, query string) ([]string, [][]string, error) {
	// Create a test memory database and register it to the default engine.

	//  ctx := sql.NewContext(context.Background(), sql.WithIndexRegistry(sql.NewIndexRegistry()), sql.WithViewRegistry(sql.NewViewRegistry())).WithCurrentDB(te.Database.Name())
	ctx := sql.NewContext(context.Background()).WithCurrentDB(te.Database.Name())

	schema, r, err := te.Engine.Query(ctx, query)
	if err != nil {
		return nil, nil, err
	}

	columns := []string{}
	matrix := [][]string{}

	for _, col := range schema {
		columns = append(columns, col.Name)
	}

	// Iterate results and print them.
	for {
		row, err := r.Next()
		if err == io.EOF {
			break
		}
		rowData := []string{}
		for _, col := range row {
			rowData = append(rowData, col.(string))
		}
		matrix = append(matrix, rowData)
	}
	return columns, matrix, nil
}
