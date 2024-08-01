package initlib

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/trimble-oss/tierceron/buildopts/coreopts"
	"github.com/trimble-oss/tierceron/pkg/core"
	eUtils "github.com/trimble-oss/tierceron/pkg/utils"
	helperkv "github.com/trimble-oss/tierceron/pkg/vaulthelper/kv"
)

func UploadTemplateDirectory(config *core.CoreConfig, mod *helperkv.Modifier, dirName string, templateFilter *string) ([]string, error) {

	dirs, err := os.ReadDir(dirName)
	if err != nil {
		fmt.Println("Read directory couldn't be completed.")
		return nil, err
	}

	// Parse each subdirectory as a service name
	for _, subDir := range dirs {
		if subDir.IsDir() {
			pathName := dirName + "/" + subDir.Name()

			if templateFilter == nil || len(*templateFilter) == 0 || strings.HasPrefix(*templateFilter, subDir.Name()) {
				warn, err := UploadTemplates(config, mod, pathName, templateFilter)
				if err != nil || len(warn) > 0 {
					fmt.Printf("Upload templates couldn't be completed. %v", err)
					return warn, err
				}
			}
		}
	}
	return nil, nil
}

func UploadTemplates(config *core.CoreConfig, mod *helperkv.Modifier, dirName string, templateFilter *string) ([]string, error) {
	// Open directory
	files, err := os.ReadDir(dirName)
	if err != nil {
		return nil, err
	}

	// Use name of containing directory as the template subdirectory
	splitDir := strings.SplitAfterN(dirName, "/", 2)
	subDir := splitDir[len(splitDir)-1]

	// Parse through files
	for _, file := range files {
		// Extract extension and name
		if file.IsDir() { // Recurse folders
			templateSubDir := dirName + "/" + file.Name()
			if templateFilter == nil || strings.Contains(templateSubDir, *templateFilter) {
				warn, err := UploadTemplates(config, mod, dirName+"/"+file.Name(), templateFilter)
				if err != nil || len(warn) > 0 {
					return warn, err
				}
			}
			continue
		}
		ext := filepath.Ext(file.Name())
		name := file.Name()
		name = name[0 : len(name)-len(ext)] // Truncate extension

		if ext == ".tmpl" { // Only upload template files
			if config != nil && config.IsShell {
				config.Log.Printf("Found template file %s for %s\n", file.Name(), mod.Env)
			} else {
				fmt.Printf("Found template file %s for %s\n", file.Name(), mod.Env)
				if config != nil {
					config.Log.Printf("Found template file %s for %s\n", file.Name(), mod.Env)
				}
			}

			// Separate name and extension one more time for saving to vault
			ext = filepath.Ext(name)
			name = name[0 : len(name)-len(ext)]
			config.Log.Printf("dirName: %s\n", dirName)
			config.Log.Printf("file name: %s\n", file.Name())
			// Extract values
			extractedValues, err := eUtils.Parse(dirName+"/"+file.Name(), subDir, name)
			if err != nil {
				return nil, err
			}

			// Open file
			f, err := os.Open(dirName + "/" + file.Name())
			if err != nil {
				return nil, err
			}
			defer f.Close()

			// Read the file
			fileInfo, err := file.Info()
			if err != nil {
				return nil, err
			}

			fileBytes := make([]byte, fileInfo.Size())
			_, err = f.Read(fileBytes)
			if err != nil {
				return nil, err
			}

			dirSplit := strings.Split(subDir, "/")
			if len(dirSplit) >= 2 {
				project, _, _, _ := coreopts.BuildOptions.FindIndexForService(dirSplit[0], dirSplit[1])
				if project != "" && strings.Contains(string(fileBytes), "{or") {
					fmt.Printf("Cannot have an indexed template with default values for or %s for %s \n", file.Name(), mod.Env)
					return nil, nil
				}
			}

			// Construct template path for vault
			templatePath := "templates/" + subDir + "/" + name + "/template-file"
			config.Log.Printf("\tUploading template to path:\t%s\n", templatePath)

			// Construct value path for vault
			valuePath := "values/" + subDir + "/" + name
			config.Log.Printf("\tUploading values to path:\t%s\n", valuePath)

			// Write templates to vault and output errors/warnings
			warn, err := mod.Write(templatePath, map[string]interface{}{"data": fileBytes, "ext": ext}, config.Log)
			if err != nil || len(warn) > 0 {
				return warn, err
			}

			// Write values to vault and output any errors/warnings
			warn, err = mod.Write(valuePath, extractedValues, config.Log)
			if err != nil || len(warn) > 0 {
				return warn, err
			}
		} else {
			config.Log.Printf("\tSkippping template (templates must end in .tmpl):\t%s\n", file.Name())
		}
	}
	return nil, nil
}
