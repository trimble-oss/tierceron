package initlib

import (
	"encoding/base64"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	eUtils "github.com/trimble-oss/tierceron/utils"
	helperkv "github.com/trimble-oss/tierceron/vaulthelper/kv"
)

func DownloadTemplates(config *eUtils.DriverConfig, mod *helperkv.Modifier, dirName string, logger *log.Logger, templatePaths *string) {
	var filterTemplatePathSlice []string
	if len(*templatePaths) > 0 {
		filterTemplatePathSlice = strings.Split(*templatePaths, ",")
	}

	for _, filterTemplatePath := range filterTemplatePathSlice {
		path := filterTemplatePath
		ext := ""
		if !strings.HasSuffix(filterTemplatePath, "/") {
			path = filterTemplatePath + "/"
		}
		tfMap, err := mod.ReadData(fmt.Sprintf("templates/%stemplate-file", path)) //Grab extention of file
		if err != nil {
			eUtils.LogErrorMessage(config, "Skipping template: "+path+" Error: "+err.Error(), false)
			continue
		}
		if _, extOk := tfMap["ext"]; extOk {
			ext = tfMap["ext"].(string)
		}

		var data string
		if _, dataOk := tfMap["data"]; dataOk {
			data = tfMap["data"].(string)
		} else {
			// TODO: In recent run in prod, sub was printing an annoying warning here
			// and yet correct templates seem to have gotten created...
			fmt.Println("No data found for: " + filterTemplatePath + "template-file")
			continue
		}
		templateBytes, decodeErr := base64.StdEncoding.DecodeString(data)
		if decodeErr != nil {
			eUtils.LogErrorMessage(config, "Couldn't decode data for: "+path+"template-file", false)
			continue
		}
		//Ensure directory has been created
		filePath := strings.Trim(path, "/")
		templateAndFilePath := fmt.Sprintf("%s/%s", dirName, filePath)
		dirPath := filepath.Dir(templateAndFilePath)
		file := filepath.Base(templateAndFilePath)
		fmt.Printf("templateDir: %s\n", templateAndFilePath)
		fmt.Printf("Dir: %s\n", dirPath)
		fmt.Printf("file: %s\n", file)
		err = os.MkdirAll(dirPath, os.ModePerm)
		if err != nil {
			eUtils.LogErrorMessage(config, "Couldn't make directory: "+dirName+filePath, false)
			continue
		}
		//create new file
		templateFile := fmt.Sprintf("%s/%s%s.tmpl", dirPath, file, ext)
		newFile, err := os.Create(templateFile)
		if err != nil {
			eUtils.LogErrorMessage(config, fmt.Sprintf("Couldn't create file: %s", templateFile), false)
			continue
		}
		//write to file
		_, err = newFile.Write(templateBytes)
		if err != nil {
			eUtils.LogErrorMessage(config, fmt.Sprintf("Couldn't write file: %s", templateFile), false)
			continue
		}
		err = newFile.Sync()
		if err != nil {
			eUtils.LogErrorMessage(config, fmt.Sprintf("Couldn't sync file: %s", templateFile), false)
			continue
		}
		newFile.Close()
		fmt.Printf("File has been writen to %s\n", templateFile)
	}
}

