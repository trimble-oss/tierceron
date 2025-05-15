package buildopts

import (
	"database/sql"
	"errors"
	"fmt"
	"io"

	prod "github.com/trimble-oss/tierceron-core/v2/prod"
)

// SetLogger is called by TrcDb and other utilities to provide the extensions
// a handle to the error logger for custom libraries that need a hook into the
// logging infrastructure.  In extension implementation,
// set global variables with type func(string, ...interface{})
// assign the globals to the return value of convLogger...
// You'll have to copy convLogger into your custom library implementation.
func SetLogger(logger interface{}) {
	convLogger(logger)
}

// SetErrorLogger is called by TrcDb and other utilities to provide the extensions
// a handle to the error logger for custom libraries that need a hook into the
// logging infrastructure.  In extension implementation,
// set global variables with type func(string, ...interface{})
// assign the globals to the return value of convLogger...
// You'll have to copy convLogger into your custom library implementation.
func SetErrorLogger(logger interface{}) {
	convLogger(logger)
}

// convLogger converts logger to the standard logger interface.
func convLogger(logger interface{}) func(string, ...interface{}) {
	switch z := logger.(type) {
	case io.Writer:
		return func(s string, v ...interface{}) {
			fmt.Fprintf(z, s, v...)
		}
	case func(string, ...interface{}) (int, error): // fmt.Printf
		return func(s string, v ...interface{}) {
			_, _ = z(s, v...)
		}
	case func(string, ...interface{}): // log.Printf
		return z
	}
	panic(fmt.Sprintf("unsupported logger type %T", logger))
}

// Local vault address
// deprecated
func GetLocalVaultAddr() string {
	return ""
}

// GetSupportedSourceRegions provide source regions supported by TrcDb.
func GetSupportedSourceRegions() []string {
	return []string{}
}

// Test configurations.
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
		"trc_templates/TrcVault/Identity/config.yml.tmpl",       // implemented
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

// GetTestDeployConfig - returns a list of templates used in defining tables for Trcdb.
// Supported attributes include templatePath, connectionPath, certifyPath, env, exitOnFailure, pluginNameList, and logNamespace:
func GetTestDeployConfig(tokenPtr *string) map[string]interface{} {
	pluginConfig := map[string]interface{}{}

	pluginConfig["env"] = "dev"
	pluginConfig["tokenptr"] = tokenPtr
	pluginConfig["regions"] = []string{}
	pluginConfig["insecure"] = true
	pluginConfig["exitOnFailure"] = true
	// Here is an example expanded templatePath:
	//
	//	 templatePath: []string{
	//		"trc_templates/DatabaseName/TableNameA/TableNameA.tmpl",
	//		"trc_templates/DatabaseName/TableNameB/TableNameB.tmpl",
	//		"trc_templates/FlumeDatabase/TierceronFlow/TierceronFlow.tmpl",
	//	}
	pluginConfig["templatePath"] = []string{
		"trc_templates/TrcVault/Certify/config.yml.tmpl",
	}
	return pluginConfig
}

// ProcessPluginEnvConfig - returns a list of templates used in defining tables for Trcdb.
// Supported attributes include templatePath, connectionPath, certifyPath, env, exitOnFailure, pluginNameList, and logNamespace:
//
// Here is an example expanded templatePath:
//
//	 templatePath: []string{
//		"trc_templates/DatabaseName/TableNameA/TableNameA.tmpl",
//		"trc_templates/DatabaseName/TableNameB/TableNameB.tmpl",
//		"trc_templates/FlumeDatabase/TierceronFlow/TierceronFlow.tmpl",
//	}
func ProcessPluginEnvConfig(pluginEnvConfig map[string]interface{}) map[string]interface{} {
	pluginEnvConfig["templatePath"] = []string{
		"trc_templates/FlumeDatabase/TierceronFlow/TierceronFlow.tmpl",
	}
	pluginEnvConfig["connectionPath"] = []string{
		"trc_templates/TrcVault/VaultDatabase/config.yml.tmpl",
		"trc_templates/TrcVault/Database/config.yml.tmpl",
		"trc_templates/TrcVault/Identity/config.yml.tmpl",
		"trc_templates/TrcVault/SpiralDatabase/config.yml.tmpl",
	}

	pluginEnvConfig["certifyPath"] = []string{
		"trc_templates/TrcVault/Certify/config.yml.tmpl",
	}

	if env, ok := pluginEnvConfig["env"].(string); ok && prod.IsStagingProd(env) {
		pluginEnvConfig["regions"] = GetSupportedSourceRegions()
	} else {
		pluginEnvConfig["regions"] = []string{}
	}

	pluginEnvConfig["exitOnFailure"] = false
	pluginEnvConfig["pluginNameList"] = []string{
		"trc-vault-plugin",
		"trcsh",
	}
	pluginEnvConfig["logNamespace"] = "db"

	return pluginEnvConfig
}

// GetSyncedTables - return a list of synced tables from a remote source in TrcDb.
// Override this function to provide a list of synced tables.
func GetSyncedTables() []string {
	return []string{}
}

// GetExtensionAuthComponents - obtains an auth components for an identity provider to be
// used by business logic flows...
// Fields to be provided by the map include: authDomain, authHeaders, bodyData, and authUrl.
// This components will by used by a standard http client to authenticate with the authUrl.
// The response from the query will be provided to the Flow Context in the ExtensionAuthData field
// That can then be used by business logic flows residing in TrcDb to query additional
// services requiring authentication.
func GetExtensionAuthComponents(config map[string]interface{}) map[string]interface{} {
	return map[string]interface{}{}
}

// Authorize - override to provide a custom authorization function for use by the web interface.
// This function should return true and use user name authorized if the user is authorized,
// false if not, and an error if an error occurs.
// The web interface is not presently maintained.
func Authorize(db *sql.DB, userIdentifier string, userPassword string) (bool, string, error) {
	return false, "", errors.New("Not implemented")
}

// CheckMemLock - override to provide a custom mem lock check function.
// Utilizing the bucket (ex: super-secrets/Index/FlumeDatabase) and key (ex: tenantId), decides whether or not to
// memlock the indicated value retrieved from vault.  Returns true if the value should be memlocked, false if not.
func CheckMemLock(bucket string, key string) bool {
	return true
}

// GetTrcDbUrl - Utilized by speculatio/fenestra to obtain a jdbc compliant connection url to the TrcDb database
// This can be used to perform direct queries against the TrcDb database using the go sql package.
// The data map is provided by the caller as convenience to provide things like dbport, etc...
// The override should return a jdbc compliant connection url to the TrcDb database.
func GetTrcDbUrl(data map[string]interface{}) string {
	return ""
}
