package data

import (
	"encoding/json"
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
	for a := 0; a < len(ArgosyFleet.ChildNodes); a++ {
		argosyBasis := ArgosyFleet.ChildNodes[a].MashupDetailedElement
		argosyBasis.Alias = "Argosy"
		DetailedElements = append(DetailedElements, &argosyBasis)

		for i := 0; i < len(ArgosyFleet.ChildNodes[a].ChildNodes); i++ {
			detailedElement := ArgosyFleet.ChildNodes[a].ChildNodes[i].MashupDetailedElement
			detailedElement.Alias = "DataFlowGroup"
			DetailedElements = append(DetailedElements, &detailedElement)
			var quartiles []float64
			for j := 0; j < len(ArgosyFleet.ChildNodes[a].ChildNodes[i].ChildNodes); j++ {
				for k := 0; k < len(ArgosyFleet.ChildNodes[a].ChildNodes[i].ChildNodes[j].ChildNodes)-1; k++ {
					stat := ArgosyFleet.ChildNodes[a].ChildNodes[i].ChildNodes[j].ChildNodes[k]
					var decodedstat interface{}
					err := json.Unmarshal(stat.Data, &decodedstat)
					if err != nil {
						log.Println("Error in decoding data in buildDataFlowStatistics")
						break
					}
					decodedStatData := decodedstat.(map[string]interface{})
					timeNanoSeconds := int64(decodedStatData["TimeSplit"].(float64))
					timeSeconds := float64(timeNanoSeconds) * math.Pow(10.0, -9.0)
					nextStat := ArgosyFleet.ChildNodes[a].ChildNodes[i].ChildNodes[j].ChildNodes[k+1]
					var nextdecodedstat interface{}
					err = json.Unmarshal(nextStat.Data, &nextdecodedstat)
					if err != nil {
						log.Println("Error in decoding data in buildDataFlowStatistics")
						break
					}
					nextDecodedStatData := nextdecodedstat.(map[string]interface{})
					nextTimeNanoSeconds := int64(nextDecodedStatData["TimeSplit"].(float64))
					nextTimeSeconds := float64(nextTimeNanoSeconds) * math.Pow(10.0, -9.0)
					if nextTimeSeconds-timeSeconds != 0 {
						quartiles = append(quartiles, nextTimeSeconds-timeSeconds)
					}
				}
			}
			sort.Float64s(quartiles)
			for j := 0; j < len(ArgosyFleet.ChildNodes[a].ChildNodes[i].ChildNodes); j++ {
				element := ArgosyFleet.ChildNodes[a].ChildNodes[i].ChildNodes[j].MashupDetailedElement
				element.Alias = "DataFlow"
				element.Data = fmt.Sprintf("%f", quartiles[len(quartiles)/4]) + "-" + fmt.Sprintf("%f", quartiles[len(quartiles)/2]) + "-" + fmt.Sprintf("%f", quartiles[3*len(quartiles)/4])
				for k := 0; k < len(ArgosyFleet.ChildNodes[a].ChildNodes[i].ChildNodes[j].ChildNodes); k++ {
					el := ArgosyFleet.ChildNodes[a].ChildNodes[i].ChildNodes[j].ChildNodes[k].MashupDetailedElement
					el.Alias = "DataFlowStatistic"
					stat := ArgosyFleet.ChildNodes[a].ChildNodes[i].ChildNodes[j].ChildNodes[k]
					var decodedstat interface{}
					err := json.Unmarshal(stat.Data, &decodedstat)
					if err != nil {
						log.Println("Error in decoding data in buildDataFlowStatistics")
						break
					}
					decodedStatData := decodedstat.(map[string]interface{})
					timeNanoSeconds := int64(decodedStatData["TimeSplit"].(float64))
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
	data, TimeData := argosyopts.GetStubbedDataFlowStatistics()

	config := eUtils.DriverConfig{Insecure: *insecure, Log: logger, ExitOnFailure: true}
	ArgosyFleet, argosyErr := argosyopts.BuildFleet(nil, logger) //mod)
	eUtils.CheckError(&config, argosyErr, true)

	dfstatData := map[string]float64{}
	statGroup := []float64{}
	testTimes := []float64{}
	pointer := 0
	DetailedElements := []*mashupsdk.MashupDetailedElement{}
	for _, argosy := range ArgosyFleet.ChildNodes {
		argosyBasis := argosy.MashupDetailedElement
		argosyBasis.Alias = "Argosy"
		DetailedElements = append(DetailedElements, &argosyBasis)
		for i := 0; i < len(argosy.ChildNodes); i++ {
			detailedElement := argosy.ChildNodes[i].MashupDetailedElement
			detailedElement.Alias = "DataFlowGroup"
			DetailedElements = append(DetailedElements, &detailedElement)
			for j := 0; j < len(argosy.ChildNodes[i].ChildNodes); j++ {
				element := argosy.ChildNodes[i].ChildNodes[j].MashupDetailedElement
				element.Alias = "DataFlow"
				DetailedElements = append(DetailedElements, &element)
				if pointer < len(data)-1 {
					pointer += 1
				} else {
					pointer = 0
				}
				for k := 0; k < len(TimeData[data[pointer]]); k++ {
					el := argosy.ChildNodes[i].ChildNodes[j].ChildNodes[k].MashupDetailedElement
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
