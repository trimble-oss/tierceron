package xutil

import (
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"reflect"

	"bitbucket.org/dexterchaney/whoville/utils"

	//"gopkg.in/yaml.v2"
	"github.com/davecgh/go-spew/spew"
	"github.com/nirekin/yaml"
)

// Manage configures the templates in vault_templates and writes them to vaultx
func Manage(startDir string, endDir string, seed string, logger *log.Logger) {

	// TODO - possibly delete later
	//sliceSections := []interface{}{[]interface{}{}, []map[string]map[string]map[string]string{}, []map[string]map[string]map[string]string{}, []int{}}

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
	maxDepth := -1

	// Get files from directory
	templatePaths, endPaths := getDirFiles(startDir, endDir)

	// Configure each template in directory
	for _, templatePath := range templatePaths {
		interfaceTemplateSection, valueSection, secretSection, templateDepth := ToSeed(templatePath, logger)
		if templateDepth > maxDepth {
			maxDepth = templateDepth
			//templateCombinedSection = interfaceTemplateSection
		}

		// Append new sections to propper slices
		sliceTemplateSection = append(sliceTemplateSection, interfaceTemplateSection)
		sliceValueSection = append(sliceValueSection, valueSection)
		sliceSecretSection = append(sliceSecretSection, secretSection)
	}

	// Combine values of slice
	combineSection(sliceTemplateSection, maxDepth, templateCombinedSection)
	combineSection(sliceValueSection, -1, valueCombinedSection)
	combineSection(sliceSecretSection, -1, secretCombinedSection)

	// Create seed file structure
	template, errT := yaml.Marshal(templateCombinedSection)
	value, errV := yaml.Marshal(valueCombinedSection)
	secret, errS := yaml.Marshal(secretCombinedSection)

	if errT != nil {
		fmt.Println(errT)
	}

	if errV != nil {
		fmt.Println(errV)
	}

	if errS != nil {
		fmt.Println(errS)
	}

	seedFile := string(template) + "\n\n\n" + string(value) + "\n\n\n" + string(secret)
	writeToFile(seedFile, endPaths[1]) // TODO: change this later

	// Print that we're done
	fmt.Println("seed created and written to ", endDir)
}

func writeToFile(data string, path string) {
	byteData := []byte(data)
	//Ensure directory has been created
	dirPath := filepath.Dir(path)
	err := os.MkdirAll(dirPath, os.ModePerm)
	utils.CheckError(err, true)
	//create new file
	newFile, err := os.Create(path)
	utils.CheckError(err, true)
	//write to file
	_, err = newFile.Write(byteData)
	utils.CheckError(err, true)
	newFile.Close()
}

func getDirFiles(dir string, endDir string) ([]string, []string) {
	files, err := ioutil.ReadDir(dir)
	filePaths := []string{}
	endPaths := []string{}
	if err != nil {
		//this is a file
		return []string{dir}, []string{endDir}
	}
	for _, file := range files {
		//add this directory to path names
		filename := file.Name()
		extension := filepath.Ext(filename)
		filePath := dir + file.Name()
		if extension == "" {
			//if subfolder add /
			filePath += "/"
		}
		//take off .tmpl extension
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

// Combines the values in a slice, creating a singular map from multiple
// Input:
//	- slice to combine
//	- template slice to combine
//	- depth of map (-1 for value/secret sections)
func combineSection(sliceSectionInterface interface{}, maxDepth int, combinedSectionInterface interface{}) {

	// Value/secret slice section
	if maxDepth < 0 {
		sliceSection := sliceSectionInterface.([]map[string]map[string]map[string]string)
		combinedSectionImpl := combinedSectionInterface.(map[string]map[string]map[string]string)
		for _, v := range sliceSection {
			for k2, v2 := range v {
				for k3, v3 := range v2 {
					combinedSectionImpl[k2][k3] = map[string]string{}
					for k4, v4 := range v3 {
						combinedSectionImpl[k2][k3][k4] = v4
					}
				}
			}
		}

		combinedSectionInterface = combinedSectionImpl

		// template slice section
	} else {

		//currDepth := 0
		//combinedSectionImpl := reflect.ValueOf(combinedSectionInterface)
		combinedSectionImpl := combinedSectionInterface.(map[string]interface{})

		//combinedSectionImpl := combinedSectionInterface.(map[string]interface{})
		sliceSection := sliceSectionInterface.([]interface{})
		needsInit := false

		for _, v := range sliceSection {
			v1 := reflect.ValueOf(v)

			for _, k2 := range v1.MapKeys() {
				v2 := v1.MapIndex(k2)

				if len(combinedSectionImpl) == 0 {
					needsInit = true
				}

				for _, k3 := range v2.MapKeys() {
					v3 := v2.MapIndex(k3)

					for _, k4 := range v3.MapKeys() {
						v4 := v3.MapIndex(k4)

						for _, k5 := range v4.MapKeys() {

							if needsInit {
								combinedSectionImpl[k2.String()] = map[string]interface{}{}
								t1 := combinedSectionImpl[k2.String()].(map[string]interface{})
								t1[k3.String()] = map[string]interface{}{}
								t2 := t1[k3.String()].(map[string]interface{})
								t2[k4.String()] = map[string]interface{}{}
								t3 := t2[k4.String()].(map[string]interface{})
								t3[k5.String()] = map[string]interface{}{}
								needsInit = false
							}
							v5 := v4.MapIndex(k5)
							//spew.Dump(v4.Interface())
							//combinedSectionDeepMap := combinedSectionInterface.(map[string]map[string]map[string]map[string]interface{})
							combinedSectionShallowMap := combinedSectionInterface.(map[string]interface{})
							combinedSectionDeepMap := reflect.ValueOf(combinedSectionShallowMap[k2.String()])

							for _, jk2 := range combinedSectionDeepMap.MapKeys() {
								jv2 := combinedSectionDeepMap.MapIndex(jk2)
								for _, jk3 := range jv2.MapKeys() {
									jv3 := jv2.MapIndex(jk3)
									for _, jk4 := range jv3.MapKeys() {
										jv4 := jv3.MapIndex(jk4)
										jv4.SetMapIndex(k5, v5)
										//combinedSectionDeepMap = combinedSectionShallowMap[k2.String()].(map[string]map[string]map[string]interface{})
										//combinedSectionDeepMap[k3.String()][k4.String()][k5.String()] = v5.Interface()
									}
								}
							}
						}
					}
				}
			}
		}
		spew.Dump(combinedSectionInterface)
	}
}
