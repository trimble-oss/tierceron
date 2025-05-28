package util

import (
	"archive/zip"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/trimble-oss/tierceron/buildopts/memprotectopts"
	"github.com/trimble-oss/tierceron/pkg/core"
	eUtils "github.com/trimble-oss/tierceron/pkg/utils"
	"github.com/trimble-oss/tierceron/pkg/utils/config"
	helperkv "github.com/trimble-oss/tierceron/pkg/vaulthelper/kv"

	"gopkg.in/yaml.v2"

	flowcore "github.com/trimble-oss/tierceron-core/v2/flow"
	vcutils "github.com/trimble-oss/tierceron/pkg/cli/trcconfigbase/utils"
	"github.com/trimble-oss/tierceron/pkg/trcx/extract"
	"github.com/trimble-oss/tierceron/pkg/trcx/xutil"

	il "github.com/trimble-oss/tierceron/pkg/trcinit/initlib"

	"log"
)

type ProcessFlowConfig func(pluginEnvConfig map[string]interface{}) map[string]interface{}
type ProcessFlowInitConfig func(flowMachineInitContext *flowcore.FlowMachineInitContext, pluginConfig map[string]interface{}, logger *log.Logger) error
type BootFlowMachineFunc func(flowMachineInitContext *flowcore.FlowMachineInitContext, driverConfig *config.DriverConfig, pluginConfig map[string]interface{}, logger *log.Logger) (any, error)

// Unused/deprecated
func GetLocalVaultHost(withPort bool, vaultHostChan chan string, vaultLookupErrChan chan error, logger *log.Logger) {
	vaultHost := "https://"
	vaultErr := errors.New("no usable local vault found")
	// Dev machines.
	vaultHost = vaultHost + "127.0.0.1"
	vaultHostChan <- vaultHost
	logger.Println("Init stage 1 success.")
	vaultErr = nil

	if vaultErr != nil {
		vaultLookupErrChan <- vaultErr
	}
}

func GetJSONFromClientByGet(config *core.CoreConfig, httpClient *http.Client, headers map[string]string, address string, body io.Reader) (map[string]interface{}, int, error) {
	var jsonData map[string]interface{}
	request, err := http.NewRequest("GET", address, body)
	if err != nil {
		eUtils.LogErrorObject(config, err, false)
	}
	for headerkey, headervalue := range headers {
		request.Header.Set(headerkey, headervalue)
	}
	// request.Header.Set("Accept", "application/json")
	// request.Header.Set("Content-Type", "application/json")
	response, err := httpClient.Do(request)
	if err != nil {
		eUtils.LogErrorObject(config, err, false)
		return nil, response.StatusCode, err
	}
	if response.Body != nil {
		defer response.Body.Close()
	}

	switch response.StatusCode {
	case http.StatusOK:
		jsonDataFromHttp, err := io.ReadAll(response.Body)

		if err != nil {
			return nil, response.StatusCode, err
		}

		err = json.Unmarshal([]byte(jsonDataFromHttp), &jsonData)

		if err != nil {
			eUtils.LogErrorObject(config, err, false)
		}

		return jsonData, response.StatusCode, nil
	case http.StatusNoContent:
		return jsonData, response.StatusCode, nil
	}

	return nil, response.StatusCode, errors.New("http status failure")
}

func GetJSONFromClientByPost(config *core.CoreConfig, httpClient *http.Client, headers map[string]string, address string, body io.Reader) (map[string]interface{}, int, error) {
	var jsonData map[string]interface{}
	request, err := http.NewRequest("POST", address, body)
	if err != nil {
		eUtils.LogErrorObject(config, err, false)
	}
	for headerkey, headervalue := range headers {
		request.Header.Set(headerkey, headervalue)
	}
	// request.Header.Set("Accept", "application/json")
	// request.Header.Set("Content-Type", "application/json")
	response, err := httpClient.Do(request)
	if err != nil {
		if response != nil {
			return nil, response.StatusCode, err
		} else {
			return nil, 204, err
		}

	}
	if response.Body != nil {
		defer response.Body.Close()
	}

	switch response.StatusCode {
	case http.StatusUnauthorized:
		return nil, response.StatusCode, fmt.Errorf("http auth failure: %d", response.StatusCode)
	case http.StatusOK:
		jsonDataFromHttp, err := io.ReadAll(response.Body)

		if err != nil {
			eUtils.LogErrorObject(config, err, false)
		}

		err = json.Unmarshal([]byte(jsonDataFromHttp), &jsonData)

		if err != nil {
			eUtils.LogErrorObject(config, err, false)
		}

		return jsonData, response.StatusCode, nil
	}
	return nil, response.StatusCode, fmt.Errorf("http status failure: %d", response.StatusCode)
}

