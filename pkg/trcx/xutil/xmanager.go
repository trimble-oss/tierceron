package xutil

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/hashicorp/vault/api"
	"github.com/trimble-oss/tierceron/buildopts/coreopts"
	vcutils "github.com/trimble-oss/tierceron/pkg/cli/trcconfigbase/utils"
	"github.com/trimble-oss/tierceron/pkg/core"
	"github.com/trimble-oss/tierceron/pkg/trcx/extract"
	xencrypt "github.com/trimble-oss/tierceron/pkg/trcx/xencrypt"
	"github.com/trimble-oss/tierceron/pkg/utils"
	eUtils "github.com/trimble-oss/tierceron/pkg/utils"
	"github.com/trimble-oss/tierceron/pkg/utils/config"
	helperkv "github.com/trimble-oss/tierceron/pkg/vaulthelper/kv"
	"gopkg.in/yaml.v2"
)

var templateResultChan = make(chan *extract.TemplateResultData, 5)

func GenerateSeedSectionFromVaultRaw(driverConfig *config.DriverConfig, templateFromVault bool, templatePaths []string) ([]byte, bool, map[string]interface{}, map[string]map[string]map[string]string, map[string]map[string]map[string]string, string, error) {
	var wg sync.WaitGroup
	// Initialize global variables
	valueCombinedSection := map[string]map[string]map[string]string{}
	valueCombinedSection["values"] = map[string]map[string]string{}

	secretCombinedSection := map[string]map[string]map[string]string{}
	secretCombinedSection["super-secrets"] = map[string]map[string]string{}

	// Declare local variables
	templateCombinedSection := map[string]interface{}{}
	sliceTemplateSection := []interface{}{}
	sliceValueSection := []map[string]map[string]map[string]string{}
	sliceSecretSection := []map[string]map[string]map[string]string{}
	var sectionPath string

	maxDepth := -1
	service := ""
	if len(driverConfig.ServiceFilter) > 0 {
		service = driverConfig.ServiceFilter[0]
	}

	//This checks whether indexed section is available in current directory.
	if len(driverConfig.SectionKey) > 0 && len(driverConfig.ProjectSections) > 0 {
		projectFound := false
		for _, projectSection := range driverConfig.ProjectSections {
			for _, templatePath := range templatePaths {
				if strings.Contains(templatePath, projectSection) {
					projectFound = true
					goto projectFound
				}
			}
		projectFound:
			if !projectFound {
				return nil, false, nil, nil, nil, "", eUtils.LogAndSafeExit(driverConfig.CoreConfig, "Unable to find indexed project in local templates.", 1)
			}
		}
	}

	multiService := false
	var mod *helperkv.Modifier

	filteredTemplatePaths := templatePaths[:0]
	if len(driverConfig.FileFilter) != 0 {
		for _, filter := range driverConfig.FileFilter {
			if !strings.HasSuffix(filter, ".tmpl") {
				filter = filter + ".tmpl"
			}
			for _, templatePath := range templatePaths {
				if strings.HasSuffix(templatePath, filter) {
					filteredTemplatePaths = append(filteredTemplatePaths, templatePath)
				}
			}
		}
	}
	if len(filteredTemplatePaths) > 0 {
		templatePaths = filteredTemplatePaths
		filteredTemplatePaths = filteredTemplatePaths[:0]
	}

	envVersion := strings.Split(driverConfig.CoreConfig.Env, "_")
	if len(envVersion) != 2 {
		// Make it so.
		envVersion = eUtils.SplitEnv(driverConfig.CoreConfig.Env)
	}
	env := envVersion[0]
	version := envVersion[1]
	envBasis := eUtils.GetEnvBasis(env)

	var tokenName string
	if eUtils.RefLength(driverConfig.CoreConfig.CurrentTokenNamePtr) > 0 {
		tokenName = *driverConfig.CoreConfig.CurrentTokenNamePtr
	} else {
		tokenName = fmt.Sprintf("config_token_%s", envBasis)
	}
	if !utils.RefEquals(driverConfig.CoreConfig.TokenCache.GetToken(tokenName), "novault") {
		var err error
		mod, err = helperkv.NewModifierFromCoreConfig(driverConfig.CoreConfig,
			tokenName,
			env, true)
		if err != nil {
			eUtils.LogErrorObject(driverConfig.CoreConfig, err, driverConfig.CoreConfig.ExitOnFailure)
		}

		mod.Env = env
		mod.Version = version
		if len(driverConfig.ProjectSections) > 0 {
			mod.ProjectIndex = driverConfig.ProjectSections
			mod.EnvBasis = strings.Split(driverConfig.CoreConfig.EnvBasis, "_")[0]
			mod.SectionName = driverConfig.SectionName
			mod.SubSectionValue = driverConfig.SubSectionValue
		}
	}

	if len(filteredTemplatePaths) > 0 {
		filteredTemplatePaths = eUtils.RemoveDuplicates(filteredTemplatePaths)
		templatePaths = filteredTemplatePaths
	}

	if driverConfig.GenAuth && mod != nil {
		_, err := mod.ReadData("apiLogins/meta")
		if err != nil {
			eUtils.LogInfo(driverConfig.CoreConfig, "Cannot genAuth with provided token.")
			return nil, false, nil, nil, nil, "", eUtils.LogAndSafeExit(driverConfig.CoreConfig, "", 1)
		}
	}

	if !utils.RefEquals(driverConfig.CoreConfig.TokenCache.GetToken(fmt.Sprintf("config_token_%s", driverConfig.CoreConfig.EnvBasis)), "novault") && mod != nil && mod.Version != "0" { //If version isn't latest or is a flag
		var noCertPaths []string
		var certPaths []string
		for _, templatePath := range templatePaths { //Separate cert vs normal paths
			if !strings.Contains(templatePath, "Common") {
				noCertPaths = append(noCertPaths, templatePath)
			} else {
				certPaths = append(certPaths, templatePath)
			}
		}

		if driverConfig.CoreConfig.WantCerts { //Remove unneeded template paths
			templatePaths = certPaths
		} else {
			templatePaths = noCertPaths
		}

		project := ""
		if len(driverConfig.VersionFilter) > 0 {
			project = driverConfig.VersionFilter[0]
		}
		for _, templatePath := range templatePaths {
			_, service, _, _ := eUtils.GetProjectService(nil, templatePath) //This checks for nested project names

			driverConfig.VersionFilter = append(driverConfig.VersionFilter, service) //Adds nested project name to filter otherwise it will be not found.
		}

		if driverConfig.CoreConfig.WantCerts { //For cert version history
			driverConfig.VersionFilter = append(driverConfig.VersionFilter, "Common")
		}

		driverConfig.VersionFilter = eUtils.RemoveDuplicates(driverConfig.VersionFilter)
		mod.VersionFilter = driverConfig.VersionFilter
		versionMetadataMap := eUtils.GetProjectVersionInfo(driverConfig, mod)

		if versionMetadataMap == nil {
			return nil, false, nil, nil, nil, "", eUtils.LogAndSafeExit(driverConfig.CoreConfig, fmt.Sprintf("No version data found - this filter was applied during search: %v\n", driverConfig.VersionFilter), 1)
		} else if version == "versionInfo" { //Version flag
			var masterKey string
			first := true
			for key := range versionMetadataMap {
				passed := false
				if driverConfig.CoreConfig.WantCerts {
					for _, service := range mod.VersionFilter {
						if !passed && strings.Contains(key, "Common") && strings.Contains(key, service) && !strings.Contains(key, project) && !strings.HasSuffix(key, "Common") {
							if len(key) > 0 {
								keySplit := strings.Split(key, "/")
								driverConfig.VersionInfo(versionMetadataMap[key], false, keySplit[len(keySplit)-1], first)
								passed = true
								first = false
							}
						}
					}
				} else {
					if len(key) > 0 && len(masterKey) < 1 {
						masterKey = key
						driverConfig.VersionInfo(versionMetadataMap[masterKey], false, "", false)
						return nil, false, nil, nil, nil, "", eUtils.LogAndSafeExit(driverConfig.CoreConfig, "Version info provided.", 1)
					}
				}
			}
			return nil, false, nil, nil, nil, "", eUtils.LogAndSafeExit(driverConfig.CoreConfig, "Version info provided.", 1)
		} else { //Version bound check
			if version != "0" {
				versionNumbers := eUtils.GetProjectVersions(driverConfig, versionMetadataMap)
				eUtils.BoundCheck(driverConfig, versionNumbers, version)
			}
		}
	}

	//Receiver for configs
	go func(dc *config.DriverConfig) {
		for {
			select {
			case tResult := <-templateResultChan:
				if dc.CoreConfig.Env == tResult.Env && dc.SubSectionValue == tResult.SubSectionValue {
					sliceTemplateSection = append(sliceTemplateSection, tResult.InterfaceTemplateSection)
					sliceValueSection = append(sliceValueSection, tResult.ValueSection)
					sliceSecretSection = append(sliceSecretSection, tResult.SecretSection)
					sectionPath = tResult.SectionPath

					if tResult.TemplateDepth > maxDepth {
						maxDepth = tResult.TemplateDepth
						//templateCombinedSection = interfaceTemplateSection
					}
					wg.Done()
				} else {
					go func(tResult *extract.TemplateResultData) {
						templateResultChan <- tResult
					}(tResult)
				}
			}
		}
	}(driverConfig)

	commonPathFound := false
	for _, tPath := range templatePaths {
		if strings.Contains(tPath, "Common") {
			commonPathFound = true
		}
	}

	commonPaths := []string{}
	tokenNameEnv := fmt.Sprintf("config_token_%s", eUtils.GetEnvBasis(env))
	if !utils.RefEquals(driverConfig.CoreConfig.TokenCache.GetToken(tokenNameEnv), "") && commonPathFound {
		var commonMod *helperkv.Modifier
		var err error
		commonMod, err = helperkv.NewModifierFromCoreConfig(driverConfig.CoreConfig, tokenNameEnv, env, true)
		commonMod.Env = driverConfig.CoreConfig.Env
		if err != nil {
			eUtils.LogErrorObject(driverConfig.CoreConfig, err, false)
		}
		envVersion := strings.Split(driverConfig.CoreConfig.Env, "_")
		if len(envVersion) == 1 {
			envVersion = append(envVersion, "0")
		}
		commonMod.Env = envVersion[0]
		commonMod.EnvBasis = eUtils.GetEnvBasis(commonMod.Env)
		commonMod.Version = envVersion[1]
		driverConfig.CoreConfig.Env = envVersion[0] + "_" + envVersion[1]
		commonMod.Version = commonMod.Version + "***X-Mode"

		commonPaths, err = vcutils.GetPathsFromProject(driverConfig.CoreConfig, commonMod, []string{"Common"}, []string{})
		if err != nil {
			eUtils.LogErrorObject(driverConfig.CoreConfig, err, false)
		}
		if len(commonPaths) > 0 && strings.Contains(commonPaths[len(commonPaths)-1], "!=!") {
			commonPaths = commonPaths[:len(commonPaths)-1]
		}
		commonMod.Release()
	}

	// Configure each template in directory
	if !utils.RefEquals(driverConfig.CoreConfig.TokenCache.GetToken(fmt.Sprintf("config_token_%s", driverConfig.CoreConfig.EnvBasis)), "novault") {
		//
		// Checking for existence of values for service in vault.
		//
		if strings.Contains(driverConfig.CoreConfig.EnvBasis, ".*") || len(driverConfig.ProjectSections) > 0 {
			anyServiceFound := false
			serviceFound := false
			var acceptedTemplatePaths []string
			for _, templatePath := range templatePaths {
				_, _, _, templatePath = eUtils.GetProjectService(driverConfig, templatePath)
				_, _, indexed, _ := helperkv.PreCheckEnvironment(mod.Env)
				//This checks whether a enterprise env has the relevant project otherwise env gets skipped when generating seed files.
				if (strings.Contains(mod.Env, ".") || len(driverConfig.ProjectSections) > 0) && !serviceFound {
					var listValues *api.Secret
					var err error
					if driverConfig.SectionKey == "/Index/" && len(driverConfig.ProjectSections) > 0 {
						listValues, err = mod.ListEnv("super-secrets/"+strings.Split(driverConfig.CoreConfig.EnvBasis, ".")[0]+driverConfig.SectionKey+driverConfig.ProjectSections[0]+"/"+driverConfig.SectionName+"/"+driverConfig.SubSectionValue+"/", driverConfig.CoreConfig.Log)
					} else if len(driverConfig.ProjectSections) > 0 { //If eid -> look inside Index and grab all environments
						listValues, err = mod.ListEnv("super-secrets/"+strings.Split(driverConfig.CoreConfig.EnvBasis, ".")[0]+driverConfig.SectionKey+driverConfig.ProjectSections[0]+"/"+driverConfig.SectionName, driverConfig.CoreConfig.Log)
						if listValues == nil {
							listValues, err = mod.ListEnv("super-secrets/"+strings.Split(driverConfig.CoreConfig.EnvBasis, ".")[0]+driverConfig.SectionKey+driverConfig.ProjectSections[0], driverConfig.CoreConfig.Log)
						}
					} else if indexed {
						listValues, err = mod.ListEnv("super-secrets/"+mod.Env+"/", driverConfig.CoreConfig.Log)
					} else {
						listValues, err = mod.ListEnv("values/"+mod.Env+"/", driverConfig.CoreConfig.Log) //Fix values to add to project to directory
					}
					if err != nil {
						eUtils.LogErrorObject(driverConfig.CoreConfig, err, false)
					} else if listValues == nil {
						//eUtils.LogInfo(config, "No values were returned under values/.")
					} else {
						serviceSlice := make([]string, 0)
						for _, valuesPath := range listValues.Data {
							for _, serviceInterface := range valuesPath.([]interface{}) {
								serviceFace := serviceInterface.(string)
								if version != "0" {
									versionMap := eUtils.GetProjectVersionInfo(driverConfig, mod) //("super-secrets/" + strings.Split(config.EnvBasis, ".")[0] + config.SectionKey + config.ProjectSections[0] + "/" + config.SectionName + "/" + config.SubSectionValue + "/" + serviceFace)
									versionNumbers := eUtils.GetProjectVersions(driverConfig, versionMap)
									eUtils.BoundCheck(driverConfig, versionNumbers, version)
								}
								serviceSlice = append(serviceSlice, serviceFace)
							}
						}
						for _, listedService := range serviceSlice {
							if service == "" && strings.Contains(templatePath, strings.TrimSuffix(listedService, "/")) {
								serviceFound = true
							} else if strings.TrimSuffix(listedService, "/") == service {
								serviceFound = true
							}
						}
					}
				}
				if serviceFound { //Exit for irrelevant enterprises
					acceptedTemplatePaths = append(acceptedTemplatePaths, templatePath)
					anyServiceFound = true
					serviceFound = false
				}
			}

			if !anyServiceFound { //Exit for irrelevant enterprises
				var errmsg error
				if driverConfig.SubSectionValue != "" {
					errmsg = errors.New("No relevant services were found for this environment: " + mod.Env + " for this value: " + driverConfig.SubSectionValue)
				} else {
					errmsg = errors.New("No relevant services were found for this environment: " + mod.Env)
				}
				eUtils.LogErrorObject(driverConfig.CoreConfig, errmsg, false)
				return nil, false, nil, nil, nil, "", errmsg
			}

			if len(acceptedTemplatePaths) > 0 {
				// template paths further trimmed by vault.
				templatePaths = acceptedTemplatePaths
			}
		}
	}

	var iFilterTemplatePaths []string
	if len(driverConfig.ServiceFilter) > 0 {
		for _, iFilter := range driverConfig.ServiceFilter {
			for _, tPath := range templatePaths {
				if strings.Contains(tPath, "/"+iFilter+"/") || strings.HasSuffix(tPath, "/"+iFilter+".yml.tmpl") {
					iFilterTemplatePaths = append(iFilterTemplatePaths, tPath)
				}
			}
		}
		templatePaths = iFilterTemplatePaths
	}
	if !utils.RefEquals(driverConfig.CoreConfig.TokenCache.GetToken(fmt.Sprintf("config_token_%s", driverConfig.CoreConfig.EnvBasis)), "novault") {
		if mod != nil {
			mod.Release()
		}
	}

	// Configure each template in directory
	for _, templatePath := range templatePaths {
		wg.Add(1)
		go func(tp string, multiService bool, dc *config.DriverConfig, cPaths []string) {
			var project, service, env, version, innerProject string
			var errSeed error
			project = ""
			service = ""
			env = ""
			version = ""
			innerProject = "Not Found"

			// Map Subsections
			var templateResult extract.TemplateResultData
			var cds *vcutils.ConfigDataStore
			var goMod *helperkv.Modifier

			templateResult.ValueSection = map[string]map[string]map[string]string{}
			templateResult.ValueSection["values"] = map[string]map[string]string{}

			templateResult.SecretSection = map[string]map[string]map[string]string{}
			templateResult.SecretSection["super-secrets"] = map[string]map[string]string{}
			envVersion := eUtils.SplitEnv(dc.CoreConfig.Env)
			env = envVersion[0]
			version = envVersion[1]
			//check for template_files directory here
			project, service, _, tp = eUtils.GetProjectService(dc, tp)
			useCache := true

			if !utils.RefEqualsAny(dc.CoreConfig.TokenCache.GetToken(fmt.Sprintf("config_token_%s", dc.CoreConfig.EnvBasis)), []string{"", "novault"}) {
				var err error
				goMod, err = helperkv.NewModifierFromCoreConfig(
					dc.CoreConfig,
					fmt.Sprintf("config_token_%s", dc.CoreConfig.EnvBasis),
					env,
					useCache)
				goMod.Env = dc.CoreConfig.Env
				if err != nil {
					if useCache && goMod != nil {
						goMod.Release()
					}
					eUtils.LogErrorObject(dc.CoreConfig, err, false)
					wg.Done()
					return
				}

				goMod.Env = env
				goMod.EnvBasis = dc.CoreConfig.EnvBasis
				goMod.Version = version
				goMod.ProjectIndex = dc.ProjectSections
				if len(goMod.ProjectIndex) > 0 {
					goMod.SectionKey = dc.SectionKey
					goMod.SectionName = dc.SectionName
					goMod.SubSectionValue = dc.SubSectionValue
				}

				relativeTemplatePathParts := strings.Split(tp, coreopts.BuildOptions.GetFolderPrefix(dc.StartDir)+"_templates")
				templateTrimmed, _ := eUtils.TrimLastDotAfterLastSlash(relativeTemplatePathParts[1])
				goMod.TemplatePath = "templates" + templateTrimmed

				cds = new(vcutils.ConfigDataStore)
				goMod.Version = goMod.Version + "***X-Mode"
				if len(dc.CoreConfig.DynamicPathFilter) > 0 {
					goMod.SectionPath = "super-secrets/" + dc.CoreConfig.DynamicPathFilter
				} else {
					// TODO: Deprecated...
					// 1-800-ROIT???  Not sure how certs play into this.
					if goMod.SectionName != "" && (goMod.SubSectionValue != "" || goMod.SectionKey == "/Restricted/" || goMod.SectionKey == "/Protected/") {
						switch goMod.SectionKey {
						case "/Index/":
							goMod.SectionPath = "super-secrets" + goMod.SectionKey + project + "/" + goMod.SectionName + "/" + goMod.SubSectionValue + "/" + service + dc.SubSectionName
						case "/Restricted/":
							if service != dc.SectionName { //TODO: Revisit why we need this comparison
								goMod.SectionPath = "super-secrets" + goMod.SectionKey + service + "/" + dc.SectionName
							} else {
								goMod.SectionPath = "super-secrets" + goMod.SectionKey + project + "/" + dc.SectionName
							}
						case "/Protected/":
							if service != dc.SectionName {
								goMod.SectionPath = "super-secrets" + goMod.SectionKey + service + "/" + dc.SectionName
							}
						default:
							goMod.SectionPath = "super-secrets" + goMod.SectionKey + project + "/" + goMod.SectionName + "/" + goMod.SubSectionValue
						}
					}
				}
				if !utils.RefEquals(dc.CoreConfig.TokenCache.GetToken(fmt.Sprintf("config_token_%s", driverConfig.CoreConfig.EnvBasis)), "novault") {
					if dc.CoreConfig.WantCerts {
						var formattedTPath string
						tempList := make([]string, 0)
						// TODO: Chebacca Monday!
						tPath := strings.Split(tp, coreopts.BuildOptions.GetFolderPrefix(dc.StartDir)+"_")[1]
						tPathSplit := strings.Split(tPath, ".")
						if len(tPathSplit) > 2 {
							formattedTPath = tPathSplit[0] + "." + tPathSplit[1]
						} else {
							wg.Done()
							return
						}
						if len(cPaths) > 0 {
							for _, cPath := range cPaths {
								if cPath == formattedTPath {
									tempList = append(tempList, cPath)
								}
							}
						}
						cPaths = tempList
					}
					cds.Init(dc.CoreConfig, goMod, dc.SecretMode, true, project, cPaths, service)
				}
				if len(goMod.VersionFilter) >= 1 && strings.Contains(goMod.VersionFilter[len(goMod.VersionFilter)-1], "!=!") {
					// TODO: should this be before cds.Init???
					innerProject = strings.Split(goMod.VersionFilter[len(goMod.VersionFilter)-1], "!=!")[1]
					goMod.VersionFilter = goMod.VersionFilter[:len(goMod.VersionFilter)-1]
					if innerProject != "Not Found" {
						project = innerProject
						service = project
					}
				}

			}

			_, _, _, templateResult.TemplateDepth, errSeed = extract.ToSeed(dc, goMod,
				cds,
				tp,
				project,
				service,
				templateFromVault,
				&(templateResult.InterfaceTemplateSection),
				&(templateResult.ValueSection),
				&(templateResult.SecretSection),
			)
			if len(dc.CoreConfig.DynamicPathFilter) > 0 {
				// Pass explicit desitination indiciated in gomod.
				templateResult.SectionPath = goMod.SectionPath
			}

			if useCache && goMod != nil {
				goMod.Release()
			}
			if errSeed != nil {
				eUtils.LogAndSafeExit(dc.CoreConfig, errSeed.Error(), -1)
				wg.Done()
				return
			}

			templateResult.Env = env + "_" + version
			templateResult.SubSectionValue = dc.SubSectionValue
			templateResultChan <- &templateResult
		}(templatePath, multiService, driverConfig, commonPaths)
	}
	wg.Wait()

	// Combine values of slice
	CombineSection(driverConfig.CoreConfig, sliceTemplateSection, maxDepth, templateCombinedSection)
	CombineSection(driverConfig.CoreConfig, sliceValueSection, -1, valueCombinedSection)
	CombineSection(driverConfig.CoreConfig, sliceSecretSection, -1, secretCombinedSection)

	var authYaml []byte
	var errA error

	// Add special auth section.
	if driverConfig.GenAuth {
		if mod != nil {
			authMod, authErr := helperkv.NewModifierFromCoreConfig(driverConfig.CoreConfig,
				fmt.Sprintf("config_token_%s", driverConfig.CoreConfig.EnvBasis),
				env,
				true)
			eUtils.LogAndSafeExit(driverConfig.CoreConfig, authErr.Error(), -1)

			connInfo, err := authMod.ReadData("apiLogins/meta")
			authMod.Release()
			if err == nil {
				authSection := map[string]interface{}{}
				authSection["apiLogins"] = map[string]interface{}{}
				authSection["apiLogins"].(map[string]interface{})["meta"] = connInfo
				authYaml, errA = yaml.Marshal(authSection)
				if errA != nil {
					eUtils.LogErrorObject(driverConfig.CoreConfig, errA, false)
				}
			} else {
				return nil, false, nil, nil, nil, "", eUtils.LogAndSafeExit(driverConfig.CoreConfig, "Attempt to gen auth for reduced privilege token failed.  No permissions to gen auth.", 1)
			}
		} else {
			authConfigurations := map[string]interface{}{}
			authConfigurations["authEndpoint"] = "<Enter Secret Here>"
			authConfigurations["pass"] = "<Enter Secret Here>"
			authConfigurations["sessionDB"] = "<Enter Secret Here>"
			authConfigurations["user"] = "<Enter Secret Here>"
			authConfigurations["trcAPITokenSecret"] = "<Enter Secret Here>"

			authSection := map[string]interface{}{}
			authSection["apiLogins"] = map[string]interface{}{}
			authSection["apiLogins"].(map[string]interface{})["meta"] = authConfigurations
			authYaml, errA = yaml.Marshal(authSection)
			if errA != nil {
				eUtils.LogErrorObject(driverConfig.CoreConfig, errA, false)
			}
		}
	}
	return authYaml, multiService, templateCombinedSection, valueCombinedSection, secretCombinedSection, sectionPath, nil
}

