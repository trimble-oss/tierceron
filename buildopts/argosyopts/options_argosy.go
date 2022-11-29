//go:build argosy && tc
// +build argosy,tc

package argosyopts

import (
	"encoding/json"
	"fmt"
	"log"
	"strings"
	//"math"
	"strconv"
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

func getGroupSize(groups []flowutil.TTDINode) (float64, float64, float64) {
	groupsize := 0.0
	flowsize := 0.0
	statsize := 0.0
	for _, group := range groups {
		groupsize += float64(len(group.ChildNodes))
		for _, flow := range group.ChildNodes {
			flowsize += float64(len(flow.ChildNodes))
			statsize = float64(len(flow.ChildNodes))
		}
	}
	return groupsize, flowsize, statsize
}

func recursiveBuildArgosies(node flowutil.TTDINode, parentID int64, notFirst bool) (flowutil.TTDINode, int64) {
	//PROBLEM: Overriding child ids with other ids --> id not updating properly --> potentially concurrency issue?
	var nodeID int64
	state := mashupsdk.Hidden
	//if node.MashupDetailedElement != nil {
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
			//return nil
		}
		decodedNodeData := decodednode.(map[string]interface{})
		if len(node.ChildNodes) == 0 && decodedNodeData != nil && decodedNodeData["TimeSplit"] != nil && decodedNodeData["StateName"] != nil {
			//Know it's a DataFlowStastic --> add to curve renderer
			nodeID = updateID() //argosyId + 1
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
				Name:           decodedNodeData["StateName"].(string) + "-" + strconv.Itoa(int(nodeID)), //"DataFlowStatistic-" + strconv.Itoa(int(argosyId)), //data[pointer], //
				Alias:          "It",
				Description:    "",
				Data:           node.MashupDetailedElement.Data,
				Custosrenderer: "TenantDataRenderer",
				Renderer:       "Curve",
				Genre:          "DataFlowStatistic",
				Subgenre:       "",
				Parentids:      parentIDs, //[]int64{parentID},
				Childids:       []int64{-1},
			}
			//flow.ChildNodes[i] = stat
			return node, node.MashupDetailedElement.Id
		} else if decodedNodeData != nil {
			//DataFlow here deal with fail or no fail
			nodeID = updateID() //startID + int64(i)*int64(1.0+float64(dfstatsize))
			elementcollectionIDs = append(elementcollectionIDs, nodeID)
			// var childIDs []int64
			// for i := nodeID + 1; i <= nodeID+int64(len(node.ChildNodes)); i++ {
			// 	childIDs = append(childIDs, i)
			// }
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
				Parentids:      parentIDs, //[]int64{parentID},
				Childids:       []int64{-2},
			}
			//children := []int64{}
			decodedNodeData["Fail"] = fail
			fail = false
			//encode data again and set =
			encodedNodeData, err := json.Marshal(&decodedNodeData)
			if err != nil {
				log.Println("Error in encoding data in InitDataFlow")
			}
			node.MashupDetailedElement.Data = string(encodedNodeData)
			// for _, id := range children {
			//     node.MashupDetailedElement.Childids = append(flow.MashupDetailedElement.Childids, id)
			// }
			// group.ChildNodes[i] = flow
		}
	} else if notFirst {
		nodeID = updateID() //startID + int64(i)*int64(1.0+float64(dfsize)+math.Pow(float64(dfstatsize), 2.0))
		elementcollectionIDs = append(elementcollectionIDs, nodeID)
		// var childIDs []int64
		// for i := nodeID + 1; i <= nodeID+int64(len(node.ChildNodes)); i++ {
		// 	childIDs = append(childIDs, i)
		// }
		//group := argosy.ChildNodes[i]
		name := node.MashupDetailedElement.Name
		if strings.HasPrefix(name, "qa14p8") {
			fmt.Println("Checking for childids")
		}
		data := node.MashupDetailedElement.Data
		//var decodednode interface{}
		//Somehow check for dataflow object --> maybe check in for loop looping thru child ids --> add fail there
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
			Parentids:      parentIDs, //[]int64{parentID},
			Childids:       []int64{-2},
		}
	}
	//}
	var childids []int64
	var childid int64
	for i := 0; i < len(node.ChildNodes); i++ {
		node.ChildNodes[i], childid = recursiveBuildArgosies(node.ChildNodes[i], nodeID, true)
		childids = append(childids, childid)
	}
	node.MashupDetailedElement.Childids = append(node.MashupDetailedElement.Childids, childids...)
	return node, node.MashupDetailedElement.Id
}

