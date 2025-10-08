//go:build argosy
// +build argosy

package argosyopts

import (
	"log"

	tccore "github.com/trimble-oss/tierceron-core/v2/core"
	"github.com/trimble-oss/tierceron/pkg/vaulthelper/kv"
)

func BuildFleet(mod *kv.Modifier, logger *log.Logger) (*tccore.TTDINode, error) {
	return &tccore.TTDINode{}, nil
}

// Unused function - candidate for future deletion
func GetDataFlowGroups(mod *kv.Modifier, argosy *tccore.TTDINode) []tccore.TTDINode {
	return nil
}
