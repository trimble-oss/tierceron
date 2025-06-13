package hcore

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"log"
	"os"

	"github.com/trimble-oss/tierceron-core/v2/buildopts/plugincoreopts"
	"github.com/trimble-oss/tierceron-core/v2/core"
	"github.com/trimble-oss/tierceron/atrium/vestibulum/trcflow/flows/argossocii"

	tccore "github.com/trimble-oss/tierceron-core/v2/core"
	flowcore "github.com/trimble-oss/tierceron-core/v2/flow"
	coreutil "github.com/trimble-oss/tierceron-core/v2/util"
)

var configContext *tccore.ConfigContext
var tfmContext flowcore.FlowMachineContext
var sender chan error
var dfstat *tccore.TTDINode

var isProd bool = false

func SetProd(prod bool) {
	isProd = prod
}

func IsProd() bool {
	return isProd
}

func receiver(receive_chan chan tccore.KernelCmd) {
	for {
		event := <-receive_chan
		switch {
		case event.Command == tccore.PLUGIN_EVENT_START:
			go start(event.PluginName)
		case event.Command == tccore.PLUGIN_EVENT_STOP:
			go stop(event.PluginName)
			sender <- errors.New("trcdb shutting down")
			return
		case event.Command == tccore.PLUGIN_EVENT_STATUS:
			//TODO
		default:
			//TODO
		}
	}
}

func init() {
	if plugincoreopts.BuildOptions.IsPluginHardwired() {
		return
	}
	peerExe, err := os.Open("plugins/trcdb.so")
	if err != nil {
		fmt.Println("Unable to sha256 plugin")
		return
	}

	defer peerExe.Close()

	h := sha256.New()
	if _, err := io.Copy(h, peerExe); err != nil {
		fmt.Printf("Unable to copy file for sha256 of plugin: %s\n", err)
		return
	}
	sha := hex.EncodeToString(h.Sum(nil))
	fmt.Printf("trcdb Version: %s\n", sha)
}

func send_dfstat() {
	if configContext == nil || configContext.DfsChan == nil || dfstat == nil {
		fmt.Println("Dataflow Statistic channel not initialized properly for trcdb.")
		return
	}
	dfsctx, _, err := dfstat.GetDeliverStatCtx()
	if err != nil {
		configContext.Log.Println("Failed to get dataflow statistic context: ", err)
		send_err(err)
		return
	}
	tccore.SendDfStat(configContext, dfsctx, dfstat)
}

func send_err(err error) {
	if configContext == nil || configContext.ErrorChan == nil || err == nil {
		fmt.Println("Failure to send error message, error channel not initialized properly for trcdb.")
		return
	}
	if dfstat != nil {
		dfsctx, _, err := dfstat.GetDeliverStatCtx()
		if err != nil {
			configContext.Log.Println("Failed to get dataflow statistic context: ", err)
			return
		}
		dfstat.UpdateDataFlowStatistic(dfsctx.FlowGroup,
			dfsctx.FlowName,
			dfsctx.StateName,
			dfsctx.StateCode,
			2,
			func(msg string, err error) {
				configContext.Log.Println(msg, err)
			})
		tccore.SendDfStat(configContext, dfsctx, dfstat)
	}
	*configContext.ErrorChan <- err
}

func chat_receiver(chat_receive_chan chan *tccore.ChatMsg) {
	for {
		event := <-chat_receive_chan
		switch {
		case event == nil:
			continue
		case *event.Name == "SHUTDOWN":
			configContext.Log.Println("trcdb shutting down message receiver")
			return
		case event.Response != nil && *((*event).Response) == "Service unavailable":
			configContext.Log.Println("Trcdb unable to access chat service.")
			return
		case event.ChatId != nil && (*event).ChatId != nil && *event.ChatId == "PROGRESS":
			configContext.Log.Println("Sending progress results back to kernel.")
			progressResp := "Running Trcdb Queries..."
			(*event).Response = &progressResp
			*configContext.ChatSenderChan <- event
		case event.ChatId != nil && (*event).ChatId != nil && *event.ChatId != "PROGRESS" && event.TrcdbExchange != nil && (*event).TrcdbExchange != nil:
			configContext.Log.Println("Received trcdb request.")
			processTrcdb((*event).TrcdbExchange)
			handledResponse := "Handled Trcdb request successfully"
			(*event).Response = &handledResponse
			configContext.Log.Println("Sending all test results back to kernel.")

			*configContext.ChatSenderChan <- event
		default:
			configContext.Log.Println("trcdb received chat message")
		}
	}
}

func processTrcdb(trcdbExchange *core.TrcdbExchange) {
	if trcdbExchange == nil || tfmContext == nil {
		configContext.Log.Println("Invalid TrcdbExchange received, shutting down Trcdb processing.")
		return
	}
	if len(trcdbExchange.Flows) > 0 {
		tfCtx := tfmContext.GetFlowContext(flowcore.FlowNameType(trcdbExchange.Flows[0]))
		if tfCtx == nil {
			configContext.Log.Println("No flow context available for TrcdbExchange processing")
			trcdbExchange.Response = core.TrcdbResponse{}
			return
		}
		query := make(map[string]any)
		query["TrcQuery"] = trcdbExchange.Query
		responseMatrix, changed := tfmContext.CallDBQuery(tfCtx, query, nil, false, trcdbExchange.Operation, nil, "")
		if len(responseMatrix) == 0 {
			configContext.Log.Println("TrcdbExchange operation did not get any results.  returning empty response.")
		}
		trcdbExchange.Response = core.TrcdbResponse{
			Rows:    responseMatrix,
			Success: changed,
		}
	}
}

