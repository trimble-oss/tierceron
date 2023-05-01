//go:build argosy && tc
// +build argosy,tc

package argosyopts

import (
	"encoding/json"
	flowcore "github.com/trimble-oss/tierceron/trcflow/core"
	"github.com/trimble-oss/tierceron/vaulthelper/kv"
	"log"
	"strconv"

	tcbuildopts "VaultConfig.TenantConfig/util/buildopts"
	"github.com/trimble-oss/tierceron-nute/mashupsdk"
)

var elementcollectionIDs []int64
var curvecollectionIDs []int64
var currentID int64
var fail bool

// Updates the ID --> May need to change if parallelizing the process
func updateID() int64 {
	currentID += 1
	return currentID
}

// Returns stubbed data
func GetStubbedDataFlowStatistics() ([]string, map[string][]float64) {
	return tcbuildopts.GetStubbedDataFlowStatistics()
}

// Populates tenant tree with ID and other data and maintains Childnode ordering
// Returns a parent node that points to the first layer of tenants, the current node's ID, and encoded string
// of updated parent node's data
func recursiveBuildArgosies(node flowcore.TTDINode, parent *flowcore.TTDINode, notFirst bool) (flowcore.TTDINode, int64, string) {
	var nodeID int64
	state := mashupsdk.Hidden
	var parentIDs []int64
	if parent.MashupDetailedElement.Id != 0 {
		parentIDs = []int64{parent.MashupDetailedElement.Id}
	} else {
		state = mashupsdk.Init
	}
	var decodedparent interface{}
	decodedparentNode := make(map[string]interface{})
	if parent.MashupDetailedElement.Data != "" {
		err := json.Unmarshal([]byte(parent.MashupDetailedElement.Data), &decodedparent)
		if err != nil {
			log.Println("Error in decoding data in recursiveBuildArgosies")
		}
		decodedparentNode = decodedparent.(map[string]interface{})
	}
	if node.MashupDetailedElement.Data != "" {
		var decodednode interface{}
		err := json.Unmarshal([]byte(node.MashupDetailedElement.Data), &decodednode)
		if err != nil {
			log.Println("Error in decoding data in recursiveBuildArgosies")
		}
		decodedNodeData := decodednode.(map[string]interface{})
		if len(node.ChildNodes) == 0 && decodedNodeData != nil && decodedNodeData["TimeSplit"] != nil && decodedNodeData["StateName"] != nil {
			nodeID = updateID()
			curvecollectionIDs = append(curvecollectionIDs, nodeID)
			if decodedNodeData["Mode"] != nil {
				if decodedparentNode["Mode"] == nil || decodedparentNode["Mode"] != nil && decodedparentNode["Mode"].(float64) != 2 {
					decodedparentNode["Mode"] = decodedNodeData["Mode"].(float64)
					encodedParentNodeData, err := json.Marshal(&decodedparentNode)
					if err != nil {
						log.Println("Error in encoding data in InitDataFlow")
					}
					parent.MashupDetailedElement.Data = string(encodedParentNodeData)
				}
			}
			node.MashupDetailedElement = mashupsdk.MashupDetailedElement{
				Id:             nodeID,
				State:          &mashupsdk.MashupElementState{Id: nodeID, State: int64(state)},
				Name:           decodedNodeData["StateName"].(string) + "-" + strconv.Itoa(int(nodeID)),
				Alias:          "It",
				Description:    "",
				Data:           node.MashupDetailedElement.Data,
				Custosrenderer: "TenantDataRenderer",
				Renderer:       "Curve",
				Genre:          "DataFlowStatistic",
				Subgenre:       "",
				Parentids:      parentIDs,
				Childids:       []int64{-1},
			}
			encodedParentNodeData, err := json.Marshal(&decodedparentNode)
			if err != nil {
				log.Println("Error in encoding data in InitDataFlow")
			}
			return node, node.MashupDetailedElement.Id, string(encodedParentNodeData)
		} else if decodedNodeData != nil {
			nodeID = updateID()
			elementcollectionIDs = append(elementcollectionIDs, nodeID)
			name := node.MashupDetailedElement.Name
			data := node.MashupDetailedElement.Data
			node.MashupDetailedElement = mashupsdk.MashupDetailedElement{
				Id:             nodeID,
				State:          &mashupsdk.MashupElementState{Id: nodeID, State: int64(state)},
				Name:           name + "-" + strconv.Itoa(int(nodeID)),
				Alias:          "It",
				Description:    "",
				Data:           data,
				Custosrenderer: "TenantDataRenderer",
				Renderer:       "Element",
				Genre:          "",
				Subgenre:       "",
				Parentids:      parentIDs,
				Childids:       []int64{-2},
			}
			decodedNodeData["Fail"] = fail
			fail = false
			encodedNodeData, err := json.Marshal(&decodedNodeData)
			if err != nil {
				log.Println("Error in encoding data in InitDataFlow")
			}
			node.MashupDetailedElement.Data = string(encodedNodeData)
		}
	} else if notFirst {
		nodeID = updateID()
		elementcollectionIDs = append(elementcollectionIDs, nodeID)
		name := node.MashupDetailedElement.Name
		data := node.MashupDetailedElement.Data
		node.MashupDetailedElement = mashupsdk.MashupDetailedElement{
			Id:             nodeID,
			State:          &mashupsdk.MashupElementState{Id: nodeID, State: int64(state)},
			Name:           name + "-" + strconv.Itoa(int(nodeID)),
			Alias:          "It",
			Description:    "",
			Data:           data,
			Custosrenderer: "TenantDataRenderer",
			Renderer:       "Element",
			Genre:          "",
			Subgenre:       "",
			Parentids:      parentIDs,
			Childids:       []int64{-2},
		}
	}
	var childids []int64
	var childid int64
	for i := 0; i < len(node.ChildNodes); i++ {
		node.ChildNodes[i], childid, node.MashupDetailedElement.Data = recursiveBuildArgosies(node.ChildNodes[i], &node, true)
		childids = append(childids, childid)
	}
	node.MashupDetailedElement.Childids = append(node.MashupDetailedElement.Childids, childids...)
	encodedParentNodeData, err := json.Marshal(&decodedparentNode)
	if err != nil {
		log.Println("Error in encoding data in InitDataFlow")
	}
	return node, node.MashupDetailedElement.Id, string(encodedParentNodeData)
}

