package util

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httputil"
	"strconv"
	"strings"

	"tierceron/vaulthelper/kv"
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

func GetLocalVaultHost() (string, error) {
	vaultHost := "https://"
	vaultErr := errors.New("No usable local vault found.")
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

	// Now, look for vault.
	for i := 8190; i < 8300; i++ {
		vh := vaultHost + ":" + strconv.Itoa(i)
		_, err := sys.NewVault(true, vh, "", false, true, true)
		if err == nil {
			vaultHost = vaultHost + ":" + strconv.Itoa(i)
			vaultErr = nil
			break
		}
	}

	return vaultHost, vaultErr
}

func GetJSONFromClient(httpClient *http.Client, headers map[string]string, address string, body io.Reader) map[string]interface{} {
	var jsonData map[string]interface{}
	request, err := http.NewRequest("POST", address, body)
	if err != nil {
		panic(err)
	}
	for headerkey, headervalue := range headers {
		request.Header.Set(headerkey, headervalue)
	}
	// request.Header.Set("Accept", "application/json")
	// request.Header.Set("Content-Type", "application/json")
	response, err := httpClient.Do(request)
	if err != nil {
		panic(err)
	}
	defer response.Body.Close()

	if response.StatusCode == http.StatusOK {
		responseData, err := httputil.DumpResponse(response, true)
		responseString := string(responseData)

		jsonStartIndex := strings.Index(responseString, "{\"")

		jsonDataFromHttp := responseString[jsonStartIndex:]
		//		r.Body = http.MaxBytesReader(w, r.Body, 1048576)

		//		err := json.NewDecoder(response.Body).Decode(&jsonData)

		if err != nil {
			panic(err)
		}

		err = json.Unmarshal([]byte(jsonDataFromHttp), &jsonData)

		if err != nil {
			panic(err)
		}

		return jsonData
	}
	return nil
}

func GetSeedTemplate(templateResult *extract.TemplateResultData, goMod *helperkv.Modifier, project string, service string, templatePath string) {
	templateResult.ValueSection = map[string]map[string]map[string]string{}
	templateResult.ValueSection["values"] = map[string]map[string]string{}

	templateResult.SecretSection = map[string]map[string]map[string]string{}
	templateResult.SecretSection["super-secrets"] = map[string]map[string]string{}

	var cds *vcutils.ConfigDataStore
	commonPaths := make([]string, 1, 1)
	if goMod != nil {
		cds = new(vcutils.ConfigDataStore)
		goMod.Version = goMod.Version + "***X-Mode"
		cds.Init(goMod, true, true, project, commonPaths, service) //CommonPaths = "" - empty - not needed for tenant config
	}

	_, _, _, templateResult.TemplateDepth = extract.ToSeed(goMod,
		cds,
		templatePath,
		&log.Logger{},
		project,
		service,
		true,
		&(templateResult.InterfaceTemplateSection),
		&(templateResult.ValueSection),
		&(templateResult.SecretSection),
	)
}

func SeedVaultWithTenant(templateResult extract.TemplateResultData, goMod *kv.Modifier, tenantConfiguration map[string]string, service string, address string, token string) error {
	valueCombinedSection := map[string]map[string]map[string]string{}
	valueCombinedSection["values"] = map[string]map[string]string{}

	secretCombinedSection := map[string]map[string]map[string]string{}
	secretCombinedSection["super-secrets"] = map[string]map[string]string{}

	// Declare local variables
	templateCombinedSection := map[string]interface{}{}
	sliceTemplateSection := []interface{}{}
	sliceValueSection := []map[string]map[string]map[string]string{}
	sliceSecretSection := []map[string]map[string]map[string]string{}
	for key, value := range tenantConfiguration {
		templateResult.SecretSection["super-secrets"][service][key] = value
	}
	maxDepth := templateResult.TemplateDepth
	// Combine values of slice

	sliceTemplateSection = append(sliceTemplateSection, templateResult.InterfaceTemplateSection)
	sliceValueSection = append(sliceValueSection, templateResult.ValueSection)
	sliceSecretSection = append(sliceSecretSection, templateResult.SecretSection)

	xutil.CombineSection(sliceTemplateSection, maxDepth, templateCombinedSection)
	xutil.CombineSection(sliceValueSection, -1, valueCombinedSection)
	xutil.CombineSection(sliceSecretSection, -1, secretCombinedSection)

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
	il.SeedVaultFromData(true, []byte(seedData), address, token, goMod.Env, log.Default(), service, false, goMod.Env+"."+tenantConfiguration["enterpriseId"])
	return nil
}
