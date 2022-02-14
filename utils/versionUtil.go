package utils

import (
	"fmt"
	"log"
	"os"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"tierceron/vaulthelper/kv"
)

func SplitEnv(env string) []string {
	envVersion := make([]string, 2)
	lastIndex := strings.LastIndex(env, "_")
	if lastIndex == -1 {
		envVersion[0] = env
		envVersion[1] = "0"
	} else {
		envVersion[0] = env[0:lastIndex]
		envVersion[1] = env[lastIndex+1:]
	}

	return envVersion
}

func GetProjectVersionInfo(config DriverConfig, mod *kv.Modifier, logger *log.Logger) map[string]map[string]interface{} {
	versionMetadataMap := make(map[string]map[string]interface{})
	mod.VersionFilter = config.VersionFilter
	var secretMetadataMap map[string]map[string]interface{}
	var err error
	mod.RawEnv = strings.Split(config.EnvRaw, ".")[0]
	if !config.WantCerts {
		secretMetadataMap, err = mod.GetVersionValues(mod, config.WantCerts, "super-secrets", logger)
		if secretMetadataMap == nil {
			secretMetadataMap, err = mod.GetVersionValues(mod, config.WantCerts, "values", logger)
		}
	} else {
		//Certs are in values, not super secrets
		secretMetadataMap, err = mod.GetVersionValues(mod, config.WantCerts, "values", logger)
	}
	var foundKey string
	for key, value := range secretMetadataMap {
		foundService := false
		for _, service := range mod.VersionFilter {
			if strings.HasSuffix(key, service) && !foundService {
				foundService = true
				foundKey = key
			}
		}
		if foundService && len(foundKey) > 0 {
			versionMetadataMap[foundKey] = value
		}
	}

	if err != nil {
		fmt.Println("No version data available for this env")
		LogErrorObject(err, logger, false)
	}
	if len(versionMetadataMap) == 0 {
		fmt.Println("No version data available for this env")
		LogErrorObject(err, logger, false)
	}

	return versionMetadataMap
}

func GetProjectVersions(config DriverConfig, versionMetadataMap map[string]map[string]interface{}) []int {
	var versionNumbers []int
	for valuePath, data := range versionMetadataMap {
		projectFound := false
		for _, project := range config.VersionFilter {
			if strings.Contains(valuePath, project) {
				projectFound = true
				for key := range data {
					versionNo, err := strconv.Atoi(key)
					if err != nil {
						fmt.Printf("Could not convert %s into a int", key)
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
			fmt.Println(Cyan + "This version " + config.Env + " is not available as the latest version is " + strconv.Itoa(versionNumbers[len(versionNumbers)-1]) + " and oldest version available is " + strconv.Itoa(versionNumbers[0]) + Reset)
			os.Exit(1)
		}
	} else {
		fmt.Println(Cyan + "No version data found" + Reset)
		os.Exit(1)
	}
}

func GetProjectServices(templateFiles []string) ([]string, []string, []string) {
	projects := []string{}
	services := []string{}
	templateFilesContents := []string{}

	for _, templateFile := range templateFiles {
		project, service, templateFileContent := GetProjectService(templateFile)

		projects = append(projects, project)
		services = append(services, service)
		templateFilesContents = append(templateFilesContents, templateFileContent)
	}

	return projects, services, templateFilesContents
}

func GetProjectService(templateFile string) (string, string, string) {
	templateFile = strings.ReplaceAll(strings.ReplaceAll(templateFile, "\\\\", "/"), "\\", "/")
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
	if !strings.Contains(templateFile, "Common") {
		dotIndex := strings.Index(service, ".")
		if dotIndex > 0 && dotIndex <= len(service) {
			service = service[0:dotIndex]
		}
	} else if strings.Contains(service, ".mf.tmpl") {
		service = strings.Split(service, ".mf.tmpl")[0]
	}

	return project, service, templateFile
}

func GetTemplateFileName(templateFile string, service string) string {
	templateSplit := strings.Split(templateFile, service+"/")
	templateFileName := strings.Split(templateSplit[len(templateSplit)-1], ".")[0]

	return templateFileName
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
