//go:build darwin || linux
// +build darwin linux

package main

// World is a basic gomobile app.
import (
	"embed"
	"flag"
	"log"

	"os"

	eUtils "tierceron/utils"

	"tierceron/buildopts/argosyopts"
	"tierceron/trcgorillaz/ttdivisualizer/ttdirender"

	"github.com/mrjrieke/nute/g3nd/g3nworld"
	"github.com/mrjrieke/nute/g3nd/worldg3n/g3nrender"
	"github.com/mrjrieke/nute/mashupsdk"
	"github.com/mrjrieke/nute/mashupsdk/client"
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

	} else if *custos {
		worldApp.MashupContext = client.BootstrapInit("ttdiserver", worldApp.MSdkApiHandler, []string{"HOME=" + os.Getenv("HOME")}, nil, insecure) //=true

	}
	if *custos {
		libraryElementBundle, upsertErr := worldApp.MashupContext.Client.GetMashupElements(
			worldApp.MashupContext, &mashupsdk.MashupEmpty{AuthToken: worldApp.GetAuthToken()},
		)
		if upsertErr != nil {
			log.Printf("G3n Element initialization failure: %s\n", upsertErr.Error())
		}

		DetailedElements = libraryElementBundle.DetailedElements
		worldApp.MSdkApiHandler.UpsertMashupElements(
			&mashupsdk.MashupDetailedElementBundle{
				AuthToken:        "",
				DetailedElements: DetailedElements,
			})

	} else if *headless && !*custos {
		config := eUtils.DriverConfig{Insecure: *insecure, Log: logger, ExitOnFailure: true}
		ArgosyFleet, argosyErr := argosyopts.BuildFleet(nil, logger)
		eUtils.CheckError(&config, argosyErr, true)

		dfstatData := map[string]float64{}
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
					for k := 0; k < len(TimeData[data[pointer]]); k++ {
						el := argosy.Groups[i].Flows[j].Statistics[k].MashupDetailedElement
						el.Alias = "DataFlowStatistic"
						timeSeconds := TimeData[data[pointer]][k]
						dfstatData[el.Name] = timeSeconds
						DetailedElements = append(DetailedElements, &el)
					}
				}
			}
		}
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
					AuthToken:        worldApp.GetAuthToken(),
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
