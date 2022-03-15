package util

import (
	"tierceron/trcconfig/utils"
	eUtils "tierceron/utils"

	"tierceron/vaulthelper/kv"

	sys "tierceron/vaulthelper/system"
)

//Properties stores all configuration properties for a project.
type Properties struct {
	mod          *kv.Modifier
	authMod      *kv.Modifier
	AuthEndpoint string
	cds          *utils.ConfigDataStore
}

func NewProperties(config *eUtils.DriverConfig, v *sys.Vault, mod *kv.Modifier, env string, project string, service string) (*Properties, error) {
	properties := Properties{}
	properties.mod = mod
	properties.mod.Env = env
	if mod.SectionName != "" && mod.SubSectionValue != "" {
		if mod.SectionKey == "/Index/" {
			properties.mod.SectionPath = "super-secrets" + mod.SectionKey + project + "/" + mod.SectionName + "/" + mod.SubSectionValue + "/" + service
		} else {
			properties.mod.SectionPath = "super-secrets" + mod.SectionKey + project + "/" + mod.SectionName + "/" + mod.SubSectionValue
		}
	} else {
		properties.mod.SectionPath = ""
	}
	properties.cds = new(utils.ConfigDataStore)
	var commonPaths []string
	propertyerr := properties.cds.Init(config, properties.mod, true, true, project, commonPaths, service)
	if propertyerr != nil {
		return nil, propertyerr
	}

	return &properties, nil
}

//GetValue gets an invididual configuration value for a service from the data store.
func (p *Properties) GetValue(service string, keyPath []string, key string) (string, error) {
	return p.cds.GetValue(service, keyPath, key)
}

//GetConfigValue gets an invididual configuration value for a service from the data store.
func (p *Properties) GetConfigValue(service string, config string, key string) (string, bool) {
	return p.cds.GetConfigValue(service, config, key)
}

//GetConfigValues gets an invididual configuration value for a service from the data store.
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
	case "staging":
		tokenNamePtr = "config_token_staging"
	default:
		tokenNamePtr = "config_token_local"
	}
	return tokenNamePtr
}
