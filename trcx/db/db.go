package db

import (
	"context"
	"io"
	"sort"
	"strings"
	"sync"

	vcutils "tierceron/trcconfig/utils"
	"tierceron/trcx/extract"
	eUtils "tierceron/utils"
	helperkv "tierceron/vaulthelper/kv"

	sqle "github.com/dolthub/go-mysql-server"
	"github.com/dolthub/go-mysql-server/memory"
	"github.com/dolthub/go-mysql-server/sql"
)

var m sync.Mutex

func writeToTable(te *TierceronEngine, envEnterprise string, version string, project string, projectAlias string, service string, templateResult *extract.TemplateResultData) {

	//
	// What we need is in ValueSection and SecretSection...
	//
	if templateResult.InterfaceTemplateSection == nil {
		// No templates, no configs, no tables.
		return
	}

	// Create tables with naming convention: Service.configFileName  Column names should be template variable names.
	configTableMap := templateResult.InterfaceTemplateSection.(map[string]interface{})["templates"].(map[string]interface{})[project].(map[string]interface{})[service].(map[string]interface{})
	for configTableName, _ := range configTableMap {
		tableSql, tableOk, _ := te.Database.GetTableInsensitive(te.Context, configTableName)
		var table *memory.Table

		valueColumns := templateResult.ValueSection["values"][service]
		secretColumns := templateResult.SecretSection["super-secrets"][service]

		// TODO: Do we want back lookup by enterpriseId on all rows?
		// if enterpriseId, ok := secretColumns["EnterpriseId"]; ok {
		// 	valueColumns["_EnterpriseId_"] = enterpriseId
		// }
		// valueColumns["_Version_"] = version

		if !tableOk {
			// This is cacheable...
			tableSchema := sql.NewPrimaryKeySchema([]*sql.Column{})

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
				column := sql.Column{Name: columnKey, Type: sql.Text, Source: configTableName}
				tableSchema.Schema = append(tableSchema.Schema, &column)
			}

			table = memory.NewTable(configTableName, tableSchema)
			m.Lock()
			te.Database.AddTable(configTableName, table)
			m.Unlock()
		} else {
			table = tableSql.(*memory.Table)
		}

		row := []interface{}{}

		// TODO: Add Enterprise, Environment, and Version....
		for _, column := range table.Schema() {
			if value, ok := valueColumns[column.Name]; ok {
				if value == "<Enter Secret Here>" {
					value = ""
				}
				row = append(row, value)
			} else if secretValue, svOk := secretColumns[column.Name]; svOk {
				if secretValue == "<Enter Secret Here>" {
					secretValue = ""
				}
				row = append(row, secretValue)
			}
		}

		m.Lock()
		table.Insert(te.Context, sql.NewRow(row...))
		m.Unlock()
	}
}

func removeDuplicateValues(slice []string) []string {
	keys := make(map[string]bool)
	list := []string{}
	for _, entry := range slice {
		if _, value := keys[entry]; !value {
			keys[entry] = true
			list = append(list, entry)
		}
	}
	return list
}

func templateToTableRowHelper(goMod *helperkv.Modifier, te *TierceronEngine, envEnterprise string, version string, project string, projectAlias string, service string, templatePath string, config *eUtils.DriverConfig) error {
	cds := new(vcutils.ConfigDataStore)
	var templateResult extract.TemplateResultData
	templateResult.ValueSection = map[string]map[string]map[string]string{}
	templateResult.ValueSection["values"] = map[string]map[string]string{}

	templateResult.SecretSection = map[string]map[string]map[string]string{}
	templateResult.SecretSection["super-secrets"] = map[string]map[string]string{}

	err := cds.Init(config, goMod, config.SecretMode, true, project, nil, service)
	if err != nil {
		eUtils.LogErrorObject(config, err, false)
	}

	var errSeed error

	_, _, _, templateResult.TemplateDepth, errSeed = extract.ToSeed(config,
		goMod,
		cds,
		templatePath,
		project,
		service,
		true,
		&(templateResult.InterfaceTemplateSection),
		&(templateResult.ValueSection),
		&(templateResult.SecretSection),
	)

	if errSeed != nil {
		return errSeed
	}

	writeToTable(te, envEnterprise, version, project, projectAlias, service, &templateResult)
	return nil
}

func TransformConfig(goMod *helperkv.Modifier, te *TierceronEngine, envEnterprise string, version string, project string, projectAlias string, service string, config *eUtils.DriverConfig) error {
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

	templatePaths = removeDuplicateValues(templatePaths)

	// TODO: Make this async for performance...
	for _, templatePath := range templatePaths {
		var indexValues []string = []string{}

		if goMod != nil {
			goMod.Env = envEnterprise
			goMod.Version = version

			index, indexErr := goMod.FindIndexForService(project, service)
			if indexErr == nil && index != "" {
				goMod.SectionName = index
			}
			if goMod.SectionName != "" {
				indexValues, err = goMod.ListSubsection("/Index/", project, goMod.SectionName)
				if err != nil {
					eUtils.LogErrorObject(config, err, false)
					return err
				}
			}
		}

		for _, indexValue := range indexValues {
			if indexValue != "" {
				goMod.SectionKey = "/Index/"
				//	goMod.SubSectionValue = flowService
				goMod.SectionPath = "super-secrets/Index/" + project + "/" + goMod.SectionName + "/" + indexValue + "/" + service
				subsectionValues, err := goMod.List(goMod.SectionPath)
				if err != nil {
					return err
				}
				if subsectionValues != nil {
					for _, subsectionValue := range subsectionValues.Data["keys"].([]interface{}) {
						goMod.SectionPath = "super-secrets/Index/" + project + "/" + goMod.SectionName + "/" + indexValue + "/" + service + "/" + subsectionValue.(string)
						rowErr := templateToTableRowHelper(goMod, te, config.Env, "0", project, projectAlias, service, templatePath, config)
						if rowErr != nil {
							return rowErr
						}
					}
				} else {
					rowErr := templateToTableRowHelper(goMod, te, config.Env, "0", project, projectAlias, service, templatePath, config)
					if rowErr != nil {
						return rowErr
					}
				}
			} else {
				rowErr := templateToTableRowHelper(goMod, te, config.Env, "0", project, projectAlias, service, templatePath, config)
				if rowErr != nil {
					return rowErr
				}
			}
		}

	}

	return nil
}

