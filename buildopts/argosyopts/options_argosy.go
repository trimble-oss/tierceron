//go:build argosy && tc
// +build argosy,tc

package argosyopts

import (
	"encoding/json"
	"fmt"
	"log"
	"strconv"
	"strings"
	"tierceron/trcvault/flowutil"
	"tierceron/vaulthelper/kv"

	tcbuildopts "VaultConfig.TenantConfig/util/buildopts"
	"github.com/mrjrieke/nute/mashupsdk"
)

var elementcollectionIDs []int64
var curvecollectionIDs []int64
var currentID int64
var fail bool

func updateID() int64 {
	currentID += 1
	return currentID
}

func GetStubbedDataFlowStatistics() ([]string, map[string][]float64) {
	return tcbuildopts.GetStubbedDataFlowStatistics()
}

func recursiveBuildArgosies(node flowutil.TTDINode, parentID int64, notFirst bool) (flowutil.TTDINode, int64) {
	var nodeID int64
	state := mashupsdk.Hidden
	var parentIDs []int64
	if parentID != 0 {
		parentIDs = []int64{parentID}
	} else {
		state = mashupsdk.Init
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
				mode := decodedNodeData["Mode"].(float64)
				if mode == 2 {
					fail = true
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
			return node, node.MashupDetailedElement.Id
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
		if strings.HasPrefix(name, "qa14p8") {
			fmt.Println("Checking for childids")
		}
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
		node.ChildNodes[i], childid = recursiveBuildArgosies(node.ChildNodes[i], nodeID, true)
		childids = append(childids, childid)
	}
	node.MashupDetailedElement.Childids = append(node.MashupDetailedElement.Childids, childids...)
	return node, node.MashupDetailedElement.Id
}

func BuildFleet(mod *kv.Modifier, logger *log.Logger) (flowutil.TTDINode, error) {
	if mod == nil {
		return BuildStubFleet(mod, logger)
	}

	argosies := []flowutil.TTDINode{
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
			[]flowutil.TTDINode{},
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
			[]flowutil.TTDINode{},
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
			[]flowutil.TTDINode{},
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
			[]flowutil.TTDINode{},
		},
	}
	args, err := flowutil.InitArgosyFleet(mod, "TenantDatabase", logger)
	var count int
	for _, arg := range args.ChildNodes {
		count++
		for _, dfg := range arg.ChildNodes {
			count++
			for _, df := range dfg.ChildNodes {
				count++
				count += len(df.ChildNodes)
			}
		}
	}
	if err != nil {
		return flowutil.TTDINode{}, err
	}
	// elementCollection := []int64{}
	//fail = false
	currentID = 8
	//args, elementCollection, _ = buildArgosies(8, args)
	args, _ = recursiveBuildArgosies(args, currentID, false)
	//elementCollection = elementcollectionIDs
	argosies = append(argosies, flowutil.TTDINode{
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
		[]flowutil.TTDINode{},
	})
	curvecollectionIDs = append(curvecollectionIDs, 1)
	argosies = append(argosies, flowutil.TTDINode{
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
		[]flowutil.TTDINode{},
	})
	argosies = append(argosies, flowutil.TTDINode{
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
		[]flowutil.TTDINode{},
	})

	args.ChildNodes = append(args.ChildNodes, argosies...)
	return args, nil
}

func GetDataFlowGroups(mod *kv.Modifier, argosy *flowutil.TTDINode) []flowutil.TTDINode {
	return nil
}
