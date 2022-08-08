//go:build argosystub
// +build argosystub

package argosyopts

import (
	"github.com/mrjrieke/nute/mashupsdk"
	"strconv"
	"tierceron/trcvault/util"
	"tierceron/vaulthelper/kv"
)

func BuildFleet(mod *kv.Modifier) util.ArgosyFleet {
	Argosys := []util.Argosy{
		{
			mashupsdk.MashupDetailedElement{
				Id:          6,
				State:       &mashupsdk.MashupElementState{Id: 6, State: int64(mashupsdk.Init)},
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
				Renderer:    "SubSpiral",
				Genre:       "Solid",
				Subgenre:    "Ento",
				Parentids:   []int64{},
				Childids:    []int64{5},
			},
			"SubSpiral",
			[]util.DataFlowGroup{},
		},
		{
			mashupsdk.MashupDetailedElement{
				Id:          3,
				State:       &mashupsdk.MashupElementState{Id: 3, State: int64(mashupsdk.Hidden)},
				Name:        "SubSpiralGroupOne",
				Description: "SubSpirals",
				Renderer:    "SubSpiral",
				Genre:       "Collection",
				Subgenre:    "SubSpiral",
				Parentids:   nil,
				Childids:    []int64{5},
			},
			"SubSpiralGroupOne",
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
				Childids:      []int64{1},
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
				Genre:         "Abstract",
				Subgenre:      "",
				Parentids:     nil,
				Childids:      []int64{-1},
			},
			"CurvePathEntity-1",
			[]util.DataFlowGroup{},
		},
		{
			mashupsdk.MashupDetailedElement{
				Id:            2,
				State:         &mashupsdk.MashupElementState{Id: 2, State: int64(mashupsdk.Init)},
				Name:          "CurvesGroupOne",
				Description:   "Curves",
				Renderer:      "Curve",
				Colabrenderer: "Path",
				Genre:         "Collection",
				Subgenre:      "Curve",
				Parentids:     nil,
				Childids:      []int64{1},
			},
			"CurvesGroupOne",
			[]util.DataFlowGroup{},
		},
		{
			mashupsdk.MashupDetailedElement{
				Basisid:     -2,
				State:       &mashupsdk.MashupElementState{Id: -2, State: int64(mashupsdk.Mutable)},
				Name:        "{0}-Path",
				Alias:       "It",
				Description: "Path was selected",
				Renderer:    "Path",
				Genre:       "Solid",
				Subgenre:    "Ento",
				Parentids:   nil,
				Childids:    []int64{-4},
			},
			"{0}-Path",
			[]util.DataFlowGroup{},
		},
		{
			mashupsdk.MashupDetailedElement{
				Id:          7,
				State:       &mashupsdk.MashupElementState{Id: 7, State: int64(mashupsdk.Init)},
				Name:        "PathEntity-7",
				Alias:       "It",
				Description: "",
				Renderer:    "Path",
				Genre:       "Abstract",
				Subgenre:    "",
				Parentids:   nil,
				Childids:    []int64{-2, 5}, //, 5
			},
			"PathEntity-7",
			[]util.DataFlowGroup{{
				MashupDetailedElement: mashupsdk.MashupDetailedElement{
					Id:          5,
					State:       &mashupsdk.MashupElementState{Id: int64(5), State: int64(mashupsdk.Hidden)},
					Name:        "SubSpiralEntity-" + strconv.Itoa(5),
					Alias:       "It",
					Description: "",
					Renderer:    "SubSpiral",
					Genre:       "Abstract",
					Subgenre:    "",
					Parentids:   []int64{},
					Childids:    []int64{-4},
				},
				Name:  "SubSpiralEntity-5",
				Flows: []util.DataFlow{},
			}},
		},
		// {
		// 	mashupsdk.MashupDetailedElement{
		// 		Id:          9,
		// 		State:       &mashupsdk.MashupElementState{Id: int64(9), State: int64(mashupsdk.Hidden)},
		// 		Name:        "SubSpiralEntity-" + strconv.Itoa(int(9)),
		// 		Alias:       "It",
		// 		Description: "",
		// 		Renderer:    "SubSpiral",
		// 		Genre:       "Abstract",
		// 		Subgenre:    "",
		// 		Parentids:   []int64{},
		// 		Childids:    []int64{-4},
		// 	},
		// 	"SubSpiralEntity-" + strconv.Itoa(int(9)),
		// 	[]util.DataFlowGroup{},
		// },
		{
			mashupsdk.MashupDetailedElement{
				Id:          4,
				State:       &mashupsdk.MashupElementState{Id: 4, State: int64(mashupsdk.Init)},
				Name:        "PathGroupOne",
				Description: "Paths",
				Renderer:    "Path",
				Genre:       "Collection",
				Subgenre:    "Path",
				Parentids:   []int64{},
				Childids:    []int64{7},
			},
			"PathGroupOne",
			[]util.DataFlowGroup{},
		},
	}
	totalElements := 10
	for totalElements = 10; totalElements < 20; totalElements = totalElements + 1 {
		argosyId := int64(8 + totalElements)
		Argosys = append(Argosys, util.Argosy{
			mashupsdk.MashupDetailedElement{
				Id:          argosyId,
				State:       &mashupsdk.MashupElementState{Id: argosyId, State: int64(mashupsdk.Init)},
				Name:        "PathEntity-" + strconv.Itoa(int(argosyId)),
				Alias:       "It",
				Description: "",
				Renderer:    "Path",
				Genre:       "Abstract",
				Subgenre:    "",
				Parentids:   []int64{},
				Childids:    []int64{-2, argosyId*100 + 1}, //, argosyId*100 + 1
			},
			"PathEntity-" + strconv.Itoa(int(argosyId)),
			[]util.DataFlowGroup{{
				MashupDetailedElement: mashupsdk.MashupDetailedElement{
					Id:          argosyId*100 + 1,
					State:       &mashupsdk.MashupElementState{Id: int64(argosyId*100 + 1), State: int64(mashupsdk.Hidden)},
					Name:        "SubSpiralEntity-" + strconv.Itoa(int(argosyId*100+1)),
					Alias:       "It",
					Description: "",
					Renderer:    "SubSpiral",
					Genre:       "Abstract",
					Subgenre:    "",
					Parentids:   []int64{},
					Childids:    []int64{-4},
				},
				Name:  "SubSpiralEntity-" + strconv.Itoa(int(argosyId*100+1)),
				Flows: []util.DataFlow{},
			}},
		})
	}

	return util.ArgosyFleet{
		ArgosyName: "Dev Environment",
		Argosies:   []util.Argosy(Argosys),
	}
}

func GetDataFlowGroups(mod *kv.Modifier, argosy *util.Argosy) []util.DataFlowGroup {
	return nil
}
