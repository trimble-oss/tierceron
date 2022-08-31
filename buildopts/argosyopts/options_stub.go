//go:build argosystub
// +build argosystub

package argosyopts

import (
	"fmt"
	"github.com/mrjrieke/nute/mashupsdk"
	"math"
	"strconv"
	"tierceron/trcvault/util"
	"tierceron/vaulthelper/kv"
	"time"
)

var data []string = []string{"UpdateBudget", "AddChangeOrder", "UpdateChangeOrder", "AddChangeOrderItem", "UpdateChangeOrderItem",
	"UpdateChangeOrderItemApprovalDate", "AddChangeOrderStatus", "UpdateChangeOrderStatus", "AddContract",
	"UpdateContract", "AddCustomer", "UpdateCustomer", "AddItemAddon", "UpdateItemAddon", "AddItemCost",
	"UpdateItemCost", "AddItemMarkup", "UpdateItemMarkup", "AddPhase", "UpdatePhase", "AddScheduleOfValuesFixedPrice",
	"UpdateScheduleOfValuesFixedPrice", "AddScheduleOfValuesUnitPrice", "UpdateScheduleOfValuesUnitPrice"}

//using tests from 8/24/22
var TimeData = map[string][]float64{
	data[0]:  []float64{0.0, .650, .95, 5.13, 317.85, 317.85},
	data[1]:  []float64{0.0, 0.3, 0.56, 5.06, 78.4, 78.4},
	data[2]:  []float64{0.0, 0.2, 0.38, 5.33, 78.4, 78.4},
	data[3]:  []float64{0.0, 0.34, 0.36, 5.25, 141.93, 141.93},
	data[4]:  []float64{0.0, 0.24, 0.52, 4.87, 141.91, 141.91},
	data[5]:  []float64{0.0, 0.24, 0.6, 5.39, 148.01, 148.01},
	data[6]:  []float64{0.0, 0.11, 0.13, 4.89, 32.47, 32.47},
	data[7]:  []float64{0.0, 0.08, 0.1, 4.82, 32.49, 32.49},
	data[8]:  []float64{0.0, 0.33, 0.5, 5.21, 89.53, 89.53},
	data[9]:  []float64{0.0, 0.3, 0.62, 5, 599.99}, //when test fails no repeat at end
	data[10]: []float64{0.0, 0.19, 0.47, 4.87, 38.5, 38.5},
	data[11]: []float64{0.0, 0.26, 0.58, 5, 39.08, 39.08},
	data[12]: []float64{0.0, 0.36, 0.37, 5.32, 69.09, 69.06},
	data[13]: []float64{0.0, 0.09, 0.13, 4.73, 164.1, 164.1},
	data[14]: []float64{0.0, 0.61, 0.61, 0.92, 5.09, 108.35, 108.35},
	data[15]: []float64{0.0, 0.48, 0.66, 5.02, 108.46, 108.46},
	data[16]: []float64{0.0, 0.34, 0.36, 4.87, 53.42, 53.42},
	data[17]: []float64{0.0, 0.14, 0.23, 5.11, 53.29, 53.29},
	data[18]: []float64{0.0, 0.69, 0.88, 5.07, 102.38, 102.38},
	data[19]: []float64{0.0, 0.73, 1.03, 5.01, 104.31, 104.31},
	data[20]: []float64{0.0, 0.19, 0.22, 4.82, 218.8, 218.8},
	data[21]: []float64{0.0, 0.19, 0.36, 5.21, 218.66, 218.66},
	data[22]: []float64{0.0, 0.36, 0.41, 4.93, 273.66, 273.66},
	data[23]: []float64{0.0, 0.22, 0.39, 4.87, 273.24, 273.24},
}

var pointer int

