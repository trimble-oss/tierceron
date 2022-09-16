package data

import (
	"fmt"
	"log"
	"math"
	"sort"
	"strconv"
	"tierceron/buildopts/argosyopts"

	eUtils "tierceron/utils"
	helperkv "tierceron/vaulthelper/kv"

	"github.com/mrjrieke/nute/mashupsdk"
)

// Stub data
var data []string = []string{"UpdateBudget", "AddChangeOrder", "UpdateChangeOrder", "AddChangeOrderItem", "UpdateChangeOrderItem",
	"UpdateChangeOrderItemApprovalDate", "AddChangeOrderStatus", "UpdateChangeOrderStatus", "AddContract",
	"UpdateContract", "AddCustomer", "UpdateCustomer", "AddItemAddon", "UpdateItemAddon", "AddItemCost",
	"UpdateItemCost", "AddItemMarkup", "UpdateItemMarkup", "AddPhase", "UpdatePhase", "AddScheduleOfValuesFixedPrice",
	"UpdateScheduleOfValuesFixedPrice", "AddScheduleOfValuesUnitPrice", "UpdateScheduleOfValuesUnitPrice"}

//using tests from 8/24/22
var TimeData = map[string][]float64{
	data[0]:  {0.0, .650, .95, 5.13, 317.85, 317.85},
	data[1]:  {0.0, 0.3, 0.56, 5.06, 78.4, 78.4},
	data[2]:  {0.0, 0.2, 0.38, 5.33, 78.4, 78.4},
	data[3]:  {0.0, 0.34, 0.36, 5.25, 141.93, 141.93},
	data[4]:  {0.0, 0.24, 0.52, 4.87, 141.91, 141.91},
	data[5]:  {0.0, 0.24, 0.6, 5.39, 148.01, 148.01},
	data[6]:  {0.0, 0.11, 0.13, 4.89, 32.47, 32.47},
	data[7]:  {0.0, 0.08, 0.1, 4.82, 32.49, 32.49},
	data[8]:  {0.0, 0.33, 0.5, 5.21, 89.53, 89.53},
	data[9]:  {0.0, 0.3, 0.62, 5, 599.99},
	data[10]: {0.0, 0.19, 0.47, 4.87, 38.5, 38.5},
	data[11]: {0.0, 0.26, 0.58, 5, 39.08, 39.08},
	data[12]: {0.0, 0.36, 0.37, 5.32, 69.09, 69.06},
	data[13]: {0.0, 0.09, 0.13, 4.73, 164.1, 164.1},
	data[14]: {0.0, 0.61, 0.61, 0.92, 5.09, 108.35, 108.35},
	data[15]: {0.0, 0.48, 0.66, 5.02, 108.46, 108.46},
	data[16]: {0.0, 0.34, 0.36, 4.87, 53.42, 53.42},
	data[17]: {0.0, 0.14, 0.23, 5.11, 53.29, 53.29},
	data[18]: {0.0, 0.69, 0.88, 5.07, 102.38, 102.38},
	data[19]: {0.0, 0.73, 1.03, 5.01, 104.31, 104.31},
	data[20]: {0.0, 0.19, 0.22, 4.82, 218.8, 218.8},
	data[21]: {0.0, 0.19, 0.36, 5.21, 218.66, 218.66},
	data[22]: {0.0, 0.36, 0.41, 4.93, 273.66, 273.66},
	data[23]: {0.0, 0.22, 0.39, 4.87, 273.24, 273.24},
}

