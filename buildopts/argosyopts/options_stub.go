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
	Argosys := []*util.Argosy{
		{
			mashupsdk.MashupDetailedElement{
				Id:          6,
				State:       &mashupsdk.MashupElementState{Id: 6, State: int64(mashupsdk.Init)},
				Name:        "Outside",
				Alias:       "Outside",
				Description: "",
				Renderer:    "Background",
				Genre:       "Space",
				Subgenre:    "Exo",
				Parentids:   nil,
				Childids:    nil,
			},
			[]util.DataFlowGroup{},
		},
		{
			mashupsdk.MashupDetailedElement{
				Basisid:     -4,
				State:       &mashupsdk.MashupElementState{Id: -4, State: int64(mashupsdk.Mutable)},
				Name:        "SubSpiral",
				Alias:       "It",
				Description: "",
				Renderer:    "SubSpiral",
				//Colabrenderer: "Path",
				Genre:     "Solid",
				Subgenre:  "Skeletal", //if you want clickable: "Ento",
				Parentids: []int64{},
				Childids:  []int64{5},
			},
			[]util.DataFlowGroup{},
		},
		{
			mashupsdk.MashupDetailedElement{
				Id:          3,
				State:       &mashupsdk.MashupElementState{Id: 3, State: int64(mashupsdk.Init)},
				Name:        "SubSpiralGroupOne",
				Description: "SubSpirals",
				Renderer:    "SubSpiral",
				//Colabrenderer: "Path",
				Genre:     "Collection",
				Subgenre:  "SubSpiral",
				Parentids: nil,        //[]int64{},
				Childids:  []int64{5}, //NOTE: If you want to add all children need to include children in for loop!
			},
			[]util.DataFlowGroup{},
		},
		{
			mashupsdk.MashupDetailedElement{
				Id:          5,
				State:       &mashupsdk.MashupElementState{Id: int64(5), State: int64(mashupsdk.Init)},
				Name:        "SubSpiralEntity-" + strconv.Itoa(1),
				Alias:       "It",
				Description: "",
				Renderer:    "SubSpiral",
				Genre:       "Abstract",
				Subgenre:    "",
				Parentids:   []int64{},
				Childids:    []int64{-4}, // -3 -- generated and replaced by server since it is immutable.
			},
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
			[]util.DataFlowGroup{},
		},
		{
			mashupsdk.MashupDetailedElement{
				Id:            1,
				State:         &mashupsdk.MashupElementState{Id: 1, State: int64(mashupsdk.Init)},
				Name:          "CurvePathEntity-One",
				Description:   "",
				Renderer:      "Curve",
				Colabrenderer: "Path",
				Genre:         "Abstract",
				Subgenre:      "",
				Parentids:     nil,         //[]int64{10},
				Childids:      []int64{-1}, // -3 -- generated and replaced by server since it is immutable.
			},
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
				Parentids:     nil,        //[]int64{},
				Childids:      []int64{1}, //NOTE: If you want to add all children need to include children in for loop!
			},
			[]util.DataFlowGroup{},
		},
		{
			mashupsdk.MashupDetailedElement{
				Basisid:     -2,
				State:       &mashupsdk.MashupElementState{Id: -2, State: int64(mashupsdk.Mutable)},
				Name:        "{0}-Path",
				Alias:       "It",
				Description: "",
				Renderer:    "Path",
				Genre:       "Solid",
				Subgenre:    "Ento",
				Parentids:   []int64{},
				Childids:    []int64{3},
			},
			[]util.DataFlowGroup{},
		},
		{
			mashupsdk.MashupDetailedElement{
				Id:          7,
				State:       &mashupsdk.MashupElementState{Id: 7, State: int64(mashupsdk.Init)},
				Name:        "PathEntity-1",
				Alias:       "It",
				Description: "",
				Renderer:    "Path",
				Genre:       "Abstract",
				Subgenre:    "",
				Parentids:   nil,         //[]int64{10},
				Childids:    []int64{-2}, // -3 -- generated and replaced by server since it is immutable.
			},
			[]util.DataFlowGroup{},
		},
		{
			mashupsdk.MashupDetailedElement{
				Id:          4,
				State:       &mashupsdk.MashupElementState{Id: 4, State: int64(mashupsdk.Init)},
				Name:        "PathGroupOne",
				Description: "Paths",
				Renderer:    "Path",
				Genre:       "Collection",
				Subgenre:    "Path",
				Parentids:   []int64{},  //[]int64{},
				Childids:    []int64{7}, //NOTE: If you want to add all children need to include children in for loop!
			},
			[]util.DataFlowGroup{},
		},
	}
	totalElements := 0
	for totalElements = 0; totalElements < 100; totalElements++ {
		Argosys = append(Argosys, &util.Argosy{
			mashupsdk.MashupDetailedElement{
				Id:          int64(8 + totalElements),
				State:       &mashupsdk.MashupElementState{Id: int64(8 + totalElements), State: int64(mashupsdk.Init)},
				Name:        "PathEntity-" + strconv.Itoa(2+totalElements),
				Alias:       "It",
				Description: "",
				Renderer:    "Path",
				Genre:       "Abstract",
				Subgenre:    "",
				Parentids:   []int64{},
				Childids:    []int64{-2}, // -3 -- generated and replaced by server since it is immutable.
			},
			[]util.DataFlowGroup{},
		})
	}

	for i := totalElements + 1; i < totalElements+150; i++ {
		Argosys = append(Argosys, &util.Argosy{
			mashupsdk.MashupDetailedElement{
				Id:          int64(1000 + totalElements + i),
				State:       &mashupsdk.MashupElementState{Id: int64(1000 + totalElements + i), State: int64(mashupsdk.Init)},
				Name:        "SubSpiralEntity-" + strconv.Itoa(2+i),
				Alias:       "It",
				Description: "",
				Renderer:    "SubSpiral",
				Genre:       "Abstract",
				Subgenre:    "",
				Parentids:   []int64{},
				Childids:    []int64{-4}, // -3 -- generated and replaced by server since it is immutable.
			},
			[]util.DataFlowGroup{},
		})
	}

	return util.ArgosyFleet{
		Fleet: Argosys,
		Name:  "Dev Environment",
	}
}

func GetDataFlowGroups(mod *kv.Modifier, argosy *util.Argosy) []util.DataFlowGroup {
	return nil
}
