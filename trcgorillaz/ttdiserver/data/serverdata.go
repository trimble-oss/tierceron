package data

import (
	"encoding/json"
	"fmt"
	"log"
	"math"
	"sort"
	"strconv"
	"tierceron/buildopts/argosyopts"
	flowcore "tierceron/trcflow/core"

	eUtils "tierceron/utils"
	helperkv "tierceron/vaulthelper/kv"

	"github.com/mrjrieke/nute/mashupsdk"
)

var maxTime int64
var avg float64
var idForData int
var count int

func createDetailedElements(detailedElements []*mashupsdk.MashupDetailedElement, node flowcore.TTDINode, testTimes []float64, depth int) ([]*mashupsdk.MashupDetailedElement, []float64) {
	//testTimes := []float64{}
	//quartiles := []float64{}
	//idForData := 0
	//avg := 0.0
	//count := 0
	// if node.MashupDetailedElement.Id == 2 {
	// 	idForData = len(detailedElements) - 1
	// }
	//fail := false
	for _, child_node := range node.ChildNodes {
		if child_node.MashupDetailedElement.Genre == "DataFlowStatistic" {
			node.MashupDetailedElement.Genre = "DataFlow"
			var decoded_child_node interface{}
			var decodedChildNodeData map[string]interface{}
			if child_node.MashupDetailedElement.Data != "" {
				err := json.Unmarshal([]byte(child_node.MashupDetailedElement.Data), &decoded_child_node)
				if err != nil {
					log.Println("Error in decoding data in recursiveBuildArgosies")
					//return nil
				}
				decodedChildNodeData = decoded_child_node.(map[string]interface{})
			}
			if decodedChildNodeData["Mode"] != nil {
				// mode := decodedChildNodeData["Mode"].(float64)
				// if mode == 2 {
				// 	fail = true
				// }
			}
			for i := 0; i < len(node.ChildNodes)-1; i++ {
				stat := node.ChildNodes[i].MashupDetailedElement
				if stat.State == nil {
					stat.State = &mashupsdk.MashupElementState{Id: stat.Id, State: int64(mashupsdk.Init)}
				}
				detailedElements = append(detailedElements, &stat)
				var decodedstat interface{}
				err := json.Unmarshal([]byte(stat.Data), &decodedstat)
				if err != nil {
					log.Println("Error in decoding data in buildDataFlowStatistics")
				}
				decodedStatData := decodedstat.(map[string]interface{})
				timeNanoSeconds := int64(decodedStatData["TimeSplit"].(float64))
				if timeNanoSeconds > int64(maxTime) {
					maxTime = timeNanoSeconds
				}
				timeSeconds := float64(timeNanoSeconds) * math.Pow(10.0, -9.0)
				nextStat := node.ChildNodes[i+1].MashupDetailedElement
				if i == len(node.ChildNodes)-2 {
					detailedElements = append(detailedElements, &nextStat)
				}
				var nextdecodedstat interface{}
				err = json.Unmarshal([]byte(nextStat.Data), &nextdecodedstat)
				if err != nil {
					log.Println("Error in decoding data in GetData")
				}
				nextDecodedStatData := nextdecodedstat.(map[string]interface{})
				nextTimeNanoSeconds := int64(nextDecodedStatData["TimeSplit"].(float64))
				nextTimeSeconds := float64(nextTimeNanoSeconds) * math.Pow(10.0, -9.0)
				if nextTimeSeconds-timeSeconds >= 0 {
					testTimes = append(testTimes, nextTimeSeconds-timeSeconds)
				} else {
					avg += timeSeconds
					count++
				}
			}
			break
		}
	}
	if node.MashupDetailedElement.Genre == "DataFlow" {
		// var dfdecoded interface{}
		// dfdecodedData := make(map[string]interface{})
		// if node.MashupDetailedElement.Data != "" {
		// 	err := json.Unmarshal([]byte(node.MashupDetailedElement.Data), &dfdecoded)
		// 	if err != nil {
		// 		log.Println("Error in decoding data in GetData")
		// 	}
		// 	dfdecodedData = dfdecoded.(map[string]interface{})
		// }
		// dfdecodedData["Fail"] = fail
		// encoded, err := json.Marshal(&dfdecodedData)
		// if err != nil {
		// 	log.Println("Error in encoding data in GetData")
		// }
		// node.MashupDetailedElement.Data = string(encoded)
	}
	node.MashupDetailedElement.Alias = strconv.Itoa(depth)
	if node.MashupDetailedElement.Id != 0 {
		detailedElements = append(detailedElements, &node.MashupDetailedElement)
		if node.MashupDetailedElement.Id == 2 {
			idForData = len(detailedElements) - 1
		}
	}

	for i := 0; i < len(node.ChildNodes); i++ {
		detailedElements, testTimes = createDetailedElements(detailedElements, node.ChildNodes[i], testTimes, depth+1)
	}
	return detailedElements, testTimes
}