func LoadBaseTemplate(driverConfig *config.DriverConfig, templateResult *extract.TemplateResultData, goMod *helperkv.Modifier, project string, service string, templatePath string) error {
	templateResult.ValueSection = map[string]map[string]map[string]string{}
	templateResult.ValueSection["values"] = map[string]map[string]string{}

	templateResult.SecretSection = map[string]map[string]map[string]string{}
	templateResult.SecretSection["super-secrets"] = map[string]map[string]string{}

	var cds *vcutils.ConfigDataStore
	commonPaths := make([]string, 1)
	if goMod != nil {
		cds = new(vcutils.ConfigDataStore)
		goMod.Version = goMod.Version + "***X-Mode"
		servicePath := fmt.Sprintf("%s/%s", service, service)
		cds.Init(driverConfig.CoreConfig, goMod, true, true, project, commonPaths, servicePath) //CommonPaths = "" - empty - not needed for tenant config
	}

	var errSeed error
	_, _, _, templateResult.TemplateDepth, errSeed = extract.ToSeed(driverConfig, goMod,
		cds,
		templatePath,
		project,
		service,
		true,
		&(templateResult.InterfaceTemplateSection),
		&(templateResult.ValueSection),
		&(templateResult.SecretSection),
	)

	return errSeed
}

func SeedVaultById(driverConfig *config.DriverConfig, goMod *helperkv.Modifier, service string, addressPtr *string, tokenPtr *string, baseTemplate *extract.TemplateResultData, tableData map[string]interface{}, indexPath string, project string) error {
	// Copy the base template
	templateResult := *baseTemplate
	valueCombinedSection := map[string]map[string]map[string]string{}
	valueCombinedSection["values"] = map[string]map[string]string{}

	secretCombinedSection := map[string]map[string]map[string]string{}
	secretCombinedSection["super-secrets"] = map[string]map[string]string{}

	// Declare local variables
	templateCombinedSection := map[string]interface{}{}
	sliceTemplateSection := []interface{}{}
	sliceValueSection := []map[string]map[string]map[string]string{}
	sliceSecretSection := []map[string]map[string]map[string]string{}
	for key, value := range tableData {
		if _, ok := templateResult.SecretSection["super-secrets"][service][key]; ok {
			if valueString, sOk := value.(string); sOk {
				templateResult.SecretSection["super-secrets"][service][key] = valueString
			} else if iValue, iOk := value.(int64); iOk {
				templateResult.SecretSection["super-secrets"][service][key] = fmt.Sprintf("%d", iValue)
			} else if i8Value, i8Ok := value.(int8); i8Ok {
				templateResult.SecretSection["super-secrets"][service][key] = fmt.Sprintf("%d", i8Value)
			} else if tValue, tOk := value.(time.Time); tOk {
				templateResult.SecretSection["super-secrets"][service][key] = tValue.String()
			} else {
				if value != nil {
					templateResult.SecretSection["super-secrets"][service][key] = fmt.Sprintf("%v", value)
				} else {
					templateResult.SecretSection["super-secrets"][service][key] = ""
				}
			}
		}
	}
	for key, value := range tableData {
		if _, ok := templateResult.ValueSection["values"][service][key]; ok {
			if valueString, sOk := value.(string); sOk {
				templateResult.ValueSection["values"][service][key] = valueString
			} else if iValue, iOk := value.(int64); iOk {
				templateResult.ValueSection["values"][service][key] = fmt.Sprintf("%d", iValue)
			} else if i8Value, i8Ok := value.(int8); i8Ok {
				templateResult.ValueSection["values"][service][key] = fmt.Sprintf("%d", i8Value)
			} else if tValue, tOk := value.(time.Time); tOk {
				templateResult.ValueSection["values"][service][key] = tValue.String()
			} else {
				if value != nil {
					templateResult.ValueSection["values"][service][key] = fmt.Sprintf("%v", value)
				} else {
					templateResult.ValueSection["values"][service][key] = ""
				}
			}
		}
	}
	maxDepth := templateResult.TemplateDepth
	// Combine values of slice

	sliceTemplateSection = append(sliceTemplateSection, templateResult.InterfaceTemplateSection)
	sliceValueSection = append(sliceValueSection, templateResult.ValueSection)
	sliceSecretSection = append(sliceSecretSection, templateResult.SecretSection)

	xutil.CombineSection(driverConfig.CoreConfig, sliceTemplateSection, maxDepth, templateCombinedSection)
	xutil.CombineSection(driverConfig.CoreConfig, sliceValueSection, -1, valueCombinedSection)
	xutil.CombineSection(driverConfig.CoreConfig, sliceSecretSection, -1, secretCombinedSection)

	template, errT := yaml.Marshal(templateCombinedSection)
	value, errV := yaml.Marshal(valueCombinedSection)
	secret, errS := yaml.Marshal(secretCombinedSection)

	if errT != nil {
		return errT
	}

	if errV != nil {
		return errV
	}

	if errS != nil {
		return errS
	}
	templateData := string(template)
	// Remove single quotes generated by Marshal
	templateData = strings.ReplaceAll(templateData, "'", "")
	seedData := templateData + "\n\n\n" + string(value) + "\n\n\n" + string(secret) + "\n\n\n"
	//VaultX Section Ends
	//VaultInit Section Begins
	if strings.Contains(indexPath, "/PublicIndex/") {
		driverConfig.ServicesWanted = []string{""}
		driverConfig.CoreConfig.WantCerts = false
		il.SeedVaultFromData(driverConfig, indexPath, []byte(seedData))
	} else {
		driverConfig.ServicesWanted = []string{service}
		driverConfig.CoreConfig.WantCerts = false
		il.SeedVaultFromData(driverConfig, "Index/"+project+indexPath, []byte(seedData))
	}
	return nil
}

