package hcore

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/trimble-oss/tierceron-core/v2/buildopts/plugincoreopts"
	"github.com/trimble-oss/tierceron-core/v2/core"
	"github.com/trimble-oss/tierceron-core/v2/core/coreconfig"
	coreprod "github.com/trimble-oss/tierceron-core/v2/prod"
	coreutil "github.com/trimble-oss/tierceron-core/v2/util"

	flowcore "github.com/trimble-oss/tierceron-core/v2/flow"
)

var (
	configContext *core.ConfigContext
	tfmContext    flowcore.FlowMachineContext
	sender        chan error
	dfstat        *core.TTDINode
)

func SetProd(prod bool) {
	coreprod.SetProd(prod)
}

func IsProd() bool {
	return coreprod.IsProd()
}

func receiver(receive_chan chan core.KernelCmd) {
	for {
		event := <-receive_chan
		switch {
		case event.Command == core.PLUGIN_EVENT_START:
			go start(event.PluginName)
		case event.Command == core.PLUGIN_EVENT_STOP:
			go stop(event.PluginName)
			sender <- errors.New("trcdb shutting down")
			return
		case event.Command == core.PLUGIN_EVENT_STATUS:
			// TODO
		default:
			// TODO
		}
	}
}

func init() {
	if plugincoreopts.BuildOptions.IsPluginHardwired() {
		return
	}
	peerExe, err := os.Open("plugins/trcdb.so")
	if err != nil {
		fmt.Fprintln(os.Stderr, "Trcdb unable to sha256 plugin")
		return
	}

	defer peerExe.Close()

	h := sha256.New()
	if _, err := io.Copy(h, peerExe); err != nil {
		fmt.Fprintf(os.Stderr, "Unable to copy file for sha256 of plugin: %s\n", err)
		return
	}
	sha := hex.EncodeToString(h.Sum(nil))
	fmt.Fprintf(os.Stderr, "trcdb Version: %s\n", sha)
}

func send_dfstat() {
	if configContext == nil || configContext.DfsChan == nil || dfstat == nil {
		fmt.Fprintln(os.Stderr, "Dataflow Statistic channel not initialized properly for trcdb.")
		return
	}
	dfsctx, _, err := dfstat.GetDeliverStatCtx()
	if err != nil {
		configContext.Log.Println("Failed to get dataflow statistic context: ", err)
		send_err(err)
		return
	}
	dfstat.Name = configContext.ArgosId
	dfstat.FinishStatistic("", "", "", configContext.Log, true, dfsctx)
	configContext.Log.Printf("Sending dataflow statistic to kernel: %s\n", dfstat.Name)
	dfstatClone := *dfstat
	go func(dsc *core.TTDINode) {
		if configContext != nil && *configContext.DfsChan != nil {
			*configContext.DfsChan <- dsc
		}
	}(&dfstatClone)
}

func send_err(err error) {
	if configContext == nil || configContext.ErrorChan == nil || err == nil {
		fmt.Fprintln(os.Stderr, "Failure to send error message, error channel not initialized properly for trcdb.")
		return
	}
	configContext.Log.Println("trcdb sending error message to kernel: ", err)
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
		core.SendDfStat(configContext, dfsctx, dfstat)
		configContext.Log.Println("Sent dataflow statistic with error to kernel: ", dfstat.Name)
	}
	*configContext.ErrorChan <- err
}

