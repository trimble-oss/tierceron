//go:build !argosy && !tc && !argosystub && !trcx
// +build !argosy,!tc,!argosystub,!trcx

package argosyopts

import (
	"log"

	tccore "github.com/trimble-oss/tierceron-core/v2/core"
	"github.com/trimble-oss/tierceron/pkg/vaulthelper/kv"
)

var data []string = []string{"One", "Two", "Three", "Four", "Five",
	"Six", "Seven", "Eight", "Nine",
	"Ten", "Eleven", "Twelve"}

// using tests from 8/24/22
var TimeData = map[string][]float64{
	data[0]:  {0.0, .650, .95, 5.13, 317.85, 317.85},
	data[1]:  {0.0, 0.3, 0.56, 5.06, 78.4, 78.4},
	data[2]:  {0.0, 0.2, 0.38, 5.33, 78.4, 78.4},
	data[3]:  {0.0, 0.34, 0.36, 5.25, 141.93, 141.93},
	data[4]:  {0.0, 0.24, 0.52, 4.87, 141.91, 141.91},
	data[5]:  {0.0, 0.24, 0.6, 5.39, 148.01, 148.01},
	data[6]:  {0.0, 0.11, 0.13, 4.89, 32.47, 32.47},
	data[7]:  {0.0, 0.08, 0.1, 4.82, 32.49, 32.49},
	data[8]:  {0.0, 0.33, 0.5, 5.21, 89.53, 89.53},
	data[9]:  {0.0, 0.3, 0.62, 5, 599.99}, //when test fails no repeat at end
	data[10]: {0.0, 0.19, 0.47, 4.87, 38.5, 38.5},
	data[11]: {0.0, 0.26, 0.58, 5, 39.08, 39.08},
}

// GetStubbedDataFlowStatistics returns the list data being tracked along with time data for the data being tracked.
func GetStubbedDataFlowStatistics() ([]string, map[string][]float64) {
	//	return data, TimeData
	return data, TimeData
}

// BuildFleet loads a set of TTDINodes utilizing the modifier.
// TTDINodes returned are used to build the data spiral.
// * TTDINodes are defined recursively, with each node containing a list of child nodes.
// * this enabled the data to be rendered 3-dimensionally.
// The modifier is used to access the secret provider.
func BuildFleet(mod *kv.Modifier, logger *log.Logger) (*tccore.TTDINode, error) {
	return &tccore.TTDINode{}, nil
}

// Unused function - candidate for future deletion
func GetDataFlowGroups(mod *kv.Modifier, argosy *tccore.TTDINode) []tccore.TTDINode {
	return nil
}
