package utils

import (
	"errors"
	"fmt"
	"log"
	"strings"

	eUtils "github.com/trimble-oss/tierceron/utils"
	helperkv "github.com/trimble-oss/tierceron/vaulthelper/kv"
)

// ConfigDataStore stores the data needed to configure the specified template files
type ConfigDataStore struct {
	dataMap map[string]interface{}
	Regions []string
}

func (cds *ConfigDataStore) Init(config *eUtils.DriverConfig,
	mod *helperkv.Modifier,
	secretMode bool,
	useDirs bool,
	project string,
	commonPaths []string,
	servicesWanted ...string) error {
	cds.Regions = mod.Regions
	cds.dataMap = make(map[string]interface{})

	var dataPathsFull []string

	if project == "Common" {
		mod.SectionKey = ""
		mod.SectionName = ""
		mod.SectionPath = ""
	}

	if project == "Common" && commonPaths != nil && len(commonPaths) > 0 {
		dataPathsFull = commonPaths
	} else {
		//get paths where the data is stored
		dp, err := GetPathsFromProject(config, mod, []string{project}, servicesWanted)
		if len(dp) > 1 && strings.Contains(dp[len(dp)-1], "!=!") {
			mod.VersionFilter = append(mod.VersionFilter, strings.Split(dp[len(dp)-1], "!=!")[0])
			dp = dp[:len(dp)-1]
		}

		if err != nil {
			eUtils.LogInfo(config, fmt.Sprintf("Uninitialized environment.  Please initialize environment. %v\n", err))
			return err
		}
		dataPathsFull = dp
	}

	dataPaths := []string{}
	for _, fullPath := range dataPathsFull {
		if strings.HasSuffix(fullPath, "/") {
			continue
		} else {
			dataPaths = append(dataPaths, fullPath)
		}
	}
	if len(dataPaths) < len(dataPathsFull)/3 && len(dataPaths) != len(dataPathsFull) {
		eUtils.LogInfo(config, "Unexpected vault pathing.  Dropping optimization.")
		dataPaths = dataPathsFull
	}

	ogKeys := []string{}
	valueMaps := [][]string{}

	if len(dataPaths) == 0 {
		eUtils.LogInfo(config, "No data paths found when initing CDS. \n")
		return errors.New("No data paths found when initing CDS")
	}
	for _, path := range dataPaths {
		//for each path, read the secrets there
		pathParts := strings.Split(path, "/")
		foundWantedService := false
		for i := 0; i < len(servicesWanted); i++ {
			splitService := strings.Split(servicesWanted[i], ".")
			if len(pathParts) >= 2 && (pathParts[2] == servicesWanted[i] || splitService[0] == pathParts[2] || (len(pathParts) >= 4 && pathParts[3] == servicesWanted[i])) {
				foundWantedService = true
				break
			}
		}
		if !foundWantedService {
			continue
		}

		secrets, err := mod.ReadData(path)
		if err != nil {
			return err
		}

		//get the keys and values in secrets
		for key, value := range secrets {
			_, ok := value.(string)
			if !ok {
				//if it's a string, it's not the data we're looking for (we want maps)
				ogKeys = append(ogKeys, strings.Replace(key, ".", "_", -1))
				newVal := value.([]interface{})
				newValues := []string{}
				for _, val := range newVal {
					newValues = append(newValues, val.(string))
				}
				valueMaps = append(valueMaps, newValues)
			} else if !secretMode {
				//add value straight to template
				cds.dataMap[key] = value.(string)
			}
		}
		if useDirs {
			s := strings.Split(path, "/")
			projectDir := s[1]
			serviceDir := s[2]
			fileDir := ""
			if len(s) > 4 {
				i := 3
				for i < len(s) {
					fileDir = fileDir + "/" + s[i]
					i = i + 1
				}
			} else {
				fileDir = s[len(s)-1]
			}
			if len(fileDir) == 0 || len(serviceDir) == 0 || len(projectDir) == 0 {
				continue
			}
			values, _ := mod.ReadData(path)
			valuesScrubbed := map[string]interface{}{}
			// Scrub keys.  Ugly, but does the trick.  Would like to do this differently in the future.
			for k, v := range values {
				valuesScrubbed[strings.Replace(k, ".", "_", -1)] = v
			}
			values = valuesScrubbed
			commonValues := map[string]interface{}{}
			noValueKeys := []string{}

			secretBuckets := map[string]interface{}{}

			// Substitute in values
			for k, v := range values {
				if link, ok := v.([]interface{}); ok {
					bucket := link[0].(string)
					var secretBucket map[string]interface{}
					var ok bool
					if secretBucket, ok = secretBuckets[bucket].(map[string]interface{}); !ok {
						secretBucket, err = mod.ReadData(bucket)
						if err != nil {
							noValueKeys = append(noValueKeys, k)
						} else {
							secretBuckets[bucket] = secretBucket
						}
					}

					newVaultValue, readErr := mod.ReadMapValue(secretBucket, bucket, link[1].(string))
					if link[0].(string) == "super-secrets/Common" {
						commonValues[k] = newVaultValue
					} else {
						if readErr == nil {
							values[k] = newVaultValue
						} else {
							noValueKeys = append(noValueKeys, k)
						}
					}

					// TODO: improve this M*N complexity algorithm.
					for _, region := range mod.Regions {
						regionPath := link[1].(string) + "~" + region
						newVaultValue, readErr := mod.ReadMapValue(secretBucket, bucket, regionPath)
						if readErr == nil {
							values[k+"~"+region] = newVaultValue
						}
					}

				}
			}
			if len(noValueKeys) > 0 {
				for _, noValueKey := range noValueKeys {
					delete(values, noValueKey)
				}
			}

			if len(commonValues) > 0 {
				//not sure about this part with projects structure
				if subDir, ok := cds.dataMap["Common"].(map[string]interface{}); ok {
					subDir[fileDir] = commonValues
				} else if cds.dataMap["Common"] == nil {
					cds.dataMap["Common"] = map[string]interface{}{
						fileDir: commonValues,
					}
				}
				for commonKeyD := range commonValues {
					delete(values, commonKeyD)
				}
			}

			//not sure about this part with projects structure
			if subDir, ok := cds.dataMap[serviceDir].(map[string]interface{}); ok {
				subDir[fileDir] = values
			} else if cds.dataMap[serviceDir] == nil {
				cds.dataMap[serviceDir] = map[string]interface{}{
					fileDir: values,
				}
			}
		} else {
			secretBuckets := map[string]interface{}{}

			for i, valueMap := range valueMaps {
				//these should be [path, key] maps
				if len(valueMap) != 2 {
					return errors.New("value path is not the correct length")
				} else {
					//first element is the path
					bucket := valueMap[0]
					if secretMode {
						//get rid of non-secret paths
						dirs := strings.Split(bucket, "/")
						if dirs[0] == "super-secrets" {
							key := valueMap[1]
							var secretBucket map[string]interface{}
							var ok bool
							if secretBucket, ok = secretBuckets[bucket].(map[string]interface{}); !ok {
								secretBucket, err = mod.ReadData(bucket)
								if err == nil {
									secretBuckets[bucket] = secretBucket
								}
							}

							value, _ := mod.ReadMapValue(secretBucket, bucket, key)

							//put the original key with the correct value
							cds.dataMap[ogKeys[i]] = value
						}
					} else {
						//second element is the key
						key := valueMap[1]
						var secretBucket map[string]interface{}
						var ok bool
						if secretBucket, ok = secretBuckets[bucket].(map[string]interface{}); !ok {
							secretBucket, err = mod.ReadData(bucket)
							if err == nil {
								secretBuckets[bucket] = secretBucket
							}
						}

						value, _ := mod.ReadMapValue(secretBucket, bucket, key)
						//put the original key with the correct value
						cds.dataMap[ogKeys[i]] = value
					}
				}
			}
		}

	}
	if cds.dataMap == nil {
		config.Log.Println("Failed to populate cds")
	}
	return nil
}

