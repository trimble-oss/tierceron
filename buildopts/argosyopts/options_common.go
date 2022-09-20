//go:build !argosy && !tc
// +build !argosy,!tc

package argosyopts

import (
	"log"
	"tierceron/trcvault/util"
	"tierceron/vaulthelper/kv"
)

func GetStubbedDataFlowStatistics() ([]string, map[string][]float64) {
	//	return data, TimeData
	return nil, nil
}

func BuildFleet(mod *kv.Modifier, logger *log.Logger) (util.ArgosyFleet, error) {
	return util.ArgosyFleet{}, nil
}

func GetDataFlowGroups(mod *kv.Modifier, argosy *util.Argosy) []util.DataFlowGroup {
	return nil
}
