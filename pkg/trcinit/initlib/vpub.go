package initlib

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	flowcore "github.com/trimble-oss/tierceron-core/v2/flow"

	"github.com/trimble-oss/tierceron-core/v2/core/coreconfig"
	"github.com/trimble-oss/tierceron/buildopts/coreopts"
	eUtils "github.com/trimble-oss/tierceron/pkg/utils"
	helperkv "github.com/trimble-oss/tierceron/pkg/vaulthelper/kv"
)

func UploadTemplateDirectory(tfmContext flowcore.FlowMachineContext, config *coreconfig.CoreConfig, mod *helperkv.Modifier, dirName string, templateFilter *string) ([]string, error) {
	dirs, err := os.ReadDir(dirName)
	if err != nil {
		fmt.Fprintln(os.Stderr, "Read directory couldn't be completed.")
		return nil, err
	}

	// Parse each subdirectory as a service name
	for _, subDir := range dirs {
		if subDir.IsDir() {
			pathName := dirName + "/" + subDir.Name()

			if templateFilter == nil || len(*templateFilter) == 0 || strings.HasPrefix(*templateFilter, subDir.Name()) {
				warn, err := UploadTemplates(tfmContext, config, mod, pathName, templateFilter)
				if err != nil || len(warn) > 0 {
					fmt.Fprintf(os.Stderr, "Upload templates couldn't be completed. %v", err)
					return warn, err
				}
			}
		}
	}
	return nil, nil
}

func getTemplateSubDir(dirName string) string {
	splitDir := strings.SplitAfterN(dirName, "/", 2)
	return splitDir[len(splitDir)-1]
}

func collectTemplateNamesByPath(dirName string, templateFilter *string) (map[string]map[string]struct{}, error) {
	templatesByPath := map[string]map[string]struct{}{}

	err := filepath.WalkDir(dirName, func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			return nil
		}

		ext := filepath.Ext(d.Name())
		if ext != ".tmpl" {
			return nil
		}

		relPath, err := filepath.Rel(dirName, path)
		if err != nil {
			return err
		}
		relPath = filepath.ToSlash(relPath)

		if templateFilter != nil && len(*templateFilter) != 0 && !strings.Contains(relPath, *templateFilter) {
			return nil
		}

		relDir := filepath.ToSlash(filepath.Dir(relPath))
		if relDir == "." {
			relDir = ""
		}

		name := d.Name()
		name = name[0 : len(name)-len(ext)]
		ext = filepath.Ext(name)
		name = name[0 : len(name)-len(ext)]

		if _, ok := templatesByPath[relDir]; !ok {
			templatesByPath[relDir] = map[string]struct{}{}
		}
		templatesByPath[relDir][name] = struct{}{}

		return nil
	})
	if err != nil {
		return nil, err
	}

	return templatesByPath, nil
}

func PruneVaultTemplatesNotOnDisk(config *coreconfig.CoreConfig, mod *helperkv.Modifier, dirName string, templateFilter *string) error {
	templatesByPath, err := collectTemplateNamesByPath(dirName, templateFilter)
	if err != nil {
		return err
	}

	for subDir, templateNames := range templatesByPath {
		if strings.HasPrefix(subDir, "Common") || strings.HasPrefix(subDir, "Azure") {
			continue
		}
		vaultPath := "templates"
		if len(subDir) > 0 {
			vaultPath += "/" + subDir
		}

		secret, err := mod.List(vaultPath, config.Log)
		if err != nil {
			return err
		}
		if secret == nil || secret.Data == nil || secret.Data["keys"] == nil {
			continue
		}

		vaultTemplateNames, ok := secret.Data["keys"].([]any)
		if !ok {
			continue
		}

		for _, vaultTemplateName := range vaultTemplateNames {
			templateName, ok := vaultTemplateName.(string)
			if !ok {
				continue
			}

			templateName = strings.TrimSuffix(templateName, "/")
			if len(templateName) == 0 {
				continue
			}

			if _, ok := templateNames[templateName]; ok {
				continue
			}

			if len(subDir) > 0 {
				config.Log.Printf("Deleting stale vault template %s/%s\n", subDir, templateName)
			} else {
				config.Log.Printf("Deleting stale vault template %s\n", templateName)
			}

			templatePath := "templates/" + templateName
			if len(subDir) > 0 {
				templatePath = "templates/" + subDir + "/" + templateName
			}
			fmt.Println("vault template path to delete:", templatePath)

			if _, err := mod.SoftDelete(templatePath+"/template-file", config.Log); err != nil {
				return err
			}
		}
	}

	return nil
}

func UploadTemplates(tfmContext flowcore.FlowMachineContext, config *coreconfig.CoreConfig, mod *helperkv.Modifier, dirName string, templateFilter *string) ([]string, error) {
	// Open directory
	files, err := os.ReadDir(dirName)
	if err != nil {
		return nil, err
	}

	// Use name of containing directory as the template subdirectory
	subDir := getTemplateSubDir(dirName)

	// Parse through files
	for _, file := range files {
		// Extract extension and name
		if file.IsDir() { // Recurse folders
			templateSubDir := dirName + "/" + file.Name()
			if templateFilter == nil || strings.Contains(templateSubDir, *templateFilter) {
				warn, err := UploadTemplates(tfmContext, config, mod, dirName+"/"+file.Name(), templateFilter)
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
				fmt.Fprintf(os.Stderr, "Found template file %s for %s\n", file.Name(), mod.Env)
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
				project, _, _, _ := coreopts.BuildOptions.FindIndexForService(tfmContext, dirSplit[0], dirSplit[1])
				if project != "" && strings.Contains(string(fileBytes), "{or") {
					fmt.Fprintf(os.Stderr, "Cannot have an indexed template with default values for or %s for %s \n", file.Name(), mod.Env)
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
			warn, err := mod.Write(templatePath, map[string]any{"data": fileBytes, "ext": ext}, config.Log)
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
