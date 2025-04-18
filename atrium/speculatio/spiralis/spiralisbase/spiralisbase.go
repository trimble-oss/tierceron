//go:build darwin || linux
// +build darwin linux

package spiralisbase

// World is a basic gomobile app.
import (
	"embed"
	"log"

	"os"

	"github.com/trimble-oss/tierceron/buildopts/coreopts"
	"github.com/trimble-oss/tierceron/pkg/core"
	tiercerontls "github.com/trimble-oss/tierceron/pkg/tls"
	eUtils "github.com/trimble-oss/tierceron/pkg/utils"
	"github.com/trimble-oss/tierceron/pkg/utils/config"

	"github.com/trimble-oss/tierceron/atrium/buildopts/argosyopts"
	"github.com/trimble-oss/tierceron/atrium/speculatio/spiralis/ttdirender"

	"github.com/trimble-oss/tierceron-nute-core/mashupsdk"
	"github.com/trimble-oss/tierceron-nute/g3nd/g3nworld"
	"github.com/trimble-oss/tierceron-nute/g3nd/worldg3n/g3nrender"
	"github.com/trimble-oss/tierceron-nute/mashupsdk/client"
)

func CommonMain(
	mashupCert embed.FS,
	mashupKey embed.FS,
	callerCreds *string,
	insecure *bool,
	custos *bool,
	headless *bool) {

	coreopts.NewOptionsBuilder(coreopts.LoadOptions())
	tiercerontls.InitRoot()

	worldLog, err := os.OpenFile("spiralis.log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
	if err != nil {
		log.Fatal(err)
	}
	logger := log.New(worldLog, "[spiralis]", log.LstdFlags)
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
	mashupRenderer.AddRenderer("GuiRenderer", &ttdirender.GuiRenderer{GuiNodeMap: map[string]interface{}{}})

	worldApp := g3nworld.NewWorldApp(*headless, true, mashupRenderer, nil)
	DetailedElements := []*mashupsdk.MashupDetailedElement{}
	if *custos && *headless {
		worldApp.MashupContext = client.BootstrapInitWithMessageExt("fenestra", worldApp.MSdkApiHandler, []string{"HOME=" + os.Getenv("HOME")}, []string{"-headless=true"}, insecure, 500*10*1024) //=true
	} else if *custos {
		worldApp.MashupContext = client.BootstrapInitWithMessageExt("fenestra", worldApp.MSdkApiHandler, []string{"HOME=" + os.Getenv("HOME")}, nil, insecure, 500*10*1024) //=true
	}
	if *custos {
		libraryElementBundle, upsertErr := worldApp.MashupContext.Client.GetElements(
			worldApp.MashupContext, &mashupsdk.MashupEmpty{AuthToken: worldApp.GetAuthToken()},
		)
		if upsertErr != nil {
			log.Printf("G3n Element initialization failure: %s\n", upsertErr.Error())
		}
		mashupRenderer.AddRenderer("Curve", curveRenderer)
		mashupRenderer.AddRenderer("Background", &ttdirender.BackgroundRenderer{})
		mashupRenderer.AddRenderer("Element", curveRenderer.CollaboratingRenderer)
		mashupRenderer.AddRenderer("GuiRenderer", &ttdirender.GuiRenderer{GuiNodeMap: map[string]interface{}{}})

		DetailedElements = libraryElementBundle.DetailedElements
	} else if *headless && !*custos {
		data, TimeData := argosyopts.GetStubbedDataFlowStatistics()
		driverConfig := config.DriverConfig{
			CoreConfig: &core.CoreConfig{
				ExitOnFailure: true,
				Insecure:      *insecure,
				Log:           logger,
			},
		}
		ArgosyFleet, argosyErr := argosyopts.BuildFleet(nil, logger)
		eUtils.CheckError(driverConfig.CoreConfig, argosyErr, true)

		dfstatData := map[string]float64{}
		pointer := 0
		for _, argosy := range ArgosyFleet.ChildNodes {
			argosyBasis := argosy.MashupDetailedElement
			argosyBasis.Alias = "Argosy"
			DetailedElements = append(DetailedElements, argosyBasis)
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
					// TODO: This looks kinda like a hack.
					for k := 0; k < len(TimeData[data[pointer]]) && k < len(argosy.ChildNodes[i].ChildNodes[j].ChildNodes); k++ {
						el := argosy.ChildNodes[i].ChildNodes[j].ChildNodes[k].MashupDetailedElement
						el.Alias = "DataFlowStatistic"
						timeSeconds := TimeData[data[pointer]][k]
						dfstatData[el.Name] = timeSeconds
						DetailedElements = append(DetailedElements, el)
					}
				}
			}
		}
	} else {
		worldApp.InitServer(*callerCreds, *insecure, 500*1024*1024)
	}

	if *custos || *headless {
		//
		// Generate concrete elements from library elements.
		//
		generatedElementsBundle, genErr := worldApp.MSdkApiHandler.UpsertElements(
			&mashupsdk.MashupDetailedElementBundle{
				AuthToken:        "",
				DetailedElements: DetailedElements,
			})

		if !*headless {
			//
			// Upsert concrete elements to custos
			//
			_, custosUpsertErr := worldApp.MashupContext.Client.UpsertElements(
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

			worldApp.MSdkApiHandler.TweakStates(&elementStateBundle)
		}
		go worldApp.MSdkApiHandler.OnDisplayChange(&mashupsdk.MashupDisplayHint{Width: 1600, Height: 800})
	}

	// Initialize the main window.
	go worldApp.InitMainWindow()
}
