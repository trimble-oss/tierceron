package core

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"

	//"os"

	"strings"

	tccore "github.com/trimble-oss/tierceron-core/core"
	"github.com/trimble-oss/tierceron/buildopts"
	"github.com/trimble-oss/tierceron/buildopts/coreopts"
	"github.com/trimble-oss/tierceron/pkg/core"
	eUtils "github.com/trimble-oss/tierceron/pkg/utils"
	"github.com/trimble-oss/tierceron/pkg/vaulthelper/kv"

	"time"

	trcdbutil "github.com/trimble-oss/tierceron/pkg/core/dbutil"

	dfssql "github.com/trimble-oss/tierceron/atrium/vestibulum/trcflow/flows/flowsql"

	"github.com/trimble-oss/tierceron-nute/mashupsdk"
)

// type DataFlowStatistic struct {
// 	mashupsdk.MashupDetailedElement
// 	FlowGroup string
// 	FlowName  string
// 	StateName string
// 	StateCode string
// 	TimeSplit time.Duration
// 	Mode      int
// }

// type DataFlow struct {
// 	mashupsdk.MashupDetailedElement
// 	Name       string
// 	TimeStart  time.Time
// 	Statistics []DataFlowStatistic
// 	LogStat    bool
// 	LogFunc    func(string, error)
// }

// type DataFlowGroup struct {
// 	mashupsdk.MashupDetailedElement
// 	Name  string
// 	Flows []DataFlow
// }

// type Argosy struct {
// 	mashupsdk.MashupDetailedElement
// 	ArgosyID string
// 	Groups   []DataFlowGroup
// }

// type ArgosyFleet struct {
// 	ArgosyName string
// 	Argosies   []Argosy
// }

// type TTDINode struct {
// 	*mashupsdk.MashupDetailedElement
// 	//Data       []byte
// 	ChildNodes []*TTDINode
// }

