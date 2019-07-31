package xutil

import (
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"strings"

	"bitbucket.org/dexterchaney/whoville/utils"

	//"gopkg.in/yaml.v2"

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
		//fmt.Println(templat)
		//check for template_files directory here
		s := strings.Split(templatePath, "/")
		//figure out which path is vault_templates
		dirIndex := -1
		for j, piece := range s {
			if piece == "vault_templates" {
				dirIndex = j
			}
		}
		//project := ""
		service := ""
		if dirIndex != -1 {
			//project = s[dirIndex+1]
			service = s[dirIndex+2]
		}

		interfaceTemplateSection, valueSection, secretSection, templateDepth := ToSeed(templatePath, logger, service)
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
		sliceSection := sliceSectionInterface.([]interface{})

		for _, v := range sliceSection {
			MergeMaps(combinedSectionInterface, v)
		}
	}
}
