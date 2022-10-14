package util

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"strconv"
	"strings"
	"tierceron/vaulthelper/kv"
	"time"

	"github.com/mrjrieke/nute/mashupsdk"
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

type TTDINode struct {
	mashupsdk.MashupDetailedElement
	//Data       []byte
	ChildNodes []TTDINode
}

// New API -> Argosy, return dataFlowGroups populated
func InitArgosyFleet(mod *kv.Modifier, project string, logger *log.Logger) (TTDINode, error) {
	var aFleet TTDINode
	aFleet.MashupDetailedElement.Name = project
	aFleet.ChildNodes = make([]TTDINode, 0)
	idNameListData, serviceListErr := mod.List("super-secrets/PublicIndex/"+project, logger)
	if serviceListErr != nil || idNameListData == nil {
		return aFleet, serviceListErr
	}

	if serviceListErr != nil || idNameListData == nil {
		return aFleet, errors.New("No project was found for argosyFleet")
	}

	for _, idNameList := range idNameListData.Data {
		for _, idName := range idNameList.([]interface{}) {
			idListData, idListErr := mod.List("super-secrets/Index/"+project+"/tenantId", logger)
			if idListErr != nil || idListData == nil {
				return aFleet, idListErr
			}

			if idListData == nil {
				return aFleet, errors.New("No argosId were found for argosyFleet")
			}
			idName = strings.Trim(idName.(string), "/")

			for _, idList := range idListData.Data {
				for _, id := range idList.([]interface{}) {
					id = strings.Trim(id.(string), "/")
					serviceListData, serviceListErr := mod.List("super-secrets/PublicIndex/"+project+"/"+idName.(string)+"/"+id.(string)+"/DataFlowStatistics/DataFlowGroup", logger)
					if serviceListErr != nil {
						return aFleet, serviceListErr
					}
					var new TTDINode
					new.MashupDetailedElement.Name = strings.TrimSuffix(id.(string), "/")
					new.ChildNodes = make([]TTDINode, 0)

					if serviceListData == nil { //No existing dfs for this tenant -> continue
						aFleet.ChildNodes = append(aFleet.ChildNodes, new)
						continue
					}

					for _, serviceList := range serviceListData.Data {
						for _, service := range serviceList.([]interface{}) {
							var dfgroup TTDINode
							dfgroup.MashupDetailedElement.Name = strings.TrimSuffix(service.(string), "/")

							statisticNameList, statisticNameListErr := mod.List("super-secrets/PublicIndex/"+project+"/"+idName.(string)+"/"+id.(string)+"/DataFlowStatistics/DataFlowGroup/"+service.(string)+"/dataFlowName/", logger)
							if statisticNameListErr != nil {
								return aFleet, statisticNameListErr
							}

							if statisticNameList == nil {
								continue
							}

							for _, statisticName := range statisticNameList.Data {
								for _, statisticName := range statisticName.([]interface{}) {
									newDf := InitDataFlow(nil, strings.TrimSuffix(statisticName.(string), "/"), false)
									newDf.RetrieveStatistic(mod, id.(string), project, idName.(string), service.(string), statisticName.(string), logger)
									dfgroup.ChildNodes = append(dfgroup.ChildNodes, newDf)
								}
							}
							new.ChildNodes = append(new.ChildNodes, dfgroup)
						}
					}
					aFleet.ChildNodes = append(aFleet.ChildNodes, new)
				}
			}
		}
	}

	//var newDFStatistic = DataFlowGroup{Name: name, TimeStart: time.Now(), Statistics: nil, LogStat: false, LogFunc: nil}
	return aFleet, nil
}

func InitDataFlow(logF func(string, error), name string, logS bool) TTDINode {
	var stats []TTDINode
	data := make(map[string]interface{})
	data["TimeStart"] = time.Now()
	data["LogStat"] = logS
	if logF != nil {
		data["LogFunc"] = logF
	}
	encodedData, err := json.Marshal(&data)
	if err != nil {
		log.Println("Error in encoding data in InitDataFlow")
		return TTDINode{}
	}
	ttdiNode := TTDINode{mashupsdk.MashupDetailedElement{Name: name, Data: string(encodedData)}, stats}
	//var newDFStatistic = DataFlow{Name: name, TimeStart: time.Now(), Statistics: stats, LogStat: logS, LogFunc: logF}
	return ttdiNode
}

