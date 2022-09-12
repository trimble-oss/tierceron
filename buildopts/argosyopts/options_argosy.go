//go:build argosy
// +build argosy

package argosyopts

import (
	"fmt"
	"github.com/mrjrieke/nute/mashupsdk"
	"math"
	"strconv"
	"tierceron/trcvault/util"
	"tierceron/vaulthelper/kv"
)

func getGroupSize(groups []util.DataFlowGroup) (float64, float64, float64) {
	groupsize := 0.0
	flowsize := 0.0
	statsize := 0.0
	for _, group := range groups {
		groupsize += float64(len(group.Flows))
		for _, flow := range group.Flows {
			flowsize += float64(len(flow.Statistics))
			statsize = float64(len(flow.Statistics))
		}
	}
	return groupsize, flowsize, statsize
}

func buildArgosies(startID int64, args util.ArgosyFleet) (util.ArgosyFleet, []int64, []int64) {
	argosyId := startID - 1
	collectionIDs := []int64{}
	curveCollection := []int64{}
	for i := 0; i < len(args.Argosies); i++ {
		dfgsize, dfsize, dfstatsize := getGroupSize(args.Argosies[i].Groups)
		argosyId = startID + int64(i)*int64(1.0+float64(dfgsize)+math.Pow(float64(dfsize), 2.0)+math.Pow(float64(dfstatsize), 3.0))
		collectionIDs = append(collectionIDs, argosyId)
		argosy := args.Argosies[i]
		argosy.MashupDetailedElement = mashupsdk.MashupDetailedElement{
			Id:             argosyId,
			State:          &mashupsdk.MashupElementState{Id: argosyId, State: int64(mashupsdk.Init)},
			Name:           argosy.ArgosyID,
			Alias:          "It",
			Description:    "Testing to see if description will properly change!",
			Data:           "",
			Custosrenderer: "TenantDataRenderer",
			Renderer:       "Element",
			Genre:          "Argosy",
			Subgenre:       "",
			Parentids:      []int64{},
			Childids:       []int64{-2},
		}

		collection := []int64{}
		children := []int64{}

		argosy.Groups, collection, children, curveCollection, argosy = buildDataFlowGroups(argosyId+1, argosy, dfgsize, dfsize, dfstatsize, argosyId)
		for _, id := range collection {
			collectionIDs = append(collectionIDs, id)
		}
		for _, id := range children {
			argosy.MashupDetailedElement.Childids = append(argosy.MashupDetailedElement.Childids, id)
		}

		args.Argosies[i] = argosy
	}

	return args, collectionIDs, curveCollection
}

func buildDataFlowGroups(startID int64, argosy util.Argosy, dfgsize float64, dfsize float64, dfstatsize float64, parentID int64) ([]util.DataFlowGroup, []int64, []int64, []int64, util.Argosy) {
	argosyId := startID - 1
	collectionIDs := []int64{}
	childIDs := []int64{}
	curveCollection := []int64{}
	for i := 0; i < len(argosy.Groups); i++ {
		argosyId = startID + int64(i)*int64(1.0+float64(dfsize)+math.Pow(float64(dfstatsize), 2.0))
		collectionIDs = append(collectionIDs, argosyId)
		childIDs = append(childIDs, argosyId)
		group := argosy.Groups[i]
		group.MashupDetailedElement = mashupsdk.MashupDetailedElement{
			Id:             argosyId,
			State:          &mashupsdk.MashupElementState{Id: argosyId, State: int64(mashupsdk.Hidden)},
			Name:           group.Name, //"DataFlowGroup-" + strconv.Itoa(int(argosyId)),
			Alias:          "It",
			Description:    "",
			Data:           "",
			Custosrenderer: "",
			Renderer:       "Element",
			Genre:          "DataFlowGroup",
			Subgenre:       "",
			Parentids:      []int64{parentID},
			Childids:       []int64{-4},
		}
		collection := []int64{}
		children := []int64{}

		group.Flows, collection, children, curveCollection, group = buildDataFlows(argosyId+1, group, dfsize, dfstatsize, argosyId)
		for _, id := range collection {
			collectionIDs = append(collectionIDs, id)
		}
		for _, id := range children {
			group.MashupDetailedElement.Childids = append(group.MashupDetailedElement.Childids, id)
		}
		argosy.Groups[i] = group
	}
	return argosy.Groups, collectionIDs, childIDs, curveCollection, argosy
}

