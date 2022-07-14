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
}

type Argosy struct {
}

type ArgosyFleet struct {
}

func InitDataFlowGroup(name string) DataFlowGroup {
	var stats []DataFlowStatistic
	var newDFStatistic = DataFlowGroup{Name: name, TimeStart: time.Now(), Statistics: stats}
	return newDFStatistic
}

func (dfs *DataFlowGroup) UpdateDataFlowStatistic(flowG string, flowN string, stateN string, stateC string, mode int) {
	var newDFStat = DataFlowStatistic{flowG, flowN, stateN, stateC, time.Since(dfs.TimeStart), mode}
	dfs.Statistics = append(dfs.Statistics, newDFStat)
}

func (dfs *DataFlowGroup) UpdateDataFlowStatisticWithTime(flowG string, flowN string, stateN string, stateC string, mode int, elapsedTime time.Duration) {
	var newDFStat = DataFlowStatistic{flowG, flowN, stateN, stateC, elapsedTime, mode}
	dfs.Statistics = append(dfs.Statistics, newDFStat)
}

func (dfs *DataFlowGroup) FinishStatistic(logFunc func(string, error), mod *kv.Modifier, id string, indexPath string, idName string) {
	//TODO : Write Statistic to vault
	if logFunc != nil {
		dfs.FinishStatisticLog(logFunc)
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
		_, writeErr := mod.Write("super-secrets/PublicIndex/"+indexPath+"/"+idName+"/"+id+"/DataFlowGroup/"+dataFlowStatistic.flowGroup+"/dataFlowName/"+dataFlowStatistic.flowName+"/"+dataFlowStatistic.stateCode, statMap)
		if writeErr != nil && logFunc != nil {
			logFunc("Error writing out DataFlowStatistics to vault", writeErr)
		}
	}
}

func (dfs *DataFlowGroup) RetrieveStatistic(logFunc func(string, error), mod *kv.Modifier, id string, indexPath string, idName string, flowG string, flowN string) {
	listData, listErr := mod.List("super-secrets/PublicIndex/" + indexPath + "/" + idName + "/" + id + "/DataFlowGroup/" + flowG + "/dataFlowName/" + flowN)
	if listErr != nil && logFunc != nil {
		logFunc("Error reading DataFlowStatistics from vault", listErr)
	}

	for _, stateCodeList := range listData.Data {
		for _, stateCode := range stateCodeList.([]interface{}) {
			data, readErr := mod.ReadData("super-secrets/PublicIndex/" + indexPath + "/" + idName + "/" + id + "/DataFlowGroup/" + flowG + "/dataFlowName/" + flowN + "/" + stateCode.(string))
			if readErr != nil && logFunc != nil {
				logFunc("Error reading DataFlowStatistics from vault", readErr)
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

//Make success/failure placeholders later
func (dfs *DataFlowGroup) FinishStatisticLog(logFunc func(string, error)) {
	for _, stat := range dfs.Statistics {
		if strings.Contains(stat.stateName, "Failure") {
			logFunc(stat.flowName+"-"+stat.stateName, errors.New(stat.stateName))
			if stat.mode == 2 { //Update snapshot mode on failure so it doesn't repeat

			}
		} else {
			logFunc(stat.flowName+"-"+stat.stateName, nil)
		}
	}
}
