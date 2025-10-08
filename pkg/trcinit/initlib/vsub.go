package initlib

import (
	"encoding/base64"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	eUtils "github.com/trimble-oss/tierceron/pkg/utils"
	"github.com/trimble-oss/tierceron/pkg/utils/config"
	helperkv "github.com/trimble-oss/tierceron/pkg/vaulthelper/kv"
)

func DownloadTemplates(driverConfig *config.DriverConfig, mod *helperkv.Modifier, dirName string, logger *log.Logger, templatePaths *string) {
	var filterTemplatePathSlice []string
	if len(*templatePaths) > 0 {
		filterTemplatePathSlice = strings.Split(*templatePaths, ",")
	}
	for _, filterTemplatePath := range filterTemplatePathSlice {
		path := filterTemplatePath
		ext := ""
		if strings.Contains(path, "Common") {
			if !strings.Contains(path, ".") {
				eUtils.LogErrorMessage(driverConfig.CoreConfig, "Expecting file extension with filepath", false)
				fmt.Println("Expecting file extension with filepath: " + path)
				if eUtils.IsWindows() {
					fmt.Println("Make sure filepath is in quotes.")
				}
			}
		}
		if !strings.HasSuffix(filterTemplatePath, "/") {
			path = filterTemplatePath + "/"
		}
		tfMap, err := mod.ReadData(fmt.Sprintf("templates/%stemplate-file", path)) //Grab extension of file
		if err != nil {
			eUtils.LogErrorMessage(driverConfig.CoreConfig, "Skipping template: "+path+" Error: "+err.Error(), false)
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
			eUtils.LogInfo(driverConfig.CoreConfig, fmt.Sprintf("No data found for: %s template-file", path))
			continue
		}
		templateBytes, decodeErr := base64.StdEncoding.DecodeString(data)
		if decodeErr != nil {
			eUtils.LogErrorMessage(driverConfig.CoreConfig, "Couldn't decode data for: "+path+"template-file", false)
			continue
		}
		//Ensure directory has been created
		filePath := strings.Trim(path, "/")
		templateAndFilePath := fmt.Sprintf("%s/%s", dirName, filePath)
		dirPath := filepath.Dir(templateAndFilePath)
		file := filepath.Base(templateAndFilePath)
		driverConfig.CoreConfig.Log.Printf("templateDir: %s\n", templateAndFilePath)
		driverConfig.CoreConfig.Log.Printf("Dir: %s\n", dirPath)
		driverConfig.CoreConfig.Log.Printf("file: %s\n", file)
		templateFile := fmt.Sprintf("%s/%s%s.tmpl", dirPath, file, ext)

		if driverConfig.SubOutputMemCache {
			driverConfig.MemFs.WriteToMemFile(driverConfig.CoreConfig, &templateBytes, templateFile)
			eUtils.LogInfo(driverConfig.CoreConfig, fmt.Sprintf("Billy File has been written to %s\n", templateFile))
		} else {
			err = os.MkdirAll(dirPath, os.ModePerm)
			if err != nil {
				eUtils.LogErrorMessage(driverConfig.CoreConfig, "Couldn't make directory: "+dirName+filePath, false)
				continue
			}
			if eUtils.IsWindows() {
				templateFile = fmt.Sprintf("%s\\%s%s.tmpl", dirPath, file, ext)
			}
			newFile, err := os.Create(templateFile)
			if err != nil {
				eUtils.LogErrorMessage(driverConfig.CoreConfig, fmt.Sprintf("Couldn't create file: %s", templateFile), false)
				continue
			}
			defer newFile.Close()
			_, err = newFile.Write(templateBytes)
			if err != nil {
				eUtils.LogErrorMessage(driverConfig.CoreConfig, fmt.Sprintf("Couldn't write file: %s", templateFile), false)
				continue
			}
			err = newFile.Sync()
			if err != nil {
				eUtils.LogErrorMessage(driverConfig.CoreConfig, fmt.Sprintf("Couldn't sync file: %s", templateFile), false)
				continue
			}
			eUtils.LogInfo(driverConfig.CoreConfig, fmt.Sprintf("File has been written to %s\n", templateFile))
		}
	}
}