// GenerateSeedsFromVaultRaw configures the templates in trc_templates and writes them to trcx
func GenerateSeedsFromVaultRaw(driverConfig *config.DriverConfig, fromVault bool, templatePaths []string) (string, bool, string, error) {
	var projectSectionTemp []string //Used for seed file pathing; errors for -novault generation if not empty
	if len(driverConfig.Trcxe) > 2 {
		projectSectionTemp = driverConfig.ProjectSections
		driverConfig.ProjectSections = []string{}
	}
	authYaml, multiService, templateCombinedSection, valueCombinedSection, secretCombinedSection, endPath, generateErr := GenerateSeedSectionFromVaultRaw(driverConfig, fromVault, templatePaths)
	if generateErr != nil {
		eUtils.LogErrorObject(driverConfig.CoreConfig, generateErr, false)
		return "", false, "", nil
	}

	if len(driverConfig.Trcxe) > 1 { //Validate first then replace fields
		if len(projectSectionTemp) > 0 {
			driverConfig.ProjectSections = projectSectionTemp
		}
		valValidateError := xencrypt.FieldValidator(driverConfig.Trcxe[0]+","+driverConfig.Trcxe[1], secretCombinedSection, valueCombinedSection)
		if valValidateError != nil {
			eUtils.LogErrorObject(driverConfig.CoreConfig, valValidateError, false)
			return "", false, "", valValidateError
		}

		encryptSecretErr := xencrypt.SetEncryptionSecret(driverConfig)
		if encryptSecretErr != nil {
			eUtils.LogErrorObject(driverConfig.CoreConfig, encryptSecretErr, false)
			return "", false, "", encryptSecretErr
		}

		encryption, encryptErr := xencrypt.GetEncryptors(secretCombinedSection)
		if encryptErr != nil {
			eUtils.LogErrorObject(driverConfig.CoreConfig, encryptErr, false)
			return "", false, "", encryptErr
		}

		if driverConfig.Trcxr {
			xencrypt.FieldReader(xencrypt.CreateEncryptedReadMap(driverConfig.Trcxe[1]), secretCombinedSection, valueCombinedSection, encryption)
		} else {
			fieldChangedMap, encryptedChangedMap, promptErr := xencrypt.PromptUserForFields(driverConfig.Trcxe[0], driverConfig.Trcxe[1], encryption)
			if promptErr != nil {
				eUtils.LogErrorObject(driverConfig.CoreConfig, promptErr, false)
				return "", false, "", promptErr
			}
			xencrypt.FieldReplacer(fieldChangedMap, encryptedChangedMap, secretCombinedSection, valueCombinedSection)
		}
	}

	if driverConfig.CoreConfig.WantCerts && !fromVault {
		return "", false, "", nil
	}

	// Create seed file structure
	template, errT := yaml.Marshal(templateCombinedSection)
	value, errV := yaml.Marshal(valueCombinedSection)
	secret, errS := yaml.Marshal(secretCombinedSection)

	if errT != nil {
		eUtils.LogErrorObject(driverConfig.CoreConfig, errT, false)
	}

	if errV != nil {
		eUtils.LogErrorObject(driverConfig.CoreConfig, errV, false)
	}

	if errS != nil {
		eUtils.LogErrorObject(driverConfig.CoreConfig, errS, false)
	}
	templateData := string(template)
	// Remove single quotes generated by Marshal
	templateData = strings.ReplaceAll(templateData, "'", "")
	seedData := templateData + "\n\n\n" + string(value) + "\n\n\n" + string(secret) + "\n\n\n" + string(authYaml)

	return endPath, multiService, seedData, nil
}

