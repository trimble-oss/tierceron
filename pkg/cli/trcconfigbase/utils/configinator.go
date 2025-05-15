package utils

import (
	"crypto/tls"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"

	prod "github.com/trimble-oss/tierceron-core/v2/prod"
	"github.com/trimble-oss/tierceron/pkg/utils"
	eUtils "github.com/trimble-oss/tierceron/pkg/utils"
	"github.com/trimble-oss/tierceron/pkg/utils/config"
	"github.com/trimble-oss/tierceron/pkg/validator"
	helperkv "github.com/trimble-oss/tierceron/pkg/vaulthelper/kv"
)

var mutex = &sync.Mutex{}
var ConfiginatorOsPathSeparator string = string(os.PathSeparator)

func trimPath(e string, toReplace string) (string, bool) {
	if len(toReplace) == 0 || !strings.Contains(e, toReplace) {
		return e, false
	}
	if strings.HasPrefix(e, toReplace) && !strings.HasSuffix(toReplace, ConfiginatorOsPathSeparator) {
		toReplace = toReplace + ConfiginatorOsPathSeparator
	}
	if strings.HasPrefix(e, toReplace) || strings.Index(e, toReplace) != 0 && string(e[strings.Index(e, toReplace)-1]) == ConfiginatorOsPathSeparator {
		e = strings.Replace(e, toReplace, "", 1)
	} else {
		e = strings.Replace(e, toReplace, ConfiginatorOsPathSeparator, 1)
	}
	return e, true
}

func generatePaths(driverConfig *config.DriverConfig) ([]string, []string, error) {
	//initialized := false
	templatePaths := []string{}
	endPaths := []string{}
	if driverConfig == nil {
		return templatePaths, endPaths, errors.New("empty configuration passed in")
	}

	var trcProjectService string = ""
	var trcService string = ""
	var project string = ""

	if projectService, ok := driverConfig.DeploymentConfig["trcprojectservice"]; ok && len(driverConfig.ServicesWanted) == 0 || len(driverConfig.ServicesWanted) == 1 {
		if driverConfig.CoreConfig.WantCerts {
			trcProjectService = "Common"
		} else {
			if ok && len(driverConfig.ServicesWanted) == 0 {
				trcProjectService = strings.ReplaceAll(projectService.(string), "\\", "/")
			} else {
				trcProjectService = strings.ReplaceAll(driverConfig.ServicesWanted[0], "\\", "/")
			}
			if !strings.Contains(trcProjectService, "/") {
				fmt.Println("Make sure both Project/Service is specified with proper formatting.")
				return templatePaths, endPaths, errors.New("project and service specified without slash")
			}
			if !strings.HasSuffix(trcProjectService, "/") {
				trcProjectService = trcProjectService + "/"
			}
		}
		if trcProjectService != "Common" {
			trcProjectServiceParts := strings.Split(trcProjectService, "/")
			if len(trcProjectServiceParts) < 2 {
				fmt.Println("Make sure both Project/Service is specified with proper formatting.")
				return templatePaths, endPaths, errors.New("project and service not specified correctly")
			}
			project = trcProjectServiceParts[0] + "/"
			trcService = "/" + trcProjectServiceParts[1] + "/"
		}
		if len(driverConfig.StartDir) > 1 {
			// If multiple starting directories, filter starting directories based on project name
			startDirFiltered := []string{}
			for _, startDir := range driverConfig.StartDir {
				startDir = strings.ReplaceAll(startDir, "\\", "/")
				if !strings.HasSuffix(startDir, "/") {
					startDir = startDir + "/"
				}
				if strings.Index(startDir, project) != 0 && !strings.HasPrefix(project, "/") {
					project = "/" + project
				}
				if strings.Contains(startDir, project) { //HasSuffix
					startDirFiltered = append(startDirFiltered, startDir)
				}
			}
			driverConfig.StartDir = startDirFiltered
			if len(driverConfig.StartDir) == 0 {
				fmt.Println("Invalid starting directory, ensure directory includes project name.")
				return templatePaths, endPaths, errors.New("invalid starting directory passed in")
			}
		}
	}

	for _, startDir := range driverConfig.StartDir {
		if ConfiginatorOsPathSeparator == "\\" || eUtils.IsWindows() {
			// Note: Checking path separator for testing and keeping IsWindows to be safe
			startDir = strings.ReplaceAll(startDir, "/", ConfiginatorOsPathSeparator)
			trcProjectService = strings.ReplaceAll(trcProjectService, "/", ConfiginatorOsPathSeparator)
			trcService = strings.ReplaceAll(trcService, "/", ConfiginatorOsPathSeparator)
		}
		if !strings.HasSuffix(startDir, ConfiginatorOsPathSeparator) {
			startDir = startDir + ConfiginatorOsPathSeparator
		}

		//get files from directory
		tp, ep := getDirFiles(driverConfig, startDir, driverConfig.EndDir)
		if len(trcProjectService) > 0 {
			epScrubbed := []string{}
			tpScrubbed := []string{}
			// Do some scrubbing...
			for ie, e := range ep {
				matched := false
				if len(trcProjectService) > 0 && strings.Contains(e, trcProjectService) {
					e, matched = trimPath(e, trcProjectService)
				} else if len(trcService) > 0 && strings.Contains(e, trcService) {
					e, matched = trimPath(e, trcService)
				}

				if matched {
					epScrubbed = append(epScrubbed, e)
					tpScrubbed = append(tpScrubbed, tp[ie])
				}
			}
			if len(epScrubbed) > 0 {
				// Only overwrite if something
				ep = epScrubbed
				tp = tpScrubbed
			}
		}
		templatePaths = append(templatePaths, tp...)
		endPaths = append(endPaths, ep...)
	}
	return templatePaths, endPaths, nil
}