func DownloadTemplateDirectory(driverConfig *config.DriverConfig, mod *helperkv.Modifier, dirName string, logger *log.Logger, templateFilter *string) ([]string, error) {
	var filterTemplateSlice []string
	if len(*templateFilter) > 0 {
		filterTemplateSlice = strings.Split(*templateFilter, ",")
	}

	for _, filter := range filterTemplateSlice {
		if filter == "" {
			continue
		}
		allTemplateFilePaths, err1 := mod.GetTemplateFilePaths(fmt.Sprintf("templates/%s/", filter), driverConfig.CoreConfig.Log)
		if err1 != nil {
			eUtils.LogErrorMessage(driverConfig.CoreConfig, "Couldn't read into paths under templates/"+filter+"/", false)
			continue
		}
		allTemplateFilePaths = eUtils.RemoveDuplicates(allTemplateFilePaths)

		for _, path := range allTemplateFilePaths {
			if !strings.HasSuffix(path, "template-file") && !strings.HasSuffix(path, "/") {
				continue
			}
			lookupPath := path
			if !strings.HasSuffix(path, "template-file") {
				lookupPath = lookupPath + "template-file"
			}
			ext := ""
			tfMap, err := mod.ReadData(lookupPath) //Grab extension of file
			if err != nil {
				eUtils.LogErrorMessage(driverConfig.CoreConfig, "Skipping template: "+path+" Error: "+err.Error(), false)
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
				eUtils.LogInfo(driverConfig.CoreConfig, fmt.Sprintf("No data found for: %s template-file", path))
				continue
			}
			templateBytes, decodeErr := base64.StdEncoding.DecodeString(data)
			if decodeErr != nil {
				eUtils.LogErrorMessage(driverConfig.CoreConfig, "Couldn't decode data for: "+path+"template-file", false)
				continue
			}
			//Ensure directory has been created
			filePath := strings.TrimSuffix(path, "/")
			filePath = filePath[strings.Index(filePath, "/"):]
			file := filePath[strings.LastIndex(filePath, "/"):]
			dirPath := filepath.Dir(dirName + filePath)
			if err != nil {
				eUtils.LogErrorMessage(driverConfig.CoreConfig, "Couldn't make directory: "+dirName+filePath, false)
				continue
			}
			if file == "/template-file" {
				file = ""
			}

			templateFile := fmt.Sprintf("%s%s%s.tmpl", dirPath, file, ext)

			if driverConfig.SubOutputMemCache {
				driverConfig.MemFs.WriteToMemFile(driverConfig.CoreConfig, &templateBytes, templateFile)
				eUtils.LogInfo(driverConfig.CoreConfig, fmt.Sprintf("Billy File has been written to %s.tmpl\n", dirPath+file+ext))
			} else {
				err = os.MkdirAll(dirPath, os.ModePerm)
				if err != nil {
					eUtils.LogErrorMessage(driverConfig.CoreConfig, "Couldn't make directory components: "+dirPath, false)
					continue
				}
				//create new file
				newFile, err := os.Create(templateFile)

				if err != nil {
					eUtils.LogErrorMessage(driverConfig.CoreConfig, "Couldn't create file: "+dirPath+file+ext+".tmpl", false)
					continue
				}
				defer newFile.Close()
				//write to file
				_, err = newFile.Write(templateBytes)
				if err != nil {
					eUtils.LogErrorMessage(driverConfig.CoreConfig, "Couldn't write file: "+dirPath+file+ext+".tmpl", false)
					continue
				}
				err = newFile.Sync()
				if err != nil {
					eUtils.LogErrorMessage(driverConfig.CoreConfig, "Couldn't sync file: "+dirPath+file+ext+".tmpl", false)
					continue
				}
				eUtils.LogInfo(driverConfig.CoreConfig, fmt.Sprintf("File has been written to %s.tmpl\n", dirPath+file+ext))
			}
		}
	}

	return nil, nil
}