func GetPluginToolConfig(driverConfig *config.DriverConfig, mod *helperkv.Modifier, pluginConfig map[string]interface{}, defineService bool) (map[string]interface{}, error) {
	driverConfig.CoreConfig.Log.Println("GetPluginToolConfig begin processing plugins.")
	//templatePaths
	indexFound := false
	templatePaths := pluginConfig["templatePath"].([]string)

	driverConfig.CoreConfig.Log.Println("GetPluginToolConfig reading base configurations.")
	tempEnv := mod.Env
	envParts := strings.Split(mod.Env, "-")
	mod.Env = envParts[0]
	pluginToolConfig, err := mod.ReadData("super-secrets/Restricted/PluginTool/config")
	defer func(m *helperkv.Modifier, e string) {
		m.Env = e
	}(mod, tempEnv)

	if err != nil {
		driverConfig.CoreConfig.Log.Println("GetPluginToolConfig errored with missing base PluginTool configurations.")
		return nil, err
	} else {
		if len(pluginToolConfig) == 0 {
			driverConfig.CoreConfig.Log.Println("GetPluginToolConfig empty base PluginTool configurations.")
			return nil, errors.New("Tierceron plugin management presently not configured for env: " + mod.Env)
		}
	}
	pluginEnvConfigClone := make(map[string]interface{})

	for k, v := range pluginToolConfig {
		if _, okStr := v.(string); okStr {
			v2 := strings.Clone(v.(string))
			memprotectopts.MemProtect(nil, &v2)
			pluginEnvConfigClone[k] = v2
		} else {
			// Safe to share...
			pluginEnvConfigClone[k] = v
		}
	}

	for k, v := range pluginConfig {
		if _, okStr := v.(string); okStr {
			v2 := strings.Clone(v.(string))
			memprotectopts.MemProtect(nil, &v2)
			pluginEnvConfigClone[k] = v2
		} else {
			// Safe to share...
			pluginEnvConfigClone[k] = v
		}
	}

	var ptc1 map[string]interface{}

	driverConfig.CoreConfig.Log.Println("GetPluginToolConfig loading plugin data.")
	for _, templatePath := range templatePaths {
		// TODO: Chewbacca -- could pass in driverConfig but we didn't before...
		project, service, _, _ := eUtils.GetProjectService(nil, templatePath)
		driverConfig.CoreConfig.Log.Println("GetPluginToolConfig project: " + project + " plugin: " + driverConfig.SubSectionValue + " service: " + service)

		if pluginPath, pathOk := pluginToolConfig["pluginpath"]; pathOk && len(pluginPath.(string)) != 0 {
			mod.SectionPath = "super-secrets/Index/" + project + pluginPath.(string) + driverConfig.SubSectionValue + "/" + service
		} else {
			mod.SectionPath = "super-secrets/Index/" + project + "/trcplugin/" + driverConfig.SubSectionValue + "/" + service
		}
		ptc1, err = mod.ReadData(mod.SectionPath)

		pluginToolConfig["pluginpath"] = mod.SectionPath
		if err != nil || ptc1 == nil {
			driverConfig.CoreConfig.Log.Println("No data found for project: " + project + " plugin: " + driverConfig.SubSectionValue + " service: " + service)
			continue
		}
		indexFound = true

		for k, v := range ptc1 {
			if _, okStr := v.(string); okStr {
				v2 := strings.Clone(v.(string))
				memprotectopts.MemProtect(nil, &v2)
				pluginEnvConfigClone[k] = v2
			} else {
				// Safe to share...
				pluginEnvConfigClone[k] = v
			}
		}

		break
	}
	mod.SectionPath = ""
	driverConfig.CoreConfig.Log.Println("GetPluginToolConfig plugin data load process complete.")
	mod.Env = tempEnv

	if len(pluginEnvConfigClone) == 0 {
		driverConfig.CoreConfig.Log.Println("No data found for plugin.")
		if err == nil {
			err = errors.New("no data and unexpected error")
		}
		return pluginEnvConfigClone, err
	} else if !indexFound {
		if defineService {
			pluginEnvConfigClone["pluginpath"] = pluginToolConfig["pluginpath"]
		}
		return pluginEnvConfigClone, nil
	} else {
		if _, ok := pluginEnvConfigClone["trcplugin"]; ok {
			if strings.ContainsAny(pluginEnvConfigClone["trcplugin"].(string), "./") {
				err = errors.New("Invalid plugin configuration: " + pluginEnvConfigClone["trcplugin"].(string))
				return nil, err
			}
		}
		if defineService {
			pluginEnvConfigClone["pluginpath"] = pluginToolConfig["pluginpath"]
		}
	}
	driverConfig.CoreConfig.Log.Println("GetPluginToolConfig end processing plugins.")

	return pluginEnvConfigClone, nil
}

