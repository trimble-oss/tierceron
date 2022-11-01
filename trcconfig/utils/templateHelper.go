package utils

import (
	"bytes"
	"encoding/base64"
	"errors"
	"fmt"
	"io/ioutil"
	"strings"
	"text/template"

	"tierceron/buildopts/coreopts"
	eUtils "tierceron/utils"
	"tierceron/validator"
	helperkv "tierceron/vaulthelper/kv"
)

// GetProjectService - returns project, service, and path to template on filesystem.
// templateFile - full path to template file
// returns project, service, templatePath
func GetProjectService(templateFile string) (string, string, string) {
	templateFile = strings.ReplaceAll(templateFile, "\\", "/")
	splitDir := strings.Split(templateFile, "/")
	var project, service string
	offsetBase := 0

	for i, component := range splitDir {
		if component == coreopts.GetFolderPrefix()+"_templates" {
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

// GetTemplate makes a request to the vault for the template found in <project>/<service>/<file>/template-file
// Returns the template data in base64 and the template's extension. Returns any errors generated by vault
func GetTemplate(modifier *helperkv.Modifier, templatePath string) (string, error) {
	// Get template data from information in request.
	//  ./vault_templates/ServiceTech/ServiceTechAPIM/config.yml.tmpl
	project, service, templateFile := GetProjectService(templatePath)

	// templateFile currently has full path, but we don't want all that...  Scrub it down.
	splitDir := strings.Split(templateFile, "/")
	templateFile = splitDir[len(splitDir)-1]

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
		path = "templates/" + project + "/" + service + "/" + templateFile + "/template-file"
	}
	data, err := modifier.ReadData(path)
	if err != nil {
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
func ConfigTemplateRaw(config *eUtils.DriverConfig, mod *helperkv.Modifier, emptyFilePath string, configuredFilePath string, secretMode bool, project string, service string, cert bool, zc bool, exitOnFailure bool) ([]byte, error) {
	var err error

	var templateEncoded string
	templateEncoded, err = GetTemplate(mod, emptyFilePath)
	eUtils.CheckError(config, err, exitOnFailure)
	templateBytes, decodeErr := base64.StdEncoding.DecodeString(templateEncoded)
	eUtils.CheckError(config, decodeErr, exitOnFailure)

	return templateBytes, decodeErr
}

// ConfigTemplate takes a modifier object, a file path where the template is located, the target path, and two maps of data to populate the template with.
// It configures the template and writes it to the specified file path.
func ConfigTemplate(config *eUtils.DriverConfig,
	modifier *helperkv.Modifier,
	emptyFilePath string,
	secretMode bool,
	project string,
	service string,
	cert bool,
	zc bool) (string, map[int]string, bool, error) {
	var template string
	var err error

	if !config.WantCerts {
		relativeTemplatePathParts := strings.Split(emptyFilePath, coreopts.GetFolderPrefix()+"_templates")
		templatePathParts := strings.Split(relativeTemplatePathParts[1], ".")
		modifier.TemplatePath = "templates" + templatePathParts[0]
	} else {
		config.Log.Println("Configuring cert")
	}

	if zc {
		var templateEncoded string
		templateEncoded, err = GetTemplate(modifier, emptyFilePath)
		if err != nil {
			return "", nil, false, err
		}
		templateBytes, dcErr := base64.StdEncoding.DecodeString(templateEncoded)
		if dcErr != nil {
			return "", nil, false, dcErr
		}

		template = string(templateBytes)
	} else {
		emptyTemplate, err := ioutil.ReadFile(emptyFilePath)
		eUtils.CheckError(config, err, true)
		template = string(emptyTemplate)
	}
	// cert map
	certData := make(map[int]string)
	if cert && !strings.Contains(template, ".certData") {
		return "", certData, false, errors.New("Missing .certData")
	} else if !cert && strings.Contains(template, ".certData") {
		return "", certData, false, errors.New("Template with cert provided, but cert not requested.")
	}

	// Construct path for vault
	s := strings.Split(emptyFilePath, "/")

	// Remove file extensions
	filename := s[len(s)-1][0:strings.LastIndex(s[len(s)-1], ".")]

	extra := ""
	// Please rework... Urg...
	for i, component := range s {
		if component == coreopts.GetFolderPrefix()+"_templates" {
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
	template, certData, err = PopulateTemplate(config, template, modifier, secretMode, project, service, filename, cert)
	return template, certData, true, err
}

func getTemplateVersionData(config *eUtils.DriverConfig, modifier *helperkv.Modifier, project string, service string, file string) (map[string]interface{}, error) {
	cds := new(ConfigDataStore)
	return cds.InitTemplateVersionData(config, modifier, true, project, file, service)
}

// PopulateTemplate takes an empty template and a modifier.
// It populates the template and returns it in a string.
func PopulateTemplate(config *eUtils.DriverConfig,
	emptyTemplate string,
	modifier *helperkv.Modifier,
	secretMode bool,
	project string,
	service string,
	filename string,
	cert bool) (string, map[int]string, error) {
	str := emptyTemplate
	cds := new(ConfigDataStore)
	if config.Token != "novault" {
		cds.Init(config, modifier, secretMode, true, project, nil, service)
	}
	certData := make(map[int]string)
	serviceLookup := service
	i := strings.Index(service, ".")
	if i > 0 {
		serviceLookup = service[:i]
	}
	values, ok := cds.dataMap[serviceLookup].(map[string]interface{})

	if ok {

		//create new template from template string
		t := template.New("template")
		t, err := t.Parse(emptyTemplate)
		if err != nil {
			eUtils.LogErrorObject(config, err, false)
		}
		var doc bytes.Buffer
		//configure the template

		//Check if filename exists in values map

		_, data := values[filename]
		if data == false && !config.WantCerts {
			eUtils.LogInfo(config, filename+" does not exist in values. Please check seed files to verify that folder structures are correct.")
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
				certDestPath, hasCertDefinition := valueData["certDestPath"].(interface{})
				certSourcePath, hasCertSourcePath := valueData["certSourcePath"].(interface{})
				if hasCertDefinition && hasCertSourcePath {
					if !ok {
						vaultCertErr := errors.New("No certDestPath in config template section of seed for this service. Unable to generate cert.pfx")
						eUtils.LogErrorMessage(config, vaultCertErr.Error(), false)
						return "", nil, vaultCertErr
					}
					certData[0] = certDestPath.(string)
					data, ok := valueData["certData"].(interface{})
					if !ok {
						vaultCertErr := errors.New("No certData in config template section of seed for this service. Unable to generate cert.pfx")
						eUtils.LogInfo(config, vaultCertErr.Error())
						return "", nil, vaultCertErr
					}
					encoded := fmt.Sprintf("%s", data)
					//Decode cert as it was encoded in trcinit
					decoded, err := base64.StdEncoding.DecodeString(encoded)
					if err != nil {
						eUtils.LogErrorObject(config, err, false)
					}

					// Add support for jks encoding...
					if config.WantKeystore != "" {
						// This needs to be wrapped in a jks first.
						ksErr := validator.AddToKeystore(config, certSourcePath.(string), decoded)
						if ksErr != nil {
							eUtils.LogErrorObject(config, err, false)
							return "", nil, ksErr
						}
					} else {
						certData[1] = fmt.Sprintf("%s", decoded)
					}

					certData[2] = certSourcePath.(string)
					return "", certData, nil
				}
			}
		}
		err = t.Execute(&doc, values[filename])
		str = doc.String()
		if err != nil {
			eUtils.LogErrorObject(config, err, false)
		}
	}
	return str, certData, nil
}