// GenerateSeedsFromVault configures the templates in trc_templates and writes them to trcx
func GenerateSeedsFromVault(ctx config.ProcessContext, configCtx *config.ConfigContext, driverConfig *config.DriverConfig) (interface{}, error) {
	if driverConfig.Clean { //Clean flag in trcx
		if strings.HasSuffix(driverConfig.CoreConfig.Env, "_0") {
			envVersion := eUtils.SplitEnv(driverConfig.CoreConfig.Env)
			driverConfig.CoreConfig.Env = envVersion[0]
		}
		_, err1 := os.Stat(driverConfig.EndDir + driverConfig.CoreConfig.Env)
		err := os.RemoveAll(driverConfig.EndDir + driverConfig.CoreConfig.Env)

		if err != nil {
			eUtils.LogErrorObject(driverConfig.CoreConfig, err, false)
			eUtils.LogAndSafeExit(driverConfig.CoreConfig, "", 1)
		}

		if err1 == nil {
			eUtils.LogInfo(driverConfig.CoreConfig, "Seed removed from"+driverConfig.EndDir+driverConfig.CoreConfig.Env)
		}
		return nil, nil
	}

	// Get files from directory
	tempTemplatePaths := []string{}
	for _, startDir := range driverConfig.StartDir {
		//get files from directory
		tp := GetDirFiles(startDir)
		tempTemplatePaths = append(tempTemplatePaths, tp...)
	}

	if len(tempTemplatePaths) == 0 {
		eUtils.LogErrorMessage(driverConfig.CoreConfig, "No files found in "+coreopts.BuildOptions.GetFolderPrefix(driverConfig.StartDir)+"_templates", true)
	}

	//Duplicate path remover
	keys := make(map[string]bool)
	templatePaths := []string{}
	for _, path := range tempTemplatePaths {
		if _, value := keys[path]; !value {
			keys[path] = true
			templatePaths = append(templatePaths, path)
		}
	}

	if !utils.RefEquals(driverConfig.CoreConfig.TokenCache.GetToken(fmt.Sprintf("config_token_%s", driverConfig.CoreConfig.EnvBasis)), "novault") { //Filter unneeded templates
		var err error
		// TODO: Redo/deleted the indexedEnv work...
		// Get filtered using mod and templates.
		templatePathsAccepted, err := eUtils.GetAcceptedTemplatePaths(driverConfig, nil, templatePaths)
		if err != nil {
			eUtils.LogErrorObject(driverConfig.CoreConfig, err, false)
			eUtils.LogAndSafeExit(driverConfig.CoreConfig, "", 1)
		}
		templatePaths = templatePathsAccepted
	} else {
		templatePathsAccepted := []string{}
		for _, project := range driverConfig.ProjectSections {
			for _, templatePath := range templatePaths {
				if strings.Contains(templatePath, project) {
					templatePathsAccepted = append(templatePathsAccepted, templatePath)
				}
			}
		}
		if len(templatePathsAccepted) > 0 {
			templatePaths = templatePathsAccepted
		}
	}
	endPath, multiService, seedData, errGenerateSeeds := GenerateSeedsFromVaultRaw(driverConfig, false, templatePaths)
	if errGenerateSeeds != nil {
		eUtils.LogInfo(driverConfig.CoreConfig, errGenerateSeeds.Error())
		return errGenerateSeeds, nil
	}

	if endPath == "" && !multiService && seedData == "" && !driverConfig.CoreConfig.WantCerts {
		return nil, nil
	}

	suffixRemoved := ""
	envVersion := eUtils.SplitEnv(driverConfig.CoreConfig.Env)
	driverConfig.CoreConfig.Env = envVersion[0]
	if envVersion[1] != "0" {
		suffixRemoved = "_" + envVersion[1]
	}

	envBasePath, pathPart, pathInclude, _ := helperkv.PreCheckEnvironment(driverConfig.CoreConfig.Env)

	if suffixRemoved != "" {
		driverConfig.CoreConfig.Env = driverConfig.CoreConfig.Env + suffixRemoved
	}

	if multiService {
		if strings.HasPrefix(driverConfig.CoreConfig.Env, "local") {
			endPath = driverConfig.EndDir + "local/local_seed.yml"
		} else {
			if pathInclude {
				endPath = driverConfig.EndDir + envBasePath + "/" + pathPart + "/" + driverConfig.CoreConfig.Env + "_seed.yml"
			} else {
				endPath = driverConfig.EndDir + envBasePath + "/" + driverConfig.CoreConfig.Env + "_seed.yml"
			}
		}
	} else {
		if pathInclude {
			endPath = driverConfig.EndDir + envBasePath + "/" + pathPart + "/" + driverConfig.CoreConfig.Env + "_seed.yml"
		} else if len(driverConfig.ProjectSections) > 0 {
			envBasePath, _, _, _ := helperkv.PreCheckEnvironment(driverConfig.CoreConfig.EnvBasis)
			sectionNamePath := "/"
			subSectionValuePath := ""
			switch driverConfig.SectionKey {
			case "/Index/":
				sectionNamePath = "/" + driverConfig.SectionName + "/"
				subSectionValuePath = driverConfig.SubSectionValue
			case "/Restricted/":
				fallthrough
			case "/Protected/":
				sectionNamePath = "/" + driverConfig.SectionName + "/"
				subSectionValuePath = driverConfig.CoreConfig.Env
			}

			endPath = driverConfig.EndDir + envBasePath + driverConfig.SectionKey + driverConfig.ProjectSections[0] + sectionNamePath + subSectionValuePath + driverConfig.SubSectionName + "_seed.yml"
		} else if len(driverConfig.CoreConfig.DynamicPathFilter) > 0 {
			destPath := endPath
			if len(driverConfig.SectionKey) > 0 {
				destPath = strings.Replace(endPath, driverConfig.SectionName, "/", 1)
			}
			destPath = strings.Replace(destPath, "super-secrets/", "", 1)
			endPath = driverConfig.EndDir + envBasePath + "/" + destPath + "_seed.yml"
		} else {
			endPath = driverConfig.EndDir + envBasePath + "/" + driverConfig.CoreConfig.Env + "_seed.yml"
		}
	}
	//generate template or certificate
	if driverConfig.CoreConfig.WantCerts {
		var certData map[int]string
		certLoaded := false

		for _, templatePath := range tempTemplatePaths {

			project, service, _, templatePath := eUtils.GetProjectService(driverConfig, templatePath)

			envVersion := eUtils.SplitEnv(driverConfig.CoreConfig.Env)

			tokenName := fmt.Sprintf("config_token_%s", driverConfig.CoreConfig.EnvBasis)
			certMod, err := helperkv.NewModifierFromCoreConfig(driverConfig.CoreConfig, tokenName, driverConfig.CoreConfig.Env, true)

			if err != nil {
				eUtils.LogErrorObject(driverConfig.CoreConfig, err, false)
			}
			certMod.Env = envVersion[0]
			certMod.Version = envVersion[1]

			var ctErr error
			_, certData, certLoaded, ctErr = vcutils.ConfigTemplate(driverConfig, certMod, templatePath, driverConfig.SecretMode, project, service, driverConfig.CoreConfig.WantCerts, false)
			if ctErr != nil {
				if !strings.Contains(ctErr.Error(), "Missing .certData") {
					eUtils.CheckError(driverConfig.CoreConfig, ctErr, true)
				}
			}

			if utils.RefEquals(driverConfig.CoreConfig.TokenCache.GetToken(fmt.Sprintf("config_token_%s", driverConfig.CoreConfig.EnvBasis)), "novault") {
				extractedValues, parseErr := eUtils.Parse(templatePath, project, service)
				if parseErr != nil {
					eUtils.CheckError(driverConfig.CoreConfig, parseErr, true)
				}
				if okSourcePath, okDestPath := extractedValues["certSourcePath"], extractedValues["certDestPath"]; okSourcePath != nil && okDestPath != nil {
					certData[0] = extractedValues["certSourcePath"].(string)
					certData[1] = ""
					certData[2] = extractedValues["certSourcePath"].(string)
				} else {
					continue
				}
			}

			if len(certData) == 0 {
				if certLoaded {
					eUtils.LogInfo(driverConfig.CoreConfig, "Could not load cert "+templatePath)
					continue
				} else {
					continue
				}
			}

			certPath := certData[2]
			eUtils.LogInfo(driverConfig.CoreConfig, "Writing certificate: "+certPath+".")

			if strings.Contains(certPath, "ENV") {
				if len(certMod.Env) >= 5 && (certMod.Env)[:5] == "local" {
					envParts := strings.SplitN(certMod.Env, "/", 3)
					certPath = strings.Replace(certPath, "ENV", envParts[1], 1)
				} else {
					certPath = strings.Replace(certPath, "ENV", certMod.Env, 1)
				}
			}
			if certMod != nil {
				certMod.Release()
			}

			certDestination := driverConfig.EndDir + "/" + certPath
			certDestination = strings.ReplaceAll(certDestination, "//", "/")
			writeToFile(driverConfig.CoreConfig, certData[1], certDestination)
			eUtils.LogInfo(driverConfig.CoreConfig, "certificate written to "+certDestination)
		}
		return nil, nil
	}

	if driverConfig.Diff {
		if !strings.Contains(driverConfig.CoreConfig.Env, "_") {
			driverConfig.CoreConfig.Env = driverConfig.CoreConfig.Env + "_0"
		}
		driverConfig.Update(configCtx, &seedData, driverConfig.CoreConfig.Env+"||"+driverConfig.CoreConfig.Env+"_seed.yml")
	} else {
		writeToFile(driverConfig.CoreConfig, seedData, endPath)
		// Print that we're done
		if strings.Contains(driverConfig.CoreConfig.Env, "_0") {
			driverConfig.CoreConfig.Env = strings.Split(driverConfig.CoreConfig.Env, "_")[0]
		}

		eUtils.LogInfo(driverConfig.CoreConfig, "Seed created and written to "+endPath)
	}

	return nil, nil
}

