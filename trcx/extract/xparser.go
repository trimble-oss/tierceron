package extract

import (
	"errors"
	"fmt"
	"html/template"
	"io/ioutil"
	"strings"
	"text/template/parse"

	"github.com/trimble-oss/tierceron/buildopts/coreopts"
	eUtils "github.com/trimble-oss/tierceron/utils"

	vcutils "github.com/trimble-oss/tierceron/trcconfigbase/utils"
	helperkv "github.com/trimble-oss/tierceron/vaulthelper/kv"
)

const (
	defaultSecret = "<Enter Secret Here>"
)

type TemplateResultData struct {
	InterfaceTemplateSection interface{}
	ValueSection             map[string]map[string]map[string]string
	SecretSection            map[string]map[string]map[string]string
	TemplateDepth            int
	Env                      string
	SubSectionValue          string
	SectionPath              string // Where the data came from in vault
}

// ToSeed parses a <foo>.yml.tmpl file into a <foo>.yml file which then can be used for seeding vault
// Input:
//   - Directory location of .tmpl file
//   - Log file for logging support information
//
// Output:
//   - Parsed string containing the .yml file
func ToSeed(config *eUtils.DriverConfig, mod *helperkv.Modifier,
	cds *vcutils.ConfigDataStore,
	templatePath string,
	project string,
	service string,
	templateFromVault bool,
	interfaceTemplateSection *interface{},
	valueSection *map[string]map[string]map[string]string,
	secretSection *map[string]map[string]map[string]string,
) (*interface{}, *map[string]map[string]map[string]string, *map[string]map[string]map[string]string, int, error) {

	// TODO: replace string sections with maps
	templatePath = strings.ReplaceAll(templatePath, "\\", "/")
	pathSlice := strings.SplitN(templatePath, "/", -1)

	// Initialize map subsections
	templatePathSlice, templateDir, templateDepth := GetInitialTemplateStructure(pathSlice)

	// Gets the template file
	var newTemplate string
	if templateFromVault {
		templatePathExtended := ""
		serviceRaw := service
		if project == "Common" {
			templatePathExtended = templatePath
			serviceRaw = ""
		} else {
			templatePathExtended = strings.Replace(templatePath, coreopts.GetFolderPrefix()+"_templates/", "/", 1)
		}
		configuredFilePath := "./"
		templateFile, _ := vcutils.ConfigTemplateRaw(config, mod, templatePathExtended, configuredFilePath, true, project, serviceRaw, false, true, config.ExitOnFailure)
		newTemplate = string(templateFile)
	} else {
		templateFile, err := ioutil.ReadFile(templatePath)
		newTemplate = string(templateFile)
		if err != nil {
			return nil, nil, nil, 0, eUtils.LogAndSafeExit(config, err.Error(), -1)
		}
	}

	// Parse template
	t := template.New("template")
	theTemplate, err := t.Parse(newTemplate)
	if err != nil {
		return nil, nil, nil, 0, eUtils.LogAndSafeExit(config, err.Error(), -1)
	}
	commandList := theTemplate.Tree.Root

	for _, node := range commandList.Nodes {
		if node.Type() == parse.NodeAction {
			var args []string
			fields := node.(*parse.ActionNode).Pipe
			for _, arg := range fields.Cmds[0].Args {
				templateParameter := strings.ReplaceAll(arg.String(), "\\\"", "\"")
				if strings.Contains(templateParameter, "~") {
					eUtils.LogInfo(config, "Unsupported parameter name character ~: "+templateParameter)
					return nil, nil, nil, 0, errors.New("Unsupported parameter name character ~: " + templateParameter)
				}
				args = append(args, templateParameter)
			}

			// Gets the parsed file line
			errParse := Parse(config, cds,
				args,
				pathSlice[len(pathSlice)-2],
				templatePathSlice,
				templateDir,
				templateDepth,
				service,
				interfaceTemplateSection,
				valueSection,
				secretSection,
			)
			if errParse != nil {
				return nil, nil, nil, 0, errParse
			}
		}
	}

	return interfaceTemplateSection, valueSection, secretSection, templateDepth, nil
}

// GetInitialTemplateStructure Initializes the structure of the template section using the template directory path
// Input:
//   - A slice of the template file path delimited by "/"
//
// Output:
//   - String(s) containing the structure of the template section
func GetInitialTemplateStructure(templatePathSlice []string) ([]string, int, int) {

	var templateDir int
	var templateDepth int

	// Remove the file format from the name of the template file
	if strings.Index(templatePathSlice[len(templatePathSlice)-1], ".") >= 0 {
		idxFileFormat := strings.Index(templatePathSlice[len(templatePathSlice)-1], ".")
		templatePathSlice[len(templatePathSlice)-1] = templatePathSlice[len(templatePathSlice)-1][:idxFileFormat]
	}

	// Find the index in the slice of the vault_template subdirectory
	for i, folder := range templatePathSlice {
		if folder == coreopts.GetFolderPrefix()+"_templates" {
			templateDir = i
			templatePathSlice[i] = "templates"
		}
	}

	templateDepth = len(templatePathSlice) - templateDir
	return templatePathSlice, templateDir, templateDepth
}