// GenerateConfigsFromVault configures the templates in trc_templates and writes them to trcconfig
func GenerateConfigsFromVault(ctx config.ProcessContext, configCtx *config.ConfigContext, driverConfig *config.DriverConfig) (interface{}, error) {
	/*Cyan := "\033[36m"
	Reset := "\033[0m"
	if utils.IsWindows() {
		Reset = ""
		Cyan = ""
	}*/
	//Should check if driverConfig is nil here...
	tokenNamePtr := driverConfig.CoreConfig.GetCurrentToken("config_token_%s")

	modCheck, err := helperkv.NewModifierFromCoreConfig(driverConfig.CoreConfig, *tokenNamePtr, driverConfig.CoreConfig.EnvBasis, true)
	if err != nil {
		eUtils.LogErrorObject(driverConfig.CoreConfig, err, false)
		return nil, err
	}
	modCheck.Env = driverConfig.CoreConfig.Env
	version := ""
	modCheck.VersionFilter = driverConfig.VersionFilter

	//Check if templateInfo is selected for template or values
	templateInfo := false
	versionInfo := false
	if strings.Contains(driverConfig.CoreConfig.Env, "_") {
		envVersion := eUtils.SplitEnv(driverConfig.CoreConfig.Env)

		driverConfig.CoreConfig.Env = envVersion[0]
		version = envVersion[1]
		switch version {
		case "versionInfo":
			versionInfo = true
		case "templateInfo":
			templateInfo = true
		}
	}
	versionData := make(map[string]interface{})
	if !utils.RefEquals(driverConfig.CoreConfig.TokenCache.GetTokenStr(tokenNamePtr), "novault") {
		// Chewbacca: break here...
		if valid, baseDesiredPolicy, errValidateEnvironment := modCheck.ValidateEnvironment(modCheck.EnvBasis, false, "", driverConfig.CoreConfig.Log); errValidateEnvironment != nil || !valid {
			if errValidateEnvironment != nil {
				if urlErr, urlErrOk := errValidateEnvironment.(*url.Error); urlErrOk {
					if _, sErrOk := urlErr.Err.(*tls.CertificateVerificationError); sErrOk {
						return nil, eUtils.LogAndSafeExit(driverConfig.CoreConfig, "Invalid certificate.", 1)
					}
				}
			}

			if unrestrictedValid, desiredPolicy, errValidateUnrestrictedEnvironment := modCheck.ValidateEnvironment(modCheck.EnvBasis, false, "_unrestricted", driverConfig.CoreConfig.Log); errValidateUnrestrictedEnvironment != nil || !unrestrictedValid {
				return nil, eUtils.LogAndSafeExit(driverConfig.CoreConfig, fmt.Sprintf("Mismatched token for requested environment: %s base policy: %s policy: %s", driverConfig.CoreConfig.Env, baseDesiredPolicy, desiredPolicy), 1)
			}
		}
	}

	templatePaths, endPaths, err := generatePaths(driverConfig)
	if err != nil {
		eUtils.LogErrorObject(driverConfig.CoreConfig, err, false)
	}

	_, _, indexedEnv, _ := helperkv.PreCheckEnvironment(driverConfig.CoreConfig.Env)
	if indexedEnv {
		templatePaths, err = eUtils.GetAcceptedTemplatePaths(driverConfig, modCheck, templatePaths)
		if err != nil {
			eUtils.LogErrorObject(driverConfig.CoreConfig, err, false)
		}
		endPaths, err = eUtils.GetAcceptedTemplatePaths(driverConfig, modCheck, endPaths)
		if err != nil {
			eUtils.LogErrorObject(driverConfig.CoreConfig, err, false)
		}
	}

	//File filter
	templatePaths, endPaths = FilterPaths(templatePaths, endPaths, driverConfig.FileFilter, false)

	templatePaths, endPaths = FilterPaths(templatePaths, endPaths, driverConfig.ServicesWanted, false)

	if len(templatePaths) == 0 || len(endPaths) == 0 {
		eUtils.LogErrorObject(driverConfig.CoreConfig, errors.New("filtered paths returned empty template or end paths"), false)
	}

	for _, templatePath := range templatePaths {
		if !driverConfig.CoreConfig.WantCerts && strings.Contains(templatePath, "Common") {
			continue
		}
		_, service, _, _ := eUtils.GetProjectService(driverConfig, templatePath) //This checks for nested project names
		driverConfig.VersionFilter = append(driverConfig.VersionFilter, service) //Adds nested project name to filter otherwise it will be not found.
	}

	if driverConfig.CoreConfig.WantCerts && versionInfo { //For cert version history
		driverConfig.VersionFilter = append(driverConfig.VersionFilter, "Common")
	}

	driverConfig.VersionFilter = eUtils.RemoveDuplicates(driverConfig.VersionFilter)
	modCheck.VersionFilter = driverConfig.VersionFilter

	if versionInfo {
		//versionDataMap := make(map[string]map[string]interface{})
		//Gets version metadata for super secrets or values if super secrets don't exist.
		if strings.Contains(modCheck.Env, ".") {
			envVersion := eUtils.SplitEnv(modCheck.Env)
			driverConfig.VersionFilter = append(driverConfig.VersionFilter, envVersion[0])
			modCheck.Env = envVersion[0]
		}

		versionMetadataMap := eUtils.GetProjectVersionInfo(driverConfig, modCheck)
		//var masterKey string
		project := ""
		neverPrinted := true
		if len(driverConfig.VersionFilter) > 0 {
			project = driverConfig.VersionFilter[0]
		}
		first := true
		for key := range versionMetadataMap {
			passed := false
			if driverConfig.CoreConfig.WantCerts {
				//If paths were clean - this would be logic
				/*
					if len(key) > 0 {
						keySplit := strings.Split(key, "/")
						config.VersionInfo(versionMetadataMap[key], false, keySplit[len(keySplit)-1], neverPrinted)
						neverPrinted = false
					}
				*/
				//This is happening because of garbage paths that look like this -> values/{projectName}/{service}/Common/{file.cer}
				for _, service := range driverConfig.VersionFilter { //The following for loop could be removed if vault paths were clean
					if !passed && strings.Contains(key, "Common") && strings.Contains(key, service) && !strings.Contains(key, project) && !strings.HasSuffix(key, "Common") {
						if len(key) > 0 {
							keySplit := strings.Split(key, "/")
							driverConfig.VersionInfo(versionMetadataMap[key], false, keySplit[len(keySplit)-1], neverPrinted)
							passed = true
							neverPrinted = false
						}
					}
				}
			} else {
				if len(key) > 0 {
					driverConfig.VersionInfo(versionMetadataMap[key], false, key, first)
					//return nil, eUtils.LogAndSafeExit(config, "", 1)
					if first {
						neverPrinted = false
						first = false
					}
				}
			}

		}
		if neverPrinted {
			eUtils.LogInfo(driverConfig.CoreConfig, "No version data available for this env")
		}
		return nil, nil //End of -versions flag
		/*	we might need this commented code - but seems unnecessary
			for valuePath, data := range versionMetadataMap {
				projectFound := false
				for _, project := range config.VersionFilter {
					if strings.Contains(valuePath, project) {
						projectFound = true
						initialized = true
						break
					}
				}
				if !projectFound {
					continue
				}

				versionDataMap[valuePath] = data
				masterKey = valuePath
			}

			if versionDataMap != nil {
				config.VersionInfo(versionDataMap[masterKey], false, masterKey, false)
			} else if !initialized {
				eUtils.LogInfo(Cyan+"No metadata found for this environment"+Reset, config.Log)
			}
		*/
	} else if !templateInfo {
		if version != "0" { //Check requested version bounds
			versionMetadataMap := eUtils.GetProjectVersionInfo(driverConfig, modCheck)
			versionNumbers := eUtils.GetProjectVersions(driverConfig, versionMetadataMap)

			eUtils.BoundCheck(driverConfig, versionNumbers, version)
		}
	}

	var wg sync.WaitGroup
	//configure each template in directory
	driverConfig.DiffCounter = len(templatePaths)
	for i, templatePath := range templatePaths {
		wg.Add(1)
		go func(i int, templatePath string, version string, versionData map[string]interface{}) error {
			defer wg.Done()

			tokenNamePtr := driverConfig.CoreConfig.GetCurrentToken("config_token_%s")

			mod, _ := helperkv.NewModifierFromCoreConfig(driverConfig.CoreConfig, *tokenNamePtr, driverConfig.CoreConfig.EnvBasis, true)
			mod.Env = driverConfig.CoreConfig.Env
			mod.Version = version
			//check for template_files directory here
			project, service, _, templatePath := eUtils.GetProjectService(driverConfig, templatePath)

			var isCert bool
			if service != "" {
				if strings.HasSuffix(templatePath, ".DS_Store") {
					goto wait
				}

				isCert := false
				if strings.Contains(templatePath, ".pfx.mf") ||
					strings.Contains(templatePath, ".cer.mf") ||
					strings.Contains(templatePath, ".crt.mf") ||
					strings.Contains(templatePath, ".key.mf") ||
					strings.Contains(templatePath, ".pem.mf") ||
					strings.Contains(templatePath, ".jks.mf") {
					isCert = true
				}

				if driverConfig.CoreConfig.WantCerts != isCert {
					goto wait
				}

				var configuredTemplate string
				var certData map[int]string
				certLoaded := false
				if templateInfo {
					data, errTemplateVersion := getTemplateVersionData(driverConfig.CoreConfig, mod, project, service, endPaths[i])
					if errTemplateVersion != nil {
						return errTemplateVersion
					}

					mutex.Lock()
					if data == nil {
						return eUtils.LogAndSafeExit(driverConfig.CoreConfig, "Template version data could not be retrieved", 1)
					}
					versionData[endPaths[i]] = data
					mutex.Unlock()
					goto wait
				} else {
					var ctErr error
					configuredTemplate, certData, certLoaded, ctErr = ConfigTemplate(driverConfig, mod, templatePath, driverConfig.SecretMode, project, service, driverConfig.CoreConfig.WantCerts, driverConfig.ZeroConfig)
					if ctErr != nil {
						if !strings.Contains(ctErr.Error(), "Missing .certData") {
							eUtils.LogErrorObject(driverConfig.CoreConfig, ctErr, false)
							goto wait
						}
					} else if driverConfig.WantKeystore != "" && len(certData) == 0 {
						if driverConfig.KeystorePassword == "" {
							projectSecrets, err := mod.ReadData(fmt.Sprintf("super-secrets/%s", driverConfig.VersionFilter[0]))
							if err == nil {
								if trustStorePassword, tspOk := projectSecrets["trustStorePassword"].(string); tspOk {
									driverConfig.KeystorePassword = trustStorePassword
								}
							} else {
								if driverConfig.CoreConfig.TokenCache != nil {
									mod.EmptyCache()
									driverConfig.CoreConfig.TokenCache.Clear()
								}
							}
						}
					}
				}
				//generate template or certificate
				if driverConfig.CoreConfig.WantCerts && certLoaded {
					if driverConfig.WantKeystore != "" && len(certData) == 0 {
						// Keystore is serialized at end.
						goto wait
					}
					if len(certData) == 0 {
						eUtils.LogInfo(driverConfig.CoreConfig, "Could not load cert "+endPaths[i])
						goto wait
					}
					destFile := certData[0]
					certDestination := driverConfig.EndDir + "/" + destFile
					writeToFile(driverConfig, certData[1], certDestination)
					if driverConfig.OutputMemCache {
						eUtils.LogInfo(driverConfig.CoreConfig, "certificate pre-processed for "+certDestination)
					} else {
						eUtils.LogInfo(driverConfig.CoreConfig, "certificate written to "+certDestination)
					}
					goto wait
				} else {
					if driverConfig.Diff {
						if version != "" {
							driverConfig.Update(configCtx, &configuredTemplate, driverConfig.CoreConfig.Env+"_"+version+"||"+endPaths[i])
						} else {
							driverConfig.Update(configCtx, &configuredTemplate, driverConfig.CoreConfig.Env+"||"+endPaths[i])
						}
					} else {
						writeToFile(driverConfig, configuredTemplate, endPaths[i])
					}
				}
			} else {
				isCert := false
				if strings.Contains(service, ".pfx.mf") ||
					strings.Contains(service, ".cer.mf") ||
					strings.Contains(service, ".pem.mf") ||
					strings.Contains(service, ".jks.mf") {
					isCert = true
				}

				if driverConfig.CoreConfig.WantCerts != isCert {
					goto wait
				}
				//assume the starting directory was trc_templates
				var configuredTemplate string
				var certData map[int]string
				certLoaded := false
				if templateInfo {
					data, errTemplateVersion := getTemplateVersionData(driverConfig.CoreConfig, mod, project, service, endPaths[i])
					if errTemplateVersion != nil {
						goto wait
					}
					versionData[endPaths[i]] = data
					goto wait
				} else {
					var ctErr error
					configuredTemplate, certData, certLoaded, ctErr = ConfigTemplate(driverConfig, mod, templatePath, driverConfig.SecretMode, project, service, driverConfig.CoreConfig.WantCerts, false)
					if ctErr != nil {
						if !strings.Contains(ctErr.Error(), "Missing .certData") {
							eUtils.LogErrorObject(driverConfig.CoreConfig, ctErr, false)
							goto wait
						}
					}
				}
				if driverConfig.CoreConfig.WantCerts && certLoaded {
					if driverConfig.WantKeystore != "" {
						// Keystore is serialized at end.
						goto wait
					}
					certDestination := driverConfig.EndDir + "/" + certData[0]
					writeToFile(driverConfig, certData[1], certDestination)
					if driverConfig.OutputMemCache {
						eUtils.LogInfo(driverConfig.CoreConfig, "certificate pre-processed for "+certDestination)
					} else {
						eUtils.LogInfo(driverConfig.CoreConfig, "certificate written to "+certDestination)
					}
					goto wait
				} else {
					if driverConfig.Diff {
						if version != "" {
							driverConfig.Update(configCtx, &configuredTemplate, driverConfig.CoreConfig.Env+"_"+version+"||"+endPaths[i])
						} else {
							driverConfig.Update(configCtx, &configuredTemplate, driverConfig.CoreConfig.Env+"||"+endPaths[i])
						}
					} else {
						writeToFile(driverConfig, configuredTemplate, endPaths[i])
					}
				}
			}

			//print that we're done
			if !driverConfig.Diff && !isCert && !templateInfo {
				messageBase := "template configured and written to "
				if driverConfig.OutputMemCache {
					messageBase = "template configured and pre-processed for "
				}
				if utils.IsWindows() {
					eUtils.LogInfo(driverConfig.CoreConfig, messageBase+endPaths[i])
				} else {
					eUtils.LogInfo(driverConfig.CoreConfig, "\033[0;33m"+messageBase+endPaths[i]+"\033[0m")
				}
			}

		wait:
			mod.Close()

			return nil
		}(i, templatePath, version, versionData)
	}
	wg.Wait()
	if templateInfo {
		driverConfig.VersionInfo(versionData, true, "", false)
	}
	if driverConfig.WantKeystore != "" {
		// Keystore is serialized at end.
		ks, ksErr := validator.StoreKeystore(driverConfig, driverConfig.KeystorePassword)
		if ksErr != nil {
			eUtils.LogErrorObject(driverConfig.CoreConfig, ksErr, false)
		}
		certDestination := driverConfig.EndDir + "/" + driverConfig.WantKeystore
		eUtils.LogInfo(driverConfig.CoreConfig, "certificates written to "+certDestination)
		writeToFile(driverConfig, string(ks), certDestination)
	}

	return nil, nil
}