func CheckFlowStatusReport(region string, argosId string, flowGroupKey string) (string, error) {
	trcdbExchange := &core.TrcdbExchange{
		Flows:     []string{"DataFlowStatistics"},                                                                                                                                                                                // Flows
		Query:     fmt.Sprintf("SELECT * FROM %s.DataFlowStatistics where flowGroup like '%s-%s-%s' and argosId like '%s' ORDER by flowName, CAST(stateCode as UNSIGNED) asc", "%s", flowGroupKey, region, "%", string(argosId)), // Query
		Operation: "SELECT",                                                                                                                                                                                                      // query operation
	}
	ProcessTrcdb(trcdbExchange)

	if len(trcdbExchange.Response.Rows) > 0 {
		var report strings.Builder
		report.WriteString("Report for " + argosId + ":\n")
		report.WriteString("Pod | Flow | Step # | Result | Execution Date | Time Spent |  Status\n")
		report.WriteString("-----------------------------------------------------------------------------\n")

		// Get the lastTestedDate from the first row (assumed to be the same for all rows)
		var baseTimestamp time.Time
		var err error

		if len(trcdbExchange.Response.Rows) > 0 && len(trcdbExchange.Response.Rows[0]) >= 8 {
			baseTimestampStr := fmt.Sprint(trcdbExchange.Response.Rows[0][7]) // Index 7 for timestamp
			baseTimestamp, err = time.Parse(time.RFC3339, baseTimestampStr)
			if err != nil {
				// If we can't parse the timestamp, continue but note the error
				report.WriteString("Warning: Could not parse base timestamp. Execution times may be inaccurate.\n\n")
			}
		}

		// Keep track of cumulative time to add to base timestamp
		var cumulativeTime time.Duration
		var currentFlowName string

		for _, row := range trcdbExchange.Response.Rows {
			if len(row) >= 8 {
				flowName := fmt.Sprint(row[0]) // Start at index 0 for flowName

				// Reset cumulative time when flow name changes
				if currentFlowName != flowName {
					currentFlowName = flowName
					cumulativeTime = 0

					// Reset base timestamp for the new flow
					baseTimestampStr := fmt.Sprint(row[7])
					newBaseTimestamp, err := time.Parse(time.RFC3339, baseTimestampStr)
					if err == nil {
						baseTimestamp = newBaseTimestamp
					}
				}

				stateCode := fmt.Sprint(row[4]) // Start at index 4 for stateCode
				modeStr := fmt.Sprint(row[3])   // Index 3 for mode
				flowGroup := fmt.Sprint(row[1]) // Index 1 for flowGroup

				// Extract pod ID from the flowGroup (should be in the format 'prefix-region-podId')
				podId := "0"
				if strings.Contains(flowGroup, "-") {
					parts := strings.Split(flowGroup, "-")
					// Get only the last segment as the pod ID
					podId = parts[len(parts)-1]
				}

				stateName := fmt.Sprint(row[5])    // Index 5 for stateName
				timeSplitStr := fmt.Sprint(row[6]) // Index 6 for timeSplit, to be displayed unaltered

				var modeText string
				switch modeStr {
				case "0":
					modeText = "Skipped"
				case "1":
					modeText = "Success"
				case "2":
					modeText = "Failed"
				default:
					modeText = "Unknown (" + modeStr + ")"
				}

				var stepDuration time.Duration

				// Check for millisecond suffix
				if strings.HasSuffix(timeSplitStr, "ms") {
					numericPart := strings.TrimSuffix(timeSplitStr, "ms")
					valueFloat, err := strconv.ParseFloat(numericPart, 64)
					if err == nil {
						stepDuration = time.Duration(valueFloat) * time.Millisecond
					}
				} else if strings.HasSuffix(timeSplitStr, "s") { // Check for second suffix
					numericPart := strings.TrimSuffix(timeSplitStr, "s")
					valueFloat, err := strconv.ParseFloat(numericPart, 64)
					if err == nil {
						stepDuration = time.Duration(valueFloat * float64(time.Second))
					}
				} else {
					valueFloat, err := strconv.ParseFloat(timeSplitStr, 64)
					if err == nil {
						stepDuration = time.Duration(valueFloat * float64(time.Second))
					}
				}

				cumulativeTime += stepDuration

				var executionTimeDisplay string
				if currentFlowName == flowName && stateCode != "1" {
					executionTimeDisplay = "                " // Same width as time format
				} else {
					executionTime := baseTimestamp.Add(cumulativeTime)
					executionTimeDisplay = executionTime.Format("2006-01-02 15:04:05.000")
				}

				// Add to report
				report.WriteString(fmt.Sprintf("%s | %s - %s. %s » %s %s %s\n",
					podId, flowName, stateCode, stateName, executionTimeDisplay, timeSplitStr, modeText))
			}
		}

		// Add total duration at the end
		if cumulativeTime > 0 {
			report.WriteString(fmt.Sprintf("\nTotal Duration: %s\n", cumulativeTime.String()))
		}

		return report.String(), nil
	}
	return "", nil
}

var trcdbChatMessageHandlerFunc func(event *core.ChatMsg) (string, error) = defaultTrcdbChatMessageHandler

func SetTrcdbChatMessageHandlerFunc(f func(event *core.ChatMsg) (string, error)) {
	trcdbChatMessageHandlerFunc = f
}

func GetWantedTests(event *core.ChatMsg) (map[string]bool, string) {
	var argosId string
	testsSet := map[string]bool{}
	if event.ChatId != nil && strings.Contains(*(*event).ChatId, ":") {
		chatIdSlice := strings.Split(*(*event).ChatId, ":")
		// Scrub id to only alphanumeric characters
		origArgosId := chatIdSlice[0]
		var scrubbedArgosId strings.Builder
		for _, r := range origArgosId {
			if (r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
				scrubbedArgosId.WriteRune(r)
			}
		}
		argosId = scrubbedArgosId.String()
		testsCommaList := chatIdSlice[1]
		testsSlice := strings.Split(testsCommaList, ",")

		for _, test := range testsSlice {
			if len(test) > 0 {
				testsSet[test] = true
			}
		}
	}
	if len(testsSet) == 0 {
		testsSet = nil
	}

	return testsSet, argosId
}

