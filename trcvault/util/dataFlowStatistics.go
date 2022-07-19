package util

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"tierceron/vaulthelper/kv"
	"time"
)

type DataFlowStatistic struct {
	flowGroup string
	flowName  string
	stateName string
	stateCode string
	timeSplit time.Duration
	mode      int
}

type DataFlowGroup struct {
	Name       string
	TimeStart  time.Time
	Statistics []DataFlowStatistic
	LogStat    bool
	LogFunc    func(string, error)
}

type Argosy struct {
	//ID - argosID
	//DataFlowGroups
}

//New API -> Argosy, return dataFlowGroups populated

type ArgosyFleet struct {
	//[]Argosy
}

func InitDataFlowGroup(logF func(string, error), name string, logS bool) DataFlowGroup {
	var stats []DataFlowStatistic
	var newDFStatistic = DataFlowGroup{Name: name, TimeStart: time.Now(), Statistics: stats, LogStat: logS, LogFunc: logF}
	return newDFStatistic
}

func (dfs *DataFlowGroup) UpdateDataFlowStatistic(flowG string, flowN string, stateN string, stateC string, mode int) {
	var newDFStat = DataFlowStatistic{flowG, flowN, stateN, stateC, time.Since(dfs.TimeStart), mode}
	dfs.Statistics = append(dfs.Statistics, newDFStat)
	dfs.Log()
}

func (dfs *DataFlowGroup) UpdateDataFlowStatisticWithTime(flowG string, flowN string, stateN string, stateC string, mode int, elapsedTime time.Duration) {
	var newDFStat = DataFlowStatistic{flowG, flowN, stateN, stateC, elapsedTime, mode}
	dfs.Statistics = append(dfs.Statistics, newDFStat)
	dfs.Log()
}

func (dfs *DataFlowGroup) Log() {
	if dfs.LogStat {
		stat := dfs.Statistics[len(dfs.Statistics)-1]
		if strings.Contains(stat.stateName, "Failure") {
			dfs.LogFunc(stat.flowName+"-"+stat.stateName, errors.New(stat.stateName))
		} else {
			dfs.LogFunc(stat.flowName+"-"+stat.stateName, nil)
		}
	}
}

func (dfs *DataFlowGroup) FinishStatistic(mod *kv.Modifier, id string, indexPath string, idName string) {
	//TODO : Write Statistic to vault
	if !dfs.LogStat && dfs.LogFunc != nil {
		dfs.FinishStatisticLog()
	}
	mod.SectionPath = ""
	for _, dataFlowStatistic := range dfs.Statistics {
		var elapsedTime float64
		statMap := make(map[string]interface{})
		statMap["flowGroup"] = dataFlowStatistic.flowGroup
		statMap["flowName"] = dataFlowStatistic.flowName
		statMap["stateName"] = dataFlowStatistic.stateName
		statMap["stateCode"] = dataFlowStatistic.stateCode
		if dataFlowStatistic.timeSplit.Seconds() < 0 { //Covering corner case of 0 second time durations being slightly off (-.00004 seconds)
			elapsedTime = 0
		} else {
			elapsedTime = dataFlowStatistic.timeSplit.Seconds()
		}
		statMap["timeSplit"] = fmt.Sprintf("%f", elapsedTime) + " seconds"
		statMap["mode"] = dataFlowStatistic.mode

		mod.SectionPath = ""
		_, writeErr := mod.Write("super-secrets/PublicIndex/"+indexPath+"/"+idName+"/"+id+"/DataFlowStatistics/DataFlowGroup/"+dataFlowStatistic.flowGroup+"/dataFlowName/"+dataFlowStatistic.flowName+"/"+dataFlowStatistic.stateCode, statMap)
		if writeErr != nil && dfs.LogFunc != nil {
			dfs.LogFunc("Error writing out DataFlowStatistics to vault", writeErr)
		}
	}
}

func (dfs *DataFlowGroup) RetrieveStatistic(mod *kv.Modifier, id string, indexPath string, idName string, flowG string, flowN string) {
	listData, listErr := mod.List("super-secrets/PublicIndex/" + indexPath + "/" + idName + "/" + id + "/DataFlowStatistics/DataFlowGroup/" + flowG + "/dataFlowName/" + flowN)
	if listErr != nil && dfs.LogFunc != nil {
		dfs.LogFunc("Error reading DataFlowStatistics from vault", listErr)
	}

	for _, stateCodeList := range listData.Data {
		for _, stateCode := range stateCodeList.([]interface{}) {
			data, readErr := mod.ReadData("super-secrets/PublicIndex/" + indexPath + "/" + idName + "/" + id + "/DataFlowStatistics/DataFlowGroup/" + flowG + "/dataFlowName/" + flowN + "/" + stateCode.(string))
			if readErr != nil && dfs.LogFunc != nil {
				dfs.LogFunc("Error reading DataFlowStatistics from vault", readErr)
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
			timeElapsedSeconds, _ := strconv.ParseFloat(strings.Split(data["timeSplit"].(string), " seconds")[0], 64) //Convert time elapsed string to duration
			df.timeSplit = time.Duration(timeElapsedSeconds * float64(time.Second))
			dfs.Statistics = append(dfs.Statistics, df)
		}
	}
}

//Set logFunc and logStat = false to use this otherwise it logs as states change with logStat = true
func (dfs *DataFlowGroup) FinishStatisticLog() {
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

func (dfs *DataFlowGroup) StatisticToMap(dfst DataFlowStatistic) map[string]interface{} {
	var elapsedTime float64
	statMap := make(map[string]interface{})
	statMap["flowGroup"] = dfst.flowGroup
	statMap["flowName"] = dfst.flowName
	statMap["stateName"] = dfst.stateName
	statMap["stateCode"] = dfst.stateCode
	if dfst.timeSplit.Seconds() < 0 { //Covering corner case of 0 second time durations being slightly off (-.00004 seconds)
		elapsedTime = 0
	} else {
		elapsedTime = dfst.timeSplit.Seconds()
	}
	statMap["timeSplit"] = fmt.Sprintf("%f", elapsedTime) + " seconds"
	statMap["mode"] = dfst.mode

	return statMap
}
