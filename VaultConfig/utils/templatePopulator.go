package utils

import "C"

import (
	"bytes"
	"errors"
	"fmt"
	"html/template"
	"io/ioutil"
	"strings"

	"bitbucket.org/dexterchaney/whoville/utils"
	"bitbucket.org/dexterchaney/whoville/vault-helper/kv"
)

//ConfigTemplate takes a modifier object, a file path where the template is located, the target path, and two maps of data to populate the template with.
//It configures the template and writes it to the specified file path.
func ConfigTemplate(modifier *kv.Modifier, emptyFilePath string, configuredFilePath string, secretMode bool, servicesWanted ...string) string {
	//get template
	emptyTemplate, err := ioutil.ReadFile(emptyFilePath)
	template := string(emptyTemplate)
	utils.CheckError(err)
	//populate template
	template = PopulateTemplate(template, modifier, secretMode, servicesWanted...)
	return template
}

//export ConfigTemplateLib
func ConfigTemplateLib(token string, address string, certPath string, env string, templatePath string, configuredFilePath string, secretMode bool, servicesWanted []string) string {
	mod, err := kv.NewModifier(token, address, certPath)
	mod.Env = env
	if err != nil {
		panic(err)
	}
	return ConfigTemplate(mod, templatePath, configuredFilePath, secretMode, servicesWanted...)
}

//PopulateTemplate takes an empty template and a modifier.
//It populates the template and returns it in a string.
func PopulateTemplate(emptyTemplate string, modifier *kv.Modifier, secretMode bool, servicesWanted ...string) string {
	cds := new(ConfigDataStore)
	cds.init(modifier, secretMode, servicesWanted...)
	//create new template from template string
	t := template.New("template")
	t, err := t.Parse(emptyTemplate)
	if err != nil {
		panic(err)
	}
	var doc bytes.Buffer
	//configure the template
	err = t.Execute(&doc, cds.dataMap)
	str := doc.String()
	if err != nil {
		panic(err)
	}
	return str
}

//ConfigDataStore stores the data needed to configure the specified template files
type ConfigDataStore struct {
	dataMap map[string]interface{}
}

func (cds *ConfigDataStore) init(mod *kv.Modifier, secretMode bool, servicesWanted ...string) {
	cds.dataMap = make(map[string]interface{})
	dataPaths, err := getPathsFromService(mod, servicesWanted...)
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
				ogKeys = append(ogKeys, key)
				newVal := value.([]interface{})
				newValues := []string{}
				for _, val := range newVal {
					newValues = append(newValues, val.(string))
				}
				valueMaps = append(valueMaps, newValues)
			}
		}
		for i, valueMap := range valueMaps {
			//these should be [path, key] maps
			if len(valueMap) != 2 {
				panic(errors.New("value path is not the correct length"))
			} else {
				//first element is the path
				path := valueMap[0]
				if secretMode {
					//get rid of non-secret paths
					dirs := strings.Split(path, "/")
					if dirs[0] == "super-secrets" {
						key := valueMap[1]
						value := mod.ReadValue(path, key)
						//put the original key with the correct value
						cds.dataMap[ogKeys[i]] = value
					}
				} else {
					//second element is the key
					key := valueMap[1]
					value := mod.ReadValue(path, key)
					//put the original key with the correct value
					cds.dataMap[ogKeys[i]] = value
				}
			}
		}

	}
}
func getPathsFromService(mod *kv.Modifier, services ...string) ([]string, error) {
	//setup for getPaths
	paths := []string{}
	secrets, err := mod.List("templates")
	if err != nil {
		return nil, err
	} else if secrets != nil {
		availServices := secrets.Data["keys"].([]interface{})
		//if services empty, use all available services
		if len(services) > 0 {
			servicesUsed := []interface{}{}
			for _, service := range services {
				serviceAvailable := false
				for _, availService := range availServices {
					if service == availService.(string) {
						servicesUsed = append(servicesUsed, availService)
						serviceAvailable = true
					}
				}
				if !serviceAvailable {
					fmt.Println(service + " is not an available service. No values found.")
				}
			}
			availServices = servicesUsed
		}
		for _, service := range availServices {
			path := "templates/" + service.(interface{}).(string)
			paths = getPaths(mod, path, paths)
			//don't add on to paths until you're sure it's an END path
		}

		//paths = getPaths(mod, availServices, paths)
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
			path := pathName + "/" + pathEnd.(string)
			pathList = append(pathList, path)
			//don't add on to paths until you're sure it's an END path
		}
		return pathList
	}
	return pathList
}

func main() {}
