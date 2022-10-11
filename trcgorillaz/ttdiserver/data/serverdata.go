package data

import (
	"encoding/json"
	//"fmt"
	"log"
	"math"
	"sort"
	"strconv"
	"tierceron/buildopts/argosyopts"

	eUtils "tierceron/utils"
	helperkv "tierceron/vaulthelper/kv"

	"github.com/mrjrieke/nute/mashupsdk"
)

var maxTime int64

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
	DetailedElements = append(DetailedElements, &ArgosyFleet.MashupDetailedElement)
	var quartiles []float64
	maxTime := 0
	for a := 0; a < len(ArgosyFleet.ChildNodes); a++ {
		//check genre 
		
		argosyElement := ArgosyFleet.ChildNodes[a].MashupDetailedElement
		//argosyBasis.Alias = "Argosy"
		if argosyElement.Genre != "Argosy" {
			DetailedElements = append(DetailedElements, &argosyElement)
		} else {
			for i := 0; i < len(ArgosyFleet.ChildNodes[a].ChildNodes); i++ {
				dfgElement := ArgosyFleet.ChildNodes[a].ChildNodes[i].MashupDetailedElement
				//detailedElement.Alias = "DataFlowGroup"
				DetailedElements = append(DetailedElements, &dfgElement)
	
				for j := 0; j < len(ArgosyFleet.ChildNodes[a].ChildNodes[i].ChildNodes); j++ {
					dfelement := ArgosyFleet.ChildNodes[a].ChildNodes[i].ChildNodes[j].MashupDetailedElement
					DetailedElements = append(DetailedElements, &dfelement)
					for k := 0; k < len(ArgosyFleet.ChildNodes[a].ChildNodes[i].ChildNodes[j].ChildNodes)-1; k++ {
						stat := ArgosyFleet.ChildNodes[a].ChildNodes[i].ChildNodes[j].ChildNodes[k].MashupDetailedElement
						DetailedElements = append(DetailedElements, &stat)
						var decodedstat interface{}
						err := json.Unmarshal([]byte(stat.Data), &decodedstat)
						if err != nil {
							log.Println("Error in decoding data in buildDataFlowStatistics")
							break
						}
						decodedStatData := decodedstat.(map[string]interface{})
						timeNanoSeconds := int64(decodedStatData["TimeSplit"].(float64))
						if timeNanoSeconds > int64(maxTime) {
							maxTime = int(timeNanoSeconds)
						}
						timeSeconds := float64(timeNanoSeconds) * math.Pow(10.0, -9.0)
						nextStat := ArgosyFleet.ChildNodes[a].ChildNodes[i].ChildNodes[j].ChildNodes[k+1].MashupDetailedElement
						if j == len(ArgosyFleet.ChildNodes[a].ChildNodes[i].ChildNodes[j].ChildNodes)-2 {
							DetailedElements = append(DetailedElements, &nextStat)
						}
						var nextdecodedstat interface{}
						err = json.Unmarshal([]byte(nextStat.Data), &nextdecodedstat)
						if err != nil {
							log.Println("Error in decoding data in GetData")
							continue
						}
						nextDecodedStatData := nextdecodedstat.(map[string]interface{})
						nextTimeNanoSeconds := int64(nextDecodedStatData["TimeSplit"].(float64))
						nextTimeSeconds := float64(nextTimeNanoSeconds) * math.Pow(10.0, -9.0)
						if nextTimeSeconds-timeSeconds > 0 {
							quartiles = append(quartiles, nextTimeSeconds-timeSeconds)
						}
					}
				}
				sort.Float64s(quartiles)
				for j := len(DetailedElements) - 1; j >= 0; j-- {
					if DetailedElements[j].Genre == "DataFlow" {
						var newQuartiles []float64
						var decoded interface{}
						err := json.Unmarshal([]byte(DetailedElements[j].Data), &decoded)
						if err != nil {
							log.Println("Error in decoding data in GetData")
							continue
						}
						decodedData := decoded.(map[string]interface{})
						newQuartiles = append(newQuartiles, quartiles[len(quartiles)/4])
						//newQuartiles[0] = quartiles[len(quartiles)/4]
						newQuartiles = append(newQuartiles, quartiles[len(quartiles)/2])
						//newQuartiles[1] = quartiles[len(quartiles)/2]
						newQuartiles = append(newQuartiles, quartiles[(3*len(quartiles))/4])
						//newQuartiles[2] = quartiles[(3*len(quartiles))/4]
						decodedData["Quartiles"] = newQuartiles
						encoded, err := json.Marshal(&decodedData)
						if err != nil {
							log.Println("Error in encoding data in GetData")
						}
						DetailedElements[j].Data = string(encoded)
						break
					}
				}
				var decoded interface{}
				err := json.Unmarshal([]byte(argosyElement.Data), &decoded)
				if err != nil {
					log.Println("Error in decoding data in GetData")
					continue
				}
				decodedData := decoded.(map[string]interface{})
				decodedData["Quartiles"] = quartiles
				decodedData["MaxTime"] = maxTime
				encoded, err := json.Marshal(&decodedData)
				if err != nil {
					log.Println("Error in encoding data in GetData")
				}
				argosyElement.Data = string(encoded)
				DetailedElements = append(DetailedElements, &argosyElement)
	
		}
					//DetailedElements[j].Data = string(encoded)
			// for j := 0; j < len(ArgosyFleet.ChildNodes[a].ChildNodes[i].ChildNodes); j++ {
			// 	element := ArgosyFleet.ChildNodes[a].ChildNodes[i].ChildNodes[j].MashupDetailedElement
			// 	//element.Alias = "DataFlow"
			// 	element.Data = fmt.Sprintf("%f", quartiles[len(quartiles)/4]) + "-" + fmt.Sprintf("%f", quartiles[len(quartiles)/2]) + "-" + fmt.Sprintf("%f", quartiles[3*len(quartiles)/4])
			// 	for k := 0; k < len(ArgosyFleet.ChildNodes[a].ChildNodes[i].ChildNodes[j].ChildNodes); k++ {
			// 		el := ArgosyFleet.ChildNodes[a].ChildNodes[i].ChildNodes[j].ChildNodes[k].MashupDetailedElement
			// 		//el.Alias = "DataFlowStatistic"
			// 		stat := ArgosyFleet.ChildNodes[a].ChildNodes[i].ChildNodes[j].ChildNodes[k]
			// 		var decodedstat interface{}
			// 		err := json.Unmarshal([]byte(stat.MashupDetailedElement.Data), &decodedstat)
			// 		if err != nil {
			// 			log.Println("Error in decoding data in buildDataFlowStatistics")
			// 			break
			// 		}
			// 		decodedStatData := decodedstat.(map[string]interface{})
			// 		timeNanoSeconds := int64(decodedStatData["TimeSplit"].(float64))
			// 		el.Data = strconv.FormatInt(timeNanoSeconds, 10)
			// 		DetailedElements = append(DetailedElements, &el)
			// 	}
			// 	DetailedElements = append(DetailedElements, &element)
			// }
		}

		// var decoded interface{}
		// err := json.Unmarshal([]byte(argosyElement.Data), &decoded)
		// if err != nil {
		// 	log.Println("Error in decoding data in GetData")
		// }
		// decodedData := decoded.(map[string]interface{})
		// decodedData["Quartiles"] = quartiles
		//encoded, err := json.Marshal(&decodedData)
		// if err != nil {
		// 	log.Println("Error in encoding data in GetData")
		// }
		//DetailedElements[j].Data = string(encoded)
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
	maxTime = 0
	DetailedElements := []*mashupsdk.MashupDetailedElement{}
	for m := 0; m < len(ArgosyFleet.ChildNodes); m++ { //_, argosy := range ArgosyFleet.ChildNodes {
		argosy := ArgosyFleet.ChildNodes[m]
		argosyBasis := argosy.MashupDetailedElement
		argosyBasis.Alias = "Argosy"

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
					if maxTime < int64(timeSeconds*math.Pow(10.0, 9.0)) {
						maxTime = int64(timeSeconds * math.Pow(10.0, 9.0))
					}
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
		if m == len(ArgosyFleet.ChildNodes)-1 {
			argosyBasis.Data = strconv.Itoa(int(maxTime))

		}
		DetailedElements = append(DetailedElements, &argosyBasis)
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
