package core

import (
	"errors"
	"fmt"
	"log"

	//"os"

	"strings"

	"github.com/trimble-oss/tierceron/pkg/utils/config"

	tccore "github.com/trimble-oss/tierceron-core/v2/core"
	tcflow "github.com/trimble-oss/tierceron-core/v2/flow"
	"github.com/trimble-oss/tierceron/buildopts"
	"github.com/trimble-oss/tierceron/buildopts/coreopts"
	"github.com/trimble-oss/tierceron/pkg/core"
	"github.com/trimble-oss/tierceron/pkg/vaulthelper/kv"

	"time"

	trcdbutil "github.com/trimble-oss/tierceron/pkg/core/dbutil"

	dfssql "github.com/trimble-oss/tierceron/atrium/vestibulum/trcflow/flows/flowsql"

	"github.com/trimble-oss/tierceron-nute-core/mashupsdk"
)

var PUBLIC_INDEX_BASIS_PATH string = "super-secrets/PublicIndex/%s"
var HIVE_STAT_DFG_PATH string = fmt.Sprintf("%s%s", PUBLIC_INDEX_BASIS_PATH, "/%s/%s/DataFlowStatistics/DataFlowGroup")
var HIVE_STAT_PATH string = fmt.Sprintf("%s%s", HIVE_STAT_DFG_PATH, "/%s/dataFlowName/%s")
var HIVE_STAT_CODE_PATH string = fmt.Sprintf("%s%s", HIVE_STAT_PATH, "/%s")

