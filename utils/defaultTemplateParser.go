package utils

import (
	"io/ioutil"
	"log"
	"os"
	"regexp"
	"strings"

	"fyne.io/fyne"
)

// {{or .<key> "<value>"}}
const pattern string = `{{or \.([^"]+) "([^"]+)"}}`

type ConfigDriver func(config DriverConfig)

type DriverConfig struct {
	Window               fyne.Window
	Insecure             bool
	Token                string
	VaultAddress         string
	Env                  string
	Regions              []string
	SecretMode           bool
	ServicesWanted       []string
	StartDir             []string // Starting directory... possibly multiple
	EndDir               string
	WantCert             bool
	ZeroConfig           bool
	GenAuth              bool
	Log                  *log.Logger
	Diff                 bool
	Update               func(*string, string)
	FileFilter           []string
	VersionInfo          func(map[string]interface{}, bool, string)
	VersionProjectFilter []string
}

// ConfigControl Setup initializes the directory structures in preparation for parsing templates.
func ConfigControl(config DriverConfig, drive ConfigDriver) {
	multiProject := false

	config.EndDir = strings.Replace(config.EndDir, "\\", "/", -1)
	if config.EndDir != "." && (strings.LastIndex(config.EndDir, "/") < (len(config.EndDir) - 1)) {
		config.EndDir = config.EndDir + "/"
	}

	startDirs := []string{}

	// Satisfy needs of templating tool with path cleanup.
	if config.StartDir[0] == "vault_templates" {
		// Set up for single service configuration when available.
		// This is the most common use of the tool.
		pwd, err := os.Getwd()
		if err == nil {
			config.StartDir[0] = pwd + string(os.PathSeparator) + config.StartDir[0]
		}

		projectFilesComplete, err := ioutil.ReadDir(config.StartDir[0])
		projectFiles := []os.FileInfo{}
		for _, projectFile := range projectFilesComplete {
			if !strings.HasSuffix(projectFile.Name(), ".DS_Store") {
				projectFiles = append(projectFiles, projectFile)
			}
		}

		if len(projectFiles) == 2 && (projectFiles[0].Name() == "Common" || projectFiles[1].Name() == "Common") {
			for _, projectFile := range projectFiles {
				projectStartDir := config.StartDir[0]
				if projectFile.Name() == "Common" {
					projectStartDir = projectStartDir + string(os.PathSeparator) + projectFile.Name()
				} else if projectFile.IsDir() {
					projectStartDir = projectStartDir + string(os.PathSeparator) + projectFile.Name()
					serviceFiles, err := ioutil.ReadDir(projectStartDir)
					if err == nil && len(serviceFiles) == 1 && serviceFiles[0].IsDir() {
						projectStartDir = projectStartDir + string(os.PathSeparator) + serviceFiles[0].Name()
						config.VersionProjectFilter = append(config.VersionProjectFilter, serviceFiles[0].Name())
					}
					if strings.LastIndex(projectStartDir, string(os.PathSeparator)) < (len(projectStartDir) - 1) {
						projectStartDir = projectStartDir + string(os.PathSeparator)
					}
				}
				// VaultConfig is happiest with linux path separators
				projectStartDir = strings.Replace(projectStartDir, "\\", "/", -1)
				startDirs = append(startDirs, projectStartDir)
			}

			config.StartDir = startDirs
			// Drive this set of configurations.
			drive(config)

			return
		}

		if len(config.VersionProjectFilter) == 0 {
			for _, projectFile := range projectFilesComplete {
				if !strings.HasSuffix(projectFile.Name(), ".DS_Store") {
					config.VersionProjectFilter = append(config.VersionProjectFilter, projectFile.Name())
				}
			}
		}

		if err == nil && len(projectFiles) == 1 && projectFiles[0].IsDir() {
			config.StartDir[0] = config.StartDir[0] + string(os.PathSeparator) + projectFiles[0].Name()
		} else if len(projectFiles) > 1 {
			multiProject = true
		}
		serviceFiles, err := ioutil.ReadDir(config.StartDir[0])

		if err == nil && len(serviceFiles) == 1 && serviceFiles[0].IsDir() {
			config.StartDir[0] = config.StartDir[0] + string(os.PathSeparator) + serviceFiles[0].Name()
		} else if len(projectFiles) > 1 {
			multiProject = true
		}
	}

	if !multiProject && strings.LastIndex(config.StartDir[0], "/") < (len(config.StartDir[0])-1) {
		config.StartDir[0] = config.StartDir[0] + "/"
	}

	// Drive this set of configurations.
	drive(config)
}

// Parse Extracts default values as key-value pairs from template files.
// Before being uploaded, the service and filename will be appended so the uploaded value will be
// <Service>.<Filename>.<Key>
// Underscores in key names will be replaced with periods before being uploaded
func Parse(filepath string, service string, filename string) (map[string]interface{}, error) {
	workingSet := make(map[string]interface{})
	file, err := ioutil.ReadFile(filepath)
	if err != nil {
		return nil, err
	}

	regex, err := regexp.Compile(pattern)

	if err != nil {
		return nil, err
	}

	matched := regex.FindAllString(string(file), -1)

	for _, match := range matched {
		kv := regex.FindStringSubmatch(match)
		// Split and add to map
		kv[1] = strings.Replace(kv[1], "_", ".", -1)
		workingSet[kv[1]] = kv[2]
	}

	return workingSet, nil
}
