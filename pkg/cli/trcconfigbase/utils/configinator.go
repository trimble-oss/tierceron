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

	"github.com/go-git/go-billy/v5"

	"github.com/trimble-oss/tierceron/pkg/utils"
	eUtils "github.com/trimble-oss/tierceron/pkg/utils"
	"github.com/trimble-oss/tierceron/pkg/validator"
	helperkv "github.com/trimble-oss/tierceron/pkg/vaulthelper/kv"
)

var mutex = &sync.Mutex{}

// GenerateConfigsFromVault configures the templates in trc_templates and writes them to trcconfig
func GenerateConfigsFromVault(ctx eUtils.ProcessContext, configCtx *eUtils.ConfigContext, config *eUtils.DriverConfig) (interface{}, error) {
	/*Cyan := "\033[36m"
	Reset := "\033[0m"
	if utils.IsWindows() {
		Reset = ""
		Cyan = ""
	}*/
	modCheck, err := helperkv.NewModifier(config.Insecure, config.Token, config.VaultAddress, config.EnvRaw, config.Regions, true, config.Log)
	modCheck.Env = config.Env
	version := ""
	if err != nil {
		eUtils.LogErrorObject(config, err, false)
		return nil, err
	}
	modCheck.VersionFilter = config.VersionFilter

	//Check if templateInfo is selected for template or values
	templateInfo := false
	versionInfo := false
	if strings.Contains(config.Env, "_") {
		envVersion := eUtils.SplitEnv(config.Env)

		config.Env = envVersion[0]
		version = envVersion[1]
		switch version {
		case "versionInfo":
			versionInfo = true
		case "templateInfo":
			templateInfo = true
		}
	}
	versionData := make(map[string]interface{})
	if config.Token != "novault" {
		if valid, baseDesiredPolicy, errValidateEnvironment := modCheck.ValidateEnvironment(modCheck.RawEnv, false, "", config.Log); errValidateEnvironment != nil || !valid {
			if errValidateEnvironment != nil {
				if urlErr, urlErrOk := errValidateEnvironment.(*url.Error); urlErrOk {
					if _, sErrOk := urlErr.Err.(*tls.CertificateVerificationError); sErrOk {
						return nil, eUtils.LogAndSafeExit(config, "Invalid certificate.", 1)
					}
				}
			}

			if unrestrictedValid, desiredPolicy, errValidateUnrestrictedEnvironment := modCheck.ValidateEnvironment(modCheck.RawEnv, false, "_unrestricted", config.Log); errValidateUnrestrictedEnvironment != nil || !unrestrictedValid {
				return nil, eUtils.LogAndSafeExit(config, fmt.Sprintf("Mismatched token for requested environment: %s base policy: %s policy: %s", config.Env, baseDesiredPolicy, desiredPolicy), 1)
			}
		}
	}

	//initialized := false
	templatePaths := []string{}
	endPaths := []string{}

	//templatePaths
	for _, startDir := range config.StartDir {
		//get files from directory
		tp, ep := getDirFiles(startDir, config.EndDir)
		if trcProjectService, ok := config.DeploymentConfig["trcprojectservice"]; ok && strings.Contains(trcProjectService.(string), "/") {
			epScrubbed := []string{}
			// Do some scrubbing...
			for _, e := range ep {
				e = strings.Replace(e, trcProjectService.(string), "/", 1)
				projectService := strings.Replace(trcProjectService.(string), "/", "\\", 1)
				e = strings.Replace(e, projectService, "\\", 1)
				epScrubbed = append(epScrubbed, e)
			}
			ep = epScrubbed
		}

		templatePaths = append(templatePaths, tp...)
		endPaths = append(endPaths, ep...)
	}

	_, _, indexedEnv, _ := helperkv.PreCheckEnvironment(config.Env)
	if indexedEnv {
		templatePaths, err = eUtils.GetAcceptedTemplatePaths(config, modCheck, templatePaths)
		if err != nil {
			eUtils.LogErrorObject(config, err, false)
		}
		endPaths, err = eUtils.GetAcceptedTemplatePaths(config, modCheck, endPaths)
		if err != nil {
			eUtils.LogErrorObject(config, err, false)
		}
	}

	//File filter
	templatePaths, endPaths = FilterPaths(templatePaths, endPaths, config.FileFilter, false)

	templatePaths, endPaths = FilterPaths(templatePaths, endPaths, config.ServicesWanted, false)

	for _, templatePath := range templatePaths {
		if !config.WantCerts && strings.Contains(templatePath, "Common") {
			continue
		}
		_, service, _ := eUtils.GetProjectService(templatePath)      //This checks for nested project names
		config.VersionFilter = append(config.VersionFilter, service) //Adds nested project name to filter otherwise it will be not found.
	}

	if config.WantCerts && versionInfo { //For cert version history
		config.VersionFilter = append(config.VersionFilter, "Common")
	}

	config.VersionFilter = eUtils.RemoveDuplicates(config.VersionFilter)
	modCheck.VersionFilter = config.VersionFilter

	if versionInfo {
		//versionDataMap := make(map[string]map[string]interface{})
		//Gets version metadata for super secrets or values if super secrets don't exist.
		if strings.Contains(modCheck.Env, ".") {
			envVersion := eUtils.SplitEnv(modCheck.Env)
			config.VersionFilter = append(config.VersionFilter, envVersion[0])
			modCheck.Env = envVersion[0]
		}

		versionMetadataMap := eUtils.GetProjectVersionInfo(config, modCheck)
		//var masterKey string
		project := ""
		neverPrinted := true
		if len(config.VersionFilter) > 0 {
			project = config.VersionFilter[0]
		}
		first := true
		for key := range versionMetadataMap {
			passed := false
			if config.WantCerts {
				//If paths were clean - this would be logic
				/*
					if len(key) > 0 {
						keySplit := strings.Split(key, "/")
						config.VersionInfo(versionMetadataMap[key], false, keySplit[len(keySplit)-1], neverPrinted)
						neverPrinted = false
					}
				*/
				//This is happening because of garbage paths that look like this -> values/{projectName}/{service}/Common/{file.cer}
				for _, service := range config.VersionFilter { //The following for loop could be removed if vault paths were clean
					if !passed && strings.Contains(key, "Common") && strings.Contains(key, service) && !strings.Contains(key, project) && !strings.HasSuffix(key, "Common") {
						if len(key) > 0 {
							keySplit := strings.Split(key, "/")
							config.VersionInfo(versionMetadataMap[key], false, keySplit[len(keySplit)-1], neverPrinted)
							passed = true
							neverPrinted = false
						}
					}
				}
			} else {
				if len(key) > 0 {
					config.VersionInfo(versionMetadataMap[key], false, key, first)
					//return nil, eUtils.LogAndSafeExit(config, "", 1)
					if first {
						neverPrinted = false
						first = false
					}
				}
			}

		}
		if neverPrinted {
			eUtils.LogInfo(config, "No version data available for this env")
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
			versionMetadataMap := eUtils.GetProjectVersionInfo(config, modCheck)
			versionNumbers := eUtils.GetProjectVersions(config, versionMetadataMap)

			eUtils.BoundCheck(config, versionNumbers, version)
		}
	}

	var wg sync.WaitGroup
	//configure each template in directory
	config.DiffCounter = len(templatePaths)
	for i, templatePath := range templatePaths {
		wg.Add(1)
		go func(i int, templatePath string, version string, versionData map[string]interface{}) error {
			defer wg.Done()

			mod, _ := helperkv.NewModifier(config.Insecure, config.Token, config.VaultAddress, config.EnvRaw, config.Regions, true, config.Log)
			mod.Env = config.Env
			mod.Version = version
			//check for template_files directory here
			project, service, templatePath := GetProjectService(config, templatePath)

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

				if config.WantCerts != isCert {
					goto wait
				}

				if strings.HasSuffix(templatePath, ".tmpl") {
					if !config.ZeroConfig {
						if strings.HasSuffix(templatePath, "nc.properties.tmpl") {
							goto wait
						}
					} else {
						if !strings.HasSuffix(templatePath, "nc.properties.tmpl") {
							goto wait
						}
					}
				}

				var configuredTemplate string
				var certData map[int]string
				certLoaded := false
				if templateInfo {
					data, errTemplateVersion := getTemplateVersionData(config, mod, project, service, endPaths[i])
					if errTemplateVersion != nil {
						return errTemplateVersion
					}

					mutex.Lock()
					if data == nil {
						return eUtils.LogAndSafeExit(config, "Template version data could not be retrieved", 1)
					}
					versionData[endPaths[i]] = data
					mutex.Unlock()
					goto wait
				} else {
					var ctErr error
					configuredTemplate, certData, certLoaded, ctErr = ConfigTemplate(config, mod, templatePath, config.SecretMode, project, service, config.WantCerts, false)
					if ctErr != nil {
						if !strings.Contains(ctErr.Error(), "Missing .certData") {
							eUtils.CheckError(config, ctErr, true)
						}
					} else if config.WantKeystore != "" && len(certData) == 0 {
						if config.KeystorePassword == "" {
							projectSecrets, err := mod.ReadData(fmt.Sprintf("super-secrets/%s", config.VersionFilter[0]))
							if err == nil {
								if trustStorePassword, tspOk := projectSecrets["trustStorePassword"].(string); tspOk {
									config.KeystorePassword = trustStorePassword
								}
							}
						}
					}
				}
				//generate template or certificate
				if config.WantCerts && certLoaded {
					if config.WantKeystore != "" && len(certData) == 0 {
						// Keystore is serialized at end.
						goto wait
					}
					if len(certData) == 0 {
						eUtils.LogInfo(config, "Could not load cert "+endPaths[i])
						goto wait
					}
					destFile := certData[0]
					certDestination := config.EndDir + "/" + destFile
					writeToFile(config, certData[1], certDestination)
					if config.OutputMemCache {
						eUtils.LogInfo(config, "certificate pre-processed for "+certDestination)
					} else {
						eUtils.LogInfo(config, "certificate written to "+certDestination)
					}
					goto wait
				} else {
					if config.Diff {
						if version != "" {
							config.Update(configCtx, &configuredTemplate, config.Env+"_"+version+"||"+endPaths[i])
						} else {
							config.Update(configCtx, &configuredTemplate, config.Env+"||"+endPaths[i])
						}
					} else {
						writeToFile(config, configuredTemplate, endPaths[i])
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

				if config.WantCerts != isCert {
					goto wait
				}
				//assume the starting directory was trc_templates
				var configuredTemplate string
				var certData map[int]string
				certLoaded := false
				if templateInfo {
					data, errTemplateVersion := getTemplateVersionData(config, mod, project, service, endPaths[i])
					if errTemplateVersion != nil {
						return errTemplateVersion
					}
					versionData[endPaths[i]] = data
					goto wait
				} else {
					var ctErr error
					configuredTemplate, certData, certLoaded, ctErr = ConfigTemplate(config, mod, templatePath, config.SecretMode, project, service, config.WantCerts, false)
					if ctErr != nil {
						if !strings.Contains(ctErr.Error(), "Missing .certData") {
							eUtils.CheckError(config, ctErr, true)
						}
					}
				}
				if config.WantCerts && certLoaded {
					if config.WantKeystore != "" {
						// Keystore is serialized at end.
						goto wait
					}
					certDestination := config.EndDir + "/" + certData[0]
					writeToFile(config, certData[1], certDestination)
					if config.OutputMemCache {
						eUtils.LogInfo(config, "certificate pre-processed for "+certDestination)
					} else {
						eUtils.LogInfo(config, "certificate written to "+certDestination)
					}
					goto wait
				} else {
					if config.Diff {
						if version != "" {
							config.Update(configCtx, &configuredTemplate, config.Env+"_"+version+"||"+endPaths[i])
						} else {
							config.Update(configCtx, &configuredTemplate, config.Env+"||"+endPaths[i])
						}
					} else {
						writeToFile(config, configuredTemplate, endPaths[i])
					}
				}
			}

			//print that we're done
			if !config.Diff && !isCert && !templateInfo {
				messageBase := "template configured and written to "
				if config.OutputMemCache {
					messageBase = "template configured and pre-processed for "
				}
				if utils.IsWindows() {
					eUtils.LogInfo(config, messageBase+endPaths[i])
				} else {
					eUtils.LogInfo(config, "\033[0;33m"+messageBase+endPaths[i]+"\033[0m")
				}
			}

		wait:
			mod.Close()

			return nil
		}(i, templatePath, version, versionData)
	}
	wg.Wait()
	if templateInfo {
		config.VersionInfo(versionData, true, "", false)
	}
	if config.WantKeystore != "" {
		// Keystore is serialized at end.
		ks, ksErr := validator.StoreKeystore(config, config.KeystorePassword)
		if ksErr != nil {
			eUtils.LogErrorObject(config, ksErr, false)
		}
		certDestination := config.EndDir + "/" + config.WantKeystore
		eUtils.LogInfo(config, "certificates written to "+certDestination)
		writeToFile(config, string(ks), certDestination)
	}

	return nil, nil
}

var memCacheLock sync.Mutex

func writeToFile(config *eUtils.DriverConfig, data string, path string) {
	if strings.Contains(data, "${TAG}") {
		tag := os.Getenv("TRCENV_TAG")
		if len(tag) > 0 {
			var matched bool
			var err error
			if config.Env == "staging" || config.Env == "prod" {
				matched, err = regexp.MatchString("^[0-9]{1,3}\\.[0-9]{1,3}\\.[0-9]{1,3}\\.[0-9]{1,3}$", tag)
			} else {
				matched, err = regexp.MatchString("^[a-fA-F0-9]{40}$", tag)
			}

			if !matched || err != nil {
				fmt.Println("Invalid build tag")
				eUtils.LogInfo(config, "Invalid build tag was found:"+tag+"- exiting...")
				os.Exit(-1)
			}
		}
		data = strings.Replace(data, "${TAG}", tag, -1)
	}

	byteData := []byte(data)
	//Ensure directory has been created
	var newFile *os.File

	if config.OutputMemCache {
		var memFile billy.File
		memCacheLock.Lock()
		if _, err := config.MemFs.Stat(path); errors.Is(err, os.ErrNotExist) {
			if strings.HasPrefix(path, "./") {
				path = strings.TrimLeft(path, "./")
			}
			memFile, err = config.MemFs.Create(path)
			if err != nil {
				eUtils.CheckError(config, err, true)
			}
			memFile.Write(byteData)
			memFile.Close()
			memCacheLock.Unlock()
		} else {
			memCacheLock.Unlock()
			eUtils.CheckError(config, err, true)
		}
	} else {
		dirPath := filepath.Dir(path)
		err := os.MkdirAll(dirPath, os.ModePerm)
		eUtils.CheckError(config, err, true)
		//create new file
		newFile, err = os.Create(path)
		eUtils.CheckError(config, err, true)
		defer newFile.Close()
		//write to file
		_, err = newFile.Write(byteData)
		eUtils.CheckError(config, err, true)
		err = newFile.Sync()
		eUtils.CheckError(config, err, true)
	}
}

func getDirFiles(dir string, endDir string) ([]string, []string) {
	files, err := os.ReadDir(dir)
	filePaths := []string{}
	endPaths := []string{}
	if err != nil {
		//this is a file
		return []string{dir}, []string{endDir}
	}
	for _, file := range files {
		//add this directory to path names
		if dir[len(dir)-1] != '/' {
			dir = dir + "/"
		}

		filePath := dir + file.Name()

		//take off .tmpl extension
		filename := file.Name()
		if strings.HasSuffix(filename, ".DS_Store") {
			continue
		}
		extension := filepath.Ext(filename)
		endPath := ""
		if extension == ".tmpl" {
			name := filename[0 : len(filename)-len(extension)]
			endPath = endDir + "/" + name
		} else {
			endPath = endDir + "/" + filename
		}
		//recurse to next level
		newPaths, newEndPaths := getDirFiles(filePath, endPath)
		filePaths = append(filePaths, newPaths...)
		endPaths = append(endPaths, newEndPaths...)
		//add endings of path names
	}
	return filePaths, endPaths
}
