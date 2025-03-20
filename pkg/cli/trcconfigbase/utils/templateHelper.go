package utils

import (
	"bytes"
	"encoding/base64"
	"errors"
	"fmt"
	"os"
	"strings"
	"text/template"

	"github.com/trimble-oss/tierceron/atrium/vestibulum/trcdb/opts/prod"
	"github.com/trimble-oss/tierceron/buildopts/coreopts"
	"github.com/trimble-oss/tierceron/pkg/core"
	"github.com/trimble-oss/tierceron/pkg/utils"
	eUtils "github.com/trimble-oss/tierceron/pkg/utils"
	"github.com/trimble-oss/tierceron/pkg/utils/config"
	"github.com/trimble-oss/tierceron/pkg/validator"
	helperkv "github.com/trimble-oss/tierceron/pkg/vaulthelper/kv"

	"gopkg.in/yaml.v2"
)

// GetTemplate makes a request to the vault for the template found in <project>/<service>/<file>/template-file
// Returns the template data in base64 and the template's extension. Returns any errors generated by vault
func GetTemplate(driverConfig *config.DriverConfig, mod *helperkv.Modifier, templatePath string) (string, error) {
	// Get template data from information in request.
	//  ./trc_templates/Project/Service/configfile.yml.tmpl
	project, service, serviceIndex, templateFile := eUtils.GetProjectService(driverConfig, templatePath)

	// templateFile currently has full path, but we don't want all that...  Scrub it down.
	splitDir := strings.Split(templateFile, "/")
	if serviceIndex < len(splitDir)-1 {
		templateFile = strings.Join(splitDir[serviceIndex+1:], "/")
	} else {
		// Just last entry...
		templateFile = splitDir[len(splitDir)-1]
	}

	if strings.Contains(templateFile, ".tmpl") {
		templateFile = templateFile[0 : len(templateFile)-len(".tmpl")]
		if strings.HasSuffix(templateFile, ".yml") {
			templateFile = templateFile[0 : len(templateFile)-len(".yml")]
		} else {
			lastDotIndex := strings.LastIndex(templateFile, ".")
			if lastDotIndex > 0 {
				templateFile = templateFile[0:lastDotIndex]
			}
		}
	}

	var path string

	if project == "Common" {
		// No service for Common project...
		path = "templates/" + project + "/" + templateFile + "/template-file"
	} else {
		if driverConfig.ZeroConfig && mod.TemplatePath != "" && !driverConfig.CoreConfig.WantCerts {
			lastDotIndex := strings.LastIndex(templateFile, ".")
			if lastDotIndex > 0 {
				mod.TemplatePath = mod.TemplatePath + templateFile[lastDotIndex:]
			}

			path = mod.TemplatePath + "/template-file"
		} else {
			path = "templates/" + project + "/" + service + "/" + templateFile + "/template-file"
		}
	}

	data, err := mod.ReadData(path)
	if err != nil {
		if driverConfig.CoreConfig.TokenCache != nil {
			mod.EmptyCache()
			driverConfig.CoreConfig.TokenCache.Clear()
		}
		return "", err
	}
	if data == nil {
		err := errors.New("Trouble with lookup to: " + templatePath + " No file " + templateFile + " under " + project + "/" + service)
		return "", err
	}

	// Return retrieved data in response
	return data["data"].(string), nil
}

// ConfigTemplateRaw - gets a raw unpopulated template.
func ConfigTemplateRaw(driverConfig *config.DriverConfig, mod *helperkv.Modifier, emptyFilePath string, configuredFilePath string, secretMode bool, project string, service string, cert bool, zc bool, exitOnFailure bool) ([]byte, error) {
	var err error

	var templateEncoded string
	templateEncoded, err = GetTemplate(driverConfig, mod, emptyFilePath)
	eUtils.CheckError(driverConfig.CoreConfig, err, exitOnFailure)
	templateBytes, decodeErr := base64.StdEncoding.DecodeString(templateEncoded)
	eUtils.CheckError(driverConfig.CoreConfig, decodeErr, exitOnFailure)

	return templateBytes, decodeErr
}

