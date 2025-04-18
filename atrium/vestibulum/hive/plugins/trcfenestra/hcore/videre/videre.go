package videre

import (
	"log"
	"math"
	"sort"
	"strconv"

	eUtils "github.com/trimble-oss/tierceron/pkg/utils"

	"github.com/trimble-oss/tierceron-nute-core/mashupsdk"
	"github.com/trimble-oss/tierceron/atrium/buildopts/argosyopts"
	"github.com/trimble-oss/tierceron/pkg/core"
)

// TODO: update to do the thing...
func GetHeadlessData(logger *log.Logger) []*mashupsdk.MashupDetailedElement {
	data, TimeData := argosyopts.GetStubbedDataFlowStatistics()

	config := &core.CoreConfig{
		ExitOnFailure: true,
		Log:           logger,
	}
	ArgosyFleet, argosyErr := argosyopts.BuildFleet(nil, logger)
	eUtils.CheckError(config, argosyErr, true)

	dfstatData := map[string]float64{}
	statGroup := []float64{}
	testTimes := []float64{}
	pointer := 0
	var maxTime int64 = 0
	DetailedElements := []*mashupsdk.MashupDetailedElement{}
	for m := 0; m < len(ArgosyFleet.ChildNodes); m++ {
		argosy := ArgosyFleet.ChildNodes[m]
		argosyBasis := argosy.MashupDetailedElement
		argosyBasis.Alias = "Argosy"

		for i := 0; i < len(argosy.ChildNodes); i++ {
			detailedElement := argosy.ChildNodes[i].MashupDetailedElement
			detailedElement.Alias = "DataFlowGroup"
			DetailedElements = append(DetailedElements, detailedElement)
			for j := 0; j < len(argosy.ChildNodes[i].ChildNodes); j++ {
				element := argosy.ChildNodes[i].ChildNodes[j].MashupDetailedElement
				element.Alias = "DataFlow"
				DetailedElements = append(DetailedElements, element)
				if pointer < len(data)-1 {
					pointer += 1
				} else {
					pointer = 0
				}
				for k := 0; k < len(TimeData[data[pointer]]) && k < len(argosy.ChildNodes[i].ChildNodes[j].ChildNodes); k++ {
					el := argosy.ChildNodes[i].ChildNodes[j].ChildNodes[k].MashupDetailedElement
					el.Alias = "DataFlowStatistic"
					timeSeconds := TimeData[data[pointer]][k]
					if maxTime < int64(timeSeconds*math.Pow(10.0, 9.0)) {
						maxTime = int64(timeSeconds * math.Pow(10.0, 9.0))
					}
					dfstatData[el.Name] = timeSeconds
					statGroup = append(statGroup, timeSeconds)
					DetailedElements = append(DetailedElements, el)
				}
				for l := 0; l < len(statGroup)-1; l++ {
					if statGroup[l+1]-statGroup[l] > 0 {
						testTimes = append(testTimes, statGroup[l+1]-statGroup[l])
					}
				}
			}
		}
		if m == len(ArgosyFleet.ChildNodes)-1 {
			argosyBasis.Data = strconv.Itoa(int(maxTime))

		}
		DetailedElements = append(DetailedElements, argosyBasis)
	}

	DetailedElements = append(DetailedElements, &mashupsdk.MashupDetailedElement{
		Basisid:        5,
		State:          &mashupsdk.MashupElementState{Id: 5, State: int64(mashupsdk.Mutable)},
		Name:           "SearchElement",
		Alias:          "SearchElement",
		Description:    "Search Element",
		Data:           "",
		Custosrenderer: "SearchRenderer",
		Renderer:       "",
		Genre:          "SearchElement",
		Subgenre:       "Ento",
		Parentids:      nil,
		Childids:       []int64{},
	})
	sort.Float64s(testTimes)
	return DetailedElements
}