func (dfs *TTDINode) UpdateDataFlowStatistic(flowG string, flowN string, stateN string, stateC string, mode int) {
	var decoded interface{}
	err := json.Unmarshal([]byte(dfs.MashupDetailedElement.Data), &decoded)
	if err != nil {
		log.Println("Error in decoding data in UpdateDataFlowStatistic")
		return
	}
	decodedData := decoded.(map[string]interface{})
	timeStart := decodedData["TimeStart"].(time.Time)
	newData := make(map[string]interface{})
	newData["FlowGroup"] = flowG
	newData["FlowName"] = flowN
	newData["StateName"] = stateN
	newData["StateCode"] = stateC
	newData["Mode"] = mode
	newData["TimeSplit"] = time.Since(timeStart)
	newEncodedData, err := json.Marshal(newData)
	if err != nil {
		log.Println("Error in encoding data in UpdateDataFlowStatistics")
		return
	}
	newNode := TTDINode{mashupsdk.MashupDetailedElement{Data: string(newEncodedData)}, []TTDINode{}}
	//var newDFStat = DataFlowStatistic{mashupsdk.MashupDetailedElement{}, flowG, flowN, stateN, stateC, time.Since(dfs.TimeStart), mode}
	dfs.ChildNodes = append(dfs.ChildNodes, newNode)
	newData["decodedData"] = decodedData
	dfs.EfficientLog(newData)
}

func (dfs *TTDINode) UpdateDataFlowStatisticWithTime(flowG string, flowN string, stateN string, stateC string, mode int, elapsedTime time.Duration) {
	newData := make(map[string]interface{})
	newData["FlowGroup"] = flowG
	newData["FlowName"] = flowN
	newData["StateName"] = stateN
	newData["StateCode"] = stateC
	newData["Mode"] = mode
	newData["TimeSplit"] = elapsedTime
	newEncodedData, err := json.Marshal(newData)
	if err != nil {
		log.Println("Error in encoding data in UpdateDataFlowStatisticWithTime")
		return
	}
	newNode := TTDINode{mashupsdk.MashupDetailedElement{Data: string(newEncodedData)}, []TTDINode{}}
	//var newDFStat = DataFlowStatistic{mashupsdk.MashupDetailedElement{}, flowG, flowN, stateN, stateC, elapsedTime, mode}
	dfs.ChildNodes = append(dfs.ChildNodes, newNode)
	dfs.EfficientLog(newData)
}

// Doesn't deserialize statistic data for updatedataflowstatistic
func (dfs *TTDINode) EfficientLog(statMap map[string]interface{}) {
	var decodedData map[string]interface{}
	if statMap["decodedData"] == nil {
		var decoded interface{}
		err := json.Unmarshal([]byte(dfs.MashupDetailedElement.Data), &decoded)
		if err != nil {
			log.Println("Error in decoding data in Log")
			return
		}
		decodedData = decoded.(map[string]interface{})
	} else {
		decodedData = statMap["decodedData"].(map[string]interface{})
	}

	if decodedData["LogStat"] != nil && decodedData["LogStat"].(bool) {
		if statMap["StateName"] != nil && strings.Contains(statMap["StateName"].(string), "Failure") && decodedData["LogFunc"] != nil {
			logFunc := decodedData["LogFunc"].(func(string, error))
			logFunc(statMap["FlowName"].(string)+"-"+statMap["StateName"].(string), errors.New(statMap["StateName"].(string)))
			//dfs.LogFunc(stat.FlowName+"-"+stat.StateName, errors.New(stat.StateName))
		} else if decodedData["LogFunc"] != nil {
			logFunc := decodedData["LogFunc"].(func(string, error))
			logFunc(statMap["FlowName"].(string)+"-"+statMap["StateName"].(string), nil)
			//dfs.LogFunc(stat.FlowName+"-"+stat.StateName, nil)
		}
	}
}

// decodedData := decoded.(map[string]interface{})
// if decodedData["LogStat"] != nil && decodedData["LogStat"].(bool) {
// 	stat := dfs.ChildNodes[len(dfs.ChildNodes)-1]
// 	var decodedstat interface{}
// 	err := json.Unmarshal([]byte(stat.MashupDetailedElement.Data), &decodedstat)
// 	if err != nil {
// 		log.Println("Error in decoding data in Log")
// 		return
// 	}
// 	decodedStatData := decodedstat.(map[string]interface{})
// 	if decodedStatData["StateName"] != nil && strings.Contains(decodedStatData["StateName"].(string), "Failure") && decodedData["LogFunc"] != nil {
// 		logFunc := decodedData["LogFunc"].(func(string, error))
// 		logFunc(decodedStatData["FlowName"].(string)+"-"+decodedStatData["StateName"].(string), errors.New(decodedStatData["StateName"].(string)))
// 		//dfs.LogFunc(stat.FlowName+"-"+stat.StateName, errors.New(stat.StateName))
// 	} else if decodedData["LogFunc"] != nil {
// 		logFunc := decodedData["LogFunc"].(func(string, error))
// 		logFunc(decodedStatData["FlowName"].(string)+"-"+decodedStatData["StateName"].(string), nil)

