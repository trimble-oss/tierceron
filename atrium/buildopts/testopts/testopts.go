package testopts

import (
	flowcore "github.com/trimble-oss/tierceron-core/v2/flow"
)

func GetAdditionalTestFlows() []flowcore.FlowNameType {
	return []flowcore.FlowNameType{}
}

func GetAdditionalFlowsByState(teststate string) []flowcore.FlowNameType {
	return []flowcore.FlowNameType{}
}

func ProcessTestFlowController(tfmContext flowcore.FlowMachineContext, tfContext flowcore.FlowContext) error {
	return nil
}

func GetTestConfig(tokenPtr *string, wantPluginPaths bool) map[string]interface{} {
	pluginConfig := map[string]interface{}{}

	//env = "dev"
	pluginConfig["vaddress"] = "TODO"
	pluginConfig["env"] = "dev"
	pluginConfig["tokenptr"] = tokenPtr
	pluginConfig["logNamespace"] = "db"

	pluginConfig["templatePath"] = []string{
		"trc_templates/FlumeDatabase/TierceronFlow/TierceronFlow.tmpl",
	}

	// plugin configs here...
	pluginConfig["connectionPath"] = []string{
		"trc_templates/TrcVault/VaultDatabase/config.yml.tmpl",  // implemented
		"trc_templates/TrcVault/Database/config.yml.tmpl",       // implemented
		"trc_templates/TrcVault/SpiralDatabase/config.yml.tmpl", // implemented
	}
	pluginConfig["certifyPath"] = []string{
		"trc_templates/TrcVault/Certify/config.yml.tmpl", // implemented
	}

	pluginConfig["regions"] = []string{}
	pluginConfig["insecure"] = true
	pluginConfig["exitOnFailure"] = false

	if wantPluginPaths {
		pluginConfig["pluginNameList"] = []string{
			"trc-vault-plugin",
		}
	}
	return pluginConfig
}
