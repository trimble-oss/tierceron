package util

import (
	"errors"
	"strings"
	"tierceron/vaulthelper/kv"
	"time"
)

type DataFlowStatistic struct {
	flowSet   string
	flowName  string
	stateName string
	stateCode string
	timeSplit time.Duration
	mode      int
}

type DataFlowStatistics struct {
	timeStart  time.Time
	statistics []DataFlowStatistic
}

func InitDataFlowStatistic() DataFlowStatistics {
	var stats []DataFlowStatistic
	var newDFStatistic = DataFlowStatistics{time.Now(), stats}
	return newDFStatistic

}

func (dfs *DataFlowStatistics) UpdateDataFlowStatistic(flowS string, flowN string, stateN string, stateC string, mode int) {
	var newDFStat = DataFlowStatistic{flowS, flowN, stateN, stateC, time.Since(dfs.timeStart), mode}
	dfs.statistics = append(dfs.statistics, newDFStat)
}

func (dfs *DataFlowStatistics) FinishStatistic(logFunc func(string, error), mod *kv.Modifier, id string) {
	//TODO : Write Statistic to vault
	/*dfs.FinishStatisticLog(logFunc)
	var statMap map[string]interface{}

	for _, dataFlowStatistic := range dfs.statistics {

	}
	*/
}

//Make success/failure placeholders later
func (dfs *DataFlowStatistics) FinishStatisticLog(logFunc func(string, error)) {
	for _, stat := range dfs.statistics {
		if strings.Contains(stat.stateName, "Failure") {
			logFunc(stat.flowName+"-"+stat.stateName, errors.New(stat.stateName))
			if stat.mode == 2 { //Update snapshot mode on failure so it doesn't repeat

			}
		} else {
			logFunc(stat.flowName+"-"+stat.stateName, nil)
		}
	}
}