// Builds a tree of Tenants and their respective Childnodes populated with
// corresponding data and an error message
func BuildFleet(mod *kv.Modifier, logger *log.Logger) (flowcore.TTDINode, error) {
	if mod == nil {
		return BuildStubFleet(mod, logger)
	}

	argosies := []flowcore.TTDINode{
		{
			mashupsdk.MashupDetailedElement{
				Id:             3,
				State:          &mashupsdk.MashupElementState{Id: 3, State: int64(mashupsdk.Init)},
				Name:           "Outside",
				Alias:          "Outside",
				Description:    "The background was selected",
				Data:           "",
				Custosrenderer: "",
				Renderer:       "Background",
				Genre:          "Space",
				Subgenre:       "Exo",
				Parentids:      nil,
				Childids:       nil,
			},
			[]flowcore.TTDINode{},
		},
		{
			mashupsdk.MashupDetailedElement{
				Basisid:        -1,
				State:          &mashupsdk.MashupElementState{Id: -1, State: int64(mashupsdk.Mutable)},
				Name:           "Curve",
				Alias:          "It",
				Description:    "",
				Data:           "",
				Custosrenderer: "",
				Renderer:       "Curve",
				Colabrenderer:  "Element",
				Genre:          "Solid",
				Subgenre:       "Skeletal",
				Parentids:      []int64{},
				Childids:       []int64{},
			},
			[]flowcore.TTDINode{},
		},
		{
			mashupsdk.MashupDetailedElement{
				Id:             1,
				State:          &mashupsdk.MashupElementState{Id: 1, State: int64(mashupsdk.Init)},
				Name:           "CurvePathEntity-1",
				Description:    "",
				Data:           "",
				Custosrenderer: "",
				Renderer:       "Curve",
				Colabrenderer:  "Element",
				Genre:          "Solid",
				Subgenre:       "Skeletal",
				Parentids:      []int64{},
				Childids:       []int64{-1},
			},
			[]flowcore.TTDINode{},
		},
		{
			mashupsdk.MashupDetailedElement{
				Basisid:        -2,
				State:          &mashupsdk.MashupElementState{Id: -2, State: int64(mashupsdk.Mutable)},
				Name:           "{0}-Path",
				Alias:          "It",
				Description:    "Path was selected",
				Data:           "",
				Custosrenderer: "",
				Renderer:       "Element",
				Genre:          "Solid",
				Subgenre:       "Ento",
				Parentids:      []int64{},
				Childids:       []int64{},
			},
			[]flowcore.TTDINode{},
		},
	}
	args, err := flowcore.InitArgosyFleet(mod, "TenantDatabase", logger)
	if err != nil {
		return flowcore.TTDINode{}, err
	}
	currentID = 8
	args, _, _ = recursiveBuildArgosies(args, &flowcore.TTDINode{}, false)
	argosies = append(argosies, flowcore.TTDINode{
		mashupsdk.MashupDetailedElement{
			Id:             4,
			State:          &mashupsdk.MashupElementState{Id: 4, State: int64(mashupsdk.Init)},
			Name:           "PathGroupOne",
			Description:    "Paths",
			Data:           "",
			Custosrenderer: "",
			Renderer:       "Element",
			Genre:          "Collection",
			Subgenre:       "Element",
			Parentids:      []int64{},
			Childids:       elementcollectionIDs,
		},
		[]flowcore.TTDINode{},
	})
	curvecollectionIDs = append(curvecollectionIDs, 1)
	argosies = append(argosies, flowcore.TTDINode{
		mashupsdk.MashupDetailedElement{
			Id:             2,
			State:          &mashupsdk.MashupElementState{Id: 2, State: int64(mashupsdk.Init)},
			Name:           "CurvesGroupOne",
			Description:    "Curves",
			Data:           "",
			Custosrenderer: "",
			Renderer:       "Curve",
			Colabrenderer:  "Path",
			Genre:          "Collection",
			Subgenre:       "Skeletal",
			Parentids:      nil,
			Childids:       curvecollectionIDs,
		},
		[]flowcore.TTDINode{},
	})
	argosies = append(argosies, flowcore.TTDINode{
		mashupsdk.MashupDetailedElement{
			Id:             updateID(),
			State:          &mashupsdk.MashupElementState{Id: 9, State: int64(mashupsdk.Init)},
			Name:           "NodeLabel",
			Description:    "Labels colors",
			Data:           "",
			Custosrenderer: "",
			Renderer:       "GuiRenderer",
			Colabrenderer:  "",
			Genre:          "",
			Subgenre:       "",
			Parentids:      nil,
			Childids:       nil,
		},
		[]flowcore.TTDINode{},
	})

	args.ChildNodes = append(args.ChildNodes, argosies...)
	return args, nil
}

func GetDataFlowGroups(mod *kv.Modifier, argosy *flowcore.TTDINode) []flowcore.TTDINode {
	return nil
}