func (dfs *TTDINode) Log() {
	var decoded interface{}
	err := json.Unmarshal([]byte(dfs.MashupDetailedElement.Data), &decoded)
	if err != nil {
		log.Println("Error in decoding data in Log")
		return
	}
	decodedData := decoded.(map[string]interface{})
	if decodedData["LogStat"] != nil && decodedData["LogStat"].(bool) {
		stat := dfs.ChildNodes[len(dfs.ChildNodes)-1]
		var decodedstat interface{}
		err := json.Unmarshal([]byte(stat.MashupDetailedElement.Data), &decodedstat)
		if err != nil {
			log.Println("Error in decoding data in Log")
			return
		}
		decodedStatData := decodedstat.(map[string]interface{})
		if decodedStatData["StateName"] != nil && strings.Contains(decodedStatData["StateName"].(string), "Failure") && decodedData["LogFunc"] != nil {
			logFunc := decodedData["LogFunc"].(func(string, error))
			logFunc(decodedStatData["FlowName"].(string)+"-"+decodedStatData["StateName"].(string), errors.New(decodedStatData["StateName"].(string)))
			//dfs.LogFunc(stat.FlowName+"-"+stat.StateName, errors.New(stat.StateName))
		} else if decodedData["LogFunc"] != nil {
			logFunc := decodedData["LogFunc"].(func(string, error))
			logFunc(decodedStatData["FlowName"].(string)+"-"+decodedStatData["StateName"].(string), nil)
			//dfs.LogFunc(stat.FlowName+"-"+stat.StateName, nil)
		}
	}
}

func (dfs *TTDINode) FinishStatistic(mod *kv.Modifier, id string, indexPath string, idName string, logger *log.Logger) {
	//TODO : Write Statistic to vault
	var decoded interface{}
	err := json.Unmarshal([]byte(dfs.MashupDetailedElement.Data), &decoded)
	if err != nil {
		log.Println("Error in decoding data in FinishStatistic")
		return
	}
	decodedData := decoded.(map[string]interface{})
	if decodedData["LogStat"] != nil && !decodedData["LogStat"].(bool) && decodedData["LogFunc"] != nil {
		dfs.FinishStatisticLog()
	}
	mod.SectionPath = ""
	for _, dataFlowStatistic := range dfs.ChildNodes {
		var decodedstat interface{}
		err := json.Unmarshal([]byte(dataFlowStatistic.MashupDetailedElement.Data), &decodedstat)
		if err != nil {
			log.Println("Error in decoding data in FinishStatistic")
			return
		}
		decodedStatData := decodedstat.(map[string]interface{})
		var elapsedTime string
		statMap := make(map[string]interface{})
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
		lastTestedDate := ""
		if _, ok := decodedData["TimeStart"].(time.Time); ok {
			lastTestedDate = decodedData["TimeStart"].(time.Time).Format(time.RFC3339)
		} else if _, ok := decodedStatData["TimeStart"].(string); ok {
			lastTestedDate = decodedStatData["TimeStart"].(string)
		}

		statMap["lastTestedDate"] = lastTestedDate

		mod.SectionPath = ""
		_, writeErr := mod.Write("super-secrets/PublicIndex/"+indexPath+"/"+idName+"/"+id+"/DataFlowStatistics/DataFlowGroup/"+decodedStatData["FlowGroup"].(string)+"/dataFlowName/"+decodedStatData["FlowName"].(string)+"/"+decodedStatData["StateCode"].(string), statMap, logger)
		if writeErr != nil && decodedData["LogFunc"] != nil {
			logFunc := decodedData["LogFunc"].(func(string, error))
			logFunc("Error writing out DataFlowStatistics to vault", writeErr)

			//dfs.LogFunc("Error writing out DataFlowStatistics to vault", writeErr)
		}
	}
}

