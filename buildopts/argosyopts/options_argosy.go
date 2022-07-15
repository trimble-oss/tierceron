//go:build argosy
// +build argosy

package argosyopts

import (
	"tierceron/trcvault/util"
	"tierceron/vaulthelper/kv"
)

func BuildFleet(mod *kv.Modifier) util.ArgosyFleet {
	return util.ArgosyFleet{}
}

func GetDataFlowGroups(mod *kv.Modifier, argosy *util.Argosy) []util.DataFlowGroup {
	return nil
}