// Returns an array of mashup detailed elements populated with Argosy data
func GetData(insecure *bool, logger *log.Logger, envPtr *string) []*mashupsdk.MashupDetailedElement {
	config := eUtils.DriverConfig{Insecure: *insecure, Log: logger, ExitOnFailure: true}
	secretID := ""
	appRoleID := ""
	address := ""
	token := ""
	empty := ""

	autoErr := eUtils.AutoAuth(&config, &secretID, &appRoleID, &token, &empty, envPtr, &address, false)
	eUtils.CheckError(&config, autoErr, true)

	mod, modErr := helperkv.NewModifier(*insecure, token, address, *envPtr, nil, true, logger)
	mod.Direct = true
	mod.Env = *envPtr
	eUtils.CheckError(&config, modErr, true)
	logger.Printf("Building fleet.\n")
	ArgosyFleet, argosyErr := argosyopts.BuildFleet(mod, logger)
	eUtils.CheckError(&config, argosyErr, true)
	logger.Printf("Fleet built.\n")

	DetailedElements := []*mashupsdk.MashupDetailedElement{}
	//DetailedElements = append(DetailedElements, &ArgosyFleet.MashupDetailedElement)
	//var quartiles []float64
	maxTime = 0
	testTimes := []float64{}
	quartiles := []float64{}
	idForData = 0
	avg = 0.0
	count = 0
	DetailedElements, testTimes = createDetailedElements(DetailedElements, ArgosyFleet, testTimes, 0)

	// for a := 0; a < len(ArgosyFleet.ChildNodes); a++ {
	// 	//check genre
	// 	argosyElement := ArgosyFleet.ChildNodes[a].MashupDetailedElement
	// 	DetailedElements = append(DetailedElements, &argosyElement)
	// 	if argosyElement.Id == 2 {
	// 		if idForData == 0 {
	// 			idForData = len(DetailedElements) - 1
	// 		}
	// 	}
	// 	if argosyElement.Genre == "Argosy" {
	// 		//newQuartiles := []float64{}
	// 		//argosyElement.Alias = "Argosy"

	// 		DetailedElements = append(DetailedElements, &argosyElement)
	// 		for i := 0; i < len(ArgosyFleet.ChildNodes[a].ChildNodes); i++ {
	// 			dfgElement := ArgosyFleet.ChildNodes[a].ChildNodes[i].MashupDetailedElement
	// 			dfgElement.Alias = "DataFlowGroup"
	// 			DetailedElements = append(DetailedElements, &dfgElement)
	// 			for j := 0; j < len(ArgosyFleet.ChildNodes[a].ChildNodes[i].ChildNodes); j++ {
	// 				dfelement := ArgosyFleet.ChildNodes[a].ChildNodes[i].ChildNodes[j].MashupDetailedElement
	// 				dfelement.Alias = "DataFlow"
	// 				DetailedElements = append(DetailedElements, &dfelement)
	// 				//Only part that matters for recursive loop
	// 				for k := 0; k < len(ArgosyFleet.ChildNodes[a].ChildNodes[i].ChildNodes[j].ChildNodes)-1; k++ {
	// 					stat := ArgosyFleet.ChildNodes[a].ChildNodes[i].ChildNodes[j].ChildNodes[k].MashupDetailedElement
	// 					stat.Alias = "DataFlowStatistic"
	// 					if stat.State == nil {
	// 						stat.State = &mashupsdk.MashupElementState{Id: stat.Id, State: int64(mashupsdk.Init)}
	// 					}
	// 					DetailedElements = append(DetailedElements, &stat)
	// 					var decodedstat interface{}
	// 					err := json.Unmarshal([]byte(stat.Data), &decodedstat)
	// 					if err != nil {
	// 						log.Println("Error in decoding data in buildDataFlowStatistics")
	// 						break
	// 					}
	// 					decodedStatData := decodedstat.(map[string]interface{})
	// 					timeNanoSeconds := int64(decodedStatData["TimeSplit"].(float64))
	// 					if timeNanoSeconds > int64(maxTime) {
	// 						maxTime = int(timeNanoSeconds)
	// 					}
	// 					timeSeconds := float64(timeNanoSeconds) * math.Pow(10.0, -9.0)
	// 					nextStat := ArgosyFleet.ChildNodes[a].ChildNodes[i].ChildNodes[j].ChildNodes[k+1].MashupDetailedElement
	// 					if k == len(ArgosyFleet.ChildNodes[a].ChildNodes[i].ChildNodes[j].ChildNodes)-2 {
	// 						DetailedElements = append(DetailedElements, &nextStat)
	// 					}
	// 					var nextdecodedstat interface{}
	// 					err = json.Unmarshal([]byte(nextStat.Data), &nextdecodedstat)
	// 					if err != nil {
	// 						log.Println("Error in decoding data in GetData")
	// 						continue
	// 					}
	// 					nextDecodedStatData := nextdecodedstat.(map[string]interface{})
	// 					nextTimeNanoSeconds := int64(nextDecodedStatData["TimeSplit"].(float64))
	// 					nextTimeSeconds := float64(nextTimeNanoSeconds) * math.Pow(10.0, -9.0)
	// 					if nextTimeSeconds-timeSeconds >= 0 {
	// 						testTimes = append(testTimes, nextTimeSeconds-timeSeconds)
	// 					} else {
	// 						avg += timeSeconds
	// 						count++
	// 					}
	// 				}
	// 			}
	// 		}
	// 	}
	// }
	//Quartile and max time analysis here:
	sort.Float64s(testTimes)
	quartiles = append(quartiles, testTimes[len(testTimes)/4])
	quartiles = append(quartiles, testTimes[len(testTimes)/2])
	quartiles = append(quartiles, testTimes[(3*len(testTimes))/4])

	// for _, time := range testTimes {
	// 	avg += time
	// }
	avg = avg / (float64(count))
	if len(DetailedElements) >= idForData {
		argosyElement := DetailedElements[idForData]
		decodedData := make(map[string]interface{})
		if argosyElement.Data != "" {
			var decoded interface{}
			err := json.Unmarshal([]byte(argosyElement.Data), &decoded)
			if err != nil {
				log.Println("Error in decoding data in GetData")
			}
			decodedData = decoded.(map[string]interface{})
		}

		decodedData["Quartiles"] = quartiles
		decodedData["MaxTime"] = maxTime
		decodedData["Average"] = avg
		encoded, err := json.Marshal(&decodedData)
		if err != nil {
			log.Println("Error in encoding data in GetData")
		}
		argosyElement.Data = string(encoded)
		DetailedElements[idForData] = argosyElement
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
	logger.Printf("Elements built.\n")

	return DetailedElements

	// config := eUtils.DriverConfig{Insecure: *insecure, Log: logger, ExitOnFailure: true}
	// secretID := ""
	// appRoleID := ""
	// address := ""
	// token := ""
	// empty := ""

	// autoErr := eUtils.AutoAuth(&config, &secretID, &appRoleID, &token, &empty, envPtr, &address, false)
	// eUtils.CheckError(&config, autoErr, true)

	// mod, modErr := helperkv.NewModifier(*insecure, token, address, *envPtr, nil, true, logger)
	// mod.Env = *envPtr
	// eUtils.CheckError(&config, modErr, true)
	// ArgosyFleet, argosyErr := argosyopts.BuildFleet(mod, logger)
	// eUtils.CheckError(&config, argosyErr, true)

	// DetailedElements := []*mashupsdk.MashupDetailedElement{}
	// //DetailedElements = append(DetailedElements, &ArgosyFleet.MashupDetailedElement)
	// //var quartiles []float64
	// maxTime := 0
	// for a := 0; a < len(ArgosyFleet.ChildNodes); a++ {
	// 	//check genre

	// 	argosyElement := ArgosyFleet.ChildNodes[a].MashupDetailedElement
	// 	argosyElement.Alias = "Argosy"
	// 	if argosyElement.Genre != "Argosy" {
	// 		DetailedElements = append(DetailedElements, &argosyElement)
	// 	} else {
	// 		newQuartiles := []float64{}
	// 		for i := 0; i < len(ArgosyFleet.ChildNodes[a].ChildNodes); i++ {
	// 			dfgElement := ArgosyFleet.ChildNodes[a].ChildNodes[i].MashupDetailedElement
	// 			dfgElement.Alias = "DataFlowGroup"
	// 			DetailedElements = append(DetailedElements, &dfgElement)

	// 			for j := 0; j < len(ArgosyFleet.ChildNodes[a].ChildNodes[i].ChildNodes); j++ {
	// 				dfelement := ArgosyFleet.ChildNodes[a].ChildNodes[i].ChildNodes[j].MashupDetailedElement
	// 				dfelement.Alias = "DataFlow"
	// 				DetailedElements = append(DetailedElements, &dfelement)
	// 				for k := 0; k < len(ArgosyFleet.ChildNodes[a].ChildNodes[i].ChildNodes[j].ChildNodes)-1; k++ {
	// 					stat := ArgosyFleet.ChildNodes[a].ChildNodes[i].ChildNodes[j].ChildNodes[k].MashupDetailedElement
	// 					stat.Alias = "DataFlowStatistic"
	// 					DetailedElements = append(DetailedElements, &stat)
	// 					var decodedstat interface{}
	// 					err := json.Unmarshal([]byte(stat.Data), &decodedstat)
	// 					if err != nil {
	// 						log.Println("Error in decoding data in buildDataFlowStatistics")
	// 						break
	// 					}
	// 					decodedStatData := decodedstat.(map[string]interface{})
	// 					timeNanoSeconds := int64(decodedStatData["TimeSplit"].(float64))
	// 					if timeNanoSeconds > int64(maxTime) {
	// 						maxTime = int(timeNanoSeconds)
	// 					}
	// 					timeSeconds := float64(timeNanoSeconds) * math.Pow(10.0, -9.0)
	// 					nextStat := ArgosyFleet.ChildNodes[a].ChildNodes[i].ChildNodes[j].ChildNodes[k+1].MashupDetailedElement
	// 					if k == len(ArgosyFleet.ChildNodes[a].ChildNodes[i].ChildNodes[j].ChildNodes)-2 {
	// 						DetailedElements = append(DetailedElements, &nextStat)
	// 					}
	// 					var nextdecodedstat interface{}
	// 					err = json.Unmarshal([]byte(nextStat.Data), &nextdecodedstat)
	// 					if err != nil {
	// 						log.Println("Error in decoding data in GetData")
	// 						continue
	// 					}
	// 					nextDecodedStatData := nextdecodedstat.(map[string]interface{})
	// 					nextTimeNanoSeconds := int64(nextDecodedStatData["TimeSplit"].(float64))
	// 					nextTimeSeconds := float64(nextTimeNanoSeconds) * math.Pow(10.0, -9.0)
	// 					if nextTimeSeconds-timeSeconds > 0 {
	// 						quartiles = append(quartiles, nextTimeSeconds-timeSeconds)
	// 					}
	// 				}
	// 			}
	// 			sort.Float64s(quartiles)
	// 			for j := len(DetailedElements) - 1; j >= 0; j-- {
	// 				if DetailedElements[j].Genre == "DataFlow" && quartiles != nil {

	// 					var decoded interface{}
	// 					err := json.Unmarshal([]byte(DetailedElements[j].Data), &decoded)
	// 					if err != nil {
	// 						log.Println("Error in decoding data in GetData")
	// 						continue
	// 					}
	// 					decodedData := decoded.(map[string]interface{})
	// 					newQuartiles = append(newQuartiles, quartiles[len(quartiles)/4])
	// 					//newQuartiles[0] = quartiles[len(quartiles)/4]
	// 					newQuartiles = append(newQuartiles, quartiles[len(quartiles)/2])
	// 					//newQuartiles[1] = quartiles[len(quartiles)/2]
	// 					newQuartiles = append(newQuartiles, quartiles[(3*len(quartiles))/4])
	// 					//newQuartiles[2] = quartiles[(3*len(quartiles))/4]
	// 					decodedData["Quartiles"] = newQuartiles
	// 					encoded, err := json.Marshal(&decodedData)
	// 					if err != nil {
	// 						log.Println("Error in encoding data in GetData")
	// 					}
	// 					DetailedElements[j].Data = string(encoded)
	// 					continue //break
	// 				}
	// 			}
	// 		}
	// 		if len(newQuartiles) > 0 {
	// 			decodedData := make(map[string]interface{})
	// 			if argosyElement.Data != "" {
	// 				var decoded interface{}
	// 				err := json.Unmarshal([]byte(argosyElement.Data), &decoded)
	// 				if err != nil {
	// 					log.Println("Error in decoding data in GetData")
	// 					continue
	// 				}
	// 				decodedData = decoded.(map[string]interface{})
	// 			}

	// 			decodedData["Quartiles"] = newQuartiles
	// 			decodedData["MaxTime"] = maxTime
	// 			encoded, err := json.Marshal(&decodedData)
	// 			if err != nil {
	// 				log.Println("Error in encoding data in GetData")
	// 			}
	// 			argosyElement.Data = string(encoded)
	// 		}
	// 	}
	// 	DetailedElements = append(DetailedElements, &argosyElement)
	// }

	// DetailedElements = append(DetailedElements, &mashupsdk.MashupDetailedElement{
	// 	Basisid:        5,
	// 	State:          &mashupsdk.MashupElementState{Id: 5, State: int64(mashupsdk.Mutable)},
	// 	Name:           "SearchElement",
	// 	Alias:          "It",
	// 	Description:    "Search Element",
	// 	Data:           "",
	// 	Custosrenderer: "SearchRenderer",
	// 	Renderer:       "",
	// 	Genre:          "",
	// 	Subgenre:       "Ento",
	// 	Parentids:      nil,
	// 	Childids:       []int64{},
	// })
	// return DetailedElements
}

// Returns an array of mashup detailed elements populated with stub data
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
			if detailedElement.Id == 6 {
				fmt.Println("Hi")
			}
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
				for k := 0; k < len(TimeData[data[pointer]]) && k < len(argosy.ChildNodes[i].ChildNodes[j].ChildNodes); k++ {
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
