//go:build !argosy
// +build !argosy

package argosyopts

import "tierceron/trcvault/util"

func BuildFleet() util.ArgosyFleet {
	return util.ArgosyFleet{}
}

func GetDataFlowGroups(argosy *util.Argosy) []util.DataFlowGroup {
	return nil
}
