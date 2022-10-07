package seed

import (
	"encoding/base64"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
	"unsafe"

	"tierceron/buildopts/coreopts"
	vcutils "tierceron/trcconfig/utils"
	"tierceron/trcvault/opts/memonly"
	"tierceron/trcx/engine"
	"tierceron/trcx/extract"
	eUtils "tierceron/utils"
	"tierceron/utils/mlock"
	helperkv "tierceron/vaulthelper/kv"

	sqlememory "github.com/dolthub/go-mysql-server/memory"

	sqles "github.com/dolthub/go-mysql-server/sql"
)

var m sync.Mutex

// TODO: revisit this in 1.19 or later...
func stringClone(s string) string {
	b := make([]byte, len(s))
	copy(b, s)
	if memonly.IsMemonly() {
		newData := *(*string)(unsafe.Pointer(&b))
		mlock.Mlock2(nil, &newData)
		return newData
	} else {
		return *(*string)(unsafe.Pointer(&b))
	}
}

func writeToTableHelper(te *engine.TierceronEngine, configTableName string, valueColumns map[string]string, secretColumns map[string]string, config *eUtils.DriverConfig) {

	tableSql, tableOk, _ := te.Database.GetTableInsensitive(nil, configTableName)
	var table *sqlememory.Table

	// TODO: Do we want back lookup by enterpriseId on all rows?
	// if enterpriseId, ok := secretColumns["EnterpriseId"]; ok {
	// 	valueColumns["_EnterpriseId_"] = enterpriseId
	// }
	// valueColumns["_Version_"] = version

	if !tableOk {
		// This is cacheable...
		tableSchema := sqles.NewPrimaryKeySchema([]*sqles.Column{})

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
			column := sqles.Column{Name: columnKey, Type: sqles.Text, Source: configTableName}
			tableSchema.Schema = append(tableSchema.Schema, &column)
		}

		table = sqlememory.NewTable(configTableName, tableSchema, nil)
		m.Lock()
		te.Database.AddTable(configTableName, table)
		m.Unlock()
	} else {
		table = tableSql.(*sqlememory.Table)
	}

	row := []interface{}{}

	// TODO: Add Enterprise, Environment, and Version....
	allDefaults := true
	for _, column := range table.Schema() {
		if value, ok := valueColumns[column.Name]; ok {
			var iVar interface{}
			var cErr error
			if value == "<Enter Secret Here>" || value == "" || value == "0" {
				iVar, cErr = column.Type.Convert("")
				if cErr != nil {
					iVar = nil
				}
			} else {
				iVar, _ = column.Type.Convert(stringClone(value))
				allDefaults = false
			}
			row = append(row, iVar)
		} else if secretValue, svOk := secretColumns[column.Name]; svOk {
			var iVar interface{}
			var cErr error
			if column.Name == "MysqlFileContent" && secretValue != "<Enter Secret Here>" && secretValue != "" {
				var decodeErr error
				var decodedValue []byte
				if strings.HasPrefix(string(secretValue), "TierceronBase64") {
					secretValue = secretValue[len("TierceronBase64"):]
					decodedValue, decodeErr = base64.StdEncoding.DecodeString(string(secretValue))
					if decodeErr != nil {
						continue
					}
				} else {
					if _, fpOk := secretColumns["MysqlFilePath"]; fpOk {
						eUtils.LogErrorMessage(config, fmt.Sprintf("Found non encoded data for: %s", secretColumns["MysqlFilePath"]), false)
						decodedValue = []byte(secretValue)
					} else {
						eUtils.LogErrorMessage(config, "Missing MysqlFilePath.", false)
						continue
					}
				}
				iVar = []uint8(decodedValue)
			} else if secretValue == "<Enter Secret Here>" || secretValue == "" {
				iVar, cErr = column.Type.Convert("")
				if cErr != nil {
					iVar = nil
				}
			} else {
				iVar, _ = column.Type.Convert(stringClone(secretValue))
				allDefaults = false
			}
			row = append(row, iVar)
		}
	}

	if !allDefaults {
		insertErr := table.Insert(te.Context, sqles.NewRow(row...))
		if insertErr != nil {
			eUtils.LogErrorObject(config, insertErr, false)
		}
	}

}

func writeToTable(te *engine.TierceronEngine, config *eUtils.DriverConfig, envEnterprise string, version string, project string, projectAlias string, service string, templateResult *extract.TemplateResultData) {

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
		valueColumns := templateResult.ValueSection["values"][service]
		secretColumns := templateResult.SecretSection["super-secrets"][service]
		writeToTableHelper(te, configTableName, valueColumns, secretColumns, config)
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

func templateToTableRowHelper(goMod *helperkv.Modifier, te *engine.TierceronEngine, envEnterprise string, version string, project string, projectAlias string, service string, templatePath string, config *eUtils.DriverConfig) error {
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

	writeToTable(te, config, envEnterprise, version, project, projectAlias, service, &templateResult)
	return nil
}

func PathToTableRowHelper(te *engine.TierceronEngine, goMod *helperkv.Modifier, config *eUtils.DriverConfig, tableName string) error {
	dataMap, readErr := goMod.ReadData(goMod.SectionPath)
	if readErr != nil {
		return readErr
	}

	rowDataMap := make(map[string]string, 1)
	for columnName, columnData := range dataMap {
		if dataString, ok := columnData.(string); ok {
			rowDataMap[columnName] = dataString
		} else {
			return errors.New("Found data that was not a string - unable to write columnName: " + columnName + " to " + tableName)
		}
	}
	writeToTableHelper(te, tableName, nil, rowDataMap, config)

	return nil
}

func TransformConfig(goMod *helperkv.Modifier, te *engine.TierceronEngine, envEnterprise string, version string, project string, projectAlias string, service string, config *eUtils.DriverConfig, tableLock *sync.Mutex) error {
	listPath := "templates/" + project + "/" + service
	secret, err := goMod.List(listPath, config.Log)
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

			// TODO: Replace _ with secondaryIndexSlice
			index, _, indexErr := coreopts.FindIndexForService(project, service)
			if indexErr == nil && index != "" {
				goMod.SectionName = index
			}
			if goMod.SectionName != "" {
				indexValues, err = goMod.ListSubsection("/Index/", project, goMod.SectionName, config.Log)
				if err != nil {
					eUtils.LogErrorObject(config, err, false)
					return err
				}
			}
		}

		tableLock.Lock()
		for _, indexValue := range indexValues {
			if indexValue != "" {
				goMod.SectionKey = "/Index/"
				//	goMod.SubSectionValue = flowService
				goMod.SectionPath = "super-secrets/Index/" + project + "/" + goMod.SectionName + "/" + indexValue + "/" + service
				subsectionValues, err := goMod.List(goMod.SectionPath, config.Log)
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
		tableLock.Unlock()
	}

	return nil
}
