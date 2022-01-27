package main

import (
	vscutils "tierceron/trcvault/util"
)

// This executable automates the creation of seed files from template file(s).
// New seed files are written (or overwrite current seed files) to the specified directory.
func main() {
	tokenMap := map[string]interface{}{}
	tokenMap["address"] = "https://vault.whoboot.org:8200" //This should be local
	tokenMap["token"] = "s.cXIsCveFbqldF8kwz9aaBU6A"
	// TenantConfiguration, SpectrumEnterpriseConfig, Mysqlfile
	tokenMap["templatePath"] = []string{
		"trc_templates/TenantDatabase/TenantConfiguration/TenantConfiguration.tmpl", // implemented
		//		"trc_templates/TenantDatabase/SpectrumEnterpriseConfig/SpectrumEnterpriseConfig.tmpl", // not yet implemented.
		//		"trc_templates/TenantDatabase/KafkaTableConfiguration/KafkaTableConfiguration.tmpl",   // not yet implemented.
		//		"trc_templates/TenantDatabase/Mysqlfile/Mysqlfile.tmpl",                               // not yet implemented.
	}

	// plugin configs here...
	tokenMap["connectionPath"] = "trc_templates/TrcVault/Database/config.tmpl"
	vscutils.ProcessTables("QA", tokenMap)
}
