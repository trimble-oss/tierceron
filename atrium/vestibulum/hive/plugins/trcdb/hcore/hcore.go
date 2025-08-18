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
		tccore.SendDfStat(configContext, dfsctx, dfstat)
		configContext.Log.Println("Sent dataflow statistic with error to kernel: ", dfstat.Name)
	}
	*configContext.ErrorChan <- err
}

func GetWantedTests(event *core.ChatMsg) (map[string]bool, string) {
	var argosId string
	testsSet := map[string]bool{}
	if event.ChatId != nil && strings.Contains(*(*event).ChatId, ":") {
		argosTestsSlice := strings.Split(*(*event).ChatId, ":")
		// Scrub tenantId to only alphabetic characters
		origArgosId := argosTestsSlice[0]
		var scrubbedTenantId strings.Builder
		for _, r := range origArgosId {
			if (r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z') {
				scrubbedTenantId.WriteRune(r)
			}
		}
		argosId = scrubbedTenantId.String()
		testsCommaList := argosTestsSlice[1]
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

func TrcdbChatMessageHandler(event *core.ChatMsg) (string, error) {
	if event.ChatId != nil && strings.Contains(*(*event).ChatId, ":") {
		testsSet, tenantId := GetWantedTests(event)
		approvedProdTests := make(map[string]bool)
		for test, _ := range testsSet {
			if test == "TODO" {
				// In production, we only want to run the EnterpriseRegistration test
				approvedProdTests[test] = true
			}
		}
		if IsProd() && len(approvedProdTests) == 0 {
			return "Test not supported in production", nil
		} else if IsProd() {
			testsSet = approvedProdTests
		}

		for test := range testsSet {

			trcdbExchange := &core.TrcdbExchange{
				Flows:     []string{"DataFlowStatistics"},                                                                                                                                      // Flows
				Query:     fmt.Sprintf("SELECT * FROM %s.DataFlowStatistics where flowName='%s' and argpsId like '%s' ORDER by CAST(stateCode as UNSIGNED) asc", "%s", test, string(tenantId)), // Query
				Operation: "SELECT",                                                                                                                                                            // query operation
			}
			processTrcdb(trcdbExchange)

			if len(trcdbExchange.Response.Rows) > 0 {
				var report strings.Builder
				report.WriteString("Report for " + tenantId + ":\n")
				report.WriteString("Step # | Status | Result | Execution Date | Time Spent\n")
				report.WriteString("-----------------------------------------------------------------------------\n")

				// Get the lastTestedDate from the first row (assumed to be the same for all rows)
				var baseTimestamp time.Time
				var err error

				if len(trcdbExchange.Response.Rows) > 0 && len(trcdbExchange.Response.Rows[0]) >= 9 {
					baseTimestampStr := fmt.Sprint(trcdbExchange.Response.Rows[0][8]) // Index 8 for timestamp
					baseTimestamp, err = time.Parse(time.RFC3339, baseTimestampStr)
					if err != nil {
						// If we can't parse the timestamp, continue but note the error
						report.WriteString("Warning: Could not parse base timestamp. Execution times may be inaccurate.\n\n")
					}
				}

				// Keep track of cumulative time to add to base timestamp
				var cumulativeTime time.Duration

				for _, row := range trcdbExchange.Response.Rows {
					if len(row) >= 9 {
						stateCode := fmt.Sprint(row[5])    // Start at index 5 for stateCode
						modeStr := fmt.Sprint(row[4])      // Index 4 for mode
						stateName := fmt.Sprint(row[6])    // Index 6 for stateName
						timeSplitStr := fmt.Sprint(row[7]) // Index 7 for timeSplit, to be displayed unaltered

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

						executionTime := baseTimestamp.Add(cumulativeTime)
						executionTimeDisplay := executionTime.Format("15:04:05.000")

						// Add to report
						report.WriteString(fmt.Sprintf("%s | %s | %s | %s | %s\n",
							stateCode, modeText, stateName, executionTimeDisplay, timeSplitStr))
					}
				}

				// Add total duration at the end
				if cumulativeTime > 0 {
					report.WriteString(fmt.Sprintf("\nTotal Duration: %s\n", cumulativeTime.String()))
				}

				return report.String(), nil
			}
		}
	}
	return "", nil
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
		case event.ChatId == nil || (*event).ChatId == nil || (event.ChatId != nil && (*event).ChatId != nil && *event.ChatId != "PIPELINETEST"):
			GetConfigContext("trcdb").Log.Println("Trcdb received chat message")
			response, err := TrcdbChatMessageHandler(event)
			if err != nil {
				GetConfigContext("trcdb").Log.Printf("Trcdb errored with %s\n", err.Error())
			} else {
				if event.Name != nil && *event.Name == "trcdb" {
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

func processTrcdb(trcdbExchange *core.TrcdbExchange) {
	if tfmContext == nil {
		configContext.Log.Printf("trcdb - Request receive before initialization.")
		return
	}
	if trcdbExchange == nil {
		configContext.Log.Printf("Invalid TrcdbExchange received, ignoring request.")
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
		trcdbExchange, _ = tfmContext.CallDBQueryN(trcdbExchange, query, nil, false, trcdbExchange.Operation, nil, "")
		if len(trcdbExchange.Response.Rows) == 0 {
			configContext.Log.Println("TrcdbExchange operation did not get any results.  returning empty response.")
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
		configContext.Log.Println("trcdb reported startup.")
	} else {
		configContext.Log.Println("No command sender channel available, trcdb cannot send start event.")
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
	// switch tfContext.GetFlowHeader().FlowName() {
	// case flowcore.ArgosSociiFlow.TableName():
	// 	tfContext.SetFlowLibraryContext(argossocii.GetProcessFlowDefinition())
	// default:
	// 	return errors.New("Flow not implemented: " + tfContext.GetFlowHeader().FlowName())
	// }

	return flowcore.ProcessTableConfigurations(tfmContext, tfContext)
}

func GetDatabaseName() string {
	return "TrcDb"
}
func GetFlowDatabaseName() string {
	return "FlumeDb"
}

func IsSupportedFlow(flow string) bool {
	return flow != "" && (flow == flowcore.ArgosSociiFlow.FlowName() || flow == flowcore.DataFlowStatConfigurationsFlow.FlowName())
}

func GetFlowMachineTemplates() map[string]any {
	flowMachineTemplates := map[string]any{}
	flowMachineTemplates["templatePath"] = []string{
		"trc_templates/TrcDb/DataFlowStatistics/DataFlowStatistics.tmpl", // implemented.
		"trc_templates/TrcDb/ArgosSocii/ArgosSocii.tmpl",                 // implemented.
	}
	flowMachineTemplates["flumeTemplatePath"] = []string{
		"trc_templates/FlumeDatabase/TierceronFlow/TierceronFlow.tmpl", // implemented.
	}

	return flowMachineTemplates
}

func GetFlowMachineTemplatesHive() map[string]any {
	flowMachineTemplates := map[string]any{}
	flowMachineTemplates["templatePath"] = []string{
		"trc_templates/TrcDb/DataFlowStatistics/DataFlowStatistics.tmpl", // implemented.
		"trc_templates/TrcDb/ArgosSocii/ArgosSocii.tmpl",                 // implemented.
	}
	flowMachineTemplates["flumeTemplatePath"] = []string{
		"trc_templates/FlumeDatabase/TierceronFlow/TierceronFlow.tmpl", // implemented.
	}

	return flowMachineTemplates
}

func GetFlowMachineTemplatesEditor() map[string]any {
	flowMachineTemplates := map[string]any{}
	flowMachineTemplates["templatePath"] = []string{
		"trc_templates/TrcDb/DataFlowStatistics/DataFlowStatistics.tmpl", // implemented.
		"trc_templates/TrcDb/ArgosSocii/ArgosSocii.tmpl",                 // implemented.
	}
	flowMachineTemplates["flumeTemplatePath"] = []string{
		"trc_templates/FlumeDatabase/TierceronFlow/TierceronFlow.tmpl", // implemented.
	}

	return flowMachineTemplates
}

func GetBusinessFlows() []flowcore.FlowDefinition {
	// Not implemented for hive infrastructure yet
	// return tccutil.GetAdditionalFlows()
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
	if coreConfig != nil && coreConfig.IsEditor {
		flowMachineTemplatesFunc = GetFlowMachineTemplatesEditor
	} else {
		flowMachineTemplatesFunc = GetFlowMachineTemplatesHive
	}
	flowMachineTemplates := flowMachineTemplatesFunc()

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
		IsSupportedFlow: IsSupportedFlow,
		GetTableFlows: func() []flowcore.FlowDefinition {
			tableFlows := []flowcore.FlowDefinition{}
			for _, template := range flowMachineTemplates["templatePath"].([]string) {
				flowSource, service, _, tableTemplateName := coreutil.GetProjectService("", "trc_templates", template)
				tableName := coreutil.GetTemplateFileName(tableTemplateName, service)
				tableFlows = append(tableFlows, flowcore.FlowDefinition{
					FlowHeader: flowcore.FlowHeaderType{
						Name:      flowcore.FlowNameType(tableName),
						Source:    flowSource,
						Instances: "*"},
					FlowTemplatePath: template,
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
		configContext.Log.Println("Missing common config components")
		return
	}
	if tfmCtx, ok := (*properties)[tccore.TRCDB_RESOURCE].(flowcore.FlowMachineContext); ok {
		tfmContext = tfmCtx
	} else {
		configContext.Log.Println("No flow context available for trcdb plugin.")
		return
	}
}

func GetPluginMessages(pluginName string) []string {
	return []string{}
}
