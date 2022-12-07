//go:build !argosy && !tc
// +build !argosy,!tc

package argosyopts

import (
	"log"
	flowcore "tierceron/trcflow/core"
	"tierceron/vaulthelper/kv"
)

func GetStubbedDataFlowStatistics() ([]string, map[string][]float64) {
	//	return data, TimeData
	return nil, nil
}

func BuildFleet(mod *kv.Modifier, logger *log.Logger) (flowcore.TTDINode, error) {
	return flowcore.TTDINode{}, nil
}

func GetDataFlowGroups(mod *kv.Modifier, argosy *flowcore.TTDINode) []flowcore.TTDINode {
	return nil
}
