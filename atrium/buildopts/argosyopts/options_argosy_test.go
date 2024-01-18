//go:build argosy
// +build argosy

package argosyopts

import (
	"log"

	"github.com/trimble-oss/tierceron/pkg/vaulthelper/kv"
	"github.com/trimble-oss/tierceron/trcvault/util"
)

func BuildFleet(mod *kv.Modifier, logger *log.Logger) (util.TTDINode, error) {
	return util.TTDINode{}, nil
}

func GetDataFlowGroups(mod *kv.Modifier, argosy *util.TTDINode) []util.TTDINode {
	return nil
}