// func buildArgosies(startID int64, args flowutil.TTDINode) (flowutil.TTDINode, []int64, []int64) {
// 	var argosyId int64
// 	curveCollection := []int64{}
// 	curveIds := []int64{}
// 	for i := 0; i < len(args.ChildNodes); i++ {
// 		dfgsize, dfsize, dfstatsize := getGroupSize(args.ChildNodes[i].ChildNodes)
// 		argosyId = updateID() //startID + int64(i)*int64(1.0+float64(dfgsize)+math.Pow(float64(dfsize), 2.0)+math.Pow(float64(dfstatsize), 3.0))
// 		elementcollectionIDs = append(elementcollectionIDs, argosyId)
// 		argosy := args.ChildNodes[i]
// 		name := argosy.MashupDetailedElement.Name
// 		data := argosy.MashupDetailedElement.Data
// 		argosy.MashupDetailedElement = mashupsdk.MashupDetailedElement{
// 			Id:             argosyId,
// 			State:          &mashupsdk.MashupElementState{Id: argosyId, State: int64(mashupsdk.Init)},
// 			Name:           name + "-" + strconv.Itoa(int(argosyId)),
// 			Alias:          "It",
// 			Description:    "Testing to see if description will properly change!",
// 			Data:           data,
// 			Custosrenderer: "TenantDataRenderer",
// 			Renderer:       "Element",
// 			Genre:          "Argosy",
// 			Subgenre:       "",
// 			Parentids:      []int64{},
// 			Childids:       []int64{-2},
// 		}
// 		children := []int64{}

// 		argosy.ChildNodes, _, children, curveCollection, argosy = buildDataFlowGroups(argosyId+1, argosy, dfgsize, dfsize, dfstatsize, argosyId)

// 		argosy.MashupDetailedElement.Childids = append(argosy.MashupDetailedElement.Childids, children...)
// 		curveIds = append(curveIds, curveCollection...)

// 		args.ChildNodes[i] = argosy
// 	}
// 	return args, elementcollectionIDs, curveIds
// }

// func buildDataFlowGroups(startID int64, argosy flowutil.TTDINode, dfgsize float64, dfsize float64, dfstatsize float64, parentID int64) ([]flowutil.TTDINode, []int64, []int64, []int64, flowutil.TTDINode) {
// 	argosyId := startID - 1
// 	childIDs := []int64{}
// 	curveCollection := []int64{}
// 	for i := 0; i < len(argosy.ChildNodes); i++ {
// 		argosyId = updateID() //startID + int64(i)*int64(1.0+float64(dfsize)+math.Pow(float64(dfstatsize), 2.0))
// 		elementcollectionIDs = append(elementcollectionIDs, argosyId)
// 		childIDs = append(childIDs, argosyId)
// 		group := argosy.ChildNodes[i]
// 		name := group.MashupDetailedElement.Name
// 		data := group.MashupDetailedElement.Data
// 		group.MashupDetailedElement = mashupsdk.MashupDetailedElement{
// 			Id:             argosyId,
// 			State:          &mashupsdk.MashupElementState{Id: argosyId, State: int64(mashupsdk.Hidden)},
// 			Name:           name + "-" + strconv.Itoa(int(argosyId)),
// 			Alias:          "It",
// 			Description:    "",
// 			Data:           data,
// 			Custosrenderer: "TenantDataRenderer",
// 			Renderer:       "Element",
// 			Genre:          "DataFlowGroup",
// 			Subgenre:       "",
// 			Parentids:      []int64{parentID},
// 			Childids:       []int64{-2},
// 		}
// 		children := []int64{}

// 		group.ChildNodes, _, children, curveCollection, group = buildDataFlows(argosyId+1, group, dfsize, dfstatsize, argosyId)

// 		for _, id := range children {
// 			group.MashupDetailedElement.Childids = append(group.MashupDetailedElement.Childids, id)
// 		}
// 		argosy.ChildNodes[i] = group
// 	}
// 	return argosy.ChildNodes, elementcollectionIDs, childIDs, curveCollection, argosy
// }

