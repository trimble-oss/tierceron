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

func writeToTable(te *TierceronEngine, project string, service string, templateResult *extract.TemplateResultData) {

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
		tableName := project + "." + service + "." + configName
		tierceronTable := te.TableCache[tableName]
		valueColumns := templateResult.ValueSection["values"][service]
		secretColumns := templateResult.SecretSection["super-secrets"][service]

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

		row := []string{}
		// TODO: Add Enterprise, Column, and Version....

		for _, column := range tierceronTable.Schema {
			if value, ok := valueColumns[column.Name]; ok {
				row = append(row, value)
			} else if secretValue, svOk := secretColumns[column.Name]; svOk {
				row = append(row, secretValue)
			}
		}

		rows := []sql.Row{
			sql.NewRow(row),
		}

		for _, row := range rows {
			tierceronTable.Table.Insert(te.Context, row)
		}
	}
}

func TransformConfig(goMod *kv.Modifier, te *TierceronEngine, envVersionData string, enterpriseId string, project string, service string, config eUtils.DriverConfig) error {
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

		writeToTable(te, project, service, &templateResult)
	}

	te.Engine = sqle.NewDefault(sql.NewDatabaseProvider(te.Database))
	return nil
}

// CreateEngine - creates a Tierceron query engine for query of configurations.
func CreateEngine(config eUtils.DriverConfig,
	templatePaths []string) (*TierceronEngine, error) {

	te := &TierceronEngine{Database: memory.NewDatabase("TierceronDB"), Engine: nil, TableCache: map[string]*TierceronTable{}, Context: sql.NewEmptyContext()}

	var goMod *kv.Modifier

	envVersion := strings.Split(config.Env, "_")
	if len(envVersion) != 2 {
		// Make it so.
		config.Env = config.Env + "_0"
		envVersion = strings.Split(config.Env, "_")
	}

	env := envVersion[0]
	version := envVersion[1]

	if config.Token != "" {
		var err error
		goMod, err = kv.NewModifier(config.Insecure, config.Token, config.VaultAddress, env, config.Regions)
		if err != nil {
			panic(err)
		}
		goMod.Env = env
		goMod.Version = version
	}

	projectServiceMap, err := goMod.GetProjectServicesMap()
	if err != nil {
		return nil, err
	}

	// Fun stuff here....
	for project, services := range projectServiceMap {
		for _, service := range services {
			TransformConfig(goMod, te, config.Env, "1" /* enterpriseId */, project, service, config)
		}
	}

	return te, nil
}

// Query - queries configurations using standard ANSI SQL syntax.
// Example: select * from ServiceTechMobileAPI.configfile
func Query(te *TierceronEngine, query string) {
	// Create a test memory database and register it to the default engine.

	ctx := sql.NewContext(context.Background(), sql.WithIndexRegistry(sql.NewIndexRegistry()), sql.WithViewRegistry(sql.NewViewRegistry())).WithCurrentDB(te.Database.Name())

	_, r, err := te.Engine.Query(ctx, query)
	if err != nil {

	}

	// Iterate results and print them.
	for {
		row, err := r.Next()
		if err == io.EOF {
			break
		}

		name := row[0]
		count := row[1]

		fmt.Println(name, count)
	}

	// Output: John Doe 2
}