func DownloadTemplateDirectory(config *eUtils.DriverConfig, mod *helperkv.Modifier, dirName string, logger *log.Logger, templateFilter *string) ([]string, error) {

	var filterTemplateSlice []string
	if len(*templateFilter) > 0 {
		filterTemplateSlice = strings.Split(*templateFilter, ",")
	}

	templateList, err := mod.List("templates/", config.Log)
	if err != nil {
		eUtils.LogErrorMessage(config, "Couldn't read into paths under templates/", true)
	}
	for _, templatePath := range templateList.Data {
		for _, projectInterface := range templatePath.([]interface{}) {
			project := strings.TrimSuffix(projectInterface.(string), "/")
			if len(filterTemplateSlice) > 0 {
				projectFound := false
				for _, filter := range filterTemplateSlice {
					filterSplit := strings.Split(filter, "/")
					if project == filterSplit[0] {
						projectFound = true
					}
					if project == filter {
						projectFound = true
					}
				}
				if !projectFound {
					continue
				}
			}

			allTemplateFilePaths, err1 := mod.GetTemplateFilePaths("templates/"+project+"/", config.Log)
			if err1 != nil {
				eUtils.LogErrorMessage(config, "Couldn't read into paths under templates/"+project+"/", false)
				continue
			}

			var tempFilteredPaths []string
			if len(filterTemplateSlice) > 0 {
				for _, path := range allTemplateFilePaths {
					serviceFound := false
					for _, filter := range filterTemplateSlice {
						filterSplit := strings.Split(filter, "/")
						if len(filterSplit) > 1 {
							filterPath2nd := filterSplit[1]
							if strings.HasSuffix(filter, "/") {
								filterPath2nd = filterPath2nd + "/"
							}
							if strings.Contains(path, filterPath2nd) {
								serviceFound = true
							}
							if serviceFound {
								tempFilteredPaths = append(tempFilteredPaths, path)
							}
						} else {
							tempFilteredPaths = append(tempFilteredPaths, path)
						}
					}
					if len(tempFilteredPaths) > 0 {
						allTemplateFilePaths = tempFilteredPaths
					}
				}
			}

			allTemplateFilePaths = eUtils.RemoveDuplicates(allTemplateFilePaths)
			for _, path := range allTemplateFilePaths {
				if !strings.HasSuffix(path, "/") {
					continue
				}
				ext := ""
				tfMap, err := mod.ReadData(path + "template-file") //Grab extention of file
				if err != nil {
					eUtils.LogErrorMessage(config, "Skipping template: "+path+" Error: "+err.Error(), false)
					continue
				}
				if _, extOk := tfMap["ext"]; extOk {
					ext = tfMap["ext"].(string)
				}

				var data string
				if _, dataOk := tfMap["data"]; dataOk {
					data = tfMap["data"].(string)
				} else {
					// TODO: In recent run in prod, sub was printing an annoying warning here
					// and yet correct templates seem to have gotten created...
					fmt.Println("No data found for: " + path + "template-file")
					continue
				}
				templateBytes, decodeErr := base64.StdEncoding.DecodeString(data)
				if decodeErr != nil {
					eUtils.LogErrorMessage(config, "Couldn't decode data for: "+path+"template-file", false)
					continue
				}
				//Ensure directory has been created
				filePath := strings.TrimSuffix(path, "/")
				filePath = filePath[strings.Index(filePath, "/"):]
				file := filePath[strings.LastIndex(filePath, "/"):]
				dirPath := filepath.Dir(dirName + filePath)
				if err != nil {
					eUtils.LogErrorMessage(config, "Couldn't make directory: "+dirName+filePath, false)
					continue
				}
				err = os.MkdirAll(dirPath, os.ModePerm)
				templateFile := fmt.Sprintf("%s%s%s.tmpl", dirPath, file, ext)
				//create new file
				newFile, err := os.Create(templateFile)

				if err != nil {
					eUtils.LogErrorMessage(config, "Couldn't create file: "+dirPath+file+ext+".tmpl", false)
					continue
				}
				//write to file
				_, err = newFile.Write(templateBytes)
				if err != nil {
					eUtils.LogErrorMessage(config, "Couldn't write file: "+dirPath+file+ext+".tmpl", false)
					continue
				}
				err = newFile.Sync()
				if err != nil {
					eUtils.LogErrorMessage(config, "Couldn't sync file: "+dirPath+file+ext+".tmpl", false)
					continue
				}
				newFile.Close()
				fmt.Println("File has been writen to " + dirPath + file + ext + ".tmpl")
			}
		}
	}

	return nil, nil
}
