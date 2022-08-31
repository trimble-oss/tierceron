package util

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"tierceron/vaulthelper/kv"
	"time"

	"github.com/mrjrieke/nute/mashupsdk"
)

type DataFlowStatistic struct {
	mashupsdk.MashupDetailedElement
	FlowGroup string
	FlowName  string
	StateName string
	StateCode string
	TimeSplit time.Duration
	Mode      int
}

type DataFlow struct {
	mashupsdk.MashupDetailedElement
	Name       string
	TimeStart  time.Time
	Statistics []DataFlowStatistic
	LogStat    bool
	LogFunc    func(string, error)
}

type DataFlowGroup struct {
	mashupsdk.MashupDetailedElement
	Name  string
	Flows []DataFlow
}

type Argosy struct {
	mashupsdk.MashupDetailedElement
	ArgosyID string
	Groups   []DataFlowGroup
}

type ArgosyFleet struct {
	ArgosyName string
	Argosies   []Argosy
}

//New API -> Argosy, return dataFlowGroups populated

func InitArgosyFleet(mod *kv.Modifier, project string) (ArgosyFleet, error) {
	var aFleet ArgosyFleet
	aFleet.ArgosyName = project
	aFleet.Argosies = make([]Argosy, 0)
	idNameListData, serviceListErr := mod.List("super-secrets/PublicIndex/" + project)
	if serviceListErr != nil || idNameListData == nil {
		return aFleet, serviceListErr
	}

	if serviceListErr != nil || idNameListData == nil {
		return aFleet, errors.New("No project was found for argosyFleet")
	}

	for _, idNameList := range idNameListData.Data {
		for _, idName := range idNameList.([]interface{}) {
			idListData, idListErr := mod.List("super-secrets/Index/" + project + "/tenantId")
			if idListErr != nil || idListData == nil {
				return aFleet, idListErr
			}

			if idListData == nil {
				return aFleet, errors.New("No argosId were found for argosyFleet")
			}

			for _, idList := range idListData.Data {
				for _, id := range idList.([]interface{}) {
					serviceListData, serviceListErr := mod.List("super-secrets/PublicIndex/" + project + "/" + idName.(string) + "/" + id.(string) + "/DataFlowStatistics/DataFlowGroup")
					if serviceListErr != nil {
						return aFleet, serviceListErr
					}
					var new Argosy
					new.ArgosyID = id.(string)
					new.Groups = make([]DataFlowGroup, 0)

					if serviceListData == nil { //No existing dfs for this tenant -> continue
						aFleet.Argosies = append(aFleet.Argosies, new)
						continue
					}

					for _, serviceList := range serviceListData.Data {
						for _, service := range serviceList.([]interface{}) {
							var dfgroup DataFlowGroup
							dfgroup.Name = service.(string)

							statisticNameList, statisticNameListErr := mod.List("super-secrets/PublicIndex/" + project + "/" + idName.(string) + "/" + id.(string) + "/DataFlowStatistics/DataFlowGroup/" + service.(string) + "/dataFlowName/")
							if statisticNameListErr != nil {
								return aFleet, statisticNameListErr
							}

							if statisticNameList == nil {
								continue
							}

							for _, statisticName := range statisticNameList.Data {
								for _, statisticName := range statisticName.([]interface{}) {
									newDf := InitDataFlow(nil, statisticName.(string), false)
									newDf.RetrieveStatistic(mod, id.(string), project, idName.(string), service.(string), statisticName.(string))
									dfgroup.Flows = append(dfgroup.Flows, newDf)
								}
							}
							new.Groups = append(new.Groups, dfgroup)
						}
					}
					aFleet.Argosies = append(aFleet.Argosies, new)
				}
			}
		}
	}

	//var newDFStatistic = DataFlowGroup{Name: name, TimeStart: time.Now(), Statistics: nil, LogStat: false, LogFunc: nil}
	return aFleet, nil
}

func InitDataFlow(logF func(string, error), name string, logS bool) DataFlow {
	var stats []DataFlowStatistic
	var newDFStatistic = DataFlow{Name: name, TimeStart: time.Now(), Statistics: stats, LogStat: logS, LogFunc: logF}
	return newDFStatistic
}

func (dfs *DataFlow) UpdateDataFlowStatistic(flowG string, flowN string, stateN string, stateC string, mode int) {
	var newDFStat = DataFlowStatistic{mashupsdk.MashupDetailedElement{}, flowG, flowN, stateN, stateC, time.Since(dfs.TimeStart), mode}
	dfs.Statistics = append(dfs.Statistics, newDFStat)
	dfs.Log()
}

func (dfs *DataFlow) UpdateDataFlowStatisticWithTime(flowG string, flowN string, stateN string, stateC string, mode int, elapsedTime time.Duration) {
	var newDFStat = DataFlowStatistic{mashupsdk.MashupDetailedElement{}, flowG, flowN, stateN, stateC, elapsedTime, mode}
	dfs.Statistics = append(dfs.Statistics, newDFStat)
	dfs.Log()
}

