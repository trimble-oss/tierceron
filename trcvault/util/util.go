package util

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	eUtils "github.com/trimble-oss/tierceron/utils"
	helperkv "github.com/trimble-oss/tierceron/vaulthelper/kv"

	"gopkg.in/yaml.v2"

	vcutils "github.com/trimble-oss/tierceron/trcconfigbase/utils"
	"github.com/trimble-oss/tierceron/trcx/extract"
	"github.com/trimble-oss/tierceron/trcx/xutil"

	il "github.com/trimble-oss/tierceron/trcinit/initlib"

	"log"
)

type ProcessFlowConfig func(pluginEnvConfig map[string]interface{}) map[string]interface{}
type ProcessFlowInitConfig func(pluginConfig map[string]interface{}, logger *log.Logger) error
type ProcessFlowFunc func(pluginConfig map[string]interface{}, logger *log.Logger) error

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

func GetJSONFromClientByGet(config *eUtils.DriverConfig, httpClient *http.Client, headers map[string]string, address string, body io.Reader) (map[string]interface{}, int, error) {
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

	if response.StatusCode == http.StatusOK {
		jsonDataFromHttp, err := io.ReadAll(response.Body)

		if err != nil {
			return nil, response.StatusCode, err
		}

		err = json.Unmarshal([]byte(jsonDataFromHttp), &jsonData)

		if err != nil {
			eUtils.LogErrorObject(config, err, false)
		}

		return jsonData, response.StatusCode, nil
	} else if response.StatusCode == http.StatusNoContent {
		return jsonData, response.StatusCode, nil
	}

	return nil, response.StatusCode, errors.New("http status failure")
}

func GetJSONFromClientByPost(config *eUtils.DriverConfig, httpClient *http.Client, headers map[string]string, address string, body io.Reader) (map[string]interface{}, int, error) {
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
		return nil, response.StatusCode, err
	}
	if response.Body != nil {
		defer response.Body.Close()
	}

	if response.StatusCode == http.StatusUnauthorized {
		return nil, response.StatusCode, errors.New(fmt.Sprintf("http auth failure: %d", response.StatusCode))
	} else if response.StatusCode == http.StatusOK {
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
	return nil, response.StatusCode, errors.New(fmt.Sprintf("http status failure: %d", response.StatusCode))
}

func LoadBaseTemplate(config *eUtils.DriverConfig, templateResult *extract.TemplateResultData, goMod *helperkv.Modifier, project string, service string, templatePath string) error {
	templateResult.ValueSection = map[string]map[string]map[string]string{}
	templateResult.ValueSection["values"] = map[string]map[string]string{}

	templateResult.SecretSection = map[string]map[string]map[string]string{}
	templateResult.SecretSection["super-secrets"] = map[string]map[string]string{}

	var cds *vcutils.ConfigDataStore
	commonPaths := make([]string, 1)
	if goMod != nil {
		cds = new(vcutils.ConfigDataStore)
		goMod.Version = goMod.Version + "***X-Mode"
		cds.Init(config, goMod, true, true, project, commonPaths, service) //CommonPaths = "" - empty - not needed for tenant config
	}

	var errSeed error
	_, _, _, templateResult.TemplateDepth, errSeed = extract.ToSeed(config, goMod,
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

func SeedVaultById(config *eUtils.DriverConfig, goMod *helperkv.Modifier, service string, address string, token string, baseTemplate *extract.TemplateResultData, tableData map[string]interface{}, indexPath string, project string) error {
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

	xutil.CombineSection(config, sliceTemplateSection, maxDepth, templateCombinedSection)
	xutil.CombineSection(config, sliceValueSection, -1, valueCombinedSection)
	xutil.CombineSection(config, sliceSecretSection, -1, secretCombinedSection)

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
		config.ServicesWanted = []string{""}
		config.WantCerts = false
		il.SeedVaultFromData(config, indexPath, []byte(seedData))
	} else {
		config.ServicesWanted = []string{service}
		config.WantCerts = false
		il.SeedVaultFromData(config, "Index/"+project+indexPath, []byte(seedData))
	}
	return nil
}

func GetPluginToolConfig(config *eUtils.DriverConfig, mod *helperkv.Modifier, pluginConfig map[string]interface{}) (map[string]interface{}, error) {
	config.Log.Println("GetPluginToolConfig begin processing plugins.")
	//templatePaths
	indexFound := false
	templatePaths := pluginConfig["templatePath"].([]string)

	pluginToolConfig, err := mod.ReadData("super-secrets/Restricted/PluginTool/config")

	if err != nil {
		return nil, err
	} else {
		if len(pluginToolConfig) == 0 {
			return nil, errors.New("Tierceron plugin management presently not configured for env: " + mod.Env)
		}
	}
	for k, v := range pluginConfig {
		pluginToolConfig[k] = v
	}

	var ptc1 map[string]interface{}

	for _, templatePath := range templatePaths {
		project, service, _ := eUtils.GetProjectService(templatePath)
		config.Log.Println("GetPluginToolConfig project: " + project + " plugin: " + config.SubSectionValue + " service: " + service)

		if pluginPath, pathOk := pluginToolConfig["pluginpath"]; pathOk && len(pluginPath.(string)) != 0 {
			mod.SectionPath = "super-secrets/Index/" + project + pluginPath.(string) + config.SubSectionValue + "/" + service
		} else {
			mod.SectionPath = "super-secrets/Index/" + project + "/trcplugin/" + config.SubSectionValue + "/" + service
		}
		ptc1, err = mod.ReadData(mod.SectionPath)
		pluginToolConfig["pluginpath"] = mod.SectionPath
		if err != nil || ptc1 == nil {
			config.Log.Println("No data found.")
			continue
		}
		indexFound = true
		for k, v := range ptc1 {
			pluginToolConfig[k] = v
		}
		break
	}
	mod.SectionPath = ""

	if pluginToolConfig == nil {
		config.Log.Println("No data found for plugin.")
		if err == nil {
			err = errors.New("No data and unexpected error.")
		}
		return pluginToolConfig, err
	} else if !indexFound {
		return pluginToolConfig, nil
	}
	config.Log.Println("GetPluginToolConfig end processing plugins.")
	if strings.ContainsAny(pluginToolConfig["trcplugin"].(string), "./") {
		err = errors.New("Invalid plugin configuration: " + pluginToolConfig["trcplugin"].(string))
		return nil, err
	}

	return pluginToolConfig, nil
}
