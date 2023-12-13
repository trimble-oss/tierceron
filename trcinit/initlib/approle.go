package initlib

import (
	"fmt"
	"os"
	"strings"

	eUtils "github.com/trimble-oss/tierceron/utils"

	"gopkg.in/yaml.v2"
)

func GetApproleFileNames(config *eUtils.DriverConfig, namespace string) []string {
	var approleFileNames []string
	cwd, cwdErr := os.Getwd()
	if cwdErr != nil {
		fmt.Println("Error reading current directory.  Cannot continue.")
		eUtils.LogErrorObject(config, cwdErr, false)
		os.Exit(-1)
	}

	approleFiles, approleFilesErr := os.ReadDir(cwd + "/vault_namespaces/" + namespace + "/approle_files")
	if approleFilesErr != nil {
		fmt.Println("Error reading approle_files directory. Cannot continue.")
		eUtils.LogErrorObject(config, approleFilesErr, false)
		os.Exit(-1)
	}

	for _, approleFile := range approleFiles {
		if strings.Contains(approleFile.Name(), ".yml") {
			approleFileNames = append(approleFileNames, strings.TrimSuffix(approleFile.Name(), ".yml"))
		} else {
			fmt.Println(approleFile.Name() + " is not a yaml file. Continuing with other files.")
			eUtils.LogErrorObject(config, approleFilesErr, false)
			continue
		}
	}
	return approleFileNames
}

func ParseApproleYaml(fileName string, namespace string) (map[interface{}]interface{}, error) {
	cwd, cwdErr := os.Getwd()
	if cwdErr != nil {
		return nil, cwdErr
	}
	file, err := os.ReadFile(cwd + "/vault_namespaces/" + namespace + "/approle_files/" + fileName + ".yml")
	if err != nil {
		return nil, err
	}

	parsedData := make(map[interface{}]interface{})

	err2 := yaml.Unmarshal(file, &parsedData)
	if err2 != nil {
		return nil, err2
	}

	return parsedData, nil
}
