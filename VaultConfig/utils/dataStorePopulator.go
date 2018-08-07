package utils

import (
	"errors"
	"fmt"
	"strings"

	"bitbucket.org/dexterchaney/whoville/vault-helper/kv"
)

//ConfigDataStore stores the data needed to configure the specified template files
type ConfigDataStore struct {
	dataMap map[string]interface{}
}

func (cds *ConfigDataStore) init(mod *kv.Modifier, secretMode bool, useDirs bool, project string, servicesWanted ...string) {
	cds.dataMap = make(map[string]interface{})
	//get paths where the data is stored
	dataPaths, err := getPathsFromProject(mod, project)

	if err != nil {
		panic(err)
	}
	ogKeys := []string{}
	valueMaps := [][]string{}
	for _, path := range dataPaths {
		//for each path, read the secrets there

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
			fmt.Println("values")
			fmt.Println(values)
			// Substitute in secrets
			for k, v := range values {
				if link, ok := v.([]interface{}); ok {
					values[k], _ = mod.ReadValue(link[0].(string), link[1].(string))
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
			for i, valueMap := range valueMaps {
				//these should be [path, key] maps
				if len(valueMap) != 2 {
					panic(errors.New("value path is not the correct length"))
				} else {
					//first element is the path
					secretPath := valueMap[0]
					if secretMode {
						//get rid of non-secret paths
						dirs := strings.Split(secretPath, "/")
						if dirs[0] == "super-secrets" {
							key := valueMap[1]
							value, _ := mod.ReadValue(secretPath, key)

							//put the original key with the correct value
							cds.dataMap[ogKeys[i]] = value

						}
					} else {
						//second element is the key
						key := valueMap[1]
						value, _ := mod.ReadValue(secretPath, key)
						//put the original key with the correct value
						cds.dataMap[ogKeys[i]] = value
					}
				}
			}
		}

	}
}

func getPathsFromProject(mod *kv.Modifier, projects ...string) ([]string, error) {
	//setup for getPaths
	paths := []string{}
	secrets, err := mod.List("templates")
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
