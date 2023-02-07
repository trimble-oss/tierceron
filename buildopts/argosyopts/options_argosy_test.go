//go:build argosy
// +build argosy

package argosyopts

import (
	"github.com/trimble-oss/tierceron/trcvault/util"
	"github.com/trimble-oss/tierceron/vaulthelper/kv"
)

func BuildFleet(mod *kv.Modifier, logger *log.Logger) (util.TTDINode, error) {
	return util.TTDINode{}, nil
}

func GetDataFlowGroups(mod *kv.Modifier, argosy *util.TTDINode) []util.TTDINode {
	return nil
}