func writeToFile(driverConfig *config.DriverConfig, data string, path string) {
	if strings.Contains(data, "${TAG}") {
		tag := os.Getenv("TRCENV_TAG")
		if len(tag) > 0 {
			var matched bool
			var err error
			if prod.IsStagingProd(driverConfig.CoreConfig.Env) {
				matched, err = regexp.MatchString("^[0-9]{1,3}\\.[0-9]{1,3}\\.[0-9]{1,3}\\.[0-9]{1,3}$", tag)
				if !matched || err != nil {
					matched, err = regexp.MatchString("^v[0-9]{1,3}\\.[0-9]{1,3}\\.[0-9]{1,3}$", tag)
				}
			} else {
				matched, err = regexp.MatchString("^[a-fA-F0-9]{40}$", tag)
			}

			if !matched || err != nil {
				fmt.Println("Invalid build tag")
				eUtils.LogInfo(driverConfig.CoreConfig, "Invalid build tag was found:"+tag+"- exiting...")
				os.Exit(-1)
			}
		}
		data = strings.Replace(data, "${TAG}", tag, -1)
	}
	if strings.Contains(data, "${RELEASE}") {
		release := os.Getenv("RELEASE")
		data = strings.Replace(data, "${RELEASE}", release, -1)
	}

	byteData := []byte(data)
	//Ensure directory has been created
	var newFile *os.File

	if driverConfig.OutputMemCache {
		driverConfig.MemFs.WriteToMemFile(driverConfig.CoreConfig, &byteData, path)
	} else {
		dirPath := filepath.Dir(path)
		err := os.MkdirAll(dirPath, os.ModePerm)
		eUtils.CheckError(driverConfig.CoreConfig, err, true)
		//create new file
		newFile, err = os.Create(path)
		eUtils.CheckError(driverConfig.CoreConfig, err, true)
		defer newFile.Close()
		//write to file
		_, err = newFile.Write(byteData)
		eUtils.CheckError(driverConfig.CoreConfig, err, true)
		err = newFile.Sync()
		eUtils.CheckError(driverConfig.CoreConfig, err, true)
	}
}

