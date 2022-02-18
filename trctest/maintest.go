package main

import (
	"flag"
	"log"
	"os"
	trcflow "tierceron/trcflow/flumen"
	eUtils "tierceron/utils"
)

// This executable automates the creation of seed files from template file(s).
// New seed files are written (or overwrite current seed files) to the specified directory.
func main() {
	logFilePtr := flag.String("log", "./trcdbplugin.log", "Output path for log file")
	f, err := os.OpenFile(*logFilePtr, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0644)
	eUtils.CheckError(err, true)
	logger := log.New(f, "[trcdbplugin]", log.LstdFlags)

	pluginConfig := map[string]interface{}{}
	pluginConfig["address"] = "https://vault.whoboot.org:8200" //This should be local
	pluginConfig["token"] = "s.2OamEH93aN4psYH7XmtWODCy"
	// TenantConfiguration, SpectrumEnterpriseConfig, Mysqlfile
	pluginConfig["templatePath"] = []string{
		"trc_templates/TenantDatabase/TenantConfiguration/TenantConfiguration.tmpl",           // implemented
		"trc_templates/TenantDatabase/SpectrumEnterpriseConfig/SpectrumEnterpriseConfig.tmpl", // not yet implemented.
		//		"trc_templates/TenantDatabase/KafkaTableConfiguration/KafkaTableConfiguration.tmpl",   // not yet implemented.
		//		"trc_templates/TenantDatabase/Mysqlfile/Mysqlfile.tmpl",                               // not yet implemented.
	}

	// plugin configs here...
	// plugin configs here...
	pluginConfig["connectionPath"] = []string{
		"trc_templates/TrcVault/VaultDatabase/config.yml.tmpl", // implemented
		"trc_templates/TrcVault/Database/config.yml.tmpl",      // implemented
		"trc_templates/TrcVault/Identity/config.yml.tmpl",      // implemented
		//		"trc_templates/TenantDatabase/SpectrumEnterpriseConfig/SpectrumEnterpriseConfig.tmpl", // not yet implemented.
		//		"trc_templates/TenantDatabase/KafkaTableConfiguration/KafkaTableConfiguration.tmpl",   // not yet implemented.
		//		"trc_templates/TenantDatabase/Mysqlfile/Mysqlfile.tmpl",                               // not yet implemented.
	}
	pluginConfig["env"] = "QA"
	pluginConfig["regions"] = []string{}
	pluginConfig["insecure"] = true
	pluginConfig["exitOnFailure"] = true

	trcflow.ProcessFlows(pluginConfig, logger)
}