func writeToFile(config *core.CoreConfig, data string, path string) {
	byteData := []byte(data)
	//Ensure directory has been created
	dirPath := filepath.Dir(path)
	err := os.MkdirAll(dirPath, os.ModePerm)
	eUtils.CheckError(config, err, true)
	//create new file
	newFile, err := os.Create(path)
	eUtils.CheckError(config, err, true)
	defer newFile.Close()
	//write to file
	_, err = newFile.Write(byteData)
	eUtils.CheckError(config, err, true)
	err = newFile.Sync()
	eUtils.CheckError(config, err, true)
}

func GetDirFiles(dir string) []string {
	files, err := os.ReadDir(dir)
	filePaths := []string{}
	//endPaths := []string{}
	if err != nil {
		//this is a file
		return []string{dir}
	}
	for _, file := range files {
		//add this directory to path names
		filename := file.Name()
		if strings.HasSuffix(filename, ".DS_Store") {
			continue
		}
		extension := filepath.Ext(filename)
		filePath := dir + file.Name()
		if !strings.HasSuffix(dir, "/") {
			filePath = dir + "/" + file.Name()
		}
		if extension == "" {
			//if subfolder add /
			filePath += "/"
		}
		//recurse to next level
		newPaths := GetDirFiles(filePath)
		filePaths = append(filePaths, newPaths...)
	}
	return filePaths
}

