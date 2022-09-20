//go:build tc && argosy
// +build tc,argosy

package argosyopts

import (
	tcbuildopts "VaultConfig.TenantConfig/util/buildopts"
	"fmt"
	"github.com/mrjrieke/nute/mashupsdk"
	"log"
	"math"
	"strconv"
	"tierceron/trcvault/util"
	"tierceron/vaulthelper/kv"
	"time"
)

var pointer int
var data []string
var TimeData map[string][]float64

func buildStubArgosies(startID int64, argosysize int, dfgsize int, dfsize int, dfstatsize int) ([]util.Argosy, []int64, []int64) {
	data, TimeData = tcbuildopts.GetStubbedDataFlowStatistics()
	if TimeData != nil {
		for j := 0; j < len(data); j++ {
			for i := 0; i < len(TimeData[data[j]])-1; i++ {
				fmt.Println(TimeData[data[j]][i+1] - TimeData[data[j]][i])
			}
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

		argosy.Groups, collection, children, curveCollection = buildStubDataFlowGroups(argosyId+1, dfgsize, dfsize, dfstatsize, argosyId)
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

func buildStubDataFlowGroups(startID int64, dfgsize int, dfsize int, dfstatsize int, parentID int64) ([]util.DataFlowGroup, []int64, []int64, []int64) {
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

		group.Flows, collection, children, curveCollection = buildStubDataFlows(argosyId+1, dfsize, dfstatsize, argosyId)
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

func buildStubDataFlows(startID int64, dfsize int, dfstatsize int, parentID int64) ([]util.DataFlow, []int64, []int64, []int64) {
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

		flow.Statistics, otherIds, children, curveCollection = buildStubDataFlowStatistics(argosyId+1, dfstatsize, argosyId)
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

func buildStubDataFlowStatistics(startID int64, dfstatsize int, parentID int64) ([]util.DataFlowStatistic, []int64, []int64, []int64) {
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

func BuildStubFleet(mod *kv.Modifier, logger *log.Logger) (util.ArgosyFleet, error) {
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
	tempArgosies, collectionIDs, curveIDs := buildStubArgosies(5, 20, 10, 5, 10)
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

func GetStubDataFlowGroups(mod *kv.Modifier, argosy *util.Argosy) []util.DataFlowGroup {
	return nil
}
