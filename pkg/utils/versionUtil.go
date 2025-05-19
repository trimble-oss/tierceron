package utils

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	coreutil "github.com/trimble-oss/tierceron-core/v2/util"

	"github.com/trimble-oss/tierceron/buildopts/coreopts"
	"github.com/trimble-oss/tierceron/pkg/utils/config"
	helperkv "github.com/trimble-oss/tierceron/pkg/vaulthelper/kv"
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

func GetEnvBasis(env string) string {
	if strings.HasPrefix(env, "dev") {
		return "dev"
	} else if strings.HasPrefix(env, "QA") {
		return "QA"
	} else if strings.HasPrefix(env, "itdev") {
		return "itdev"
	} else if strings.HasPrefix(env, "RQA") {
		return "RQA"
	} else if strings.HasPrefix(env, "staging") {
		return "staging"
	} else if strings.HasPrefix(env, "prod") {
		return "prod"
	} else {
		return strings.Split(env, "_")[0]
	}
}

func GetProjectVersionInfo(driverConfig *config.DriverConfig, mod *helperkv.Modifier) map[string]map[string]interface{} {
	versionMetadataMap := make(map[string]map[string]interface{})
	mod.VersionFilter = driverConfig.VersionFilter
	var secretMetadataMap map[string]map[string]interface{}
	var err error
	mod.SectionKey = driverConfig.SectionKey
	mod.SubSectionName = driverConfig.SubSectionName
	mod.EnvBasis = strings.Split(driverConfig.CoreConfig.EnvBasis, ".")[0]
	if !driverConfig.CoreConfig.WantCerts {
		secretMetadataMap, err = mod.GetVersionValues(mod, driverConfig.CoreConfig.WantCerts, "super-secrets", driverConfig.CoreConfig.Log)
		if secretMetadataMap == nil {
			secretMetadataMap, err = mod.GetVersionValues(mod, driverConfig.CoreConfig.WantCerts, "values", driverConfig.CoreConfig.Log)
		}
	} else {
		//Certs are in values, not super secrets
		secretMetadataMap, err = mod.GetVersionValues(mod, driverConfig.CoreConfig.WantCerts, "values", driverConfig.CoreConfig.Log)
	}
	var foundKey string
	for key, value := range secretMetadataMap {
		foundService := false
		for _, service := range mod.VersionFilter {
			keyNoExt := strings.Split(key, ".")
			if strings.HasSuffix(keyNoExt[0], service) {
				foundService = true
				foundKey = key
				break
			} else if mod.SectionKey == "/Index/" && mod.SubSectionName != "" && strings.HasPrefix(key, "super-secrets"+mod.SectionKey+service+"/") {
				foundService = true
				foundKey = key
				break
			}
		}
		if foundService && len(foundKey) > 0 {
			versionMetadataMap[foundKey] = value
		}
	}

	if err != nil {
		fmt.Println("No version data available for this env")
		LogErrorObject(driverConfig.CoreConfig, err, false)
	}
	if len(versionMetadataMap) == 0 {
		fmt.Println("No version data available for this env")
		LogErrorObject(driverConfig.CoreConfig, err, false)
	}

	return versionMetadataMap
}

func GetProjectVersions(driverConfig *config.DriverConfig, versionMetadataMap map[string]map[string]interface{}) []int {
	var versionNumbers []int
	for valuePath, data := range versionMetadataMap {
		if len(driverConfig.ServiceFilter) > 0 {
			found := false
			for _, index := range driverConfig.ServiceFilter {
				if strings.Contains(valuePath, index) {
					found = true
				}
			}
			if !found {
				continue
			}
		}
		projectFound := false
		for _, project := range driverConfig.VersionFilter {
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

func BoundCheck(driverConfig *config.DriverConfig, versionNumbers []int, version string) {
	Cyan := "\033[36m"
	Reset := "\033[0m"
	if IsWindows() {
		Reset = ""
		Cyan = ""
	}
	if len(versionNumbers) >= 1 {
		latestVersion := versionNumbers[len(versionNumbers)-1]
		oldestVersion := versionNumbers[0]
		userVersion, _ := strconv.Atoi(version)
		if userVersion > latestVersion || userVersion < oldestVersion && len(versionNumbers) != 1 {
			LogAndSafeExit(driverConfig.CoreConfig, Cyan+"This version "+driverConfig.CoreConfig.Env+" is not available as the latest version is "+strconv.Itoa(versionNumbers[len(versionNumbers)-1])+" and oldest version available is "+strconv.Itoa(versionNumbers[0])+Reset, 1)
		}
	} else {
		LogAndSafeExit(driverConfig.CoreConfig, Cyan+"No version data found"+Reset, 1)
	}
}

func GetProjectServices(driverConfig *config.DriverConfig, templateFiles []string) ([]string, []string, []string) {
	projects := []string{}
	services := []string{}
	templateFilesContents := []string{}

	for _, templateFile := range templateFiles {
		project, service, _, templateFileContent := GetProjectService(driverConfig, templateFile)

		projects = append(projects, project)
		services = append(services, service)
		templateFilesContents = append(templateFilesContents, templateFileContent)
	}

	return projects, services, templateFilesContents
}

// GetProjectService - returns project, service, and path to template on filesystem.
// driverConfig - driver configuration
// templateFile - full path to template file
// returns project, service, templatePath
func GetProjectService(driverConfig *config.DriverConfig, templateFile string) (string, string, int, string) {
	var startDir []string = nil
	var deploymentDriverConfig string = ""

	if driverConfig != nil {
		startDir = driverConfig.StartDir
		if len(driverConfig.DeploymentConfig) > 0 {
			if projectService, ok := driverConfig.DeploymentConfig["trcprojectservice"]; ok {
				deploymentDriverConfig = projectService.(string)
			}
		}
	}
	trcTemplateParam := coreopts.BuildOptions.GetFolderPrefix(startDir) + "_templates"

	return coreutil.GetProjectService(deploymentDriverConfig, trcTemplateParam, templateFile)
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
