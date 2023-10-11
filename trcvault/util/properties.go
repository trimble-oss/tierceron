package util

import (
	"log"
	"strings"

	vcutils "github.com/trimble-oss/tierceron/trcconfigbase/utils"
	eUtils "github.com/trimble-oss/tierceron/utils"

	helperkv "github.com/trimble-oss/tierceron/vaulthelper/kv"

	sys "github.com/trimble-oss/tierceron/vaulthelper/system"
)

// Properties stores all configuration properties for a project.
type Properties struct {
	mod          *helperkv.Modifier
	authMod      *helperkv.Modifier
	AuthEndpoint string
	cds          *vcutils.ConfigDataStore
}

func NewProperties(config *eUtils.DriverConfig, v *sys.Vault, mod *helperkv.Modifier, env string, project string, service string) (*Properties, error) {
	properties := Properties{}
	properties.mod = mod
	properties.mod.Env = env
	if mod.SectionName != "" && mod.SubSectionValue != "" {
		if mod.SectionKey == "/Index/" {
			properties.mod.SectionPath = "super-secrets" + mod.SectionKey + project + "/" + mod.SectionName + "/" + mod.SubSectionValue + "/" + service
		} else {
			properties.mod.SectionPath = "super-secrets" + mod.SectionKey + project + "/" + mod.SectionName + "/" + mod.SubSectionValue
		}
	} else if mod.SectionKey == "/Restricted/" || mod.SectionKey == "/Protected/" {
		properties.mod.SectionPath = "super-secrets" + mod.SectionKey + service + "/" + mod.SectionName
		if project == service {
			service = mod.SectionName
		}
	} else {
		properties.mod.SectionPath = ""
	}
	properties.cds = new(vcutils.ConfigDataStore)
	var commonPaths []string
	propertyerr := properties.cds.Init(config, properties.mod, true, true, project, commonPaths, service)
	if propertyerr != nil {
		return nil, propertyerr
	}

	return &properties, nil
}

// GetValue gets an invididual configuration value for a service from the data store.
func (p *Properties) GetValue(service string, keyPath []string, key string) (string, error) {
	return p.cds.GetValue(service, keyPath, key)
}

// GetConfigValue gets an invididual configuration value for a service from the data store.
func (p *Properties) GetConfigValue(service string, config string, key string) (string, bool) {
	return p.cds.GetConfigValue(service, config, key)
}

// GetConfigValues gets an invididual configuration value for a service from the data store.
func (p *Properties) GetConfigValues(service string, config string) (map[string]interface{}, bool) {
	return p.cds.GetConfigValues(service, config)
}

func ResolveTokenName(env string) string {
	tokenNamePtr := ""
	switch env {
	case "local":
		tokenNamePtr = "config_token_local"
	case "dev":
		tokenNamePtr = "config_token_dev"
	case "QA":
		tokenNamePtr = "config_token_QA"
	case "RQA":
		tokenNamePtr = "config_token_RQA"
	case "auto":
		tokenNamePtr = "config_token_auto"
	case "staging":
		tokenNamePtr = "config_token_staging"
	default:
		tokenNamePtr = "config_token_local"
	}
	return tokenNamePtr
}

func (p *Properties) GetPluginData(region string, service string, config string, log *log.Logger) map[string]interface{} {
	valueMap, _ := p.GetConfigValues(service, config)
	//Grabs region fields and replaces into base fields if region is available.
	if region != "" {
		region = "~" + region
		regionFields := make(map[string]interface{})
		for field, value := range valueMap {
			if !strings.Contains(field, region) {
				continue
			} else {
				regionFields[field] = value
			}
		}

		if len(regionFields) == 0 {
			log.Println("Region was found, but no regional data. Continuing with base data.")
		} else {
			for field, value := range regionFields {
				valueMap[strings.TrimSuffix(field, region)] = value
			}

		}
	}

	cleanedUpFields := make(map[string]interface{})
	//Cleans up regional fields
	for field, value := range valueMap {
		if strings.Contains(field, "~") {
			continue
		} else {
			cleanedUpFields[field] = value
		}
	}
	valueMap = cleanedUpFields

	return valueMap
}
