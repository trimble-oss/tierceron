package main

import (
	"flag"
	"fmt"
	"log"
	"os"

	flowcore "github.com/trimble-oss/tierceron-core/v2/flow"
	coreutil "github.com/trimble-oss/tierceron-core/v2/util"

	"github.com/trimble-oss/tierceron/atrium/buildopts/flowopts"
	"github.com/trimble-oss/tierceron/atrium/buildopts/testopts"
	trcflow "github.com/trimble-oss/tierceron/atrium/vestibulum/trcflow/flumen"
	"github.com/trimble-oss/tierceron/buildopts/harbingeropts"
	"github.com/trimble-oss/tierceron/buildopts/memonly"
	"github.com/trimble-oss/tierceron/buildopts/memprotectopts"
	"github.com/trimble-oss/tierceron/pkg/core"
	"github.com/trimble-oss/tierceron/pkg/core/cache"
	eUtils "github.com/trimble-oss/tierceron/pkg/utils"
)

// This executable automates the creation of seed files from template file(s).
// New seed files are written (or overwrite current seed files) to the specified directory.
func main() {

	// Supported build flags:
	//    insecure harbinger tc testrunner ( mysql, testflow -- auto registration -- warning do not use!)
	logFilePtr := flag.String("log", "./trcdbplugin.log", "Output path for log file")
	tokenPtr := flag.String("token", "", "Vault access Token")
	flag.Parse()

	f, err := os.OpenFile(*logFilePtr, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0644)
	logger := log.New(f, "[trcdbplugin]", log.LstdFlags)
	eUtils.CheckError(&core.CoreConfig{ExitOnFailure: true, Log: logger}, err, true)

	pluginConfig := testopts.BuildOptions.GetTestConfig(tokenPtr, false)
	pluginConfig["address"] = os.Getenv("VAULT_ADDR")
	pluginConfig["vaddress"] = os.Getenv("VAULT_ADDR")
	pluginConfig["caddress"] = os.Getenv("VAULT_ADDR")
	if eUtils.RefLength(tokenPtr) > 0 {
		pluginConfig["tokenptr"] = tokenPtr
		pluginConfig["ctokenptr"] = tokenPtr
	} else {
		pluginConfig["tokenptr"] = os.Getenv("VAULT_TOKEN")
		pluginConfig["ctokenptr"] = pluginConfig["tokenptr"]
	}
	pluginConfig["env"] = "dev"
	pluginConfig["insecure"] = true

	if memonly.IsMemonly() {
		memprotectopts.MemUnprotectAll(nil)
		for _, value := range pluginConfig {
			if valueSlice, isValueSlice := value.([]string); isValueSlice {
				for _, valueEntry := range valueSlice {
					memprotectopts.MemProtect(nil, &valueEntry)
				}
			} else if valueString, isValueString := value.(string); isValueString {
				memprotectopts.MemProtect(nil, &valueString)
			}
		}
	}
	flowMachineInitContext := flowcore.FlowMachineInitContext{
		FlowMachineInterfaceConfigs: map[string]interface{}{},
		GetDatabaseName:             harbingeropts.BuildOptions.GetDatabaseName,
		GetTableFlows: func() []flowcore.FlowDefinition {
			tableFlows := []flowcore.FlowDefinition{}
			for _, template := range pluginConfig["templatePath"].([]string) {
				flowSource, service, _, tableTemplateName := coreutil.GetProjectService("", "trc_templates", template)
				tableName := coreutil.GetTemplateFileName(tableTemplateName, service)
				tableFlows = append(tableFlows, flowcore.FlowDefinition{
					FlowName:         flowcore.FlowNameType(tableName),
					FlowTemplatePath: template,
					FlowSource:       flowSource,
				})
			}
			return tableFlows
		},
		GetBusinessFlows:    flowopts.BuildOptions.GetAdditionalFlows,
		GetTestFlows:        testopts.BuildOptions.GetAdditionalTestFlows,
		GetTestFlowsByState: flowopts.BuildOptions.GetAdditionalFlowsByState,
		FlowController:      flowopts.BuildOptions.ProcessFlowController,
		TestFlowController:  testopts.BuildOptions.ProcessTestFlowController,
	}
	currentTokenName := fmt.Sprintf("config_token_%s_unrestricted", pluginConfig["env"])
	tokenCache := cache.NewTokenCache(currentTokenName, eUtils.RefMap(pluginConfig, "tokenptr"), eUtils.RefMap(pluginConfig, "vaddress"))

	driverConfig, err := eUtils.InitDriverConfigForPlugin(pluginConfig, tokenCache, currentTokenName, logger)
	if err != nil {
		fmt.Printf("Error initializing driver config: %v\n", err)
		os.Exit(1)
	}

	trcflow.BootFlowMachine(&flowMachineInitContext, driverConfig, pluginConfig, logger)
	wait := make(chan bool)
	<-wait
}
