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
	tokenMap["templatePath"] = "/mnt/c/trc_templates/TenantConfig/fieldtechservice/TenantConfiguration.tmpl"
	tokenMap["connectionPath"] = "trc_templates/TrcVault/Database/config.tmpl"
	vscutils.DoProcessEnvConfig("dev", tokenMap)
}
