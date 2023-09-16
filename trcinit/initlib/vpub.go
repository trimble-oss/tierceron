package initlib

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/trimble-oss/tierceron/buildopts/coreopts"
	eUtils "github.com/trimble-oss/tierceron/utils"
	helperkv "github.com/trimble-oss/tierceron/vaulthelper/kv"
)

func UploadTemplateDirectory(mod *helperkv.Modifier, dirName string, logger *log.Logger) (error, []string) {

	dirs, err := os.ReadDir(dirName)
	if err != nil {
		fmt.Println("Read directory couldn't be completed.")
		return err, nil
	}

	// Parse each subdirectory as a service name
	for _, subDir := range dirs {
		if subDir.IsDir() {
			pathName := dirName + "/" + subDir.Name()
			err, warn := UploadTemplates(mod, pathName, logger)
			if err != nil || len(warn) > 0 {
				fmt.Printf("Upload templates couldn't be completed. %v", err)
				return err, warn
			}
		}
	}
	return nil, nil
}

func UploadTemplates(mod *helperkv.Modifier, dirName string, logger *log.Logger) (error, []string) {
	// Open directory
	files, err := os.ReadDir(dirName)
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
			err, warn := UploadTemplates(mod, dirName+"/"+file.Name(), logger)
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
			extractedValues, err := eUtils.Parse(dirName+"/"+file.Name(), subDir, name)
			if err != nil {
				return err, nil
			}

			// Open file
			f, err := os.Open(dirName + "/" + file.Name())
			if err != nil {
				return err, nil
			}

			// Read the file
			fileInfo, err := file.Info()
			if err != nil {
				return err, nil
			}

			fileBytes := make([]byte, fileInfo.Size())
			_, err = f.Read(fileBytes)
			if err != nil {
				return err, nil
			}

			dirSplit := strings.Split(subDir, "/")
			if len(dirSplit) >= 2 {
				project, _, _, _ := coreopts.FindIndexForService(dirSplit[0], dirSplit[1])
				if project != "" && strings.Contains(string(fileBytes), "{or") {
					fmt.Printf("Cannot have an indexed template with default values for or %s for %s \n", file.Name(), mod.Env)
					return nil, nil
				}
			}

			// Construct template path for vault
			templatePath := "templates/" + subDir + "/" + name + "/template-file"
			logger.Printf("\tUploading template to path:\t%s\n", templatePath)

			// Construct value path for vault
			valuePath := "values/" + subDir + "/" + name
			logger.Printf("\tUploading values to path:\t%s\n", valuePath)

			// Write templates to vault and output errors/warnings
			warn, err := mod.Write(templatePath, map[string]interface{}{"data": fileBytes, "ext": ext}, logger)
			if err != nil || len(warn) > 0 {
				return err, warn
			}

			// Write values to vault and output any errors/warnings
			warn, err = mod.Write(valuePath, extractedValues, logger)
			if err != nil || len(warn) > 0 {
				return err, warn
			}
		} else {
			logger.Printf("\tSkippping template (templates must end in .tmpl):\t%s\n", file.Name())
		}
	}
	return nil, nil
}
