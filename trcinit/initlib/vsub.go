package initlib

import (
	"encoding/base64"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	eUtils "tierceron/utils"
	helperkv "tierceron/vaulthelper/kv"
)

func DownloadTemplateDirectory(config *eUtils.DriverConfig, mod *helperkv.Modifier, dirName string, logger *log.Logger, templateFilter *string) (error, []string) {

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
							if strings.Contains(path, filterSplit[1]) {
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
				err = os.MkdirAll(dirPath, os.ModePerm)
				if err != nil {
					eUtils.LogErrorMessage(config, "Couldn't make directory: "+dirName+filePath, false)
					continue
				}
				//create new file
				newFile, err := os.Create(dirPath + file + ext + ".tmpl")
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
