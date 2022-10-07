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

func BuildFleet(mod *kv.Modifier, logger *log.Logger) (util.TTDINode, error) {
	return util.TTDINode{}, nil
}

func GetDataFlowGroups(mod *kv.Modifier, argosy *util.TTDINode) []util.TTDINode {
	return nil
}