// ConfigTemplate takes a modifier object, a file path where the template is located, the target path, and two maps of data to populate the template with.
// It configures the template and writes it to the specified file path.
func ConfigTemplate(driverConfig *config.DriverConfig,
	modifier *helperkv.Modifier,
	emptyFilePath string,
	secretMode bool,
	project string,
	service string,
	cert bool,
	zc bool) (string, map[int]string, bool, error) {
	var template string
	var err error

	if !driverConfig.CoreConfig.WantCerts {
		relativeTemplatePathParts := strings.Split(emptyFilePath, coreopts.BuildOptions.GetFolderPrefix(driverConfig.StartDir)+"_templates")
		if len(relativeTemplatePathParts) == 1 {
			driverConfig.CoreConfig.Log.Println("Unable to split relative template path:" + relativeTemplatePathParts[0])
		} else if len(relativeTemplatePathParts) == 0 {
			driverConfig.CoreConfig.Log.Println("Unable to find any relative template path.")
		}
		templatePathTrimmed := eUtils.TrimDotsAfterLastSlash(relativeTemplatePathParts[1])
		modifier.TemplatePath = "templates" + templatePathTrimmed
	} else {
		driverConfig.CoreConfig.Log.Println("Configuring cert")
	}

	if zc {
		var templateEncoded string
		templateEncoded, err = GetTemplate(driverConfig, modifier, emptyFilePath)
		if err != nil {
			return "", nil, false, err
		}
		templateBytes, dcErr := base64.StdEncoding.DecodeString(templateEncoded)
		if dcErr != nil {
			return "", nil, false, dcErr
		}

		template = string(templateBytes)
	} else {
		emptyTemplate, err := os.ReadFile(emptyFilePath)
		eUtils.CheckError(driverConfig.CoreConfig, err, true)
		template = string(emptyTemplate)
	}
	// cert map
	certData := make(map[int]string)
	if cert && !strings.Contains(template, ".certData") {
		return "", certData, false, errors.New("missing .certData")
	} else if !cert && strings.Contains(template, ".certData") {
		return "", certData, false, errors.New("template with cert provided, but cert not requested: " + emptyFilePath)
	}

	// Construct path for vault
	s := strings.Split(emptyFilePath, "/")

	// Remove file extensions
	filename := s[len(s)-1][0:strings.LastIndex(s[len(s)-1], ".")]

	extra := ""
	// Please rework... Urg...
	for i, component := range s {
		if component == coreopts.BuildOptions.GetFolderPrefix(driverConfig.StartDir)+"_templates" {
			extra = ""
			continue
		}
		if component == project || component == service || component == "" || i == (len(s)-1) {
			continue
		}
		if extra == "" {
			extra = "/" + component
		} else {
			extra = extra + "/" + component
		}
	}
	if strings.Contains(filename, ".") {
		filename = filename[0:strings.Index(filename, ".")]
	}

	if extra != "" {
		filename = extra + "/" + filename
	}
	//populate template
	template, certData, err = PopulateTemplate(driverConfig, template, modifier, secretMode, project, service, filename, cert)
	return template, certData, true, err
}

func getTemplateVersionData(config *core.CoreConfig, modifier *helperkv.Modifier, project string, service string, file string) (map[string]interface{}, error) {
	cds := new(ConfigDataStore)
	return cds.InitTemplateVersionData(config, modifier, true, project, file, service)
}

