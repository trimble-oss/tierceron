package utils

import (
	"fmt"
	"os"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"tierceron/vaulthelper/kv"
)

func GetProjectVersionInfo(config DriverConfig, mod *kv.Modifier) map[string]map[string]interface{} {
	versionMetadataMap := make(map[string]map[string]interface{})
	secretMetadataMap, err := mod.GetVersionValues(mod, "super-secrets")
	if secretMetadataMap == nil {
		versionMetadataMap, err = mod.GetVersionValues(mod, "values")
	}
	for key, value := range secretMetadataMap {
		versionMetadataMap[key] = value
	}
	if versionMetadataMap == nil {
		fmt.Println("Unable to get version metadata for values")
		os.Exit(1)
	}
	if err != nil {
		panic(err)
	}
	return versionMetadataMap
}

func GetProjectVersion(config DriverConfig, versionMetadataMap map[string]map[string]interface{}) []int {
	var versionNumbers []int
	for valuePath, data := range versionMetadataMap {
		projectFound := false
		for _, project := range config.VersionFilter {
			if strings.Contains(valuePath, project) {
				projectFound = true
				for key := range data {
					versionNo, err := strconv.Atoi(key)
					if err != nil {
						fmt.Println()
					}
					versionNumbers = append(versionNumbers, versionNo)
				}
			}
			if !projectFound {
				continue
			}
		}
	}

	sort.Ints(versionNumbers)
	return versionNumbers
}

func BoundCheck(config DriverConfig, versionNumbers []int, version string) {
	Cyan := "\033[36m"
	Reset := "\033[0m"
	if runtime.GOOS == "windows" {
		Reset = ""
		Cyan = ""
	}
	if len(versionNumbers) >= 1 {
		latestVersion := versionNumbers[len(versionNumbers)-1]
		oldestVersion := versionNumbers[0]
		userVersion, _ := strconv.Atoi(version)
		if userVersion > latestVersion || userVersion < oldestVersion && len(versionNumbers) != 1 {
			fmt.Println(Cyan + "This version " + config.Env + "_" + version + " is not available as the latest version is " + strconv.Itoa(versionNumbers[len(versionNumbers)-1]) + " and oldest version available is " + strconv.Itoa(versionNumbers[0]) + Reset)
			os.Exit(1)
		}
	} else {
		fmt.Println(Cyan + "No version data found" + Reset)
		os.Exit(1)
	}
}

func GetProjectService(templateFile string) (string, string, string) {
	splitDir := strings.Split(templateFile, "/")
	var project, service string
	offsetBase := 0

	for i, component := range splitDir {
		if component == "trc_templates" {
			offsetBase = i
			break
		}
	}

	project = splitDir[offsetBase+1]
	service = splitDir[offsetBase+2]

	// Clean up service naming (Everything after '.' removed)
	dotIndex := strings.Index(service, ".")
	if dotIndex > 0 && dotIndex <= len(service) {
		service = service[0:dotIndex]
	}

	return project, service, templateFile
}

func RemoveDuplicates(versionFilter []string) []string {
	keys := make(map[string]bool) //Removes any duplicates
	cleanedFilter := []string{}
	for _, entry := range versionFilter {
		if _, value := keys[entry]; !value {
			keys[entry] = true
			cleanedFilter = append(cleanedFilter, entry)
		}
	}
	return cleanedFilter
}