func (dfs *TTDINode) RetrieveStatistic(mod *kv.Modifier, id string, indexPath string, idName string, flowG string, flowN string, logger *log.Logger) error {
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
				time.Sleep(1)
				data, readErr := mod.ReadData("super-secrets/PublicIndex/" + indexPath + "/" + idName + "/" + id + "/DataFlowStatistics/DataFlowGroup/" + flowG + "/dataFlowName/" + flowN + "/" + stateCode.(string))
				if readErr == nil && data == nil {
					return nil
				}
			}
			var df TTDINode
			newData := make(map[string]interface{})
			newData["FlowGroup"] = data["flowGroup"].(string)
			newData["FlowName"] = data["flowName"].(string)
			newData["StateName"] = data["stateName"].(string)
			newData["StateCode"] = data["stateCode"].(string)
			if mode, ok := data["mode"]; ok {
				modeStr := fmt.Sprintf("%s", mode) //Treats it as a interface due to weird typing from vault (encoding/json.Number)
				if modeInt, err := strconv.Atoi(modeStr); err == nil {
					//df.Mode = modeInt
					newData["Mode"] = modeInt
				}
			}
			if strings.Contains(data["timeSplit"].(string), "seconds") {
				data["timeSplit"] = strings.ReplaceAll(data["timeSplit"].(string), " seconds", "s")
			}
			newData["TimeSplit"], _ = time.ParseDuration(data["timeSplit"].(string))

			if testedDate, testedDateOk := data["lastTestedDate"].(string); testedDateOk {
				if testedDate == "" {
					flowData, flowReadErr := mod.ReadData("super-secrets/" + data["flowGroup"].(string))
					// if flowReadErr != nil {
					// 	return flowReadErr
					// } ***

					if _, ok := flowData["lastTestedDate"].(string); ok && flowReadErr != nil {
						newData["LastTestedDate"] = flowData["lastTestedDate"].(string)
					} else {
						newData["LastTestedDate"] = ""
					}
				} else {
					newData["LastTestedDate"] = testedDate
				}
			}
			newEncodedData, err := json.Marshal(newData)
			if err != nil {
				log.Println("Error encoding data in RetrieveStatistic")
				return nil
			}
			df.MashupDetailedElement.Data = string(newEncodedData)
			dfs.ChildNodes = append(dfs.ChildNodes, df)
		}
	}
	return nil
}

// Set logFunc and logStat = false to use this otherwise it logs as states change with logStat = true
func (dfs *TTDINode) FinishStatisticLog() {
	var decoded interface{}
	err := json.Unmarshal([]byte(dfs.MashupDetailedElement.Data), &decoded)
	if err != nil {
		log.Println("Error in decoding data in FinishStatisticLog")
		return
	}
	decodedData := decoded.(map[string]interface{})
	if decodedData["LogFunc"] == nil || (decodedData["LogStat"] != nil && decodedData["LogStat"].(bool)) {
		return
	}
	for _, stat := range dfs.ChildNodes {
		var decodedstat interface{}
		err := json.Unmarshal([]byte(stat.MashupDetailedElement.Data), &decodedstat)
		if err != nil {
			log.Println("Error in decoding data in FinishStatisticLog")
			return
		}
		decodedStatData := decodedstat.(map[string]interface{})
		if decodedStatData["StateName"] != nil && strings.Contains(decodedStatData["StateName"].(string), "Failure") && decodedData["LogFunc"] != nil {
			logFunc := decodedData["LogFunc"].(func(string, error))
			logFunc(decodedStatData["FlowName"].(string)+"-"+decodedStatData["StateName"].(string), errors.New(decodedStatData["StateName"].(string)))
			//dfs.LogFunc(stat.FlowName+"-"+stat.StateName, errors.New(stat.StateName))
			if decodedStatData["Mode"] != nil {
				if modeFloat, ok := decodedStatData["Mode"].(float64); ok {
					if modeFloat == 2 { //Update snapshot Mode on failure so it doesn't repeat

					}
				} else {
					if decodedStatData["Mode"] == 2 { //Update snapshot Mode on failure so it doesn't repeat

					}
				}
			}
		} else {
			logFunc := decodedData["LogFunc"].(func(string, error))
			logFunc(decodedStatData["FlowName"].(string)+"-"+decodedStatData["StateName"].(string), nil)

			//dfs.LogFunc(stat.FlowName+"-"+stat.StateName, nil)
		}
	}
}

// Used for flow
func (dfs *TTDINode) StatisticToMap(mod *kv.Modifier, dfst TTDINode, enrichLastTested bool) map[string]interface{} {
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
// 	"tierceron/vaulthelper/kv"
// 	"time"

// 	"github.com/mrjrieke/nute/mashupsdk"
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