func (cds *ConfigDataStore) InitTemplateVersionData(config *eUtils.DriverConfig, mod *helperkv.Modifier, useDirs bool, project string, file string, servicesWanted ...string) (map[string]interface{}, error) {
	cds.Regions = mod.Regions
	cds.dataMap = make(map[string]interface{})
	//get paths where the data is stored
	dataPathsFull, err := GetPathsFromProject(config, mod, []string{project}, servicesWanted)
	if len(dataPathsFull) > 0 && strings.Contains(dataPathsFull[len(dataPathsFull)-1], "!=!") {
		dataPathsFull = dataPathsFull[:len(dataPathsFull)-1]
	}

	if err != nil {
		return nil, eUtils.LogAndSafeExit(config, fmt.Sprintf("Uninitialized environment.  Please initialize environment. %v\n", err), 1)
	}

	dataPaths := dataPathsFull

	var deeperData map[string]interface{}
	data := make(map[string]interface{})
	for _, path := range dataPaths {
		//for each path, read the secrets there
		pathParts := strings.Split(path, "/")
		foundWantedService := false
		for i := 0; i < len(servicesWanted); i++ {
			splitService := strings.Split(servicesWanted[i], ".")
			if len(pathParts) >= 2 && (pathParts[2] == servicesWanted[i] || splitService[0] == pathParts[2] || (len(pathParts) >= 4 && pathParts[3] == servicesWanted[i])) {
				foundWantedService = true
				break
			}
		}

		if !foundWantedService {
			continue
		}

		data, err = mod.ReadVersionMetadata(path, config.Log)
		if data == nil {
			deeperData, _ = mod.ReadVersionMetadata(path+"template-file", config.Log)
		}
		if err != nil || deeperData == nil && data == nil {
			eUtils.LogInfo(config, fmt.Sprintf("Couldn't read version data for %s\n", path))
		}
	}

	if deeperData != nil && data == nil {
		return deeperData, nil
	} else {
		return data, nil
	}
}