//Returns an array of mashup detailed elements populated with Argosy data
func GetData(insecure *bool, logger *log.Logger, envPtr *string) []*mashupsdk.MashupDetailedElement {
	config := eUtils.DriverConfig{Insecure: *insecure, Log: logger, ExitOnFailure: true}
	secretID := ""
	appRoleID := ""
	address := ""
	token := ""
	empty := ""

	autoErr := eUtils.AutoAuth(&config, &secretID, &appRoleID, &token, &empty, envPtr, &address, false)
	eUtils.CheckError(&config, autoErr, true)

	mod, modErr := helperkv.NewModifier(*insecure, token, address, *envPtr, nil, logger)
	mod.Env = *envPtr
	eUtils.CheckError(&config, modErr, true)
	ArgosyFleet, argosyErr := argosyopts.BuildFleet(mod, logger)
	eUtils.CheckError(&config, argosyErr, true)

	DetailedElements := []*mashupsdk.MashupDetailedElement{}
	for a := 0; a < len(ArgosyFleet.Argosies); a++ {
		argosyBasis := ArgosyFleet.Argosies[a].MashupDetailedElement
		argosyBasis.Alias = "Argosy"
		DetailedElements = append(DetailedElements, &argosyBasis)

		for i := 0; i < len(ArgosyFleet.Argosies[a].Groups); i++ {
			detailedElement := ArgosyFleet.Argosies[a].Groups[i].MashupDetailedElement
			detailedElement.Alias = "DataFlowGroup"
			DetailedElements = append(DetailedElements, &detailedElement)
			var quartiles []float64
			for j := 0; j < len(ArgosyFleet.Argosies[a].Groups[i].Flows); j++ {
				for k := 0; k < len(ArgosyFleet.Argosies[a].Groups[i].Flows[j].Statistics)-1; k++ {
					timeNanoSeconds := int64(ArgosyFleet.Argosies[a].Groups[i].Flows[j].Statistics[k].TimeSplit)
					timeSeconds := float64(timeNanoSeconds) * math.Pow(10.0, -9.0)
					nextTimeNanoSeconds := int64(ArgosyFleet.Argosies[a].Groups[i].Flows[j].Statistics[k+1].TimeSplit)
					nextTimeSeconds := float64(nextTimeNanoSeconds) * math.Pow(10.0, -9.0)
					if nextTimeSeconds-timeSeconds != 0 {
						quartiles = append(quartiles, nextTimeSeconds-timeSeconds)
					}
				}
			}
			sort.Float64s(quartiles)
			for j := 0; j < len(ArgosyFleet.Argosies[a].Groups[i].Flows); j++ {
				element := ArgosyFleet.Argosies[a].Groups[i].Flows[j].MashupDetailedElement
				element.Alias = "DataFlow"
				element.Data = fmt.Sprintf("%f", quartiles[len(quartiles)/4]) + "-" + fmt.Sprintf("%f", quartiles[len(quartiles)/2]) + "-" + fmt.Sprintf("%f", quartiles[3*len(quartiles)/4])
				for k := 0; k < len(ArgosyFleet.Argosies[a].Groups[i].Flows[j].Statistics); k++ {
					el := ArgosyFleet.Argosies[a].Groups[i].Flows[j].Statistics[k].MashupDetailedElement
					el.Alias = "DataFlowStatistic"
					timeNanoSeconds := int64(ArgosyFleet.Argosies[a].Groups[i].Flows[j].Statistics[k].TimeSplit)
					el.Data = strconv.FormatInt(timeNanoSeconds, 10)
					DetailedElements = append(DetailedElements, &el)
				}
				DetailedElements = append(DetailedElements, &element)
			}
		}

	}
	DetailedElements = append(DetailedElements, &mashupsdk.MashupDetailedElement{
		Basisid:        5,
		State:          &mashupsdk.MashupElementState{Id: 5, State: int64(mashupsdk.Mutable)},
		Name:           "SearchElement",
		Alias:          "It",
		Description:    "Search Element",
		Data:           "",
		Custosrenderer: "SearchRenderer",
		Renderer:       "",
		Genre:          "",
		Subgenre:       "Ento",
		Parentids:      nil,
		Childids:       []int64{},
	})
	return DetailedElements
}

//Returns an array of mashup detailed elements populated with stub data
func GetHeadlessData(insecure *bool, logger *log.Logger) []*mashupsdk.MashupDetailedElement {
	config := eUtils.DriverConfig{Insecure: *insecure, Log: logger, ExitOnFailure: true}
	ArgosyFleet, argosyErr := argosyopts.BuildFleet(nil, logger) //mod)
	eUtils.CheckError(&config, argosyErr, true)

	dfstatData := map[string]float64{}
	statGroup := []float64{}
	testTimes := []float64{}
	pointer := 0
	DetailedElements := []*mashupsdk.MashupDetailedElement{}
	for _, argosy := range ArgosyFleet.Argosies {
		argosyBasis := argosy.MashupDetailedElement
		argosyBasis.Alias = "Argosy"
		DetailedElements = append(DetailedElements, &argosyBasis)
		for i := 0; i < len(argosy.Groups); i++ {
			detailedElement := argosy.Groups[i].MashupDetailedElement
			detailedElement.Alias = "DataFlowGroup"
			DetailedElements = append(DetailedElements, &detailedElement)
			for j := 0; j < len(argosy.Groups[i].Flows); j++ {
				element := argosy.Groups[i].Flows[j].MashupDetailedElement
				element.Alias = "DataFlow"
				DetailedElements = append(DetailedElements, &element)
				if pointer < len(data)-1 {
					pointer += 1
				} else {
					pointer = 0
				}
				for k := 0; k < len(TimeData[data[pointer]]); k++ {
					el := argosy.Groups[i].Flows[j].Statistics[k].MashupDetailedElement
					el.Alias = "DataFlowStatistic"
					timeSeconds := TimeData[data[pointer]][k]
					dfstatData[el.Name] = timeSeconds
					statGroup = append(statGroup, timeSeconds)
					DetailedElements = append(DetailedElements, &el)
				}
				for l := 0; l < len(statGroup)-1; l++ {
					if statGroup[l+1]-statGroup[l] > 0 {
						testTimes = append(testTimes, statGroup[l+1]-statGroup[l])
					}
				}
			}
		}
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
		Genre:          "",
		Subgenre:       "Ento",
		Parentids:      nil,
		Childids:       []int64{},
	})
	sort.Float64s(testTimes)
	return DetailedElements
}