// New API -> Argosy, return dataFlowGroups populated
func InitArgosyFleet(mod *kv.Modifier, project string, logger *log.Logger) (*tccore.TTDINode, error) {
	var aFleet tccore.TTDINode
	aFleet.MashupDetailedElement = &mashupsdk.MashupDetailedElement{}
	aFleet.MashupDetailedElement.Name = project
	aFleet.ChildNodes = make([]*tccore.TTDINode, 0)
	idNameListData, serviceListErr := mod.List(fmt.Sprintf(PUBLIC_INDEX_BASIS_PATH, project), logger)
	if serviceListErr != nil || idNameListData == nil {
		return &aFleet, serviceListErr
	}

	if serviceListErr != nil || idNameListData == nil {
		return &aFleet, errors.New("no project was found for argosyFleet")
	}

	for _, idNameList := range idNameListData.Data {
		for _, idName := range idNameList.([]interface{}) {
			idListData, idListErr := mod.List(fmt.Sprintf("super-secrets/Index/%s/tenantId", project), logger)
			if idListErr != nil || idListData == nil {
				return &aFleet, idListErr
			}

			if idListData == nil {
				return &aFleet, errors.New("no argosId were found for argosyFleet")
			}
			idName = strings.Trim(idName.(string), "/")

			if mod.Direct {
				data, readErr := mod.ReadData("super-secrets/Protected/SpiralDatabase/config")
				if readErr != nil {
					return &aFleet, readErr
				} else {
					driverConfig := &config.DriverConfig{
						CoreConfig: &core.CoreConfig{
							ExitOnFailure: true,
							Insecure:      mod.Insecure,
							Log:           logger,
						},
					}

					sourceDatabaseConnectionMap := map[string]interface{}{
						"dbsourceurl":      buildopts.BuildOptions.GetTrcDbUrl(data),
						"dbsourceuser":     data["dbuser"],
						"dbsourcepassword": data["dbpassword"],
					}
					dbsourceConn, err := trcdbutil.OpenDirectConnection(driverConfig, nil, sourceDatabaseConnectionMap["dbsourceurl"].(string), sourceDatabaseConnectionMap["dbsourceuser"].(string), func() (string, error) { return sourceDatabaseConnectionMap["dbsourcepassword"].(string), nil })

					if err != nil {
						log.Println(err)
						return &aFleet, err
					}
					// use your own select statement
					// this is just an example statement

					statement, err := dbsourceConn.Prepare("select * from DataflowStatistics order by argosid,flowGroup,flowName,stateName asc")

					if err != nil {
						log.Println(err)
						return &aFleet, err
					}

					rows, err := statement.Query() // execute our select statement

					if err != nil {
						log.Println(err)
						return &aFleet, err
					}
					defer rows.Close()

					argosyMap := map[string]*tccore.TTDINode{}

					for _, idList := range idListData.Data {
						for _, id := range idList.([]interface{}) {
							argosId := strings.Trim(id.(string), "/")
							argosNode := &tccore.TTDINode{MashupDetailedElement: &mashupsdk.MashupDetailedElement{}}
							argosNode.MashupDetailedElement = &mashupsdk.MashupDetailedElement{}
							argosNode.MashupDetailedElement.Name = argosId
							argosNode.ChildNodes = make([]*tccore.TTDINode, 0)
							argosyMap[argosId] = argosNode
						}
					}
					i := 1

					for rows.Next() {
						fmt.Printf("%d\n", i)
						i = i + 1
						var flowName, argosId, flowGroup, mode, stateCode, stateName, timeSplit, lastTestedDate string
						rows.Scan(&flowName, &argosId, &flowGroup, &mode, &stateCode, &stateName, &timeSplit, &lastTestedDate)

						data := make(map[string]interface{})
						data["flowGroup"] = flowGroup
						data["flowName"] = flowName
						data["stateName"] = stateName
						data["stateCode"] = stateCode
						data["mode"] = mode
						data["timeSplit"] = timeSplit
						data["lastTestedDate"] = lastTestedDate

						argosNode := argosyMap[argosId]
						if argosNode == nil {
							newArgosNode := tccore.TTDINode{MashupDetailedElement: &mashupsdk.MashupDetailedElement{}}
							argosNode = &newArgosNode
							newArgosNode.MashupDetailedElement.Name = argosId
							argosyMap[argosId] = argosNode
						}

						var argosDfGroup *tccore.TTDINode
						for i := 0; i < len(argosNode.ChildNodes); i++ {
							if argosNode.ChildNodes[i].Name == flowGroup {
								argosDfGroup = argosNode.ChildNodes[i]
								break
							}
						}
						if argosDfGroup == nil {
							newArgosDfGroup := tccore.TTDINode{MashupDetailedElement: &mashupsdk.MashupDetailedElement{}}
							newArgosDfGroup.MashupDetailedElement.Name = flowGroup
							argosNode.ChildNodes = append(argosNode.ChildNodes, &newArgosDfGroup)
							argosDfGroup = argosNode.ChildNodes[len(argosNode.ChildNodes)-1]
						}

						if strings.Contains(flowName, "-") {
							dashNameSplit := strings.Split(flowName, "-")
							statisticType := dashNameSplit[0] //login
							//statisticID := dashNameSplit[1]   //audguasdfniuasfd-gnasdfkj
							var dfStatTypeNode *tccore.TTDINode
							for i := 0; i < len(argosDfGroup.ChildNodes); i++ {
								if argosDfGroup.ChildNodes[i].Name == statisticType {
									dfStatTypeNode = argosDfGroup.ChildNodes[i]
									break
								}
							}
							if dfStatTypeNode == nil {
								newDfStatTypeNode := tccore.TTDINode{MashupDetailedElement: &mashupsdk.MashupDetailedElement{}}

								newDfStatTypeNode.MashupDetailedElement.Name = statisticType
								argosDfGroup.ChildNodes = append(argosDfGroup.ChildNodes, &newDfStatTypeNode)
								dfStatTypeNode = argosDfGroup.ChildNodes[len(argosDfGroup.ChildNodes)-1]
							}

							var dfStatNameTypeNode *tccore.TTDINode
							for i := 0; i < len(dfStatTypeNode.ChildNodes); i++ {
								if (*dfStatTypeNode.ChildNodes[i]).Name == flowName {
									dfStatNameTypeNode = dfStatTypeNode.ChildNodes[i]
									break
								}
							}
							if dfStatNameTypeNode == nil {
								newDfStatNameTypeNode := tccore.TTDINode{MashupDetailedElement: &mashupsdk.MashupDetailedElement{}}
								newDfStatNameTypeNode.MashupDetailedElement.Name = flowName
								dfStatTypeNode.ChildNodes = append(dfStatTypeNode.ChildNodes, &newDfStatNameTypeNode)
								dfStatNameTypeNode = dfStatTypeNode.ChildNodes[len(dfStatTypeNode.ChildNodes)-1]
							}

							// Always append this remaining flow...
							dfStatisticNode := tccore.InitDataFlow(nil, flowName, false)
							dfStatisticNode.MapStatistic(data, logger)

							dfStatNameTypeNode.ChildNodes = append(dfStatNameTypeNode.ChildNodes, dfStatisticNode)

						} else {
							var dfStatTypeNode *tccore.TTDINode
							for i := 0; i < len(argosDfGroup.ChildNodes); i++ {
								if argosDfGroup.ChildNodes[i].Name == flowName {
									dfStatTypeNode = argosDfGroup.ChildNodes[i]
									break
								}
							}
							if dfStatTypeNode == nil {
								newDfStatTypeNode := tccore.TTDINode{MashupDetailedElement: &mashupsdk.MashupDetailedElement{}}
								newDfStatTypeNode.MashupDetailedElement.Name = flowName
								argosDfGroup.ChildNodes = append(argosDfGroup.ChildNodes, &newDfStatTypeNode)
								dfStatTypeNode = argosDfGroup.ChildNodes[len(argosDfGroup.ChildNodes)-1]
							}

							dfStatisticNode := tccore.InitDataFlow(nil, flowName, false)
							dfStatisticNode.MapStatistic(data, logger)
							dfStatTypeNode.ChildNodes = append(dfStatTypeNode.ChildNodes, dfStatisticNode)
						}
					}
					for _, aArgosy := range argosyMap {
						aFleet.ChildNodes = append(aFleet.ChildNodes, aArgosy)
					}
					return &aFleet, nil
				}
			}

			for _, idList := range idListData.Data {
				for _, id := range idList.([]interface{}) {
					id = strings.Trim(id.(string), "/")
					statPath := fmt.Sprintf(
						HIVE_STAT_DFG_PATH,
						project,
						idName.(string),
						id.(string))

					serviceListData, serviceListErr := mod.List(statPath,
						logger)

					if serviceListErr != nil {
						return &aFleet, serviceListErr
					}
					var new tccore.TTDINode
					new.MashupDetailedElement.Name = strings.TrimSuffix(id.(string), "/")
					new.ChildNodes = make([]*tccore.TTDINode, 0)

					if serviceListData == nil { //No existing dfs for this tenant -> continue
						aFleet.ChildNodes = append(aFleet.ChildNodes, &new)
						continue
					}

					for _, serviceList := range serviceListData.Data {
						for _, service := range serviceList.([]interface{}) {
							var dfgroup tccore.TTDINode
							dfgroup.MashupDetailedElement = &mashupsdk.MashupDetailedElement{}
							dfgroup.MashupDetailedElement.Name = strings.TrimSuffix(service.(string), "/")
							statisticNameList, statisticNameListErr := mod.List(fmt.Sprintf("%s/%s/dataFlowName/",
								statPath,
								service.(string)),
								logger)

							if statisticNameListErr != nil {
								return &aFleet, statisticNameListErr
							}

							if statisticNameList == nil {
								continue
							}

							var innerDF tccore.TTDINode
							innerDF.MashupDetailedElement = &mashupsdk.MashupDetailedElement{}
							innerDF.MashupDetailedElement.Name = "empty"
							//Tenant -> System -> Login/Download -> USERS
							for _, statisticName := range statisticNameList.Data {
								for _, statisticName := range statisticName.([]interface{}) {
									if strings.Contains(statisticName.(string), "-") {
										dashNameSplit := strings.Split(statisticName.(string), "-")
										statisticType := dashNameSplit[0] //login
										innerDF.MashupDetailedElement.Name = strings.TrimSuffix(statisticType, "/")
										//statisticID := dashNameSplit[1]   //audguasdfniuasfd-gnasdfkj
										newDf := tccore.InitDataFlow(nil, strings.TrimSuffix(statisticName.(string), "/"), false)
										RetrieveStatistic(mod, newDf, id.(string), project, idName.(string), service.(string), statisticName.(string), logger)
										innerDF.ChildNodes = append(innerDF.ChildNodes, newDf)
									} else {
										newDf := tccore.InitDataFlow(nil, strings.TrimSuffix(statisticName.(string), "/"), false)
										RetrieveStatistic(mod, newDf, id.(string), project, idName.(string), service.(string), statisticName.(string), logger)
										dfgroup.ChildNodes = append(dfgroup.ChildNodes, newDf)
									}
								}
							}
							if innerDF.MashupDetailedElement.Name != "empty" {
								dfgroup.ChildNodes = append(dfgroup.ChildNodes, &innerDF)
							}
							new.ChildNodes = append(new.ChildNodes, &dfgroup)
						}
					}
					aFleet.ChildNodes = append(aFleet.ChildNodes, &new)
				}
			}
		}
	}
	return &aFleet, nil
}