// GetValue Provides data from the vault
func (cds *ConfigDataStore) GetValue(service string, keyPath []string, key string) (string, error) {
	serviceData, ok := cds.dataMap[service]
	if ok {

		configPart, configPartOk := serviceData.(map[string]interface{})
		if configPartOk {
			for _, keyPathPart := range keyPath {
				for configPathKey, configPathValues := range configPart {
					if configPathKey == keyPathPart {
						configPart, configPartOk = configPathValues.(map[string]interface{})
						break
					} else {
						configPartOk = false
					}
				}
				if !configPartOk {
					break
				}
			}
			if configPartOk && configPart != nil {
				configValue, okValue := configPart[key]
				if okValue {
					resultValue, okResultValue := configValue.(string)
					if okResultValue {
						return resultValue, nil
					} else {
						return "", errors.New("value not found in store")
					}
				}
			} else {
				// Try nested algorithm.
				keyPathKey := "/" + strings.Join(keyPath, "/")
				for configPathKey, configPathValues := range configPart {
					if configPathKey == keyPathKey {
						configPart, configPartOk = configPathValues.(map[string]interface{})

						if configPartOk {
							configValue, okValue := configPart[key]
							if okValue {
								resultValue, okResultValue := configValue.(string)
								if okResultValue {
									return resultValue, nil
								} else {
									return "", errors.New("value not found in store")
								}
							}
						}
					}
				}
			}
		}
	}
	return "", errors.New("value not found in store")

}

// GetConfigValues gets a set of configuration values for a service from the data store.
func (cds *ConfigDataStore) GetConfigValues(service string, config string) (map[string]interface{}, bool) {
	if serviceValues, okServiceValues := cds.dataMap[service].(map[string]interface{}); okServiceValues {
		if values, okServiceConfig := serviceValues[config].(map[string]interface{}); okServiceConfig {
			return values, true
		}
	}
	return nil, false
}

// GetConfigValue gets an invididual configuration value for a service from the data store.
func (cds *ConfigDataStore) GetConfigValue(service string, config string, key string) (string, bool) {
	key = strings.Replace(key, ".", "_", -1)
	if serviceValues, okServiceValues := cds.dataMap[service].(map[string]interface{}); okServiceValues {
		if values, okServiceConfig := serviceValues[config].(map[string]interface{}); okServiceConfig {
			if value, okValue := values[key]; okValue {
				if v, okType := value.(string); okType {
					return v, true
				}
			}
		}
	}
	return "", false
}