// CreateEngine - creates a Tierceron query engine for query of configurations.
func CreateEngine(config *eUtils.DriverConfig,
	templatePaths []string, env string, dbname string) (*TierceronEngine, error) {

	te := &TierceronEngine{Database: memory.NewDatabase(dbname), Engine: nil, TableCache: map[string]*TierceronTable{}, Context: sql.NewEmptyContext(), Config: *config}

	var goMod *helperkv.Modifier
	goMod, errModInit := helperkv.NewModifier(config.Insecure, config.Token, config.VaultAddress, "", config.Regions, config.Log)
	if errModInit != nil {
		return nil, errModInit
	}
	goMod.Env = env
	/*	This is for versioning - used below
		projectServiceMap, err := goMod.GetProjectServicesMap()
		if err != nil {
			return nil, err
		}
	*/

	var envEnterprises []string
	goMod.Env = ""
	tempEnterprises, err := goMod.List("values")
	if err != nil {
		eUtils.LogErrorObject(config, err, false)
		return nil, err
	}
	if tempEnterprises != nil {
		for _, enterprise := range tempEnterprises.Data["keys"].([]interface{}) {
			envEnterprises = append(envEnterprises, strings.Replace(enterprise.(string), "/", "", 1))
		}
		/* This is for versioning -> enhancements might be needed
			// Fun stuff here....
			var versionMetadata []string
			var wgEnterprise sync.WaitGroup
			// Load all vault table data into tierceron sql engine.
			for _, envEnterprise := range envEnterprises {
				wgEnterprise.Add(1)
				go func(config *eUtils.DriverConfig, enterpriseEnv string) {
					defer wgEnterprise.Done()
					if !strings.Contains(enterpriseEnv, ".") {
						return
					}

					tableMod, _, err := eUtils.InitVaultMod(config)
					if err != nil {
						eUtils.LogErrorMessage("Could not access vault.  Failure to start.", config.Log, false)
						return
					}

					tableMod.Env = ""
					versionMetadata = versionMetadata[:0]
					fileMetadata, err := tableMod.GetVersionValues(tableMod, config.WantCerts, "values/"+enterpriseEnv, config.Log)
					if fileMetadata == nil {
						return
					}
					if err != nil {
						eUtils.LogErrorObject(err, config.Log, false)
						return
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
							// TODO: optimize this for scale.
							for _, service := range services {
								for _, filter := range config.VersionFilter {
									if filter == service {
										TransformConfig(tableMod, te, enterpriseEnv, versionNo, project, service, config)
									}
								}
							}
						}
					}
				}(config, envEnterprise)
			}
			wgEnterprise.Wait()
		}
		*/

		te.Engine = sqle.NewDefault(memory.NewMemoryDBProvider(te.Database))
	}
	return te, nil
}

// Query - queries configurations using standard ANSI SQL syntax.
// Example: select * from ServiceTechMobileAPI.configfile
func Query(te *TierceronEngine, query string) (string, []string, [][]string, error) {
	// Create a test memory database and register it to the default engine.

	//ctx := sql.NewContext(context.Background(), sql.WithIndexRegistry(sql.NewIndexRegistry()), sql.WithViewRegistry(sql.NewViewRegistry())).WithCurrentDB(te.Database.Name())
	//ctx := sql.NewContext(context.Background()).WithCurrentDB(te.Database.Name())
	ctx := sql.NewContext(context.Background())

	m.Lock()
	//	te.Context = ctx
	schema, r, err := te.Engine.Query(ctx, query) //This query not working anymore for inserts???
	m.Unlock()

	if err != nil {
		return "", nil, nil, err
	}

	columns := []string{}
	matrix := [][]string{}
	tableName := ""

	for _, col := range schema {
		if tableName == "" {
			tableName = col.Source
		}

		columns = append(columns, col.Name)
	}

	if len(columns) > 0 {
		// Iterate results and print them.
		okResult := false
		for {
			row, err := r.Next(ctx)
			if err == io.EOF {
				break
			}
			rowData := []string{}
			if len(columns) == 1 && columns[0] == "__ok_result__" { //This is for insert statements
				okResult = true
				if len(row) > 0 {
					if sqlOkResult, ok := row[0].(sql.OkResult); ok {
						if sqlOkResult.RowsAffected > 0 {
							matrix = append(matrix, rowData)
						}
					}
				}
			} else {
				for _, col := range row {
					rowData = append(rowData, col.(string))
				}
				matrix = append(matrix, rowData)
			}
		}
		if okResult {
			return "ok", nil, matrix, nil
		}
	}

	return tableName, columns, matrix, nil
}