func DeliverStatistic(tfmContext *TrcFlowMachineContext,
	tfContext *TrcFlowContext,
	mod *kv.Modifier,
	dfs *tccore.TTDINode,
	id string,
	indexPath string,
	idName string,
	logger *log.Logger,
	vaultWriteBack bool) {
	//TODO : Write Statistic to vault
	dfs.FinishStatisticLog()
	dsc, _, err := dfs.GetDeliverStatCtx()
	if err != nil {
		logger.Printf("Unable to access deliver statistic context for DeliverStatistic: %v\n", err)
		return
	}
	mod.SectionPath = ""
	for _, dataFlowStatistic := range dfs.ChildNodes {
		dfStatDeliveryCtx, _, deliverStatErr := dataFlowStatistic.GetDeliverStatCtx()
		if deliverStatErr != nil && dsc.LogFunc != nil {
			(*dsc.LogFunc)("Error extracting deliver stat ctx", deliverStatErr)
		}

		statMap := dataFlowStatistic.FinishStatistic(id, indexPath, idName, logger, vaultWriteBack, dsc)

		mod.SectionPath = ""
		statPath := fmt.Sprintf(
			HIVE_STAT_CODE_PATH,
			indexPath,
			idName,
			id,
			dfStatDeliveryCtx.FlowGroup,
			dfStatDeliveryCtx.FlowName,
			dfStatDeliveryCtx.StateCode,
		)

		if vaultWriteBack {
			mod.SectionPath = ""
			_, writeErr := mod.Write(statPath, statMap, logger)
			if writeErr != nil && dsc.LogFunc != nil {
				(*dsc.LogFunc)("Error writing out DataFlowStatistics to vault", writeErr)
			}
		} else {
			if tfmContext != nil && tfContext != nil {
				_, changed := tfmContext.CallDBQuery(tfContext, dfssql.GetDataFlowStatisticInsertById(id, statMap, coreopts.BuildOptions.GetDatabaseName(), "DataFlowStatistics"), nil, true, "INSERT", []tcflow.FlowNameType{tcflow.FlowNameType("DataFlowStatistics")}, "")
				if !changed {
					// Write directly even if query reports nothing changed...  We want all statistics to be written
					// during registrations.
					mod.SectionPath = ""
					_, writeErr := mod.Write(statPath, statMap, logger)
					if writeErr != nil && dsc.LogFunc != nil {
						(*dsc.LogFunc)("Error writing out DataFlowStatistics to vault", writeErr)
					}
				}
			}
		}
	}
}

