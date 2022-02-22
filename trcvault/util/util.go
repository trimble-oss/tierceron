package util

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"

	"tierceron/utils"
	eUtils "tierceron/utils"
	helperkv "tierceron/vaulthelper/kv"
	sys "tierceron/vaulthelper/system"

	"gopkg.in/yaml.v2"

	vcutils "tierceron/trcconfig/utils"
	extract "tierceron/trcx/extract"

	"github.com/txn2/txeh"

	il "tierceron/trcinit/initlib"
	xutil "tierceron/trcx/xutil"

	"log"
)

func GetLocalVaultHost(withPort bool, logger *log.Logger) (string, error) {
	vaultHost := "https://"
	vaultErr := errors.New("no usable local vault found")
	hostFileLines, pherr := txeh.ParseHosts("/etc/hosts")
	if pherr != nil {
		return "", pherr
	}

	for _, hostFileLine := range hostFileLines {
		for _, host := range hostFileLine.Hostnames {
			if (strings.Contains(host, "whoboot.org") || strings.Contains(host, "dexchadev.org") || strings.Contains(host, "dexterchaney.com")) && strings.Contains(hostFileLine.Address, "127.0.0.1") {
				vaultHost = vaultHost + host
				break
			}
		}
	}

	if withPort {
		// Now, look for vault.
		for i := 8190; i < 8300; i++ {
			vh := vaultHost + ":" + strconv.Itoa(i)
			_, err := sys.NewVault(true, vh, "", false, true, true, logger)
			if err == nil {
				vaultHost = vaultHost + ":" + strconv.Itoa(i)
				vaultErr = nil
				break
			}
		}
	} else {
		vaultErr = nil
	}

	return vaultHost, vaultErr
}

func GetJSONFromClientByGet(httpClient *http.Client, headers map[string]string, address string, body io.Reader, logger *log.Logger) (map[string]interface{}, error) {
	var jsonData map[string]interface{}
	request, err := http.NewRequest("GET", address, body)
	if err != nil {
		eUtils.LogErrorObject(err, logger, false)
	}
	for headerkey, headervalue := range headers {
		request.Header.Set(headerkey, headervalue)
	}
	// request.Header.Set("Accept", "application/json")
	// request.Header.Set("Content-Type", "application/json")
	response, err := httpClient.Do(request)
	if err != nil {
		eUtils.LogErrorObject(err, logger, false)
	}
	defer response.Body.Close()

	if response.StatusCode == http.StatusOK {
		jsonDataFromHttp, err := io.ReadAll(response.Body)

		if err != nil {
			return nil, err
		}

		err = json.Unmarshal([]byte(jsonDataFromHttp), &jsonData)

		if err != nil {
			eUtils.LogErrorObject(err, logger, false)
		}

		return jsonData, nil
	}
	return nil, errors.New("http status failure")
}

func GetJSONFromClientByPost(httpClient *http.Client, headers map[string]string, address string, body io.Reader, logger *log.Logger) (map[string]interface{}, error) {
	var jsonData map[string]interface{}
	request, err := http.NewRequest("POST", address, body)
	if err != nil {
		eUtils.LogErrorObject(err, logger, false)
	}
	for headerkey, headervalue := range headers {
		request.Header.Set(headerkey, headervalue)
	}
	// request.Header.Set("Accept", "application/json")
	// request.Header.Set("Content-Type", "application/json")
	response, err := httpClient.Do(request)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()

	if response.StatusCode == http.StatusOK {
		jsonDataFromHttp, err := io.ReadAll(response.Body)

		if err != nil {
			eUtils.LogErrorObject(err, logger, false)
		}

		err = json.Unmarshal([]byte(jsonDataFromHttp), &jsonData)

		if err != nil {
			eUtils.LogErrorObject(err, logger, false)
		}

		return jsonData, nil
	}
	return nil, errors.New("http status failure")
}

func LoadBaseTemplate(config *utils.DriverConfig, templateResult *extract.TemplateResultData, goMod *helperkv.Modifier, project string, service string, templatePath string) error {
	templateResult.ValueSection = map[string]map[string]map[string]string{}
	templateResult.ValueSection["values"] = map[string]map[string]string{}

	templateResult.SecretSection = map[string]map[string]map[string]string{}
	templateResult.SecretSection["super-secrets"] = map[string]map[string]string{}

	var cds *vcutils.ConfigDataStore
	commonPaths := make([]string, 1)
	if goMod != nil {
		cds = new(vcutils.ConfigDataStore)
		goMod.Version = goMod.Version + "***X-Mode"
		cds.Init(goMod, true, true, project, commonPaths, config.Log, service) //CommonPaths = "" - empty - not needed for tenant config
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

func SeedVaultById(config *utils.DriverConfig, goMod *helperkv.Modifier, service string, address string, token string, baseTemplate *extract.TemplateResultData, tableData map[string]interface{}, indexPath string, logger *log.Logger, project string) error {
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
		templateResult.SecretSection["super-secrets"][service][key] = value.(string)
	}
	maxDepth := templateResult.TemplateDepth
	// Combine values of slice

	sliceTemplateSection = append(sliceTemplateSection, templateResult.InterfaceTemplateSection)
	sliceValueSection = append(sliceValueSection, templateResult.ValueSection)
	sliceSecretSection = append(sliceSecretSection, templateResult.SecretSection)

	xutil.CombineSection(sliceTemplateSection, maxDepth, templateCombinedSection, logger)
	xutil.CombineSection(sliceValueSection, -1, valueCombinedSection, logger)
	xutil.CombineSection(sliceSecretSection, -1, secretCombinedSection, logger)

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
	il.SeedVaultFromData(config, "Index/"+project+indexPath, []byte(seedData), service, false)
	return nil
}
