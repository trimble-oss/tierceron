package data

import (
	"encoding/json"
	"log"
	"math"
	"sort"
	"strconv"

	"github.com/trimble-oss/tierceron/buildopts/argosyopts"
	flowcore "github.com/trimble-oss/tierceron/trcflow/core"

	eUtils "github.com/trimble-oss/tierceron/utils"
	helperkv "github.com/trimble-oss/tierceron/vaulthelper/kv"

	"github.com/mrjrieke/nute/mashupsdk"
)

var maxTime int64
var avg float64
var idForData int
var count int

// Collects time data from DataFlowStatistics layer and adds data to DataFlow object
// Returns updated DetailedElements array and an array of time data from DataFlowStatistics
func createDetailedElements(detailedElements []*mashupsdk.MashupDetailedElement, node flowcore.TTDINode, testTimes []float64, depth int) ([]*mashupsdk.MashupDetailedElement, []float64) {
	for _, child_node := range node.ChildNodes {
		if child_node.MashupDetailedElement.Genre == "DataFlowStatistic" {
			node.MashupDetailedElement.Genre = "DataFlow"
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

// Returns an array of mashup detailed elements populated with each Tenant's data and Childnodes
func GetData(insecure *bool, logger *log.Logger, envPtr *string) []*mashupsdk.MashupDetailedElement {
	config := eUtils.DriverConfig{Insecure: *insecure, Log: logger, ExitOnFailure: true}
	secretID := ""
	appRoleID := ""
	address := ""
	token := ""
	empty := ""

	autoErr := eUtils.AutoAuth(&config, &secretID, &appRoleID, &token, &empty, envPtr, &address, nil, "", false)
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
	maxTime = 0
	testTimes := []float64{}
	quartiles := []float64{}
	idForData = 0
	avg = 0.0
	count = 0
	DetailedElements, testTimes = createDetailedElements(DetailedElements, ArgosyFleet, testTimes, 0)
	sort.Float64s(testTimes)
	quartiles = append(quartiles, testTimes[len(testTimes)/4])
	quartiles = append(quartiles, testTimes[len(testTimes)/2])
	quartiles = append(quartiles, testTimes[(3*len(testTimes))/4])
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
}

// Returns an array of mashup detailed elements populated with stub data
func GetHeadlessData(insecure *bool, logger *log.Logger) []*mashupsdk.MashupDetailedElement {
	data, TimeData := argosyopts.GetStubbedDataFlowStatistics()

	config := eUtils.DriverConfig{Insecure: *insecure, Log: logger, ExitOnFailure: true}
	ArgosyFleet, argosyErr := argosyopts.BuildFleet(nil, logger)
	eUtils.CheckError(&config, argosyErr, true)

	dfstatData := map[string]float64{}
	statGroup := []float64{}
	testTimes := []float64{}
	pointer := 0
	maxTime = 0
	DetailedElements := []*mashupsdk.MashupDetailedElement{}
	for m := 0; m < len(ArgosyFleet.ChildNodes); m++ {
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
