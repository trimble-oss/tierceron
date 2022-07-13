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

func (dfs *DataFlowGroup) UpdateDataFlowStatistic(flowS string, flowN string, stateN string, stateC string, mode int) {
	var newDFStat = DataFlowStatistic{flowS, flowN, stateN, stateC, time.Since(dfs.TimeStart), mode}
	dfs.Statistics = append(dfs.Statistics, newDFStat)
}

func (dfs *DataFlowGroup) FinishStatistic(logFunc func(string, error), mod *kv.Modifier, id string) {
	//TODO : Write Statistic to vault
	/*dfs.FinishStatisticLog(logFunc)
	var statMap map[string]interface{}

	for _, dataFlowStatistic := range dfs.statistics {

	}
	*/
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
