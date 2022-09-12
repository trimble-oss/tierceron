package data

import (
	"fmt"
	"log"
	"math"
	"sort"
	"strconv"
	"tierceron/buildopts/argosyopts"

	//"tierceron/trcgorillaz/trcserver/ttdisupport"

	eUtils "tierceron/utils"
	helperkv "tierceron/vaulthelper/kv"

	//"fyne.io/fyne/widget"

	"github.com/mrjrieke/nute/mashupsdk"
)

//Can't find way to communicate options_stub and main method to get access to data bc no time stat available\
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
	ArgosyFleet, argosyErr := argosyopts.BuildFleet(mod)
	eUtils.CheckError(&config, argosyErr, true)

	DetailedElements := []*mashupsdk.MashupDetailedElement{}
	//dfstatData := map[string]float64{}
	statGroup := []float64{}
	testTimes := []float64{}
	for a := 0; a < len(ArgosyFleet.Argosies); a++ {
		argosyBasis := ArgosyFleet.Argosies[a].MashupDetailedElement
		argosyBasis.Alias = "Argosy"
		DetailedElements = append(DetailedElements, &argosyBasis)

		for i := 0; i < len(ArgosyFleet.Argosies[a].Groups); i++ {
			detailedElement := ArgosyFleet.Argosies[a].Groups[i].MashupDetailedElement
			detailedElement.Alias = "DataFlowGroup"
			DetailedElements = append(DetailedElements, &detailedElement)
			for j := 0; j < len(ArgosyFleet.Argosies[a].Groups[i].Flows); j++ {
				element := ArgosyFleet.Argosies[a].Groups[i].Flows[j].MashupDetailedElement
				element.Alias = "DataFlow"
				total := 0.0
				for k := 0; k < len(ArgosyFleet.Argosies[a].Groups[i].Flows[j].Statistics); k++ {
					el := ArgosyFleet.Argosies[a].Groups[i].Flows[j].Statistics[k].MashupDetailedElement
					el.Alias = "DataFlowStatistic"
					timeNanoSeconds := int64(ArgosyFleet.Argosies[a].Groups[i].Flows[j].Statistics[k].TimeSplit)
					timeSeconds := float64(timeNanoSeconds) * math.Pow(10.0, -9.0)
					total += timeSeconds
					el.Data = strconv.FormatInt(timeNanoSeconds, 10) //time in nanoseconds
					statGroup = append(statGroup, timeSeconds)
					DetailedElements = append(DetailedElements, &el)
				}
				element.Data = fmt.Sprintf("%f", total/float64(len(ArgosyFleet.Argosies[a].Groups[i].Flows[j].Statistics)))
				DetailedElements = append(DetailedElements, &element)
				for l := 0; l < len(statGroup)-1; l++ {
					if statGroup[l+1]-statGroup[l] > 0 {
						testTimes = append(testTimes, statGroup[l+1]-statGroup[l])
					}
				}

			}
		}
	}
	sort.Float64s(testTimes)
	return DetailedElements
}

func GetHeadlessData(insecure *bool, logger *log.Logger) []*mashupsdk.MashupDetailedElement {
	config := eUtils.DriverConfig{Insecure: *insecure, Log: logger, ExitOnFailure: true}
	ArgosyFleet, argosyErr := argosyopts.BuildFleet(nil) //mod)
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
				for k := 0; k < len(TimeData[data[pointer]]); k++ { //argosy.Groups[i].Flows[j].Statistics
					el := argosy.Groups[i].Flows[j].Statistics[k].MashupDetailedElement
					el.Alias = "DataFlowStatistic"
					//timeNanoSeconds := int64(argosy.Groups[i].Flows[j].Statistics[k].TimeSplit) //change this so it iterates over global data
					timeSeconds := TimeData[data[pointer]][k] //float64(timeNanoSeconds) * math.Pow(10.0, -9.0)
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
	sort.Float64s(testTimes)
	return DetailedElements
}
