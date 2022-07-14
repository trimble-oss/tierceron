package util

import (
	"errors"
	"fmt"
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

type DataFlowGroup struct {
	Name       string
	TimeStart  time.Time
	Statistics []DataFlowStatistic
}

type Argosy struct {
	mashupsdk.MashupDetailedElement
	Groups []DataFlowGroup
}

type ArgosyFleet struct {
	Name  string
	Fleet []Argosy
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

func (dfs *DataFlowGroup) FinishStatistic(logFunc func(string, error), mod *kv.Modifier, id string, indexPath string, idName string) {
	//TODO : Write Statistic to vault
	dfs.FinishStatisticLog(logFunc)
	mod.SectionPath = ""
	for _, dataFlowStatistic := range dfs.Statistics {
		statMap := make(map[string]interface{})
		statMap["flowGroup"] = dataFlowStatistic.flowGroup
		statMap["flowName"] = dataFlowStatistic.flowName
		statMap["stateName"] = dataFlowStatistic.stateName
		statMap["stateCode"] = dataFlowStatistic.stateCode
		statMap["timeSplit"] = fmt.Sprintf("%f", dataFlowStatistic.timeSplit.Seconds()) + " seconds"
		statMap["mode"] = dataFlowStatistic.mode

		mod.SectionPath = ""
		_, writeErr := mod.Write("super-secrets/PublicIndex/"+indexPath+"/"+idName+"/"+id+"/DataFlowGroup/"+dataFlowStatistic.flowGroup+"/dataFlowName/"+dataFlowStatistic.flowName+"/"+dataFlowStatistic.stateCode, statMap)
		if writeErr != nil {
			logFunc("Error writing out DataFlowStatistics to vault", writeErr)
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