func defaultTrcdbChatMessageHandler(event *core.ChatMsg) (string, error) {
	if event.ChatId != nil && strings.Contains(*(*event).ChatId, ":") {
		testsSet, _ := GetWantedTests(event)
		approvedProdTests := make(map[string]bool)

		for test := range testsSet {
			switch test {
			case "FlowStatus":
			case "PluginStatus":
				approvedProdTests[test] = true
			}
		}
		if IsProd() && len(approvedProdTests) == 0 {
			return "Test not supported in production", nil
		} else if IsProd() {
			testsSet = approvedProdTests
		}

		if testsSet["FlowStatus"] {
			return CheckFlowStatusReport(configContext.Region, "flume", "Flows")
		} else if testsSet["PluginStatus"] {
			return CheckFlowStatusReport(configContext.Region, "hiveplugin", "System")
		}
	}
	return "", nil
}

func TrcdbChatMessageHandler(event *core.ChatMsg) (string, error) {
	return trcdbChatMessageHandlerFunc(event)
}

func chat_receiver(chat_receive_chan chan *core.ChatMsg) {
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
			if event.Name != nil && strings.Contains(*event.Name, ":") {
				pluginFlow := strings.Split(*event.Name, ":")
				if len(pluginFlow) > 1 {
					for i, flow := range pluginFlow {
						if i == 0 {
							continue
						}
						tfCtx := tfmContext.GetFlowContext(flowcore.FlowNameType(flow))
						tfCtxChatReceiverChan := tfCtx.GetFlowChatMsgReceiverChan()
						go func(tfCtxChatReceiverChan *chan *core.ChatMsg, msg *core.ChatMsg) {
							*tfCtxChatReceiverChan <- msg
							configContext.Log.Printf("Sent request to flow %s\n", flow)
						}(tfCtxChatReceiverChan, event)
					}
					continue
				}
			}
			ProcessTrcdb((*event).TrcdbExchange)
			handledResponse := "Handled Trcdb request successfully"
			(*event).Response = &handledResponse
			configContext.Log.Println("Sending all test results back to kernel.")

			*configContext.ChatSenderChan <- event
		case event.ChatId == nil || (*event).ChatId == nil || (event.ChatId != nil && (*event).ChatId != nil && *event.ChatId != "PIPELINETEST"):
			GetConfigContext("trcdb").Log.Println("Trcdb received chat message")
			response, err := TrcdbChatMessageHandler(event)
			if err != nil {
				GetConfigContext("trcdb").Log.Printf("Trcdb errored with %s\n", err.Error())
			} else {
				if event.Name != nil && *event.Name == "trcdb" && (event.Query == nil || len(*event.Query) == 0) {
					GetConfigContext("trcdb").Log.Println("Received message from trcdb, not processing to avoid feedback loop")
					continue
				}
				(*event).Response = &response
				configContext := GetConfigContext("trcdb")
				go func(csc *chan *core.ChatMsg) {
					*csc <- event
				}(configContext.ChatSenderChan)
			}
		default:
			configContext.Log.Println("trcdb received chat message")
		}
	}
}

func ProcessTrcdb(trcdbExchange *core.TrcdbExchange) {
	if tfmContext == nil {
		configContext.Log.Printf("trcdb - Request receive before initialization.")
		return
	}
	if trcdbExchange == nil {
		configContext.Log.Printf("Invalid TrcdbExchange received, ignoring request.")
		return
	}
	if len(trcdbExchange.Flows) > 0 {
		tfmContext.WaitAllFlowsLoaded()
		tfCtx := tfmContext.GetFlowContext(flowcore.FlowNameType(trcdbExchange.Flows[0]))
		if tfCtx == nil {
			configContext.Log.Println("No flow context available for TrcdbExchange processing")
			trcdbExchange.Response = core.TrcdbResponse{}
			return
		}
		query := make(map[string]any)
		query["TrcQuery"] = trcdbExchange.Query
		trcdbExchange, _ = tfmContext.CallDBQueryN(trcdbExchange, query, nil, false, trcdbExchange.Operation, nil, "")
		if len(trcdbExchange.Response.Rows) == 0 {
			configContext.Log.Println("TrcdbExchange operation did not get any results.  returning empty response.")
		}
	} else {
		configContext.Log.Println("No flow specified for TrcdbExchange processing")
		trcdbExchange.Response = core.TrcdbResponse{}
		return
	}
}