func buildDataFlows(startID int64, group util.DataFlowGroup, dfsize float64, dfstatsize float64, parentID int64) ([]util.DataFlow, []int64, []int64, []int64, util.DataFlowGroup) {
	argosyId := startID - 1
	collectionIDs := []int64{}
	childIDs := []int64{}
	curveCollection := []int64{}
	for i := 0; i < len(group.Flows); i++ {
		argosyId = startID + int64(i)*int64(1.0+float64(dfstatsize))
		collectionIDs = append(collectionIDs, argosyId)
		childIDs = append(childIDs, argosyId)
		flow := group.Flows[i]
		flow.MashupDetailedElement = mashupsdk.MashupDetailedElement{
			Id:             argosyId,
			State:          &mashupsdk.MashupElementState{Id: argosyId, State: int64(mashupsdk.Hidden)},
			Name:           flow.Name, //"DataFlow-" + strconv.Itoa(int(argosyId)),
			Alias:          "It",
			Description:    "",
			Data:           "",
			Custosrenderer: "",
			Renderer:       "Element",
			Genre:          "DataFlow",
			Subgenre:       "",
			Parentids:      []int64{parentID},
			Childids:       []int64{-4},
		}
		otherIds := []int64{}
		children := []int64{}
		var total int64

		flow.Statistics, otherIds, children, curveCollection, flow, total = buildDataFlowStatistics(argosyId+1, flow, dfstatsize, argosyId)
		flow.MashupDetailedElement.Data = fmt.Sprintf("%f", float64(total)/float64(len(flow.Statistics)))
		for _, id := range otherIds {
			collectionIDs = append(collectionIDs, id)
		}
		for _, id := range children {
			flow.MashupDetailedElement.Childids = append(flow.MashupDetailedElement.Childids, id)
		}
		group.Flows[i] = flow
	}
	return group.Flows, collectionIDs, childIDs, curveCollection, group
}

func buildDataFlowStatistics(startID int64, flow util.DataFlow, dfstatsize float64, parentID int64) ([]util.DataFlowStatistic, []int64, []int64, []int64, util.DataFlow, int64) {
	argosyId := startID - 1
	collectionIDs := []int64{}
	childIDs := []int64{}
	curveCollection := []int64{}
	total := int64(0)
	for i := 0; i < len(flow.Statistics); i++ {
		argosyId = argosyId + 1
		childIDs = append(childIDs, argosyId)
		curveCollection = append(curveCollection, argosyId)
		stat := flow.Statistics[i]
		total = int64(total) + int64(stat.TimeSplit)
		stat.MashupDetailedElement = mashupsdk.MashupDetailedElement{
			Id:             argosyId,
			State:          &mashupsdk.MashupElementState{Id: argosyId, State: int64(mashupsdk.Hidden)},
			Name:           stat.StateName + "-" + strconv.Itoa(int(argosyId)), //"DataFlowStatistic-" + strconv.Itoa(int(argosyId)), //data[pointer], //
			Alias:          "It",
			Description:    "",
			Data:           strconv.FormatInt(int64(stat.TimeSplit), 10), //time in nanoseconds
			Custosrenderer: "",
			Renderer:       "Curve",
			Genre:          "DataFlowStatistic",
			Subgenre:       "",
			//Data:        strconv.Itoa(stat.TimeSplit),
			Parentids: []int64{parentID},
			Childids:  []int64{-1},
		}
		flow.Statistics[i] = stat
	}
	return flow.Statistics, collectionIDs, childIDs, curveCollection, flow, int64(total)
}

func BuildFleet(mod *kv.Modifier) (util.ArgosyFleet, error) {
	argosies := []util.Argosy{
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
			"Outside",
			[]util.DataFlowGroup{},
		},
		{
			mashupsdk.MashupDetailedElement{
				Basisid:        -4,
				State:          &mashupsdk.MashupElementState{Id: -4, State: int64(mashupsdk.Hidden)},
				Name:           "{0}-SubSpiral",
				Alias:          "It",
				Description:    "",
				Data:           "",
				Custosrenderer: "",
				Renderer:       "Element",
				Genre:          "Solid",
				Subgenre:       "Ento",
				Parentids:      []int64{-2},
				Childids:       []int64{},
			},
			"SubSpiral",
			[]util.DataFlowGroup{},
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
				Colabrenderer:  "Path",
				Genre:          "Solid",
				Subgenre:       "Skeletal",
				Parentids:      []int64{},
				Childids:       []int64{},
			},
			"Curve",
			[]util.DataFlowGroup{},
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
				Colabrenderer:  "Path",
				Genre:          "Solid",
				Subgenre:       "Skeletal",
				Parentids:      nil,
				Childids:       []int64{-1},
			},
			"CurvePathEntity-1",
			[]util.DataFlowGroup{},
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
				Parentids:      nil,
				Childids:       []int64{-4},
			},
			"{0}-Path",
			[]util.DataFlowGroup{},
		},
	}
	args, err := util.InitArgosyFleet(mod, "TenantDatabase")
	if err != nil {
		return util.ArgosyFleet{}, err
	}
	elementCollection := []int64{}
	curveCollection := []int64{}
	args, elementCollection, curveCollection = buildArgosies(5, args)
	//args.Argosies = append(args.Argosies, argosies)
	//argosies = append(argosies, args)
	argosies = append(argosies, util.Argosy{
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
			Childids:       elementCollection,
		},
		"PathGroupOne",
		[]util.DataFlowGroup{},
	})
	curveCollection = append(curveCollection, 1)
	argosies = append(argosies, util.Argosy{
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
			Childids:       curveCollection,
		},
		"CurvesGroupOne",
		[]util.DataFlowGroup{},
	})
	for _, arg := range argosies {
		args.Argosies = append(args.Argosies, arg)
	}
	return args, nil
}

func GetDataFlowGroups(mod *kv.Modifier, argosy *util.Argosy) []util.DataFlowGroup {
	return nil
}