func UncompressZipFile(filePath string) (bool, []error) {
	errorList := []error{}
	// Open zip archive
	r, readErr := zip.OpenReader(filePath)
	if readErr != nil {
		fmt.Println("Could not open zip file -  " + readErr.Error())
		errorList = append(errorList, readErr)
	}
	defer r.Close()

	//Range archive
	for _, f := range r.File {
		// GOOD: Check that path does not contain ".." before using it - must be absolute path.
		if strings.Contains(f.Name, "..") {
			fmt.Println("Path must be absolute in archive - " + f.Name + ".")
			errorList = append(errorList, fmt.Errorf("path must be absolute in archive - %s", f.Name))
			return false, errorList
		}
		rc, openErr := f.Open()
		if openErr != nil {
			fmt.Println("Could not open file inside archive - " + f.Name + " - " + openErr.Error())
			errorList = append(errorList, openErr)
		}
		defer rc.Close()
		pathParts := strings.Split(filePath, ".")
		rootPath := pathParts[0]

		newFilePath := fmt.Sprintf("%s%c%s", rootPath, filepath.Separator, f.Name)

		// if we have a directory we have to create it
		if f.FileInfo().IsDir() {
			dirErr := os.MkdirAll(newFilePath, 0700)
			if dirErr != nil {
				fmt.Println("Could not create directory  - " + dirErr.Error())
				errorList = append(errorList, dirErr)
			}
			// we can go to next iteration
			continue
		}

		dir := filepath.Dir(newFilePath)
		if dir != "." {
			dirErr := os.MkdirAll(dir, 0700)
			if dirErr != nil {
				fmt.Println("Could not create directory  - " + dirErr.Error())
				errorList = append(errorList, dirErr)
			}
		}

		// create new uncompressed file if not directory
		uncompressedFile, createErr := os.Create(newFilePath)
		if createErr != nil {
			fmt.Println("Could not open create uncompressed file - " + createErr.Error())
			errorList = append(errorList, createErr)
		}
		_, uncompressErr := io.Copy(uncompressedFile, rc)
		if uncompressErr != nil {
			fmt.Println("Could not copy uncompressed file into directory - " + uncompressErr.Error())
			errorList = append(errorList, uncompressErr)
		}
	}

	if len(errorList) == 0 {
		return true, nil
	} else {
		return false, errorList
	}

}

func Sanitize(input interface{}) string {
	if input == nil {
		return ""
	}
	return strings.ReplaceAll(input.(string), "\n", "")
}
