package initlib

import (
	"encoding/base64"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	utils "tierceron/utils"
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

			allTemplateFilePaths, err1 := mod.GetTemplateFilePaths("templates/" + project + "/")
			if err1 != nil {
				return err1, nil
			}

			allTemplateFilePaths = utils.RemoveDuplicates(allTemplateFilePaths)

			for _, path := range allTemplateFilePaths {
				if !strings.HasSuffix(path, "/") {
					continue
				}
				ext := ""
				tfMap, err := mod.ReadData(path + "template-file") //Grab extention of file
				if err != nil {
					return err, nil
				}
				ext = tfMap["ext"].(string)
				data := tfMap["data"].(string)
				if err != nil {
					return err, nil
				}
				templateBytes, decodeErr := base64.StdEncoding.DecodeString(data)
				if decodeErr != nil {
					return decodeErr, nil
				}
				//Ensure directory has been created
				filePath := strings.TrimSuffix(path, "/")
				filePath = filePath[strings.Index(filePath, "/"):]
				file := filePath[strings.LastIndex(filePath, "/"):]
				dirPath := filepath.Dir(dirName + filePath)
				err = os.MkdirAll(dirPath, os.ModePerm)
				if err != nil {
					return err, nil
				}
				//create new file
				newFile, err := os.Create(dirPath + file + ext + ".tmpl")
				if err != nil {
					return err, nil
				}
				//write to file
				_, err = newFile.Write(templateBytes)
				if err != nil {
					return err, nil
				}
				err = newFile.Sync()
				if err != nil {
					return err, nil
				}
				newFile.Close()
				fmt.Println("File has been writen to " + dirPath + file + ext + ".tmpl")
			}
			//Add "template-file" to the end of this path ->
			//recursively grab paths
			//use paths with ReadData to write to file
		}
	}

	return nil, nil
}