func (dfs *DataFlow) Log() {
	if dfs.LogStat {
		stat := dfs.Statistics[len(dfs.Statistics)-1]
		if strings.Contains(stat.StateName, "Failure") {
			dfs.LogFunc(stat.FlowName+"-"+stat.StateName, errors.New(stat.StateName))
		} else {
			dfs.LogFunc(stat.FlowName+"-"+stat.StateName, nil)
		}
	}
}

func (dfs *DataFlow) FinishStatistic(mod *kv.Modifier, id string, indexPath string, idName string) {
	//TODO : Write Statistic to vault
	if !dfs.LogStat && dfs.LogFunc != nil {
		dfs.FinishStatisticLog()
	}
	mod.SectionPath = ""
	for _, dataFlowStatistic := range dfs.Statistics {
		var elapsedTime string
		statMap := make(map[string]interface{})
		statMap["flowGroup"] = dataFlowStatistic.FlowGroup
		statMap["flowName"] = dataFlowStatistic.FlowName
		statMap["stateName"] = dataFlowStatistic.StateName
		statMap["stateCode"] = dataFlowStatistic.StateCode
		if dataFlowStatistic.TimeSplit.Seconds() < 0 { //Covering corner case of 0 second time durations being slightly off (-.00004 seconds)
			elapsedTime = "0s"
		} else {
			elapsedTime = dataFlowStatistic.TimeSplit.Truncate(time.Millisecond * 10).String()
		}
		statMap["timeSplit"] = elapsedTime
		statMap["mode"] = dataFlowStatistic.Mode

		mod.SectionPath = ""
		_, writeErr := mod.Write("super-secrets/PublicIndex/"+indexPath+"/"+idName+"/"+id+"/DataFlowStatistics/DataFlowGroup/"+dataFlowStatistic.FlowGroup+"/dataFlowName/"+dataFlowStatistic.FlowName+"/"+dataFlowStatistic.StateCode, statMap)
		if writeErr != nil && dfs.LogFunc != nil {
			dfs.LogFunc("Error writing out DataFlowStatistics to vault", writeErr)
		}
	}
}

func (dfs *DataFlow) RetrieveStatistic(mod *kv.Modifier, id string, indexPath string, idName string, flowG string, flowN string) error {
	listData, listErr := mod.List("super-secrets/PublicIndex/" + indexPath + "/" + idName + "/" + id + "/DataFlowStatistics/DataFlowGroup/" + flowG + "/dataFlowName/" + flowN)
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
			var df DataFlowStatistic
			df.FlowGroup = data["flowGroup"].(string)
			df.FlowName = data["flowName"].(string)
			df.StateCode = data["stateCode"].(string)
			df.StateName = data["stateName"].(string)
			if mode, ok := data["mode"]; ok {
				modeStr := fmt.Sprintf("%s", mode) //Treats it as a interface due to weird typing from vault (encoding/json.Number)
				if modeInt, err := strconv.Atoi(modeStr); err == nil {
					df.Mode = modeInt
				}
			}
			if strings.Contains(data["timeSplit"].(string), "seconds") {
				data["timeSplit"] = strings.ReplaceAll(data["timeSplit"].(string), " seconds", "s")
			}
			df.TimeSplit, _ = time.ParseDuration(data["timeSplit"].(string))
			dfs.Statistics = append(dfs.Statistics, df)
		}
	}
	return nil
}

//Set logFunc and logStat = false to use this otherwise it logs as states change with logStat = true
func (dfs *DataFlow) FinishStatisticLog() {
	if dfs.LogFunc == nil || dfs.LogStat {
		return
	}
	for _, stat := range dfs.Statistics {
		if strings.Contains(stat.StateName, "Failure") {
			dfs.LogFunc(stat.FlowName+"-"+stat.StateName, errors.New(stat.StateName))
			if stat.Mode == 2 { //Update snapshot Mode on failure so it doesn't repeat

			}
		} else {
			dfs.LogFunc(stat.FlowName+"-"+stat.StateName, nil)
		}
	}
}

//Used for flow
func (dfs *DataFlow) StatisticToMap(mod *kv.Modifier, dfst DataFlowStatistic, enrichLastTested bool) map[string]interface{} {
	var elapsedTime string
	statMap := make(map[string]interface{})
	statMap["flowGroup"] = dfst.FlowGroup
	statMap["flowName"] = dfst.FlowName
	statMap["stateName"] = dfst.StateName
	statMap["stateCode"] = dfst.StateCode
	if dfst.TimeSplit.Seconds() < 0 { //Covering corner case of 0 second time durations being slightly off (-.00004 seconds)
		elapsedTime = "0s"
	} else {
		elapsedTime = dfst.TimeSplit.Truncate(time.Millisecond * 10).String()
	}
	statMap["timeSplit"] = elapsedTime
	statMap["mode"] = dfst.Mode
	statMap["lastTestedDate"] = ""

	if enrichLastTested {
		flowData, flowReadErr := mod.ReadData("super-secrets/" + dfst.FlowGroup)
		if flowReadErr != nil && dfs.LogFunc != nil {
			dfs.LogFunc("Error reading flow properties from vault", flowReadErr)
		}

		if _, ok := flowData["lastTestedDate"].(string); ok {
			statMap["lastTestedDate"] = flowData["lastTestedDate"].(string)
		}
	}

	return statMap
}
