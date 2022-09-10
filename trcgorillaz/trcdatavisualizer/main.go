//go:build darwin || linux
// +build darwin linux

package main

// World is a basic gomobile app.
import (
	"embed"
	"flag"
	"log"

	//"math"
	"os"
	"sort"

	//"strconv"
	//"strings"
	//"time"

	eUtils "tierceron/utils"
	//helperkv "tierceron/vaulthelper/kv"

	"tierceron/buildopts/argosyopts"
	"tierceron/trcgorillaz/trcdatavisualizer/ttdirender"
	"tierceron/trcgorillaz/trcserver/ttdisupport"

	"github.com/mrjrieke/nute/g3nd/g3nworld"
	"github.com/mrjrieke/nute/g3nd/worldg3n/g3nrender"
	"github.com/mrjrieke/nute/mashupsdk"
	"github.com/mrjrieke/nute/mashupsdk/client"
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

var worldCompleteChan chan bool

//go:embed tls/mashup.crt
var mashupCert embed.FS

//go:embed tls/mashup.key
var mashupKey embed.FS

func main() {
	callerCreds := flag.String("CREDS", "", "Credentials of caller")
	insecure := flag.Bool("insecure", false, "Skip server validation")
	custos := flag.Bool("custos", false, "Run in guardian mode.")
	headless := flag.Bool("headless", false, "Run headless")
	//envPtr := flag.String("env", "QA", "Environment to configure")
	flag.Parse()
	worldLog, err := os.OpenFile("ttdivisualizer.log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0666)
	if err != nil {
		log.Fatal(err)
	}
	logger := log.New(worldLog, "[ttdivisualizer]", log.LstdFlags)
	log.SetOutput(worldLog)

	mashupsdk.InitCertKeyPair(mashupCert, mashupKey)

	mashupRenderer := &g3nrender.MashupRenderer{}
	curveRenderer := &ttdirender.CurveRenderer{
		CollaboratingRenderer: &ttdirender.ElementRenderer{
			GenericRenderer: g3nrender.GenericRenderer{RendererType: g3nrender.LAYOUT},
		},
	}
	mashupRenderer.AddRenderer("Curve", curveRenderer)
	mashupRenderer.AddRenderer("Background", &ttdirender.BackgroundRenderer{})
	mashupRenderer.AddRenderer("Element", curveRenderer.CollaboratingRenderer)

	worldApp := g3nworld.NewWorldApp(*headless, true, mashupRenderer, nil)
	DetailedElements := []*mashupsdk.MashupDetailedElement{}
	if *custos && *headless {
		worldApp.MashupContext = client.BootstrapInit("ttdiserver", worldApp.MSdkApiHandler, []string{"HOME=" + os.Getenv("HOME")}, []string{"-headless=true"}, insecure) //=true
		// libraryElementBundle, upsertErr := worldApp.MashupContext.Client.GetMashupElements(
		// 	worldApp.MashupContext,
		// 	&mashupsdk.MashupEmpty{AuthToken: client.GetServerAuthToken()},
		// )
		// if upsertErr != nil {
		// 	log.Printf("G3n Element initialization failure: %s\n", upsertErr.Error())
		// }

		// DetailedElements = libraryElementBundle.DetailedElements
		// dfstatData := ttdisupport.Dfstatistics
		// timeData := ttdisupport.TestTimeData
		// // for _, el := range DetailedElements {
		// // 	el.Colabrenderer = ""
		// // }
		// curveRenderer.TimeData = dfstatData
		// curveRenderer.SortedTimes = timeData
		// worldApp.MSdkApiHandler.UpsertMashupElements( //generatedElementsBundle //genErr
		// 	&mashupsdk.MashupDetailedElementBundle{
		// 		AuthToken:        "",
		// 		DetailedElements: DetailedElements,
		// 	})

	} else if *custos {
		worldApp.MashupContext = client.BootstrapInit("ttdiserver", worldApp.MSdkApiHandler, []string{"HOME=" + os.Getenv("HOME")}, nil, insecure) //=true

	}
	if *custos {
		libraryElementBundle, upsertErr := worldApp.MashupContext.Client.GetMashupElements(
			worldApp.MashupContext,
			&mashupsdk.MashupEmpty{AuthToken: client.GetServerAuthToken()},
		)
		if upsertErr != nil {
			log.Printf("G3n Element initialization failure: %s\n", upsertErr.Error())
		}

		DetailedElements = libraryElementBundle.DetailedElements
		dfstatData := ttdisupport.Dfstatistics
		timeData := ttdisupport.TestTimeData
		// for _, el := range DetailedElements {
		// 	el.Colabrenderer = ""
		// }
		curveRenderer.TimeData = dfstatData
		curveRenderer.SortedTimes = timeData
		worldApp.MSdkApiHandler.UpsertMashupElements( //generatedElementsBundle //genErr
			&mashupsdk.MashupDetailedElementBundle{
				AuthToken:        "",
				DetailedElements: DetailedElements,
			})

	} else if *headless {
		config := eUtils.DriverConfig{Insecure: *insecure, Log: logger, ExitOnFailure: true}
		ArgosyFleet, argosyErr := argosyopts.BuildFleet(nil) //mod)
		eUtils.CheckError(&config, argosyErr, true)

		dfstatData := map[string]float64{}
		statGroup := []float64{}
		testTimes := []float64{}
		pointer := 0
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

		curveRenderer.TimeData = dfstatData
		curveRenderer.SortedTimes = testTimes

		// _, genErr := worldApp.MSdkApiHandler.UpsertMashupElements(
		// 	&mashupsdk.MashupDetailedElementBundle{
		// 		AuthToken:        "",
		// 		DetailedElements: DetailedElements,
		// 	})

		// if genErr != nil {
		// 	log.Fatalf(genErr.Error(), genErr)
		// } else {
		// 	go worldApp.MSdkApiHandler.OnResize(&mashupsdk.MashupDisplayHint{Width: 800, Height: 800})
		// }
		// go worldApp.MSdkApiHandler.OnResize(&mashupsdk.MashupDisplayHint{Width: 800, Height: 800})

		// go func() {
		// 	time.Sleep(10 * time.Second)

		// }()

	} else {
		worldApp.InitServer(*callerCreds, *insecure)
	}

	if *custos || *headless {
		//
		// Generate concrete elements from library elements.
		//
		generatedElementsBundle, genErr := worldApp.MSdkApiHandler.UpsertMashupElements(
			&mashupsdk.MashupDetailedElementBundle{
				AuthToken:        "",
				DetailedElements: DetailedElements,
			})

		if !*headless {
			//
			// Upsert concrete elements to custos
			//
			_, custosUpsertErr := worldApp.MashupContext.Client.UpsertMashupElements(
				worldApp.MashupContext,
				&mashupsdk.MashupDetailedElementBundle{
					AuthToken:        client.GetServerAuthToken(),
					DetailedElements: generatedElementsBundle.DetailedElements,
				})

			if custosUpsertErr != nil {
				log.Fatalf(custosUpsertErr.Error(), custosUpsertErr)
			}

		}

		if genErr != nil {
			log.Fatalf(genErr.Error(), genErr)
		} else {
			//
			// Pick an initial element to 'click'
			//
			generatedElementsBundle.DetailedElements[3].State.State = int64(mashupsdk.Clicked)

			elementStateBundle := mashupsdk.MashupElementStateBundle{
				AuthToken:     "",
				ElementStates: []*mashupsdk.MashupElementState{generatedElementsBundle.DetailedElements[3].State},
			}

			worldApp.MSdkApiHandler.UpsertMashupElementsState(&elementStateBundle)
		}
		go worldApp.MSdkApiHandler.OnResize(&mashupsdk.MashupDisplayHint{Width: 1600, Height: 800})
	}

	// Initialize the main window.
	go worldApp.InitMainWindow()

	<-worldCompleteChan
}
