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
	Data       []byte
	ChildNodes []TTDINode
}

//New API -> Argosy, return dataFlowGroups populated
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

			for _, idList := range idListData.Data {
				for _, id := range idList.([]interface{}) {
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
	ttdiNode := TTDINode{mashupsdk.MashupDetailedElement{Name: name}, encodedData, stats}
	//var newDFStatistic = DataFlow{Name: name, TimeStart: time.Now(), Statistics: stats, LogStat: logS, LogFunc: logF}
	return ttdiNode
}

func (dfs *TTDINode) UpdateDataFlowStatistic(flowG string, flowN string, stateN string, stateC string, mode int) {
	var decoded interface{}
	err := json.Unmarshal(dfs.Data, &decoded)
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
	newNode := TTDINode{mashupsdk.MashupDetailedElement{}, newEncodedData, []TTDINode{}}
	//var newDFStat = DataFlowStatistic{mashupsdk.MashupDetailedElement{}, flowG, flowN, stateN, stateC, time.Since(dfs.TimeStart), mode}
	dfs.ChildNodes = append(dfs.ChildNodes, newNode)
	dfs.Log()
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
	newNode := TTDINode{mashupsdk.MashupDetailedElement{}, newEncodedData, []TTDINode{}}
	//var newDFStat = DataFlowStatistic{mashupsdk.MashupDetailedElement{}, flowG, flowN, stateN, stateC, elapsedTime, mode}
	dfs.ChildNodes = append(dfs.ChildNodes, newNode)
	dfs.Log()
}

func (dfs *TTDINode) Log() {
	var decoded interface{}
	err := json.Unmarshal(dfs.Data, &decoded)
	if err != nil {
		log.Println("Error in decoding data in UpdateDataFlowStatistic")
		return
	}
	decodedData := decoded.(map[string]interface{})
	if decodedData["LogStat"] != nil && decodedData["LogStat"].(bool) {
		stat := dfs.ChildNodes[len(dfs.ChildNodes)-1]
		var decodedstat interface{}
		err := json.Unmarshal(stat.Data, &decodedstat)
		if err != nil {
			log.Println("Error in decoding data in UpdateDataFlowStatistic")
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
	err := json.Unmarshal(dfs.Data, &decoded)
	if err != nil {
		log.Println("Error in decoding data in UpdateDataFlowStatistic")
		return
	}
	decodedData := decoded.(map[string]interface{})
	if decodedData["LogStat"] != nil && !decodedData["LogStat"].(bool) && decodedData["LogFunc"] != nil {
		dfs.FinishStatisticLog()
	}
	mod.SectionPath = ""
	for _, dataFlowStatistic := range dfs.ChildNodes {
		var decodedstat interface{}
		err := json.Unmarshal(dataFlowStatistic.Data, &decodedstat)
		if err != nil {
			log.Println("Error in decoding data in UpdateDataFlowStatistic")
			return
		}
		decodedStatData := decodedstat.(map[string]interface{})
		var elapsedTime string
		statMap := make(map[string]interface{})
		statMap["flowGroup"] = decodedStatData["FlowGroup"]
		statMap["flowName"] = decodedStatData["FlowName"]
		statMap["stateName"] = decodedStatData["StateName"]
		statMap["stateCode"] = decodedStatData["StateCode"]
		if decodedStatData["TimeSplit"] != nil && decodedStatData["TimeSplit"].(time.Duration).Seconds() < 0 { //Covering corner case of 0 second time durations being slightly off (-.00004 seconds)
			elapsedTime = "0s"
		} else {
			elapsedTime = decodedStatData["TimeSplit"].(time.Duration).Truncate(time.Millisecond * 10).String()
		}
		statMap["timeSplit"] = elapsedTime
		statMap["mode"] = decodedStatData["Mode"]

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
			newEncodedData, err := json.Marshal(newData)
			if err != nil {
				log.Println("Error encoding data in RetrieveStatistic")
				return nil
			}
			df.Data = newEncodedData
			dfs.ChildNodes = append(dfs.ChildNodes, df)
		}
	}
	return nil
}

//Set logFunc and logStat = false to use this otherwise it logs as states change with logStat = true
func (dfs *TTDINode) FinishStatisticLog() {
	var decoded interface{}
	err := json.Unmarshal(dfs.Data, &decoded)
	if err != nil {
		log.Println("Error in decoding data in UpdateDataFlowStatistic")
		return
	}
	decodedData := decoded.(map[string]interface{})
	if decodedData["LogFunc"] == nil || (decodedData["LogStat"] != nil && decodedData["LogStat"].(bool)) {
		return
	}
	for _, stat := range dfs.ChildNodes {
		var decodedstat interface{}
		err := json.Unmarshal(stat.Data, &decodedstat)
		if err != nil {
			log.Println("Error in decoding data in UpdateDataFlowStatistic")
			return
		}
		decodedStatData := decodedstat.(map[string]interface{})
		if decodedStatData["StateName"] != nil && strings.Contains(decodedStatData["StateName"].(string), "Failure") && decodedData["LogFunc"] != nil {
			logFunc := decodedData["LogFunc"].(func(string, error))
			logFunc(decodedStatData["FlowName"].(string)+"-"+decodedStatData["StateName"].(string), errors.New(decodedStatData["StateName"].(string)))
			//dfs.LogFunc(stat.FlowName+"-"+stat.StateName, errors.New(stat.StateName))
			if decodedStatData["Mode"] != nil && decodedStatData["Mode"].(int) == 2 { //Update snapshot Mode on failure so it doesn't repeat

			}
		} else {
			logFunc := decodedData["LogFunc"].(func(string, error))
			logFunc(decodedStatData["FlowName"].(string)+"-"+decodedStatData["StateName"].(string), nil)

			//dfs.LogFunc(stat.FlowName+"-"+stat.StateName, nil)
		}
	}
}

//Used for flow
func (dfs *TTDINode) StatisticToMap(mod *kv.Modifier, dfst TTDINode, enrichLastTested bool) map[string]interface{} {
	var elapsedTime string
	statMap := make(map[string]interface{})
	var decodedstat interface{}
	err := json.Unmarshal(dfst.Data, &decodedstat)
	if err != nil {
		log.Println("Error in decoding data in UpdateDataFlowStatistic")
		return statMap
	}
	decodedStatData := decodedstat.(map[string]interface{})

	statMap["flowGroup"] = decodedStatData["FlowGroup"]
	statMap["flowName"] = decodedStatData["FlowName"]
	statMap["stateName"] = decodedStatData["StateName"]
	statMap["stateCode"] = decodedStatData["StateCode"]
	if decodedStatData["TimeSplit"] != nil && decodedStatData["TimeSplit"].(time.Duration).Seconds() < 0 { //Covering corner case of 0 second time durations being slightly off (-.00004 seconds)
		elapsedTime = "0s"
	} else {
		elapsedTime = decodedStatData["TimeSplit"].(time.Duration).Truncate(time.Millisecond * 10).String()
	}
	statMap["timeSplit"] = elapsedTime
	statMap["mode"] = decodedStatData["Mode"]
	statMap["lastTestedDate"] = ""

	if enrichLastTested {
		var decoded interface{}
		err := json.Unmarshal(dfs.Data, &decoded)
		if err != nil {
			log.Println("Error in decoding data in UpdateDataFlowStatistic")
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
		}
	}

	return statMap
}
