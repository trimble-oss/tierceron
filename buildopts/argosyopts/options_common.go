//go:build !argosy && !argosystub
// +build !argosy,!argosystub

package argosyopts

import (
	"log"
	"tierceron/trcvault/util"
	"tierceron/vaulthelper/kv"
)

func BuildFleet(mod *kv.Modifier, logger *log.Logger) (util.TTDINode, error) {
	return util.TTDINode{}, nil
}

func GetDataFlowGroups(mod *kv.Modifier, argosy *util.TTDINode) []util.TTDINode {
	return nil
}
