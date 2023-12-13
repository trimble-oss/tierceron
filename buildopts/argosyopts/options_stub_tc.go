//go:build tc && argosy
// +build tc,argosy

package argosyopts

import (
	tcbuildopts "VaultConfig.TenantConfig/util/buildopts"

	"encoding/json"
	"log"
	"math"
	"strconv"

	"github.com/trimble-oss/tierceron-nute/mashupsdk"
	flowcore "github.com/trimble-oss/tierceron/trcflow/core"
	"github.com/trimble-oss/tierceron/vaulthelper/kv"
)

var pointer int
var data []string
var TimeData map[string][]float64

func getStubGroupSize(data []string) float64 {
	statsize := 0.0
	for i := 0; i < len(data); i++ {
		statsize += float64(len(TimeData[data[i]]))
	}
	// groupsize := 1.0
	// flowsize := float64(len(data))
	// statsize := 0.0
	// for i := 0; i < len(data); i++ {
	// 	statsize += float64(len(TimeData[data[i]]))
	// }
	return statsize
}

func buildStubArgosies(startID int64, argosysize int, dfgsize int) ([]flowcore.TTDINode, []int64, []int64) {
	data, TimeData = tcbuildopts.GetStubbedDataFlowStatistics()
	if data == nil || TimeData == nil {
		log.Println("Error in obtaining stub data in buildStubArgosies")
		return []flowcore.TTDINode{}, []int64{}, []int64{}
	}
	// if TimeData != nil {
	// 	for j := 0; j < len(data); j++ {
	// 		for i := 0; i < len(TimeData[data[j]])-1; i++ {
	// 			fmt.Println(TimeData[data[j]][i+1] - TimeData[data[j]][i])
	// 		}
	// 	}
	// }
	argosyId := startID - 1
	pointer = 0
	argosies := []flowcore.TTDINode{}
	collectionIDs := []int64{}
	curveCollection := []int64{}
	dfsize := len(data)
	dfstatsize := int(getStubGroupSize(data))
	for i := 0; i < argosysize; i++ {
		argosyId = startID + int64(i)*int64(1.0+float64(dfgsize)+math.Pow(float64(dfsize), 2.0)+math.Pow(float64(dfstatsize), 3.0))
		collectionIDs = append(collectionIDs, argosyId)
		argosy := flowcore.TTDINode{
			&mashupsdk.MashupDetailedElement{
				Id:          argosyId,
				State:       &mashupsdk.MashupElementState{Id: argosyId, State: int64(mashupsdk.Init)},
				Name:        "Argosy-" + strconv.Itoa(int(argosyId)),
				Alias:       "It",
				Description: "Testing to see if description will properly change!",
				Renderer:    "Element",
				Genre:       "Argosy",
				Subgenre:    "",
				Parentids:   []int64{},
				Childids:    []int64{-2},
			},
			[]*flowcore.TTDINode{},
		}
		collection := []int64{}
		children := []int64{}

		argosy.ChildNodes, collection, children, curveCollection = buildStubDataFlowGroups(argosyId+1, dfgsize, dfsize, dfstatsize, argosyId)
		for _, id := range collection {
			collectionIDs = append(collectionIDs, id)
		}
		for _, id := range children {
			argosy.MashupDetailedElement.Childids = append(argosy.MashupDetailedElement.Childids, id)
		}
		argosies = append(argosies, argosy)
	}

	return argosies, collectionIDs, curveCollection
}

func buildStubDataFlowGroups(startID int64, dfgsize int, dfsize int, dfstatsize int, parentID int64) ([]*flowcore.TTDINode, []int64, []int64, []int64) {
	argosyId := startID - 1
	collectionIDs := []int64{}
	childIDs := []int64{}
	groups := []*flowcore.TTDINode{}
	curveCollection := []int64{}
	for i := 0; i < dfgsize; i++ {
		argosyId = startID + int64(i)*int64(1.0+float64(dfsize)+math.Pow(float64(dfstatsize), 2.0))
		collectionIDs = append(collectionIDs, argosyId)
		childIDs = append(childIDs, argosyId)
		group := &flowcore.TTDINode{
			&mashupsdk.MashupDetailedElement{
				Id:          argosyId,
				State:       &mashupsdk.MashupElementState{Id: argosyId, State: int64(mashupsdk.Hidden)},
				Name:        "DataFlowGroup-" + strconv.Itoa(int(argosyId)),
				Alias:       "It",
				Description: "",
				Renderer:    "Element",
				Genre:       "DataFlowGroup",
				Subgenre:    "",
				Parentids:   []int64{parentID},
				Childids:    []int64{-4},
			},
			[]*flowcore.TTDINode{},
		}
		collection := []int64{}
		children := []int64{}

		group.ChildNodes, collection, children, curveCollection = buildStubDataFlows(argosyId+1, dfsize, dfstatsize, argosyId)
		for _, id := range collection {
			collectionIDs = append(collectionIDs, id)
		}
		for _, id := range children {
			group.MashupDetailedElement.Childids = append(group.MashupDetailedElement.Childids, id)
		}

		groups = append(groups, group)
	}
	return groups, collectionIDs, childIDs, curveCollection
}

