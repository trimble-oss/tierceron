//go:build !argosy && !argosystub
// +build !argosy,!argosystub

package argosyopts

import (
	"tierceron/trcvault/util"
	"tierceron/vaulthelper/kv"
)

func BuildFleet(mod *kv.Modifier) (util.ArgosyFleet, error) {
	return util.ArgosyFleet{}, nil
}

func GetDataFlowGroups(mod *kv.Modifier, argosy *util.Argosy) []util.DataFlowGroup {
	return nil
}