func GetPathsFromProject(config *eUtils.DriverConfig, mod *helperkv.Modifier, projects []string, services []string) ([]string, error) {
	//setup for getPaths
	if len(config.DynamicPathFilter) > 0 && !config.WantCerts && mod.TemplatePath != "" {
		pathErr := verifyTemplatePath(mod, config.Log)
		if pathErr != nil {
			return nil, pathErr
		}
		return []string{mod.TemplatePath}, nil
	}

	paths := []string{}
	var err error

	if mod.SecretDictionary == nil {
		mod.SecretDictionary, err = mod.List("templates", config.Log)
	}
	secrets := mod.SecretDictionary
	var innerService string
	if err != nil {
		return nil, err
	} else if secrets != nil {
		availProjects := secrets.Data["keys"].([]interface{})
		//if projects empty, use all available projects
		if len(projects) > 0 {
			projectsUsed := []interface{}{}
			for _, project := range projects {
				project = project + "/"
				projectAvailable := false
				for _, availProject := range availProjects { //Look for project in top path
					if projectAvailable {
						break
					}
					if project == availProject.(string) {
						projectsUsed = append(projectsUsed, availProject)
						projectAvailable = true
					}
				}

				if !projectAvailable { //If project not found, search one path deeper
					for _, availProject := range availProjects {
						innerPathList, err := mod.List("templates/"+availProject.(string), config.Log) //Looks for services one path deeper
						if err != nil {
							eUtils.LogInfo(config, "Unable to read into nested template path: "+err.Error())
						}
						if innerPathList == nil || availProject.(string) == "Common/" {
							continue
						}
						innerPaths := innerPathList.Data["keys"].([]interface{})
						for _, innerPath := range innerPaths {
							if projectAvailable {
								break
							}
							if project == innerPath.(string) {
								innerPath = availProject.(string) + innerPath.(string)
								innerService = "!=!" + innerPath.(string) //Pass project back somehow?
								projectsUsed = append(projectsUsed, innerPath)
								projectAvailable = true
							}
						}
					}
				}
				if !projectAvailable {
					if !projectAvailable {
						if len(projects) > 1 || project != "Common/" {
							eUtils.LogInfo(config, project+" is not an available project. No values found.")
						}
					}
				}
			}
			availProjects = projectsUsed
		}
		var pathErr error
		if !config.WantCerts && mod.TemplatePath != "" {
			pathErr = verifyTemplatePath(mod, config.Log)
			if pathErr != nil {
				return nil, pathErr
			}
			paths = append(paths, mod.TemplatePath)
		} else {
			// Not provided template, so look it up.
			for _, project := range availProjects {
				if !config.WantCerts && len(services) > 0 {
					for _, service := range services {
						mod.ProjectIndex = []string{project.(string)}
						path := "templates/" + project.(string) + service + "/"
						paths, pathErr = getPaths(config, mod, path, paths, false)
						//don't add on to paths until you're sure it's an END path
						if pathErr != nil {
							return nil, pathErr
						}
					}

				} else {
					mod.ProjectIndex = []string{project.(string)}
					path := "templates/" + project.(string)
					paths, pathErr = getPaths(config, mod, path, paths, false)
					//don't add on to paths until you're sure it's an END path
					if pathErr != nil {
						return nil, pathErr
					}
				}
			}
		}

		if strings.HasPrefix(innerService, "!=!") {
			paths = append(paths, innerService)
		}
		//paths = getPaths(mod, availProjects, paths)
		if paths == nil {
			return nil, errors.New("no available projects found")
		}
		return paths, err
	} else {
		return nil, errors.New("no paths found from templates engine")
	}
}

func verifyTemplatePath(mod *helperkv.Modifier, logger *log.Logger) error {
	secrets, err := mod.List(mod.TemplatePath, logger)
	if err != nil {
		return err
	} else if secrets != nil {
		//add paths
		slicey := secrets.Data["keys"].([]interface{})
		if len(slicey) == 1 && slicey[0].(string) == "template-file" {
			return nil
		}
	}
	return fmt.Errorf("Template not found in vault: %s", mod.TemplatePath)
}

func getPaths(config *eUtils.DriverConfig, mod *helperkv.Modifier, pathName string, pathList []string, isDir bool) ([]string, error) {
	secrets, err := mod.List(pathName, config.Log)
	if err != nil {
		return nil, err
	} else if secrets != nil {
		//add paths
		slicey := secrets.Data["keys"].([]interface{})
		if len(slicey) == 1 && slicey[0].(string) == "template-file" {
			pathList = append(pathList, pathName)
			if isDir {
				pathList = append(pathList, strings.TrimRight(pathName, "/"))
			}
		} else {
			dirMap := map[string]bool{}

			for _, s := range slicey {
				if strings.HasSuffix(s.(string), "/") {
					dirMap[s.(string)] = true
				}
			}
			for _, pathEnd := range slicey {
				if pathEnd == mod.ProjectIndex[0] {
					// Ignore nested project paths.
					eUtils.LogWarningMessage(config, "Nested project name in path.  Skipping: "+pathEnd.(string), false)
					continue
				}
				path := pathName + pathEnd.(string)
				lookAhead, err2 := mod.List(path, config.Log)
				if err2 != nil || lookAhead == nil {
					//don't add on to paths until you're sure it's an END path
					pathList = append(pathList, path)
				} else {
					if !strings.HasSuffix(pathEnd.(string), "/") && dirMap[pathEnd.(string)+"/"] {
						// Deduplicate drilldown.
						continue
					}
					// This recursion is much slower, but used less frequently now.
					pathList, err = getPaths(config, mod, path, pathList, dirMap[pathEnd.(string)])
				}
			}
		}

		return pathList, err
	}
	return pathList, err
}