func getDirFiles(driverConfig *config.DriverConfig, dir string, endDir string) ([]string, []string) {
	filePaths := []string{}
	endPaths := []string{}

	if driverConfig.ReadMemCache {
		files, err := driverConfig.MemFs.ReadDir(dir)
		if err != nil || len(files) == 0 {
			dirInfo, err := driverConfig.MemFs.Lstat(dir)
			if err == nil && !dirInfo.IsDir() {
				//this is a file
				return []string{dir}, []string{endDir}
			}
		}
		for _, file := range files {
			//add this directory to path names
			if dir[len(dir)-1] != os.PathSeparator {
				dir = dir + string(os.PathSeparator)
			}
			//take off .tmpl extension
			filename := file.Name()
			filePath := dir + filename
			if strings.HasSuffix(filename, ".DS_Store") {
				continue
			}
			extension := filepath.Ext(filename)
			endPath := ""
			if extension == ".tmpl" {
				name := filename[0 : len(filename)-len(extension)]
				endPath = endDir + string(os.PathSeparator) + name
			} else {
				endPath = endDir + string(os.PathSeparator) + filename
			}
			//recurse to next level
			newPaths, newEndPaths := getDirFiles(driverConfig, filePath, endPath)
			filePaths = append(filePaths, newPaths...)
			endPaths = append(endPaths, newEndPaths...)
			//add endings of path names
		}

	} else {
		files, err := os.ReadDir(dir)
		if err != nil {
			//this is a file
			return []string{dir}, []string{endDir}
		}
		for _, file := range files {
			//add this directory to path names
			if dir[len(dir)-1] != os.PathSeparator {
				dir = dir + string(os.PathSeparator)
			}
			//take off .tmpl extension
			filename := file.Name()
			filePath := dir + filename
			if strings.HasSuffix(filename, ".DS_Store") {
				continue
			}
			extension := filepath.Ext(filename)
			endPath := ""
			if extension == ".tmpl" {
				name := filename[0 : len(filename)-len(extension)]
				endPath = endDir + string(os.PathSeparator) + name
			} else {
				endPath = endDir + string(os.PathSeparator) + filename
			}
			//recurse to next level
			newPaths, newEndPaths := getDirFiles(driverConfig, filePath, endPath)
			filePaths = append(filePaths, newPaths...)
			endPaths = append(endPaths, newEndPaths...)
			//add endings of path names
		}

	}
	return filePaths, endPaths
}
