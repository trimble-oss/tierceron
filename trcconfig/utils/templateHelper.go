package utils

import (
	"bytes"
	"encoding/base64"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"strings"
	"text/template"

	"tierceron/utils"
	"tierceron/vaulthelper/kv"
)

//GetProjectService - returns project, service, and path to template on filesystem.
// templateFile - full path to template file
// returns project, service, templatePath
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

	return project, service, templateFile
}

// GetTemplate makes a request to the vault for the template found in <project>/<service>/<file>/template-file
// Returns the template data in base64 and the template's extension. Returns any errors generated by vault
func GetTemplate(modifier *kv.Modifier, templatePath string) (string, error) {
	// Get template data from information in request.
	//  ./vault_templates/ServiceTech/ServiceTechAPIM/config.yml.tmpl
	project, service, templateFile := GetProjectService(templatePath)

	templateFile = templateFile[0 : len(templateFile)-len(".tmpl")]
	if strings.HasSuffix(templateFile, ".yml") {
		templateFile = templateFile[0 : len(templateFile)-len(".yml")]
	} else {
		lastDotIndex := strings.LastIndex(templateFile, ".")
		if lastDotIndex > 0 {
			templateFile = templateFile[0:lastDotIndex]
		}
	}

	path := "templates/" + project + "/" + service + "/" + templateFile + "/template-file"
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

//ConfigTemplateRaw - gets a raw unpopulated template.
func ConfigTemplateRaw(modifier *kv.Modifier, emptyFilePath string, configuredFilePath string, secretMode bool, project string, service string, cert bool, zc bool) ([]byte, error) {
	var err error

	var templateEncoded string
	templateEncoded, err = GetTemplate(modifier, emptyFilePath)
	utils.CheckError(err, true)
	templateBytes, decodeErr := base64.StdEncoding.DecodeString(templateEncoded)
	utils.CheckError(decodeErr, true)

	return templateBytes, decodeErr
}

//ConfigTemplate takes a modifier object, a file path where the template is located, the target path, and two maps of data to populate the template with.
//It configures the template and writes it to the specified file path.
func ConfigTemplate(modifier *kv.Modifier, emptyFilePath string, secretMode bool, project string, service string, cert bool, zc bool) (string, map[int]string, bool) {
	var template string
	var err error

	if zc {
		var templateEncoded string
		templateEncoded, err = GetTemplate(modifier, emptyFilePath)
		utils.CheckError(err, true)
		templateBytes, decodeErr := base64.StdEncoding.DecodeString(templateEncoded)
		utils.CheckError(decodeErr, true)

		template = string(templateBytes)
	} else {
		emptyTemplate, err := ioutil.ReadFile(emptyFilePath)
		utils.CheckError(err, true)
		template = string(emptyTemplate)
	}
	// cert map
	certData := make(map[int]string)
	if cert && !strings.Contains(template, ".certData") {
		return "", certData, false
	}

	// Construct path for vault
	s := strings.Split(emptyFilePath, "/")

	// Remove file extensions
	filename := s[len(s)-1][0:strings.LastIndex(s[len(s)-1], ".")]

	extra := ""
	// Please rework... Urg...
	for i, component := range s {
		if component == "trc_templates" {
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
	if strings.Index(filename, ".") != -1 {
		filename = filename[0:strings.Index(filename, ".")]
	}

	if extra != "" {
		filename = extra + "/" + filename
	}
	//populate template
	template, certData = PopulateTemplate(template, modifier, secretMode, project, service, filename, cert)
	return template, certData, true
}

func getTemplateVersionData(modifier *kv.Modifier, secretMode bool, project string, service string, file string) map[string]interface{} {
	cds := new(ConfigDataStore)
	versionData := cds.InitTemplateVersionData(modifier, secretMode, true, project, file, service)
	return versionData
}

//PopulateTemplate takes an empty template and a modifier.
//It populates the template and returns it in a string.
func PopulateTemplate(emptyTemplate string, modifier *kv.Modifier, secretMode bool, project string, service string, filename string, cert bool) (string, map[int]string) {
	str := emptyTemplate
	cds := new(ConfigDataStore)
	cds.Init(modifier, secretMode, true, project, service)
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
			panic(err)
		}
		var doc bytes.Buffer
		//configure the template

		//Check if filename exists in values map

		_, data := values[filename]
		if data == false {
			fmt.Println(filename + " does not exist in values. Please check seed files to verify that folder structures are correct.")
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
						fmt.Println("No certDestPath in config template section of seed for this service. Unable to generate cert.pfx")
						os.Exit(1)
					}
					certData[0] = certDestPath.(string)
					data, ok := valueData["certData"].(interface{})
					if !ok {
						fmt.Println("No certData in config template section of seed for this service. Unable to generate cert.pfx")
						os.Exit(1)
					}
					encoded := fmt.Sprintf("%s", data)
					//Decode cert as it was encoded in trcinit
					decoded, err := base64.StdEncoding.DecodeString(encoded)
					if err != nil {
						panic(err)
					}
					certData[1] = fmt.Sprintf("%s", decoded)
					certData[2] = certSourcePath.(string)
					return "", certData
				}
			}
		}
		err = t.Execute(&doc, values[filename])
		str = doc.String()
		if err != nil {
			panic(err)
		}
	}
	return str, certData
}
