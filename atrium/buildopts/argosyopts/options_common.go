//go:build !argosy && !tc
// +build !argosy,!tc

package argosyopts

import (
	"log"

	flowcore "github.com/trimble-oss/tierceron/atrium/trcflow/core"
	"github.com/trimble-oss/tierceron/pkg/vaulthelper/kv"
)

func GetStubbedDataFlowStatistics() ([]string, map[string][]float64) {
	//	return data, TimeData
	return nil, nil
}

// BuildFleet loads a set of TTDINodes utilizing the modifier.
// TTDINodes returned are used to build the data spiral.
// * TTDINodes are defined recursively, with each node containing a list of child nodes.
// * this enabled the data to be rendered 3-dimensionally.
// The modifier is used to access the secret provider.
func BuildFleet(mod *kv.Modifier, logger *log.Logger) (*flowcore.TTDINode, error) {
	return &flowcore.TTDINode{}, nil
}

// Unused function - candidate for future deletion
func GetDataFlowGroups(mod *kv.Modifier, argosy *flowcore.TTDINode) []flowcore.TTDINode {
	return nil
}