// New API -> Argosy, return dataFlowGroups populated
func InitArgosyFleet(mod *kv.Modifier, project string, logger *log.Logger) (*tccore.TTDINode, error) {
	var aFleet tccore.TTDINode
	aFleet.MashupDetailedElement = &mashupsdk.MashupDetailedElement{}
	aFleet.MashupDetailedElement.Name = project
	aFleet.ChildNodes = make([]*tccore.TTDINode, 0)
	idNameListData, serviceListErr := mod.List("super-secrets/PublicIndex/"+project, logger)
	if serviceListErr != nil || idNameListData == nil {
		return &aFleet, serviceListErr
	}

	if serviceListErr != nil || idNameListData == nil {
		return &aFleet, errors.New("no project was found for argosyFleet")
	}

	for _, idNameList := range idNameListData.Data {
		for _, idName := range idNameList.([]interface{}) {
			idListData, idListErr := mod.List("super-secrets/Index/"+project+"/tenantId", logger)
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
					driverConfig := &eUtils.DriverConfig{
						CoreConfig: core.CoreConfig{
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
					dbsourceConn, err := trcdbutil.OpenDirectConnection(&driverConfig.CoreConfig, sourceDatabaseConnectionMap["dbsourceurl"].(string), sourceDatabaseConnectionMap["dbsourceuser"].(string), sourceDatabaseConnectionMap["dbsourcepassword"].(string))

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
						if mode == "2" {
							fmt.Println("hi")
						}
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
					serviceListData, serviceListErr := mod.List("super-secrets/PublicIndex/"+project+"/"+idName.(string)+"/"+id.(string)+"/DataFlowStatistics/DataFlowGroup", logger)
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

							statisticNameList, statisticNameListErr := mod.List("super-secrets/PublicIndex/"+project+"/"+idName.(string)+"/"+id.(string)+"/DataFlowStatistics/DataFlowGroup/"+service.(string)+"/dataFlowName/", logger)
							if statisticNameListErr != nil {
								return &aFleet, statisticNameListErr
							}

							if statisticNameList == nil {
								continue
							}

							var innerDF tccore.TTDINode
							innerDF.MashupDetailedElement = &mashupsdk.MashupDetailedElement{}
							innerDF.MashupDetailedElement.Name = "empty"
							//Tenant -> System -> LOgin/Download -> USERS
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

	//var newDFStatistic = DataFlowGroup{Name: name, TimeStart: time.Now(), Statistics: nil, LogStat: false, LogFunc: nil}
	return &aFleet, nil
}

func DeliverStatistic(tfmContext *TrcFlowMachineContext, tfContext *TrcFlowContext, mod *kv.Modifier, dfs *tccore.TTDINode, id string, indexPath string, idName string, logger *log.Logger, vaultWriteBack bool) {
	//TODO : Write Statistic to vault
	dfs.FinishStatisticLog()
	dsc, err := dfs.GetDeliverStatCtx()
	if err != nil {
		logger.Printf("Unable to access deliver statistic context for DeliverStatistic: %v\n", err)
		return
	}
	mod.SectionPath = ""
	for _, dataFlowStatistic := range dfs.ChildNodes {
		dfStatDeliveryCtx, deliverStatErr := dataFlowStatistic.GetDeliverStatCtx()
		if deliverStatErr != nil && dsc.LogFunc != nil {
			(*dsc.LogFunc)("Error extracting deliver stat ctx", deliverStatErr)
		}

		statMap := dataFlowStatistic.FinishStatistic(id, indexPath, idName, logger, vaultWriteBack, dsc)

		mod.SectionPath = ""
		if vaultWriteBack {
			mod.SectionPath = ""
			_, writeErr := mod.Write("super-secrets/PublicIndex/"+indexPath+"/"+idName+"/"+id+"/DataFlowStatistics/DataFlowGroup/"+statMap["flowGroup"].(string)+"/dataFlowName/"+statMap["flowName"].(string)+"/"+statMap["stateCode"].(string), statMap, logger)
			if writeErr != nil && dsc.LogFunc != nil {
				// logFunc := dsc.LogFunc.(func(string, error))
				(*dsc.LogFunc)("Error writing out DataFlowStatistics to vault", writeErr)

				//dfs.LogFunc("Error writing out DataFlowStatistics to vault", writeErr)
			}
		} else {
			if tfmContext != nil && tfContext != nil {
				_, changed := tfmContext.CallDBQuery(tfContext, dfssql.GetDataFlowStatisticInsert(id, statMap, coreopts.BuildOptions.GetDatabaseName(), "DataFlowStatistics"), nil, true, "INSERT", []FlowNameType{FlowNameType("DataFlowStatistics")}, "")
				if !changed {
					// Write directly even if query reports nothing changed...  We want all statistics to be written
					// during registrations.
					mod.SectionPath = ""
					_, writeErr := mod.Write("super-secrets/PublicIndex/"+indexPath+"/"+idName+"/"+id+"/DataFlowStatistics/DataFlowGroup/"+dfStatDeliveryCtx.FlowGroup+"/dataFlowName/"+dfStatDeliveryCtx.FlowName+"/"+dfStatDeliveryCtx.StateCode, statMap, logger)
					if writeErr != nil && dsc.LogFunc != nil {
						// logFunc := decodedData["LogFunc"].(func(string, error))
						(*dsc.LogFunc)("Error writing out DataFlowStatistics to vault", writeErr)
					}
				}
			}
		}
	}
}

func RetrieveStatistic(mod *kv.Modifier, dfs *tccore.TTDINode, id string, indexPath string, idName string, flowG string, flowN string, logger *log.Logger) error {
	listData, listErr := mod.List("super-secrets/PublicIndex/"+indexPath+"/"+idName+"/"+id+"/DataFlowStatistics/DataFlowGroup/"+flowG+"/dataFlowName/"+flowN, logger)
	if listErr != nil {
		return listErr
	}

	for _, stateCodeList := range listData.Data {
		for _, stateCode := range stateCodeList.([]interface{}) {
			data, readErr := mod.ReadData("super-secrets/PublicIndex/" + indexPath + "/" + idName + "/" + id + "/DataFlowStatistics/DataFlowGroup/" + flowG + "/dataFlowName/" + flowN + "/" + stateCode.(string))
			if readErr != nil {
				return readErr
			}
			if data == nil {
				time.Sleep(1000)
				data, readErr := mod.ReadData("super-secrets/PublicIndex/" + indexPath + "/" + idName + "/" + id + "/DataFlowStatistics/DataFlowGroup/" + flowG + "/dataFlowName/" + flowN + "/" + stateCode.(string))
				if readErr == nil && data == nil {
					return nil
				}
			}
			if testedDate, testedDateOk := data["lastTestedDate"].(string); testedDateOk {
				if testedDate == "" {
					flowData, flowReadErr := mod.ReadData("super-secrets/" + data["flowGroup"].(string))
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

// Used for flow
func StatisticToMap(mod *kv.Modifier, dfs *tccore.TTDINode, dfst *tccore.TTDINode, enrichLastTested bool) map[string]interface{} {
	var elapsedTime string
	statMap := make(map[string]interface{})
	var decodedstat interface{}
	err := json.Unmarshal([]byte(dfst.MashupDetailedElement.Data), &decodedstat)
	if err != nil {
		log.Println("Error in decoding data in StatisticToMap")
		return statMap
	}
	decodedStatData := decodedstat.(map[string]interface{})

	statMap["flowGroup"] = decodedStatData["FlowGroup"]
	statMap["flowName"] = decodedStatData["FlowName"]
	statMap["stateName"] = decodedStatData["StateName"]
	statMap["stateCode"] = decodedStatData["StateCode"]
	if _, ok := decodedStatData["TimeSplit"].(time.Duration); ok {
		if decodedStatData["TimeSplit"] != nil && decodedStatData["TimeSplit"].(time.Duration).Seconds() < 0 { //Covering corner case of 0 second time durations being slightly off (-.00004 seconds)
			elapsedTime = "0s"
		} else {
			elapsedTime = decodedStatData["TimeSplit"].(time.Duration).Truncate(time.Millisecond * 10).String()
		}
	} else if timeFloat, ok := decodedStatData["TimeSplit"].(float64); ok {
		elapsedTime = time.Duration(timeFloat * float64(time.Nanosecond)).Truncate(time.Millisecond * 10).String()
	}
	statMap["timeSplit"] = elapsedTime
	if modeFloat, ok := decodedStatData["Mode"].(float64); ok {
		statMap["mode"] = int(modeFloat)
	} else {
		statMap["mode"] = decodedStatData["Mode"]
	}

	statMap["lastTestedDate"] = ""

	if _, ok := decodedStatData["LastTestedDate"].(string); ok {
		if enrichLastTested && decodedStatData["LastTestedDate"].(string) == "" {
			var decoded interface{}
			err := json.Unmarshal([]byte(dfs.MashupDetailedElement.Data), &decoded)
			if err != nil {
				log.Println("Error in decoding data in StatisticToMap")
				return statMap
			}
			decodedData := decoded.(map[string]interface{})
			flowData, flowReadErr := mod.ReadData("super-secrets/" + decodedStatData["FlowGroup"].(string))
			if flowReadErr != nil && decodedData["LogFunc"] != nil {
				logFunc := decodedData["LogFunc"].(func(string, error))
				logFunc("Error reading flow properties from vault", flowReadErr)
				//dfs.LogFunc("Error reading flow properties from vault", flowReadErr)
			}

			if _, ok := flowData["lastTestedDate"].(string); ok {
				statMap["lastTestedDate"] = flowData["lastTestedDate"].(string)
			} else {
				statMap["lastTestedDate"] = ""
			}
		} else {
			statMap["lastTestedDate"] = decodedStatData["LastTestedDate"].(string)
		}
	} else {
		statMap["lastTestedDate"] = ""
	}

	return statMap
}

// package util

// import (
// 	"errors"
// 	"fmt"
// 	"log"
// 	"strconv"
// 	"strings"
// 	"github.com/trimble-oss/tierceron/pkg/vaulthelper/kv"
// 	"time"

// 	"github.com/trimble-oss/tierceron-nute/mashupsdk"
// )

// type DataFlowStatistic struct {
// 	mashupsdk.MashupDetailedElement
// 	FlowGroup string
// 	FlowName  string
// 	StateName string
// 	StateCode string
// 	TimeSplit time.Duration
// 	Mode      int
// }

// type DataFlow struct {
// 	mashupsdk.MashupDetailedElement
// 	Name       string
// 	TimeStart  time.Time
// 	Statistics []DataFlowStatistic
// 	LogStat    bool
// 	LogFunc    func(string, error)
// }

// type DataFlowGroup struct {
// 	mashupsdk.MashupDetailedElement
// 	Name  string
// 	Flows []DataFlow
// }

// type Argosy struct {
// 	mashupsdk.MashupDetailedElement
// 	ArgosyID string
// 	Groups   []DataFlowGroup
// }

// type ArgosyFleet struct {
// 	ArgosyName string
// 	Argosies   []Argosy
// }

// //New API -> Argosy, return dataFlowGroups populated
// func InitArgosyFleet(mod *kv.Modifier, project string, logger *log.Logger) (ArgosyFleet, error) {
// 	var aFleet ArgosyFleet
// 	aFleet.ArgosyName = project
// 	aFleet.Argosies = make([]Argosy, 0)
// 	idNameListData, serviceListErr := mod.List("super-secrets/PublicIndex/"+project, logger)
// 	if serviceListErr != nil || idNameListData == nil {
// 		return aFleet, serviceListErr
// 	}

// 	if serviceListErr != nil || idNameListData == nil {
// 		return aFleet, errors.New("No project was found for argosyFleet")
// 	}

// 	for _, idNameList := range idNameListData.Data {
// 		for _, idName := range idNameList.([]interface{}) {
// 			idListData, idListErr := mod.List("super-secrets/Index/"+project+"/tenantId", logger)
// 			if idListErr != nil || idListData == nil {
// 				return aFleet, idListErr
// 			}

// 			if idListData == nil {
// 				return aFleet, errors.New("No argosId were found for argosyFleet")
// 			}

// 			for _, idList := range idListData.Data {
// 				for _, id := range idList.([]interface{}) {
// 					serviceListData, serviceListErr := mod.List("super-secrets/PublicIndex/"+project+"/"+idName.(string)+"/"+id.(string)+"/DataFlowStatistics/DataFlowGroup", logger)
// 					if serviceListErr != nil {
// 						return aFleet, serviceListErr
// 					}
// 					var new Argosy
// 					new.ArgosyID = strings.TrimSuffix(id.(string), "/")
// 					new.Groups = make([]DataFlowGroup, 0)

// 					if serviceListData == nil { //No existing dfs for this tenant -> continue
// 						aFleet.Argosies = append(aFleet.Argosies, new)
// 						continue
// 					}

// 					for _, serviceList := range serviceListData.Data {
// 						for _, service := range serviceList.([]interface{}) {
// 							var dfgroup DataFlowGroup
// 							dfgroup.Name = strings.TrimSuffix(service.(string), "/")

// 							statisticNameList, statisticNameListErr := mod.List("super-secrets/PublicIndex/"+project+"/"+idName.(string)+"/"+id.(string)+"/DataFlowStatistics/DataFlowGroup/"+service.(string)+"/dataFlowName/", logger)
// 							if statisticNameListErr != nil {
// 								return aFleet, statisticNameListErr
// 							}

// 							if statisticNameList == nil {
// 								continue
// 							}

// 							for _, statisticName := range statisticNameList.Data {
// 								for _, statisticName := range statisticName.([]interface{}) {
// 									newDf := InitDataFlow(nil, strings.TrimSuffix(statisticName.(string), "/"), false)
// 									newDf.RetrieveStatistic(mod, id.(string), project, idName.(string), service.(string), statisticName.(string), logger)
// 									dfgroup.Flows = append(dfgroup.Flows, newDf)
// 								}
// 							}
// 							new.Groups = append(new.Groups, dfgroup)
// 						}
// 					}
// 					aFleet.Argosies = append(aFleet.Argosies, new)
// 				}
// 			}
// 		}
// 	}

// 	//var newDFStatistic = DataFlowGroup{Name: name, TimeStart: time.Now(), Statistics: nil, LogStat: false, LogFunc: nil}
// 	return aFleet, nil
// }

// func InitDataFlow(logF func(string, error), name string, logS bool) DataFlow {
// 	var stats []DataFlowStatistic
// 	var newDFStatistic = DataFlow{Name: name, TimeStart: time.Now(), Statistics: stats, LogStat: logS, LogFunc: logF}
// 	return newDFStatistic
// }

// func (dfs *DataFlow) UpdateDataFlowStatistic(flowG string, flowN string, stateN string, stateC string, mode int) {
// 	var newDFStat = DataFlowStatistic{mashupsdk.MashupDetailedElement{}, flowG, flowN, stateN, stateC, time.Since(dfs.TimeStart), mode}
// 	dfs.Statistics = append(dfs.Statistics, newDFStat)
// 	dfs.Log()
// }

// func (dfs *DataFlow) UpdateDataFlowStatisticWithTime(flowG string, flowN string, stateN string, stateC string, mode int, elapsedTime time.Duration) {
// 	var newDFStat = DataFlowStatistic{mashupsdk.MashupDetailedElement{}, flowG, flowN, stateN, stateC, elapsedTime, mode}
// 	dfs.Statistics = append(dfs.Statistics, newDFStat)
// 	dfs.Log()
// }

// func (dfs *DataFlow) Log() {
// 	if dfs.LogStat {
// 		stat := dfs.Statistics[len(dfs.Statistics)-1]
// 		if strings.Contains(stat.StateName, "Failure") {
// 			dfs.LogFunc(stat.FlowName+"-"+stat.StateName, errors.New(stat.StateName))
// 		} else {
// 			dfs.LogFunc(stat.FlowName+"-"+stat.StateName, nil)
// 		}
// 	}
// }

// func (dfs *DataFlow) FinishStatistic(mod *kv.Modifier, id string, indexPath string, idName string, logger *log.Logger) {
// 	//TODO : Write Statistic to vault
// 	if !dfs.LogStat && dfs.LogFunc != nil {
// 		dfs.FinishStatisticLog()
// 	}
// 	mod.SectionPath = ""
// 	for _, dataFlowStatistic := range dfs.Statistics {
// 		var elapsedTime string
// 		statMap := make(map[string]interface{})
// 		statMap["flowGroup"] = dataFlowStatistic.FlowGroup
// 		statMap["flowName"] = dataFlowStatistic.FlowName
// 		statMap["stateName"] = dataFlowStatistic.StateName
// 		statMap["stateCode"] = dataFlowStatistic.StateCode
// 		if dataFlowStatistic.TimeSplit.Seconds() < 0 { //Covering corner case of 0 second time durations being slightly off (-.00004 seconds)
// 			elapsedTime = "0s"
// 		} else {
// 			elapsedTime = dataFlowStatistic.TimeSplit.Truncate(time.Millisecond * 10).String()
// 		}
// 		statMap["timeSplit"] = elapsedTime
// 		statMap["mode"] = dataFlowStatistic.Mode

// 		mod.SectionPath = ""
// 		_, writeErr := mod.Write("super-secrets/PublicIndex/"+indexPath+"/"+idName+"/"+id+"/DataFlowStatistics/DataFlowGroup/"+dataFlowStatistic.FlowGroup+"/dataFlowName/"+dataFlowStatistic.FlowName+"/"+dataFlowStatistic.StateCode, statMap, logger)
// 		if writeErr != nil && dfs.LogFunc != nil {
// 			dfs.LogFunc("Error writing out DataFlowStatistics to vault", writeErr)
// 		}
// 	}
// }

// func (dfs *DataFlow) RetrieveStatistic(mod *kv.Modifier, id string, indexPath string, idName string, flowG string, flowN string, logger *log.Logger) error {
// 	listData, listErr := mod.List("super-secrets/PublicIndex/"+indexPath+"/"+idName+"/"+id+"/DataFlowStatistics/DataFlowGroup/"+flowG+"/dataFlowName/"+flowN, logger)
// 	if listErr != nil {
// 		return listErr
// 	}

// 	for _, stateCodeList := range listData.Data {
// 		for _, stateCode := range stateCodeList.([]interface{}) {
// 			data, readErr := mod.ReadData("super-secrets/PublicIndex/" + indexPath + "/" + idName + "/" + id + "/DataFlowStatistics/DataFlowGroup/" + flowG + "/dataFlowName/" + flowN + "/" + stateCode.(string))
// 			if readErr != nil {
// 				return readErr
// 			}
// 			if data == nil {
// 				time.Sleep(1)
// 				data, readErr := mod.ReadData("super-secrets/PublicIndex/" + indexPath + "/" + idName + "/" + id + "/DataFlowStatistics/DataFlowGroup/" + flowG + "/dataFlowName/" + flowN + "/" + stateCode.(string))
// 				if readErr == nil && data == nil {
// 					return nil
// 				}
// 			}
// 			var df DataFlowStatistic
// 			df.FlowGroup = data["flowGroup"].(string)
// 			df.FlowName = data["flowName"].(string)
// 			df.StateCode = data["stateCode"].(string)
// 			df.StateName = data["stateName"].(string)
// 			if mode, ok := data["mode"]; ok {
// 				modeStr := fmt.Sprintf("%s", mode) //Treats it as a interface due to weird typing from vault (encoding/json.Number)
// 				if modeInt, err := strconv.Atoi(modeStr); err == nil {
// 					df.Mode = modeInt
// 				}
// 			}
// 			if strings.Contains(data["timeSplit"].(string), "seconds") {
// 				data["timeSplit"] = strings.ReplaceAll(data["timeSplit"].(string), " seconds", "s")
// 			}
// 			df.TimeSplit, _ = time.ParseDuration(data["timeSplit"].(string))
// 			dfs.Statistics = append(dfs.Statistics, df)
// 		}
// 	}
// 	return nil
// }

// //Set logFunc and logStat = false to use this otherwise it logs as states change with logStat = true
// func (dfs *DataFlow) FinishStatisticLog() {
// 	if dfs.LogFunc == nil || dfs.LogStat {
// 		return
// 	}
// 	for _, stat := range dfs.Statistics {
// 		if strings.Contains(stat.StateName, "Failure") {
// 			dfs.LogFunc(stat.FlowName+"-"+stat.StateName, errors.New(stat.StateName))
// 			if stat.Mode == 2 { //Update snapshot Mode on failure so it doesn't repeat

// 			}
// 		} else {
// 			dfs.LogFunc(stat.FlowName+"-"+stat.StateName, nil)
// 		}
// 	}
// }

// //Used for flow
// func (dfs *DataFlow) StatisticToMap(mod *kv.Modifier, dfst DataFlowStatistic, enrichLastTested bool) map[string]interface{} {
// 	var elapsedTime string
// 	statMap := make(map[string]interface{})
// 	statMap["flowGroup"] = dfst.FlowGroup
// 	statMap["flowName"] = dfst.FlowName
// 	statMap["stateName"] = dfst.StateName
// 	statMap["stateCode"] = dfst.StateCode
// 	if dfst.TimeSplit.Seconds() < 0 { //Covering corner case of 0 second time durations being slightly off (-.00004 seconds)
// 		elapsedTime = "0s"
// 	} else {
// 		elapsedTime = dfst.TimeSplit.Truncate(time.Millisecond * 10).String()
// 	}
// 	statMap["timeSplit"] = elapsedTime
// 	statMap["mode"] = dfst.Mode
// 	statMap["lastTestedDate"] = ""

// 	if enrichLastTested {
// 		flowData, flowReadErr := mod.ReadData("super-secrets/" + dfst.FlowGroup)
// 		if flowReadErr != nil && dfs.LogFunc != nil {
// 			dfs.LogFunc("Error reading flow properties from vault", flowReadErr)
// 		}

// 		if _, ok := flowData["lastTestedDate"].(string); ok {
// 			statMap["lastTestedDate"] = flowData["lastTestedDate"].(string)
// 		}
// 	}

// 	return statMap
// }