func buildStubDataFlows(startID int64, dfsize int, dfstatsize int, parentID int64) ([]*flowcore.TTDINode, []int64, []int64, []int64) {
	argosyId := startID - 1
	collectionIDs := []int64{}
	childIDs := []int64{}
	flows := []*flowcore.TTDINode{}
	curveCollection := []int64{}
	for i := 0; i < dfsize; i++ {
		argosyId = startID + int64(i)*int64(1.0+float64(dfstatsize))
		collectionIDs = append(collectionIDs, argosyId)
		childIDs = append(childIDs, argosyId)
		pointer = i
		flow := &flowcore.TTDINode{
			&mashupsdk.MashupDetailedElement{
				Id:             argosyId,
				State:          &mashupsdk.MashupElementState{Id: argosyId, State: int64(mashupsdk.Hidden)},
				Name:           data[i] + "-" + strconv.Itoa(int(argosyId)), //"DataFlow-" + strconv.Itoa(int(argosyId)),
				Alias:          "It",
				Description:    "",
				Renderer:       "Element",
				Genre:          "DataFlow",
				Custosrenderer: "TenantDataRenderer",
				Subgenre:       "",
				Parentids:      []int64{parentID},
				Childids:       []int64{-4},
			},
			[]*flowcore.TTDINode{},
		}
		otherIds := []int64{}
		children := []int64{}

		flow.ChildNodes, otherIds, children, curveCollection = buildStubDataFlowStatistics(argosyId+1, dfstatsize, argosyId)
		for _, id := range otherIds {
			collectionIDs = append(collectionIDs, id)
		}
		for _, id := range children {
			flow.MashupDetailedElement.Childids = append(flow.MashupDetailedElement.Childids, id)
		}
		flows = append(flows, flow)
	}
	return flows, collectionIDs, childIDs, curveCollection
}

func buildStubDataFlowStatistics(startID int64, dfstatsize int, parentID int64) ([]*flowcore.TTDINode, []int64, []int64, []int64) {
	argosyId := startID - 1
	collectionIDs := []int64{}
	childIDs := []int64{}
	curveCollection := []int64{}
	stats := []*flowcore.TTDINode{}
	for i := 0; i < len(TimeData[data[pointer]]); i++ {
		argosyId = argosyId + 1
		childIDs = append(childIDs, argosyId)
		curveCollection = append(curveCollection, argosyId)
		stat := &flowcore.TTDINode{
			&mashupsdk.MashupDetailedElement{
				Id:          argosyId,
				State:       &mashupsdk.MashupElementState{Id: argosyId, State: int64(mashupsdk.Hidden)},
				Name:        "DataFlowStatistic-" + strconv.Itoa(int(argosyId)), //data[pointer], //
				Alias:       "It",
				Description: "",
				Renderer:    "Curve",
				Genre:       "DataFlowStatistic",
				Subgenre:    "",
				Parentids:   []int64{parentID},
				Childids:    []int64{-1},
			},
			[]*flowcore.TTDINode{},
		}
		statdata := make(map[string]interface{})
		statdata["TimeSplit"] = (TimeData[data[pointer]][i]) * math.Pow(10.0, 9.0)
		encodedData, err := json.Marshal(&statdata)
		if err != nil {
			log.Println("Error in encoding data in buildStubDataFlowStatistics")
		}
		stat.MashupDetailedElement.Data = string(encodedData)
		// pointer = pointer + 1
		// if pointer == 24 {
		// 	pointer = 0
		// }
		stats = append(stats, stat)
	}
	return stats, collectionIDs, childIDs, curveCollection
}