func start(pluginName string) {
	if configContext == nil {
		fmt.Println("no config context initialized for trcdb")
		return
	}

	dfstat = tccore.InitDataFlow(nil, configContext.ArgosId, false)
	dfstat.UpdateDataFlowStatistic("System",
		pluginName,
		"Start up",
		"1",
		1,
		func(msg string, err error) {
			configContext.Log.Println(msg, err)
		})
	send_dfstat()
	if configContext.CmdSenderChan != nil {
		*configContext.CmdSenderChan <- tccore.KernelCmd{
			Command: tccore.PLUGIN_EVENT_START,
		}
	}
}

func stop(pluginName string) {
	if configContext != nil {
		configContext.Log.Println("trcdb received shutdown message from kernel.")
	}
	if configContext != nil {
		configContext.Log.Println("Stopped server for trcdb.")
		dfstat.UpdateDataFlowStatistic("System",
			pluginName,
			"Shutdown",
			"0",
			1, func(msg string, err error) {
				if err != nil {
					configContext.Log.Println(tccore.SanitizeForLogging(err.Error()))
				} else {
					configContext.Log.Println(tccore.SanitizeForLogging(msg))
				}
			})
		send_dfstat()
		*configContext.CmdSenderChan <- tccore.KernelCmd{PluginName: pluginName, Command: tccore.PLUGIN_EVENT_STOP}
	}
	dfstat = nil
}

func GetConfigContext(pluginName string) *tccore.ConfigContext { return configContext }

func GetConfigPaths(pluginName string) []string {
	return []string{
		"-templateFilter=Hive/PluginTrcdb",
	}
}

// ProcessFlowController - override to provide a custom flow controller.  You will need a custom
// flow controller if you define any additional flows other than the default flows:
// 1. DataFlowStatConfigurationsFlow
func ProcessFlowController(tfmContext flowcore.FlowMachineContext, tfContext flowcore.FlowContext) error {
	switch tfContext.GetFlowName() {
	case flowcore.ArgosSociiFlow.TableName():
		tfContext.SetFlowDefinitionContext(argossocii.GetProcessFlowDefinition())
	default:
		return errors.New("Flow not implemented: " + tfContext.GetFlowName())
	}

	return flowcore.ProcessTableConfigurations(tfmContext, tfContext)
}

// GetDatabaseName - returns a name to be used by TrcDb.
func GetDatabaseName() string {
	return "TrcDb"
}

func GetDbProject() string {
	return "TrcDb"
}

func GetFlowMachineTemplates() map[string]any {
	flowMachineTemplates := map[string]any{}
	flowMachineTemplates["templatePath"] = []string{
		"trc_templates/FlumeDatabase/TierceronFlow/TierceronFlow.tmpl",                             // implemented.
		fmt.Sprintf("trc_templates/%s/DataFlowStatistics/DataFlowStatistics.tmpl", GetDbProject()), // implemented.
		fmt.Sprintf("trc_templates/%s/ArgosSocii/ArgosSocii.tmpl", GetDbProject()),                 // implemented.
	}
	return flowMachineTemplates
}

func GetBusinessFlows() []flowcore.FlowNameType {
	return []flowcore.FlowNameType{}
}

func GetTestFlows() []flowcore.FlowNameType {
	return []flowcore.FlowNameType{}
}

func GetTestFlowsByState(teststate string) []flowcore.FlowNameType {
	return []flowcore.FlowNameType{}
}

func TestFlowController(tfmContext flowcore.FlowMachineContext, tfContext flowcore.FlowContext) error {
	return nil
}

func GetFlowMachineInitContext(pluginName string) *flowcore.FlowMachineInitContext {
	pluginConfig := GetFlowMachineTemplates()

	return &flowcore.FlowMachineInitContext{
		GetFlowMachineTemplates:     GetFlowMachineTemplates,
		FlowMachineInterfaceConfigs: map[string]any{},
		GetDatabaseName:             GetDatabaseName,
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
		GetBusinessFlows:    GetBusinessFlows,
		GetTestFlows:        GetTestFlows,
		GetTestFlowsByState: GetTestFlowsByState,
		FlowController:      ProcessFlowController,
		TestFlowController:  TestFlowController,
	}
}

func PostInit(configContext *tccore.ConfigContext) {
	configContext.Start = start
	sender = *configContext.ErrorChan
	go receiver(*configContext.CmdReceiverChan)
}

func Init(pluginName string, properties *map[string]any) {
	var err error

	configContext, err = tccore.Init(properties,
		tccore.TRCSHHIVEK_CERT,
		tccore.TRCSHHIVEK_KEY,
		"", // No additional config file used/managed by the trcdb plugin itself.
		"trcdb",
		start,
		receiver,
		chat_receiver,
	)
	if err != nil {
		(*properties)["log"].(*log.Logger).Printf("Initialization error: %v", err)
		return
	}
	if _, ok := (*properties)[flowcore.HARBINGER_INTERFACE_CONFIG]; !ok {
		fmt.Println("Missing common config components")
		return
	}
	if tfmCtx, ok := (*properties)[tccore.TRCDB_RESOURCE].(flowcore.FlowMachineContext); ok {
		tfmContext = tfmCtx
	} else {
		fmt.Println("No flow context available for trcdb plugin.")
		return
	}
}

func GetPluginMessages(pluginName string) []string {
	return []string{}
}
