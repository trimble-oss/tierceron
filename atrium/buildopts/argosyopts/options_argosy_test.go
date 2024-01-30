//go:build argosy
// +build argosy

package argosyopts

import (
	"log"

	flowcore "github.com/trimble-oss/tierceron/atrium/trcflow/core"
	"github.com/trimble-oss/tierceron/pkg/vaulthelper/kv"
)

func BuildFleet(mod *kv.Modifier, logger *log.Logger) (*flowcore.TTDINode, error) {
	return &flowcore.TTDINode{}, nil
}

func GetDataFlowGroups(mod *kv.Modifier, argosy *flowcore.TTDINode) []flowcore.TTDINode {
	return nil
}