func RetrieveStatistic(mod *kv.Modifier, dfs *tccore.TTDINode, id string, indexPath string, idName string, flowG string, flowN string, logger *log.Logger) error {
	statPath := fmt.Sprintf(
		HIVE_STAT_PATH,
		indexPath,
		idName,
		id,
		flowG,
		flowN)

	listData, listErr := mod.List(statPath, logger)
	if listErr != nil {
		return listErr
	}

	for _, stateCodeList := range listData.Data {
		for _, stateCode := range stateCodeList.([]interface{}) {
			path := fmt.Sprintf("%s/%s", statPath, stateCode.(string))
			data, readErr := mod.ReadData(path)
			if readErr != nil {
				return readErr
			}
			if data == nil {
				time.Sleep(1000)
				data, readErr := mod.ReadData(path)
				if readErr == nil && data == nil {
					return nil
				}
			}
			if testedDate, testedDateOk := data["lastTestedDate"].(string); testedDateOk {
				if testedDate == "" {
					flowData, flowReadErr := mod.ReadData(fmt.Sprintf("super-secrets/%s", data["flowGroup"].(string)))
					// if flowReadErr != nil {
					// 	return flowReadErr
					// } ***

					if _, ok := flowData["lastTestedDate"].(string); ok && flowReadErr != nil {
						data["lastTestedDate"] = flowData["lastTestedDate"].(string)
					} else {
						data["lastTestedDate"] = ""
					}
				} else {
					data["lastTestedDate"] = testedDate
				}
			}
			df := tccore.TTDINode{MashupDetailedElement: &mashupsdk.MashupDetailedElement{}}
			df.MapStatistic(data, logger)
			dfs.ChildNodes = append(dfs.ChildNodes, &df)
		}
	}
	return nil
}

func UpdateLastTestedDate(mod *kv.Modifier, dfs *tccore.DeliverStatCtx, statMap map[string]interface{}) {
	if _, ok := statMap["lastTestedDate"].(string); ok {
		if statMap["lastTestedDate"].(string) == "" {
			flowData, flowReadErr := mod.ReadData(fmt.Sprintf("super-secrets/%s", dfs.FlowGroup))
			if flowReadErr != nil && dfs.LogFunc != nil {
				(*dfs.LogFunc)("Error reading flow properties from vault", flowReadErr)
			}

			if _, ok := flowData["lastTestedDate"].(string); ok {
				statMap["lastTestedDate"] = flowData["lastTestedDate"].(string)
			}
		}
	}
}
