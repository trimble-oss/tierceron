package certify

func WriteMapUpdate(writeMap map[string]interface{}, pluginToolConfig map[string]interface{}, defineServicePtr bool, pluginTypePtr string, pathParamPtr string) map[string]interface{} {
	if pluginTypePtr != "trcshservice" {
		writeMap["trcplugin"] = pluginToolConfig["trcplugin"].(string)
		writeMap["trctype"] = pluginTypePtr
		if pluginToolConfig["instances"] == nil {
			pluginToolConfig["instances"] = "0"
		}
		writeMap["instances"] = pluginToolConfig["instances"].(string)
	}
	if defineServicePtr {
		writeMap["trccodebundle"] = pluginToolConfig["trccodebundle"].(string)
		writeMap["trcservicename"] = pluginToolConfig["trcservicename"].(string)
		writeMap["trcprojectservice"] = pluginToolConfig["trcprojectservice"].(string)
		writeMap["trcdeployroot"] = pluginToolConfig["trcdeployroot"].(string)
	}
	if _, imgShaOk := pluginToolConfig["imagesha256"].(string); imgShaOk {
		writeMap["trcsha256"] = pluginToolConfig["imagesha256"].(string) // Pull image sha from registry...
	} else {
		writeMap["trcsha256"] = pluginToolConfig["trcsha256"].(string) // Pull image sha from registry...
	}
	if pathParamPtr != "" { //optional if not found.
		writeMap["trcpathparam"] = pathParamPtr
	} else if pathParam, pathOK := writeMap["trcpathparam"].(string); pathOK {
		writeMap["trcpathparam"] = pathParam
	}

	if newRelicAppName, nameOK := pluginToolConfig["newrelicAppName"].(string); newRelicAppName != "" && nameOK && pluginTypePtr == "vault" { //optional if not found.
		writeMap["newrelic_app_name"] = newRelicAppName
	}
	if newRelicLicenseKey, keyOK := pluginToolConfig["newrelicLicenseKey"].(string); newRelicLicenseKey != "" && keyOK && pluginTypePtr == "vault" { //optional if not found.
		writeMap["newrelic_license_key"] = newRelicLicenseKey
	}

	if trcbootstrap, ok := pluginToolConfig["trcbootstrap"].(string); ok {
		writeMap["trcbootstrap"] = trcbootstrap
	}

	writeMap["copied"] = false
	writeMap["deployed"] = false
	return writeMap
}