// PopulateTemplate takes an empty template and a modifier.
// It populates the template and returns it in a string.
func PopulateTemplate(driverConfig *config.DriverConfig,
	emptyTemplate string,
	modifier *helperkv.Modifier,
	secretMode bool,
	project string,
	service string,
	filename string,
	cert bool) (string, map[int]string, error) {
	values := make(map[string]interface{}, 0)
	ok := false
	str := emptyTemplate
	cds := new(ConfigDataStore)
	if !utils.RefEquals(driverConfig.CoreConfig.TokenCache.GetToken(fmt.Sprintf("config_token_%s", driverConfig.CoreConfig.EnvBasis)), "novault") {
		cds.Init(driverConfig.CoreConfig, modifier, secretMode, true, project, nil, service)
	} else {
		rawFile, err := os.ReadFile(strings.Split(driverConfig.StartDir[0], coreopts.BuildOptions.GetFolderPrefix(driverConfig.StartDir)+"_")[0] + coreopts.BuildOptions.GetFolderPrefix(driverConfig.StartDir) + "_seeds/" + driverConfig.CoreConfig.Env + "/" + driverConfig.CoreConfig.Env + "_seed.yml")
		if err != nil {
			eUtils.LogErrorObject(driverConfig.CoreConfig, errors.New("unable to open seed file for -novault: "+err.Error()), false)
		}

		var rawYaml interface{}
		err = yaml.Unmarshal(rawFile, &rawYaml)
		if err != nil {
			eUtils.LogErrorAndSafeExit(driverConfig.CoreConfig, err, 1)
		}

		seed, seedOk := rawYaml.(map[interface{}]interface{})
		if !seedOk {
			if driverConfig.NoVault {
				driverConfig.CoreConfig.ExitOnFailure = true
				eUtils.LogAndSafeExit(driverConfig.CoreConfig, "novault option requires a seed file for trcconfig to function", 1)
			} else {
				eUtils.LogAndSafeExit(driverConfig.CoreConfig, "Invalid yaml file.  Refusing to continue.", 1)
			}
		}
		tempMap := make(map[string]interface{}, 0)
		for seedSectionKey, seedSection := range seed {
			if seedSectionKey.(string) == "templates" {
				continue
			}
			for _, seedSubSection := range seedSection.(map[interface{}]interface{}) {
				for k, v := range seedSubSection.(map[interface{}]interface{}) {
					values[k.(string)] = v
				}
			}
		}
		if len(values) == 0 {
			eUtils.LogAndSafeExit(driverConfig.CoreConfig, "Invalid yaml file.  Refusing to continue.", 1)
		}
		tempMap[filename] = values
		values = tempMap
		ok = true
	}
	certData := make(map[int]string)
	serviceLookup := service
	i := strings.Index(service, ".")
	if i > 0 {
		serviceLookup = service[:i]
	}

	if len(values) == 0 {
		values, ok = cds.dataMap[serviceLookup].(map[string]interface{})
	}

	if ok {
		//create new template from template string
		t := template.New("template")
		t, err := t.Parse(emptyTemplate)
		if err != nil {
			eUtils.LogErrorObject(driverConfig.CoreConfig, err, false)
		}
		var doc bytes.Buffer
		//configure the template

		//Check if filename exists in values map

		_, hasData := values[filename]
		if !hasData && !driverConfig.CoreConfig.WantCerts {
			eUtils.LogInfo(driverConfig.CoreConfig, filename+" does not exist in values. Please check seed files to verify that folder structures are correct.")
		}

		if len(cds.Regions) > 0 {
			if serviceValues, ok := values[filename]; ok {
				valueData := serviceValues.(map[string]interface{})
				for valueKey, valueEntry := range valueData {
					regionSuffix := "~" + cds.Regions[0]
					if strings.HasSuffix(valueKey, regionSuffix) {
						baseKey := strings.Replace(valueKey, regionSuffix, "", 1)

						if _, ok := valueData[baseKey]; ok {
							valueData[baseKey] = valueEntry
						}
					}
				}
			}
		}

		if cert {
			if serviceValues, ok := values[serviceLookup]; ok {
				valueData := serviceValues.(map[string]interface{})
				certDestPath, hasCertDefinition := valueData["certDestPath"]
				certSourcePath, hasCertSourcePath := valueData["certSourcePath"]
				certPasswordVaultPath, hasCertPasswordVaultPath := valueData["certPasswordVaultPath"]
				certBundleJks, hasCertBundleJks := valueData["certBundleJks"]

				if driverConfig.CertPathOverrides[filename] != "" {
					certDestPath = driverConfig.CertPathOverrides[filename]
				}

				if hasCertDefinition && hasCertSourcePath {
					if !ok {
						vaultCertErr := errors.New("No certDestPath in config template section of seed for this service. Unable to generate: " + certDestPath.(string))
						eUtils.LogErrorMessage(driverConfig.CoreConfig, vaultCertErr.Error(), false)
						return "", nil, vaultCertErr
					}
					certData[0] = certDestPath.(string)
					data, ok := valueData["certData"]
					if !ok {
						vaultCertErr := errors.New("No certData in config template section of seed for this service. Unable to generate: " + certDestPath.(string))
						eUtils.LogInfo(driverConfig.CoreConfig, vaultCertErr.Error())
						return "", nil, vaultCertErr
					}
					encoded := fmt.Sprintf("%s", data)
					//Decode cert as it was encoded in trcinit
					decoded, err := base64.StdEncoding.DecodeString(encoded)
					if err != nil {
						eUtils.LogErrorObject(driverConfig.CoreConfig, err, false)
					}

					// Add support for jks encoding...
					if hasCertBundleJks && driverConfig.WantKeystore != "" {
						certPassword := ""
						if hasCertPasswordVaultPath {
							if certPasswordVaultPath != "" {
								// TODO: Take path defined here and look up in vault the password to use to decrypt a cert.
								// certPassword = from vault...
								certPasswordVaultPath = ""
							}
						}
						// This needs to be wrapped in a jks first.
						ksErr := validator.AddToKeystore(driverConfig, certSourcePath.(string), []byte(certPassword), certBundleJks.(string), decoded)
						if ksErr != nil {
							eUtils.LogErrorObject(driverConfig.CoreConfig, err, false)
							return "", nil, ksErr
						} else {
							return "", nil, nil
						}
					} else {
						certData[1] = string(decoded)
					}

					certData[2] = certSourcePath.(string)
					return "", certData, nil
				}
			}
		}

		if !prod.IsProd() {
			// Override trcEnvParam if it was specified in original call
			data, exists := values[filename].(map[string]interface{})
			if exists {
				data["trcEnvParam"] = &driverConfig.CoreConfig.Env
				values[filename] = data
			}
		}

		err = t.Execute(&doc, values[filename])
		str = doc.String()
		if err != nil {
			eUtils.LogErrorObject(driverConfig.CoreConfig, err, false)
		}
	}
	return str, certData, nil
}
