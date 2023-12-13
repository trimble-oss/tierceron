package core

import (
	"sync"
	"unsafe"

	"github.com/trimble-oss/tierceron/buildopts/memprotectopts"
	"github.com/trimble-oss/tierceron/trcvault/opts/memonly"
)

var m sync.Mutex

// TODO: revisit this in 1.19 or later...
func stringClone(s string) string {
	b := make([]byte, len(s))
	copy(b, s)
	if memonly.IsMemonly() {
		newData := *(*string)(unsafe.Pointer(&b))
		memprotectopts.MemProtect(nil, &newData)
		return newData
	} else {
		return *(*string)(unsafe.Pointer(&b))
	}
}

/*	Methods are no longer used for initial row insertion.
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
*/
