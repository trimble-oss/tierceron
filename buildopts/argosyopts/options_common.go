//go:build !argosy && !tc
// +build !argosy,!tc

package argosyopts

import (
	"log"
	"tierceron/trcvault/flowutil"
	"tierceron/vaulthelper/kv"
)

func GetStubbedDataFlowStatistics() ([]string, map[string][]float64) {
	//	return data, TimeData
	return nil, nil
}

func BuildFleet(mod *kv.Modifier, logger *log.Logger) (flowutil.TTDINode, error) {
	return flowutil.TTDINode{}, nil
}

func GetDataFlowGroups(mod *kv.Modifier, argosy *flowutil.TTDINode) []flowutil.TTDINode {
	return nil
}
