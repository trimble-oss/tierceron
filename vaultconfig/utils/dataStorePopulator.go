package utils

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"Vault.Whoville/vaulthelper/kv"
)

//ConfigDataStore stores the data needed to configure the specified template files
type ConfigDataStore struct {
	dataMap map[string]interface{}
	Regions []string
}

func (cds *ConfigDataStore) Init(mod *kv.Modifier, secretMode bool, useDirs bool, project string, servicesWanted ...string) {
	cds.Regions = mod.Regions
	cds.dataMap = make(map[string]interface{})
	//get paths where the data is stored
	dataPathsFull, err := getPathsFromProject(mod, project)

	if err != nil {
		fmt.Printf("Uninitialized environment.  Please initialize environment. %v\n", err)
		os.Exit(1)
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
		fmt.Println("Unexpected vault pathing.  Dropping optimization.")
		dataPaths = dataPathsFull
	}

	ogKeys := []string{}
	valueMaps := [][]string{}

	for _, path := range dataPaths {
		//for each path, read the secrets there
		pathParts := strings.Split(path, "/")
		foundWantedService := false
		for i := 0; i < len(servicesWanted); i++ {
			if servicesWanted[i] == pathParts[2] {
				foundWantedService = true
				break
			}
		}
		if !foundWantedService {
			continue
		}

		secrets, err := mod.ReadData(path)
		if err != nil {
			panic(err)
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
			} else if secretMode == false {
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
				for commonKeyD, _ := range commonValues {
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
					panic(errors.New("value path is not the correct length"))
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
}

func (cds *ConfigDataStore) InitTemplateVersionData(mod *kv.Modifier, secretMode bool, useDirs bool, project string, file string, servicesWanted ...string) map[string]interface{} {
	cds.Regions = mod.Regions
	cds.dataMap = make(map[string]interface{})
	//get paths where the data is stored
	dataPathsFull, err := getPathsFromProject(mod, project)

	if err != nil {
		fmt.Printf("Uninitialized environment.  Please initialize environment. %v\n", err)
		os.Exit(1)
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
		fmt.Println("Unexpected vault pathing.  Dropping optimization.")
		dataPaths = dataPathsFull
	}

	data := make(map[string]interface{})
	for _, path := range dataPaths {
		//for each path, read the secrets there
		pathParts := strings.Split(path, "/")
		foundWantedService := false
		for i := 0; i < len(servicesWanted); i++ {
			if servicesWanted[i] == pathParts[2] {
				foundWantedService = true
				break
			}
		}
		if !foundWantedService {
			continue
		}

		data, err = mod.ReadTemplateVersions(path)
		if err != nil {
			fmt.Printf("Couldn't read version data for %s\n", path)
		}

	}
	return data
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

//GetConfigValues gets a set of configuration values for a service from the data store.
func (cds *ConfigDataStore) GetConfigValues(service string, config string) (map[string]interface{}, bool) {
	if serviceValues, okServiceValues := cds.dataMap[service].(map[string]interface{}); okServiceValues {
		if values, okServiceConfig := serviceValues[config].(map[string]interface{}); okServiceConfig {
			return values, true
		}
	}
	return nil, false
}

//GetConfigValue gets an invididual configuration value for a service from the data store.
func (cds *ConfigDataStore) GetConfigValue(service string, config string, key string) (string, bool) {
	if strings.Index(key, ".") >= 0 {
		key = strings.Replace(key, ".", "_", -1)
	}
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

func getPathsFromProject(mod *kv.Modifier, projects ...string) ([]string, error) {
	//setup for getPaths
	paths := []string{}
	var err error
	if mod.SecretDictionary == nil {
		mod.SecretDictionary, err = mod.List("templates")
	}
	secrets := mod.SecretDictionary

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
				for _, availProject := range availProjects {
					if project == availProject.(string) {
						projectsUsed = append(projectsUsed, availProject)
						projectAvailable = true
					}
				}
				if !projectAvailable {
					fmt.Println(project + " is not an available project. No values found.")
				}
			}
			availProjects = projectsUsed
		}
		for _, project := range availProjects {
			path := "templates/" + project.(interface{}).(string)
			paths = getPaths(mod, path, paths)
			//don't add on to paths until you're sure it's an END path
		}

		//paths = getPaths(mod, availProjects, paths)
		return paths, err
	} else {
		return nil, errors.New("no paths found from templates engine")
	}
}
func getPaths(mod *kv.Modifier, pathName string, pathList []string) []string {
	secrets, err := mod.List(pathName)
	if err != nil {
		panic(err)
	} else if secrets != nil {
		//add paths
		slicey := secrets.Data["keys"].([]interface{})
		for _, pathEnd := range slicey {
			path := pathName + pathEnd.(string)
			if pathEnd.(string) == "template-file" {
				pathList = append(pathList, pathName)
				break
			}
			lookAhead, err2 := mod.List(path)
			if err2 != nil || lookAhead == nil {
				//don't add on to paths until you're sure it's an END path
				pathList = append(pathList, path)
			} else {
				pathList = getPaths(mod, path, pathList)
			}
		}
		return pathList
	}
	return pathList
}
