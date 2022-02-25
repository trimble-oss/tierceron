package initlib

import (
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"strings"

	"tierceron/utils"
	"tierceron/vaulthelper/kv"
)

func DownloadTemplateDirectory(mod *kv.Modifier, dirName string, logger *log.Logger, templateFilter *string) (error, []string) {

	var filterTemplateSlice []string
	if len(*templateFilter) > 0 {
		filterTemplateSlice = strings.Split(*templateFilter, ",")
	}

	templateList, err := mod.List("templates/")
	if err != nil {
		return err, nil
	}
	for _, templatePath := range templateList.Data {
		for _, projectInterface := range templatePath.([]interface{}) {
			project := projectInterface.(string)
			if len(filterTemplateSlice) > 0 {
				projectFound := false
				for _, filter := range filterTemplateSlice {
					if strings.Contains(filter, "/") { //For Project/Service in filter
						filterSplit := strings.Split(filter, "/")
						if project == filterSplit[0] {
							projectFound = true
						}
					}
					if project == filter {
						projectFound = true
					}
				}
				if !projectFound {
					continue
				}
			}

			allTemplateFilePaths, err1 := mod.GetTemplateFilePaths("templates/" + project)
			if err1 != nil {
				return err1, nil
			}

			for _, path := range allTemplateFilePaths {
				fmt.Println(path)
			}
			//recursively grab paths
			//use paths with ReadData to write to file
		}
	}

	dirs, err := ioutil.ReadDir(dirName)
	if err != nil {
		fmt.Println("Read directory couldn't be completed.")
		return err, nil
	}

	// Parse each subdirectory as a service name
	for _, subDir := range dirs {
		if subDir.IsDir() {
			pathName := dirName + "/" + subDir.Name()
			err, warn := DownloadTemplates(mod, pathName, logger)
			if err != nil || len(warn) > 0 {
				fmt.Printf("Download templates couldn't be completed. %v", err)
				return err, warn
			}
		}
	}
	return nil, nil
}

func DownloadTemplates(mod *kv.Modifier, dirName string, logger *log.Logger) (error, []string) {
	// Open directory
	files, err := ioutil.ReadDir(dirName)
	if err != nil {
		return err, nil
	}

	// Use name of containing directory as the template subdirectory
	splitDir := strings.SplitAfterN(dirName, "/", 2)
	subDir := splitDir[len(splitDir)-1]

	// Parse through files
	for _, file := range files {
		// Extract extension and name
		if file.IsDir() { // Recurse folders
			err, warn := DownloadTemplates(mod, dirName+"/"+file.Name(), logger)
			if err != nil || len(warn) > 0 {
				return err, warn
			}
			continue
		}
		ext := filepath.Ext(file.Name())
		name := file.Name()
		name = name[0 : len(name)-len(ext)] // Truncate extension

		if ext == ".tmpl" { // Only upload template files
			fmt.Printf("Found template file %s for %s\n", file.Name(), mod.Env)
			logger.Println(fmt.Sprintf("Found template file %s for %s", file.Name(), mod.Env))

			// Seperate name and extension one more time for saving to vault
			ext = filepath.Ext(name)
			name = name[0 : len(name)-len(ext)]
			logger.Printf("dirName: %s\n", dirName)
			logger.Printf("file name: %s\n", file.Name())
			// Extract values
			extractedValues, err := utils.Parse(dirName+"/"+file.Name(), subDir, name)
			if err != nil {
				return err, nil
			}

			// Open file
			f, err := os.Open(dirName + "/" + file.Name())
			if err != nil {
				return err, nil
			}

			// Read the file
			fileBytes := make([]byte, file.Size())
			_, err = f.Read(fileBytes)
			if err != nil {
				return err, nil
			}

			// Construct template path for vault
			templatePath := "templates/" + subDir + "/" + name + "/template-file"
			logger.Printf("\tDownloading template to path:\t%s\n", templatePath)

			// Construct value path for vault
			valuePath := "values/" + subDir + "/" + name
			logger.Printf("\tDownloading values to path:\t%s\n", valuePath)

			// Write templates to vault and output errors/warnings
			warn, err := mod.Write(templatePath, map[string]interface{}{"data": fileBytes, "ext": ext})
			if err != nil || len(warn) > 0 {
				return err, warn
			}

			// Write values to vault and output any errors/warnings
			warn, err = mod.Write(valuePath, extractedValues)
			if err != nil || len(warn) > 0 {
				return err, warn
			}
		} else {
			logger.Printf("\tSkippping template (templates must end in .tmpl):\t%s\n", file.Name())
		}
	}
	return nil, nil
}
