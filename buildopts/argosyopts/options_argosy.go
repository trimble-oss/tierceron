//go:build argosy
// +build argosy

package argosyopts

import "github.com/dolthub/go-mysql-server/server"

func BuildFleet() util.ArgosyFleet {
	return util.ArgosyFleet{}
}

func GetDataFlowGroups(argosy *util.Argosy) []util.DataFlowGroup {
	return nil
}
