//go:build !tc
// +build !tc

package buildopts

import (
	"database/sql"
	"errors"
)

func SetLogger(logger interface{}) {
	return
}

func SetErrorLogger(logger interface{}) {
	return
}

// Local vault address
func GetLocalVaultAddr() string {
	return ""
}

// Supported regions
func GetSupportedSourceRegions() []string {
	return []string{}
}

// Test configurations.
func GetTestConfig(token string, wantPluginPaths bool) map[string]interface{} {
	return map[string]interface{}{}
}

func GetTestDeployConfig(token string) map[string]interface{} {
	return map[string]interface{}{}
}

func ProcessPluginEnvConfig(pluginEnvConfig map[string]interface{}) map[string]interface{} {
	return map[string]interface{}{}
}

<<<<<<< HEAD
// Process a test flow.
func ProcessTestFlowController(tfmContext *flowcore.TrcFlowMachineContext, trcFlowContext *flowcore.TrcFlowContext) error {
	return errors.New("Table not implemented.")
}

func ProcessFlowController(tfmContext *flowcore.TrcFlowMachineContext, trcFlowContext *flowcore.TrcFlowContext) error {
	return nil
}

// Name of database
func GetDatabaseName() string {
	return ""
}

func GetFlowDatabaseName() string {
	return ""
}

=======
>>>>>>> develop
// Which tables synced..
func GetSyncedTables() []string {
	return []string{}
}

// GetExtensionAuthComponents - obtains an auth components
func GetExtensionAuthComponents(config map[string]interface{}) map[string]interface{} {
	return map[string]interface{}{}
}

func Authorize(db *sql.DB, userIdentifier string, userPassword string) (bool, string, error) {
	return false, "", errors.New("Not implemented")
}

func GetSupportedTemplates() []string {
	return []string{}
}