func parseAndSetSection(cds *vcutils.ConfigDataStore,
	sectionMap *map[string]map[string]map[string]string,
	sectionType string,
	service string,
	keyPath []string,
	keyName string,
	value string,
	existingDefault string) {

	var okValue bool
	var existingValue string

	if _, ok := (*sectionMap)[sectionType][service]; ok {
		existingValue, okValue = (*sectionMap)[sectionType][service][keyName]
	}
	if keyName == "certData" {
		value = "data"
	} else {
		if cds != nil {
			// Load from Config Data Store.
			vaultValue, vaultError := cds.GetValue(service, keyPath, keyName)
			if vaultError == nil {
				value = vaultValue
				okValue = true
			}
		}
	}

	if !okValue {
		if strings.Contains(keyName, "~") {
			// No override, then skip.
			return
		}
	}

	if _, ok := (*sectionMap)[sectionType][service]; ok {
		if !okValue {
			(*sectionMap)[sectionType][service][keyName] = value
		} else {
			if existingValue == existingDefault || existingValue == "" {
				(*sectionMap)[sectionType][service][keyName] = value
			}
		}
	} else {
		(*sectionMap)[sectionType][service] = map[string]string{}
		(*sectionMap)[sectionType][service][keyName] = value
	}
}

// Parse Parses a .tmpl file line into .yml file line(s)
// Input:
//   - .tmlp file line
//   - The current template directory
//
// Output:
//   - String(s) containing the .yml file subsections
func Parse(config *eUtils.DriverConfig, cds *vcutils.ConfigDataStore,
	args []string,
	currentDir string,
	templatePathSlice []string,
	templateDir int,
	templateDepth int,
	service string,
	interfaceTemplateSection *interface{},
	valueSection *map[string]map[string]map[string]string,
	secretSection *map[string]map[string]map[string]string,
) error {
	if len(args) == 3 { //value
		keySlice := args[1]
		keyName := keySlice[1:]
		valueSlice := args[2]
		var value string
		if len(valueSlice) > 1 {
			value = valueSlice[1 : len(valueSlice)-1]
		} else {
			value = valueSlice
		}
		fileOffsetIndex := 3
		if templatePathSlice[templateDir+1] == "Common" {
			fileOffsetIndex = 2
		}
		keyPath := templatePathSlice[templateDir+fileOffsetIndex : len(templatePathSlice)]

		AppendToTemplateSection(interfaceTemplateSection,
			valueSection,
			secretSection,
			templatePathSlice,
			templateDir,
			templateDepth,
			false,
			keyName,
			service,
		)

		parseAndSetSection(cds,
			valueSection,
			"values",
			service,
			keyPath,
			keyName,
			value,
			defaultSecret)

		if cds != nil {
			for _, region := range cds.Regions {
				parseAndSetSection(cds,
					valueSection,
					"values",
					service,
					keyPath,
					keyName+"~"+region,
					value,
					defaultSecret)
			}

		}
	} else if len(args) == 1 { //super-secrets
		// Get the secret name
		keySlice := args[0]
		keyName := keySlice[1:]
		keyFileIndex := templateDir + 3
		if len(templatePathSlice)-1 < keyFileIndex {
			keyFileIndex = len(templatePathSlice) - 1
		}
		keyPath := templatePathSlice[keyFileIndex:len(templatePathSlice)]
		secret := defaultSecret
		if keyName == "certData" {
			secret = "data"
		}

		// Add parsed line to appropriate line sections
		AppendToTemplateSection(interfaceTemplateSection,
			valueSection,
			secretSection,
			templatePathSlice, templateDir, templateDepth, true, keyName, service)

		// Sets the secret from Config Data Store.
		parseAndSetSection(cds,
			secretSection,
			"super-secrets",
			service,
			keyPath,
			keyName,
			secret,
			defaultSecret)

		if cds != nil {
			for _, region := range cds.Regions {
				parseAndSetSection(cds,
					secretSection,
					"super-secrets",
					service,
					keyPath,
					keyName+"~"+region,
					secret,
					defaultSecret)
			}
		}
	} else {
		parseMsg := fmt.Sprintf("Template: %s Incorrect template element count: %d Syntax error: %v", templatePathSlice[templateDir+3:len(templatePathSlice)], len(args), args)
		return eUtils.LogAndSafeExit(config, parseMsg, 1)
	}
	return nil
}

// AppendToTemplateSection Add parse line to template section
func AppendToTemplateSection(
	interfaceTemplateSection *interface{},
	valueSection *map[string]map[string]map[string]string,
	secretSection *map[string]map[string]map[string]string,
	templatePathSlice []string,
	templateDir int,
	templateDepth int,
	isSecret bool,
	name ...string) {
	subSection := "[values/"
	if isSecret {
		subSection = "[super-secrets/"
	}

	wholeName := true
	if len(name) == 2 {
		wholeName = false
	}
	if _, ok := (*interfaceTemplateSection).(map[string]interface{}); !ok {
		*interfaceTemplateSection = map[string]interface{}{}
	}

	itLevel := (*interfaceTemplateSection).(map[string]interface{})

	for i := templateDir; i < len(templatePathSlice); i++ {
		currentEntry := templatePathSlice[i]
		if _, ok := itLevel[currentEntry].(map[string]interface{}); !ok {
			itLevel[currentEntry] = map[string]interface{}{}
		}
		itLevel = itLevel[currentEntry].(map[string]interface{})
	}
	//name[0] = keyName, name[1] = service name
	if wholeName {
		itLevel[name[0]] = subSection + name[1] + ", " + name[0] + "]"
	} else {
		itLevel[name[0]] = subSection + name[1] + ", " + name[0] + "]"
	}
}