func start(pluginName string) {
	if configContext == nil {
		fmt.Fprintln(os.Stderr, "no config context initialized for trcdb")
		return
	}

	dfstat = core.InitDataFlow(nil, configContext.ArgosId, false)
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
		*configContext.CmdSenderChan <- core.KernelCmd{
			Command: core.PLUGIN_EVENT_START,
		}
		configContext.Log.Println("trcdb reported startup.")
	} else {
		configContext.Log.Println("No command sender channel available, trcdb cannot send start event.")
	}
}

func startFlowMachineListener() {
	if tfmContext == nil || tfmContext.GetFlowChatMsgSenderChan() == nil {
		configContext.Log.Println("No flow machine context available for trcdb plugin.")
		return
	}
	go func() {
		for {
			event := <-*tfmContext.GetFlowChatMsgSenderChan()
			if event == nil {
				continue
			}
			if event.TrcdbExchange != nil && len(event.TrcdbExchange.Flows) > 0 {
				configContext := GetConfigContext("trcdb")
				go func(csc *chan *core.ChatMsg) {
					*csc <- event
				}(configContext.ChatSenderChan)
				configContext.Log.Printf("trcdb received message for flows: %v\n", event.TrcdbExchange.Flows)
			}
		}
	}()
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
					configContext.Log.Println(core.SanitizeForLogging(err.Error()))
				} else {
					configContext.Log.Println(core.SanitizeForLogging(msg))
				}
			})
		send_dfstat()
		*configContext.CmdSenderChan <- core.KernelCmd{PluginName: pluginName, Command: core.PLUGIN_EVENT_STOP}
	}
	dfstat = nil
}

func GetConfigContext(pluginName string) *core.ConfigContext { return configContext }

func PostInit(configContext *core.ConfigContext) {
	configContext.Start = start
	sender = *configContext.ErrorChan
	go receiver(*configContext.CmdReceiverChan)
}

func GetConfigPaths(pluginName string) []string {
	return []string{
		"-templateFilter=Hive/PluginTrcdb",
	}
}

func Init(pluginName string, properties *map[string]any) {
	var err error

	configContext, err = core.Init(properties,
		core.TRCSHHIVEK_CERT,
		core.TRCSHHIVEK_KEY,
		"",           // No additional config file used/managed by the trcdb plugin itself.
		"hiveplugin", // Categorize as hiveplugin
		start,
		receiver,
		chat_receiver,
	)
	if err != nil {
		(*properties)["log"].(*log.Logger).Printf("Initialization error: %v", err)
		return
	}
	if _, ok := (*properties)[flowcore.HARBINGER_INTERFACE_CONFIG]; !ok {
		configContext.Log.Println("Missing common config components")
		return
	}
	if tfmCtx, ok := (*properties)[core.TRCDB_RESOURCE].(flowcore.FlowMachineContext); ok {
		tfmContext = tfmCtx
	} else {
		configContext.Log.Println("No flow context available for trcdb plugin.")
		return
	}
	startFlowMachineListener()
}

func GetPluginMessages(pluginName string) []string {
	return []string{}
}

// ProcessFlowController - override to provide a custom flow controller.  You will need a custom
// flow controller if you define any additional flows other than the default flows:
// 1. DataFlowStatConfigurationsFlow
func ProcessFlowController(tfmContext flowcore.FlowMachineContext, tfContext flowcore.FlowContext) error {
	switch tfContext.GetFlowHeader().FlowName() {
	// TODO: implement custom flows here.
	}

	return errors.New("flow not implemented")
}

func GetDatabaseName() string {
	return "TrcDb"
}

func GetFlowDatabaseName() string {
	return "FlumeDatabase"
}

func GetFlowMachineTemplates() map[string]any {
	pluginEnvConfig := map[string]any{}
	if IsProd() { // Use local IsProd function
		pluginEnvConfig["templatePath"] = []string{
			"trc_templates/TrcDb/DataFlowStatistics/DataFlowStatistics.tmpl",
		}
	} else {
		pluginEnvConfig["templatePath"] = []string{
			"trc_templates/TrcDb/DataFlowStatistics/DataFlowStatistics.tmpl",
		}
	}

	pluginEnvConfig["flumeTemplatePath"] = []string{
		"trc_templates/FlumeDatabase/TierceronFlow/TierceronFlow.tmpl", // implemented.
	}
	return pluginEnvConfig
}

