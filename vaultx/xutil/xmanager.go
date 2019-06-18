package xutil

import (
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"strings"

	"bitbucket.org/dexterchaney/whoville/utils"
)

//TODO
// Manage configures the templates in vault_templates and writes them to vaultx
func Manage(startDir string, endDir string, seed string, logger *log.Logger) {
	//get files from directory
	templatePaths, endPaths := getDirFiles(startDir, endDir)
	//configure each template in directory
	fmt.Println(templatePaths)
	for i, templatePath := range templatePaths {

		//check for template_files directory here
		s := strings.Split(templatePath, "/")
		//figure out which path is vault_templates
		dirIndex := -1
		for j, piece := range s {
			if piece == "vault_templates" {
				dirIndex = j
			}
		}
		if dirIndex != -1 {
			configuredSeed := ToSeed(templatePath, endPaths[i], secretMode, s[dirIndex+1], s[dirIndex+2])
			writeToFile(configuredSeed, endPaths[i])
		} else {
			//assume the starting directory was vault_templates
			configuredSeed := ToSeed(templatePath, endPaths[i], secretMode, s[1], s[2])
			writeToFile(configuredSeed, endPaths[i])
		}
	}
	//print that we're done
	//endDir = strings.Split(endDir, "/")[0]
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
		filePath := dir + "/" + file.Name()
		//take off .tmpl extension
		filename := file.Name()
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
