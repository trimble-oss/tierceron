//go:build !tc
// +build !tc

package buildopts

import (
	"database/sql"
	"errors"
	flowcore "tierceron/trcflow/core"
	eUtils "tierceron/utils"
	"tierceron/vaulthelper/kv"

	"github.com/dolthub/go-mysql-server/server"
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

// Flow names
func GetAdditionalFlows() []flowcore.FlowNameType {
	return []flowcore.FlowNameType{}
}

func GetAdditionalTestFlows() []flowcore.FlowNameType {
	return []flowcore.FlowNameType{} // Noop
}

func GetAdditionalFlowsByState(teststate string) []flowcore.FlowNameType {
	return []flowcore.FlowNameType{}
}

// Test configurations.
func GetTestConfig(token string, wantPluginPaths bool) map[string]interface{} {
	return map[string]interface{}{}
}

func GetTestDeployConfig(token string) map[string]interface{} {
	return map[string]interface{}{}
}

// Enrich plugin configs
func ProcessDeployPluginEnvConfig(pluginEnvConfig map[string]interface{}) map[string]interface{} {
	return map[string]interface{}{}
}

func ProcessPluginEnvConfig(pluginEnvConfig map[string]interface{}) map[string]interface{} {
	return map[string]interface{}{}
}

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

// Which tables synced..
func GetSyncedTables() []string {
	return []string{}
}

// GetExtensionAuthComponents - obtains an auth components
func GetExtensionAuthComponents(config map[string]interface{}) map[string]interface{} {
	return map[string]interface{}{}
}

// Build interface
func BuildInterface(config *eUtils.DriverConfig, goMod *kv.Modifier, tfmContext *flowcore.TrcFlowMachineContext, vaultDatabaseConfig map[string]interface{}, serverListener server.ServerEventListener) error {
	return nil
}

func Authorize(db *sql.DB, userIdentifier string, userPassword string) (bool, string, error) {
	return false, "", errors.New("Not implemented")
}

func GetSupportedTemplates() []string {
	return []string{}
}