func GetFlowMachineTemplatesHive() map[string]any {
	pluginEnvConfig := map[string]any{}
	pluginEnvConfig["templatePath"] = []string{
		"trc_templates/TrcDb/DataFlowStatistics/DataFlowStatistics.tmpl",
	}
	pluginEnvConfig["flumeTemplatePath"] = []string{
		"trc_templates/FlumeDatabase/TierceronFlow/TierceronFlow.tmpl", // implemented.
	}
	// Add connections.
	pluginEnvConfig["connectionPath"] = []string{
		"trc_templates/TrcVault/Database/config.yml.tmpl", // Connections to databases
		"trc_templates/TrcVault/Identity/config.yml.tmpl", // Connections to identity service
	}

	return pluginEnvConfig
}

func IsSupportedFlow(flow string) bool {
	return flow != "" && (flow == flowcore.TierceronControllerFlow.FlowName() || flow == flowcore.ArgosSociiFlow.FlowName() || flow == flowcore.DataFlowStatConfigurationsFlow.FlowName())
}

func IsHiveSupportedFlow(flow string) bool {
	return flow != "" && (flow == flowcore.TierceronControllerFlow.FlowName() || flow == flowcore.DataFlowStatConfigurationsFlow.FlowName())
}

func GetFlowMachineTemplatesEditor() map[string]any {
	pluginEnvConfig := map[string]any{}
	pluginEnvConfig["templatePath"] = []string{
		"trc_templates/TrcDb/DataFlowStatistics/DataFlowStatistics.tmpl",
		"trc_templates/TrcDb/ArgosSocii/ArgosSocii.tmpl",
	}
	pluginEnvConfig["flumeTemplatePath"] = []string{
		"trc_templates/FlumeDatabase/TierceronFlow/TierceronFlow.tmpl", // implemented.
	}
	return pluginEnvConfig
}

func GetBusinessFlows() []flowcore.FlowDefinition {
	// Not implemented for hive infrastructure yet
	return []flowcore.FlowDefinition{}
}

func GetTestFlows() []flowcore.FlowDefinition {
	return []flowcore.FlowDefinition{}
}

func GetTestFlowsByState(teststate string) []flowcore.FlowDefinition {
	return []flowcore.FlowDefinition{}
}

func TestFlowController(tfmContext flowcore.FlowMachineContext, tfContext flowcore.FlowContext) error {
	return nil
}

func GetFlowMachineInitContext(coreConfig *coreconfig.CoreConfig, pluginName string) *flowcore.FlowMachineInitContext {
	var flowMachineTemplatesFunc func() map[string]any
	var isSupportedFlow func(flowName string) bool
	if coreConfig != nil && coreConfig.IsEditor {
		flowMachineTemplatesFunc = GetFlowMachineTemplatesEditor
		isSupportedFlow = IsSupportedFlow
	} else {
		flowMachineTemplatesFunc = GetFlowMachineTemplatesHive
		isSupportedFlow = IsHiveSupportedFlow
	}
	flowMachineTemplates := flowMachineTemplatesFunc()
	flowChatMsgSenderChan := make(chan *core.ChatMsg)

	return &flowcore.FlowMachineInitContext{
		GetFlowMachineTemplates:     flowMachineTemplatesFunc,
		FlowMachineInterfaceConfigs: map[string]any{},
		GetDatabaseName: func(flumeDbType flowcore.FlumeDbType) string {
			switch flumeDbType {
			case flowcore.TrcDb:
				return GetDatabaseName()
			case flowcore.TrcFlumeDb:
				return GetFlowDatabaseName()
			}
			return GetDatabaseName()
		},
		IsSupportedFlow: isSupportedFlow,
		GetTableFlows: func() []flowcore.FlowDefinition {
			tableFlows := []flowcore.FlowDefinition{}
			for _, template := range flowMachineTemplates["templatePath"].([]string) {
				flowSource, service, _, tableTemplateName := coreutil.GetProjectService("", "trc_templates", template)
				tableName := coreutil.GetTemplateFileName(tableTemplateName, service)
				tableFlows = append(tableFlows, flowcore.FlowDefinition{
					FlowHeader: flowcore.FlowHeaderType{
						Name:      flowcore.FlowNameType(tableName),
						Source:    flowSource,
						Instances: "*",
					},
					FlowTemplatePath: template,
				})
			}
			return tableFlows
		},
		GetBusinessFlows:      GetBusinessFlows,
		GetTestFlows:          GetTestFlows,
		GetTestFlowsByState:   GetTestFlowsByState,
		FlowController:        ProcessFlowController,
		TestFlowController:    TestFlowController,
		FlowChatMsgSenderChan: &flowChatMsgSenderChan,
	}
}