func buildArgosies(startID int64, argosysize int, dfgsize int, dfsize int, dfstatsize int) ([]util.Argosy, []int64, []int64) {
	for j := 0; j < len(data); j++ {
		for i := 0; i < len(TimeData[data[j]])-1; i++ {
			fmt.Println(TimeData[data[j]][i+1] - TimeData[data[j]][i])
		}
	}
	argosyId := startID - 1
	pointer = 0
	argosies := []util.Argosy{}
	collectionIDs := []int64{}
	curveCollection := []int64{}
	for i := 0; i < argosysize; i++ {
		argosyId = startID + int64(i)*int64(1.0+float64(dfgsize)+math.Pow(float64(dfsize), 2.0)+math.Pow(float64(dfstatsize), 3.0))
		collectionIDs = append(collectionIDs, argosyId)
		argosy := util.Argosy{
			MashupDetailedElement: mashupsdk.MashupDetailedElement{
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
			ArgosyID: "Argosy-" + strconv.Itoa(int(argosyId)),
			Groups:   []util.DataFlowGroup{},
		}
		collection := []int64{}
		children := []int64{}

		argosy.Groups, collection, children, curveCollection = buildDataFlowGroups(argosyId+1, dfgsize, dfsize, dfstatsize, argosyId)
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

func buildDataFlowGroups(startID int64, dfgsize int, dfsize int, dfstatsize int, parentID int64) ([]util.DataFlowGroup, []int64, []int64, []int64) {
	argosyId := startID - 1
	collectionIDs := []int64{}
	childIDs := []int64{}
	groups := []util.DataFlowGroup{}
	curveCollection := []int64{}
	for i := 0; i < dfgsize; i++ {
		argosyId = startID + int64(i)*int64(1.0+float64(dfsize)+math.Pow(float64(dfstatsize), 2.0))
		collectionIDs = append(collectionIDs, argosyId)
		childIDs = append(childIDs, argosyId)
		group := util.DataFlowGroup{
			MashupDetailedElement: mashupsdk.MashupDetailedElement{
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
			Name:  "DataFlowGroup-" + strconv.Itoa(int(argosyId)),
			Flows: []util.DataFlow{},
		}
		collection := []int64{}
		children := []int64{}

		group.Flows, collection, children, curveCollection = buildDataFlows(argosyId+1, dfsize, dfstatsize, argosyId)
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

func buildDataFlows(startID int64, dfsize int, dfstatsize int, parentID int64) ([]util.DataFlow, []int64, []int64, []int64) {
	argosyId := startID - 1
	collectionIDs := []int64{}
	childIDs := []int64{}
	flows := []util.DataFlow{}
	curveCollection := []int64{}
	for i := 0; i < dfsize; i++ {
		argosyId = startID + int64(i)*int64(1.0+float64(dfstatsize))
		collectionIDs = append(collectionIDs, argosyId)
		childIDs = append(childIDs, argosyId)
		flow := util.DataFlow{
			MashupDetailedElement: mashupsdk.MashupDetailedElement{
				Id:          argosyId,
				State:       &mashupsdk.MashupElementState{Id: argosyId, State: int64(mashupsdk.Hidden)},
				Name:        data[pointer] + "-" + strconv.Itoa(int(argosyId)), //"DataFlow-" + strconv.Itoa(int(argosyId)),
				Alias:       "It",
				Description: "",
				Renderer:    "Element",
				Genre:       "DataFlow",
				Subgenre:    "",
				Parentids:   []int64{parentID},
				Childids:    []int64{-4},
			},
			Name:       data[pointer] + strconv.Itoa(int(argosyId)), //"DataFlow-" + strconv.Itoa(int(argosyId)),
			TimeStart:  time.Now(),
			Statistics: []util.DataFlowStatistic{},
		}
		otherIds := []int64{}
		children := []int64{}

		flow.Statistics, otherIds, children, curveCollection = buildDataFlowStatistics(argosyId+1, dfstatsize, argosyId)
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

func buildDataFlowStatistics(startID int64, dfstatsize int, parentID int64) ([]util.DataFlowStatistic, []int64, []int64, []int64) {
	argosyId := startID - 1
	collectionIDs := []int64{}
	childIDs := []int64{}
	curveCollection := []int64{}
	stats := []util.DataFlowStatistic{}
	for i := 0; i < dfstatsize; i++ {
		argosyId = argosyId + 1
		childIDs = append(childIDs, argosyId)
		curveCollection = append(curveCollection, argosyId)
		stat := util.DataFlowStatistic{
			MashupDetailedElement: mashupsdk.MashupDetailedElement{
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
		}
		pointer = pointer + 1
		if pointer == 24 {
			pointer = 0
		}
		stats = append(stats, stat)
	}
	return stats, collectionIDs, childIDs, curveCollection
}

func BuildFleet(mod *kv.Modifier) (util.ArgosyFleet, error) {
	argosy, err := util.InitArgosyFleet(mod, "TenantDatabase")
	if err != nil {
		return util.ArgosyFleet{}, err
	}
	fmt.Println(argosy)
	Argosys := []util.Argosy{
		{
			mashupsdk.MashupDetailedElement{
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
			"Outside",
			[]util.DataFlowGroup{},
		},
		{
			mashupsdk.MashupDetailedElement{
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
			"SubSpiral",
			[]util.DataFlowGroup{},
		},
		{
			mashupsdk.MashupDetailedElement{
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
			"Curve",
			[]util.DataFlowGroup{},
		},
		{
			mashupsdk.MashupDetailedElement{
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
			"CurvePathEntity-1",
			[]util.DataFlowGroup{},
		},
		{
			mashupsdk.MashupDetailedElement{
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
			"{0}-Path",
			[]util.DataFlowGroup{},
		},
	}
	tempArgosies, collectionIDs, curveIDs := buildArgosies(5, 20, 10, 5, 10)
	for _, argosy := range tempArgosies {
		Argosys = append(Argosys, argosy)
	}
	Argosys = append(Argosys, util.Argosy{
		mashupsdk.MashupDetailedElement{
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
		"PathGroupOne",
		[]util.DataFlowGroup{},
	})
	curveIDs = append(curveIDs, 1)
	Argosys = append(Argosys, util.Argosy{
		mashupsdk.MashupDetailedElement{
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
		"CurvesGroupOne",
		[]util.DataFlowGroup{},
	})

	return util.ArgosyFleet{
		ArgosyName: "Dev Environment",
		Argosies:   []util.Argosy(Argosys),
	}, nil
}

func GetDataFlowGroups(mod *kv.Modifier, argosy *util.Argosy) []util.DataFlowGroup {
	return nil
}