func BuildStubFleet(mod *kv.Modifier, logger *log.Logger) (*flowcore.TTDINode, error) {
	Argosys := []*flowcore.TTDINode{
		{
			&mashupsdk.MashupDetailedElement{
				Id:          3,
				State:       &mashupsdk.MashupElementState{Id: 3, State: int64(mashupsdk.Init)},
				Name:        "Outside",
				Alias:       "Outside",
				Description: "The background was selected",
				Renderer:    "Background",
				Genre:       "Space",
				Subgenre:    "Exo",
				Parentids:   nil,
				Childids:    nil,
			},
			[]*flowcore.TTDINode{},
		},
		{
			&mashupsdk.MashupDetailedElement{
				Basisid:     -4,
				State:       &mashupsdk.MashupElementState{Id: -4, State: int64(mashupsdk.Hidden)},
				Name:        "{0}-SubSpiral",
				Alias:       "It",
				Description: "",
				Renderer:    "Element",
				Genre:       "Solid",
				Subgenre:    "Ento",
				Parentids:   []int64{-2},
				Childids:    []int64{},
			},
			[]*flowcore.TTDINode{},
		},
		{
			&mashupsdk.MashupDetailedElement{
				Basisid:       -1,
				State:         &mashupsdk.MashupElementState{Id: -1, State: int64(mashupsdk.Mutable)},
				Name:          "Curve",
				Alias:         "It",
				Description:   "",
				Renderer:      "Curve",
				Colabrenderer: "Path",
				Genre:         "Solid",
				Subgenre:      "Skeletal",
				Parentids:     []int64{},
				Childids:      []int64{},
			},
			[]*flowcore.TTDINode{},
		},
		{
			&mashupsdk.MashupDetailedElement{
				Id:            1,
				State:         &mashupsdk.MashupElementState{Id: 1, State: int64(mashupsdk.Init)},
				Name:          "CurvePathEntity-1",
				Description:   "",
				Renderer:      "Curve",
				Colabrenderer: "Path",
				Genre:         "Solid",
				Subgenre:      "Skeletal",
				Parentids:     nil,
				Childids:      []int64{-1},
			},
			[]*flowcore.TTDINode{},
		},
		{
			&mashupsdk.MashupDetailedElement{
				Basisid:     -2,
				State:       &mashupsdk.MashupElementState{Id: -2, State: int64(mashupsdk.Mutable)},
				Name:        "{0}-Path",
				Alias:       "It",
				Description: "Path was selected",
				Renderer:    "Element",
				Genre:       "Solid",
				Subgenre:    "Ento",
				Parentids:   nil,
				Childids:    []int64{-4},
			},
			[]*flowcore.TTDINode{},
		},
	}
	tempArgosies, collectionIDs, curveIDs := buildStubArgosies(5, 10, 10)
	for _, argosy := range tempArgosies {
		Argosys = append(Argosys, &argosy)
	}
	Argosys = append(Argosys, &flowcore.TTDINode{
		&mashupsdk.MashupDetailedElement{
			Id:          4,
			State:       &mashupsdk.MashupElementState{Id: 4, State: int64(mashupsdk.Init)},
			Name:        "PathGroupOne",
			Description: "Paths",
			Renderer:    "Element",
			Genre:       "Collection",
			Subgenre:    "Element",
			Parentids:   []int64{},
			Childids:    collectionIDs,
		},
		[]*flowcore.TTDINode{},
	})
	curveIDs = append(curveIDs, 1)
	Argosys = append(Argosys, &flowcore.TTDINode{
		&mashupsdk.MashupDetailedElement{
			Id:            2,
			State:         &mashupsdk.MashupElementState{Id: 2, State: int64(mashupsdk.Init)},
			Name:          "CurvesGroupOne",
			Description:   "Curves",
			Renderer:      "Curve",
			Colabrenderer: "Path",
			Genre:         "Collection",
			Subgenre:      "Skeletal",
			Parentids:     nil,
			Childids:      curveIDs,
		},
		[]*flowcore.TTDINode{},
	})

	return &flowcore.TTDINode{ChildNodes: Argosys}, nil
	// flowcore.ArgosyFleet{
	// 	ArgosyName: "Dev Environment",
	// 	Argosies:   []flowcore.Argosy(Argosys),
	// }, nil
}

func GetStubDataFlowGroups(mod *kv.Modifier, argosy *flowcore.TTDINode) []flowcore.TTDINode {
	return nil
}
