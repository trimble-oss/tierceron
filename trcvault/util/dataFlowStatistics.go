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
	flowGroup string
	flowName  string
	stateName string
	stateCode string
	timeSplit time.Duration
	mode      int
}

type DataFlow struct {
	Name       string
	TimeStart  time.Time
	Statistics []DataFlowStatistic
	LogStat    bool
	LogFunc    func(string, error)
}

type DataFlowGroup struct {
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

func InitArgosyFleet(mod *kv.Modifier, project string, idName string) (ArgosyFleet, error) {
	var aFleet ArgosyFleet
	aFleet.ArgosyName = project
	aFleet.Argosies = make([]Argosy, 0)
	idListData, idListErr := mod.List("super-secrets/PublicIndex/" + project + "/" + idName)
	if idListErr != nil {
		return aFleet, idListErr
	}
	for _, idList := range idListData.Data {
		for _, id := range idList.([]interface{}) {
			serviceListData, serviceListErr := mod.List("super-secrets/PublicIndex/" + project + "/" + idName + "/" + id.(string) + "/DataFlowStatistics/DataFlowGroup")
			if serviceListErr != nil {
				return aFleet, serviceListErr
			}
			var new Argosy
			new.ArgosyID = id.(string)
			new.Groups = make([]DataFlowGroup, 0)
			for _, serviceList := range serviceListData.Data {
				for _, service := range serviceList.([]interface{}) {
					var dfgroup DataFlowGroup
					dfgroup.Name = service.(string)

					statisticNameList, statisticNameListErr := mod.List("super-secrets/PublicIndex/" + project + "/" + idName + "/" + id.(string) + "/DataFlowStatistics/DataFlowGroup/" + service.(string) + "/dataFlowName/")
					if statisticNameListErr != nil {
						return aFleet, statisticNameListErr
					}

					for _, statisticName := range statisticNameList.Data {
						for _, statisticName := range statisticName.([]interface{}) {
							newDf := InitDataFlow(nil, statisticName.(string), false)
							newDf.RetrieveStatistic(mod, id.(string), project, idName, service.(string), statisticName.(string))
							dfgroup.Flows = append(dfgroup.Flows, newDf)
						}
					}
					new.Groups = append(new.Groups, dfgroup)
				}
			}
			aFleet.Argosies = append(aFleet.Argosies, new)
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
	var newDFStat = DataFlowStatistic{flowG, flowN, stateN, stateC, time.Since(dfs.TimeStart), mode}
	dfs.Statistics = append(dfs.Statistics, newDFStat)
	dfs.Log()
}

func (dfs *DataFlow) UpdateDataFlowStatisticWithTime(flowG string, flowN string, stateN string, stateC string, mode int, elapsedTime time.Duration) {
	var newDFStat = DataFlowStatistic{flowG, flowN, stateN, stateC, elapsedTime, mode}
	dfs.Statistics = append(dfs.Statistics, newDFStat)
	dfs.Log()
}

func (dfs *DataFlow) Log() {
	if dfs.LogStat {
		stat := dfs.Statistics[len(dfs.Statistics)-1]
		if strings.Contains(stat.stateName, "Failure") {
			dfs.LogFunc(stat.flowName+"-"+stat.stateName, errors.New(stat.stateName))
		} else {
			dfs.LogFunc(stat.flowName+"-"+stat.stateName, nil)
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
		statMap["flowGroup"] = dataFlowStatistic.flowGroup
		statMap["flowName"] = dataFlowStatistic.flowName
		statMap["stateName"] = dataFlowStatistic.stateName
		statMap["stateCode"] = dataFlowStatistic.stateCode
		if dataFlowStatistic.timeSplit.Seconds() < 0 { //Covering corner case of 0 second time durations being slightly off (-.00004 seconds)
			elapsedTime = "0s"
		} else {
			elapsedTime = dataFlowStatistic.timeSplit.Truncate(time.Millisecond * 10).String()
		}
		statMap["timeSplit"] = elapsedTime
		statMap["mode"] = dataFlowStatistic.mode

		mod.SectionPath = ""
		_, writeErr := mod.Write("super-secrets/PublicIndex/"+indexPath+"/"+idName+"/"+id+"/DataFlowStatistics/DataFlowGroup/"+dataFlowStatistic.flowGroup+"/dataFlowName/"+dataFlowStatistic.flowName+"/"+dataFlowStatistic.stateCode, statMap)
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
			var df DataFlowStatistic
			df.flowGroup = data["flowGroup"].(string)
			df.flowName = data["flowName"].(string)
			df.stateCode = data["stateCode"].(string)
			df.stateName = data["stateName"].(string)
			if mode, ok := data["mode"]; ok {
				modeStr := fmt.Sprintf("%s", mode) //Treats it as a interface due to weird typing from vault (encoding/json.Number)
				if modeInt, err := strconv.Atoi(modeStr); err == nil {
					df.mode = modeInt
				}
			}
			if strings.Contains(data["timeSplit"].(string), "seconds") {
				data["timeSplit"] = strings.ReplaceAll(data["timeSplit"].(string), " seconds", "s")
			}
			df.timeSplit, _ = time.ParseDuration(data["timeSplit"].(string))
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
		if strings.Contains(stat.stateName, "Failure") {
			dfs.LogFunc(stat.flowName+"-"+stat.stateName, errors.New(stat.stateName))
			if stat.mode == 2 { //Update snapshot mode on failure so it doesn't repeat

			}
		} else {
			dfs.LogFunc(stat.flowName+"-"+stat.stateName, nil)
		}
	}
}

//Used for flow
func (dfs *DataFlow) StatisticToMap(mod *kv.Modifier, dfst DataFlowStatistic, enrichLastTested bool) map[string]interface{} {
	var elapsedTime string
	statMap := make(map[string]interface{})
	statMap["flowGroup"] = dfst.flowGroup
	statMap["flowName"] = dfst.flowName
	statMap["stateName"] = dfst.stateName
	statMap["stateCode"] = dfst.stateCode
	if dfst.timeSplit.Seconds() < 0 { //Covering corner case of 0 second time durations being slightly off (-.00004 seconds)
		elapsedTime = "0s"
	} else {
		elapsedTime = dfst.timeSplit.Truncate(time.Millisecond * 10).String()
	}
	statMap["timeSplit"] = elapsedTime
	statMap["mode"] = dfst.mode
	statMap["lastTestedDate"] = ""

	if enrichLastTested {
		flowData, flowReadErr := mod.ReadData("super-secrets/" + dfst.flowGroup)
		if flowReadErr != nil && dfs.LogFunc != nil {
			dfs.LogFunc("Error reading flow properties from vault", flowReadErr)
		}

		if _, ok := flowData["lastTestedDate"].(string); ok {
			statMap["lastTestedDate"] = flowData["lastTestedDate"].(string)
		}
	}

	return statMap
}