// func buildDataFlows(startID int64, group flowutil.TTDINode, dfsize float64, dfstatsize float64, parentID int64) ([]flowutil.TTDINode, []int64, []int64, []int64, flowutil.TTDINode) {
// 	argosyId := startID - 1
// 	childIDs := []int64{}
// 	curveCollection := []int64{}
// 	for i := 0; i < len(group.ChildNodes); i++ {
// 		argosyId = updateID() //startID + int64(i)*int64(1.0+float64(dfstatsize))
// 		elementcollectionIDs = append(elementcollectionIDs, argosyId)
// 		childIDs = append(childIDs, argosyId)
// 		flow := group.ChildNodes[i]
// 		name := flow.MashupDetailedElement.Name
// 		data := flow.MashupDetailedElement.Data
// 		flow.MashupDetailedElement = mashupsdk.MashupDetailedElement{
// 			Id:             argosyId,
// 			State:          &mashupsdk.MashupElementState{Id: argosyId, State: int64(mashupsdk.Hidden)},
// 			Name:           name + "-" + strconv.Itoa(int(argosyId)),
// 			Alias:          "It",
// 			Description:    "",
// 			Data:           data,
// 			Custosrenderer: "TenantDataRenderer",
// 			Renderer:       "Element",
// 			Genre:          "DataFlow",
// 			Subgenre:       "",
// 			Parentids:      []int64{parentID},
// 			Childids:       []int64{-2},
// 		}
// 		children := []int64{}
// 		fail := false
// 		flow.ChildNodes, _, children, curveCollection, flow, fail = buildDataFlowStatistics(argosyId+1, flow, dfstatsize, argosyId)
// 		var decodedFlow interface{}
// 		decodedFlowData := make(map[string]interface{})
// 		if flow.MashupDetailedElement.Data != "" {
// 			err := json.Unmarshal([]byte(data), &decodedFlow)
// 			if err != nil {
// 				log.Println("Error in decoding data in buildDataFlowStatistics")
// 				continue
// 			}
// 			decodedFlowData = decodedFlow.(map[string]interface{})
// 		}
// 		decodedFlowData["Fail"] = fail
// 		//encode data again and set =
// 		encodedFlowData, err := json.Marshal(&decodedFlowData)
// 		if err != nil {
// 			log.Println("Error in encoding data in InitDataFlow")
// 		}
// 		flow.MashupDetailedElement.Data = string(encodedFlowData)
// 		for _, id := range children {
// 			flow.MashupDetailedElement.Childids = append(flow.MashupDetailedElement.Childids, id)
// 		}
// 		group.ChildNodes[i] = flow
// 	}
// 	return group.ChildNodes, elementcollectionIDs, childIDs, curveCollection, group
// }

// func buildDataFlowStatistics(startID int64, flow flowutil.TTDINode, dfstatsize float64, parentID int64) ([]flowutil.TTDINode, []int64, []int64, []int64, flowutil.TTDINode, bool) {
// 	argosyId := startID - 1
// 	childIDs := []int64{}
// 	curveCollection := []int64{}
// 	fail := false
// 	for i := 0; i < len(flow.ChildNodes); i++ {
// 		argosyId = updateID() //argosyId + 1
// 		curvecollectionIDs = append(curvecollectionIDs, argosyId)
// 		childIDs = append(childIDs, argosyId)
// 		curveCollection = append(curveCollection, argosyId)
// 		stat := flow.ChildNodes[i]
// 		data := stat.MashupDetailedElement.Data
// 		var decodedstat interface{}
// 		err := json.Unmarshal([]byte(data), &decodedstat)
// 		if err != nil {
// 			log.Println("Error in decoding data in buildDataFlowStatistics")
// 			break
// 		}
// 		decodedStatData := decodedstat.(map[string]interface{})
// 		if decodedStatData["TimeSplit"] == nil || decodedStatData["StateName"] == nil {
// 			log.Println("Error in decoding data in buildDataFlowStatistics because data not initialized properly")
// 			break
// 		}
// 		if decodedStatData["Mode"] != nil {
// 			mode := decodedStatData["Mode"].(float64)
// 			if mode == 2 {
// 				fail = true
// 			}
// 		}
// 		stat.MashupDetailedElement = mashupsdk.MashupDetailedElement{
// 			Id:             argosyId,
// 			State:          &mashupsdk.MashupElementState{Id: argosyId, State: int64(mashupsdk.Hidden)},
// 			Name:           decodedStatData["StateName"].(string) + "-" + strconv.Itoa(int(argosyId)), //"DataFlowStatistic-" + strconv.Itoa(int(argosyId)), //data[pointer], //
// 			Alias:          "It",
// 			Description:    "",
// 			Data:           data,
// 			Custosrenderer: "TenantDataRenderer",
// 			Renderer:       "Curve",
// 			Genre:          "DataFlowStatistic",
// 			Subgenre:       "",
// 			Parentids:      []int64{parentID},
// 			Childids:       []int64{-1},
// 		}
// 		flow.ChildNodes[i] = stat
// 	}
// 	return flow.ChildNodes, elementcollectionIDs, childIDs, curveCollection, flow, fail
// }

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