// MergeMaps - merges 2 maps recursively.
func MergeMaps(x1, x2 interface{}) interface{} {
	switch x1 := x1.(type) {
	case map[string]interface{}:
		x2, ok := x2.(map[string]interface{})
		if !ok {
			return x1
		}
		for k, v2 := range x2 {
			if v1, ok := x1[k]; ok {
				x1[k] = MergeMaps(v1, v2)
			} else {
				x1[k] = v2
			}
		}
	case nil:
		x2, ok := x2.(map[string]interface{})
		if ok {
			return x2
		}
	}
	return x1
}

// Combines the values in a slice, creating a singular map from multiple
// Input:
//   - slice to combine
//   - template slice to combine
//   - depth of map (-1 for value/secret sections)
func CombineSection(config *core.CoreConfig, sliceSectionInterface interface{}, maxDepth int, combinedSectionInterface interface{}) {
	_, okMap := sliceSectionInterface.([]map[string]map[string]map[string]string)

	// Value/secret slice section
	if maxDepth < 0 && okMap {
		sliceSection := sliceSectionInterface.([]map[string]map[string]map[string]string)
		combinedSectionImpl := combinedSectionInterface.(map[string]map[string]map[string]string)
		for _, v := range sliceSection {
			for k2, v2 := range v {
				for k3, v3 := range v2 {
					if _, ok := combinedSectionImpl[k2][k3]; !ok {
						combinedSectionImpl[k2][k3] = map[string]string{}
					}
					for k4, v4 := range v3 {
						combinedSectionImpl[k2][k3][k4] = v4
					}
				}
			}
		}

		combinedSectionInterface = combinedSectionImpl

		// template slice section
	} else {
		if maxDepth < 0 && !okMap {
			eUtils.LogInfo(config, fmt.Sprintf("Env failed to gen.  MaxDepth: %d, okMap: %t\n", maxDepth, okMap))
		}
		sliceSection := sliceSectionInterface.([]interface{})

		for _, v := range sliceSection {
			MergeMaps(combinedSectionInterface, v)
		}
	}
}
