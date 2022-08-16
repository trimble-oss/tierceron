//go:build darwin || linux
// +build darwin linux

package main

// World is a basic gomobile app.
import (
	"embed"
	"flag"
	"log"
	"os"
	"time"

	"tierceron/buildopts/argosyopts"
	"tierceron/trcgorillaz/trcdatavisualizer/ttdirender"

	"github.com/mrjrieke/nute/g3nd/g3nworld"
	"github.com/mrjrieke/nute/g3nd/worldg3n/g3nrender"
	"github.com/mrjrieke/nute/mashupsdk"
)

var worldCompleteChan chan bool

//go:embed tls/mashup.crt
var mashupCert embed.FS

//go:embed tls/mashup.key
var mashupKey embed.FS

func main() {
	callerCreds := flag.String("CREDS", "", "Credentials of caller")
	insecure := flag.Bool("insecure", false, "Skip server validation")
	headless := flag.Bool("headless", false, "Run headless")
	flag.Parse()
	worldLog, err := os.OpenFile("trcdatavisualizer.log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0666)
	if err != nil {
		log.Fatal(err)
	}
	log.SetOutput(worldLog)

	mashupsdk.InitCertKeyPair(mashupCert, mashupKey)

	mashupRenderer := &g3nrender.MashupRenderer{}

	curveRenderer := &ttdirender.CurveRenderer{
		CollaboratingRenderer: &ttdirender.ElementRenderer{}}

	mashupRenderer.AddRenderer("Background", &ttdirender.BackgroundRenderer{})
	mashupRenderer.AddRenderer("Path", curveRenderer.CollaboratingRenderer)
	mashupRenderer.AddRenderer("Curve", curveRenderer)
	mashupRenderer.AddRenderer("SubSpiral", &ttdirender.SubSpiralRenderer{GenericRenderer: g3nrender.GenericRenderer{RendererType: g3nrender.LAYOUT}})
	mashupRenderer.AddRenderer("Element", &ttdirender.ElementRenderer{GenericRenderer: g3nrender.GenericRenderer{RendererType: g3nrender.LAYOUT}})
	worldApp := g3nworld.NewWorldApp(*headless, mashupRenderer)

	worldApp.InitServer(*callerCreds, *insecure)

	if *headless {
		ArgosyFleet := argosyopts.BuildFleet(nil)
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
				}
			}
		}

		_, genErr := worldApp.MSdkApiHandler.UpsertMashupElements(
			&mashupsdk.MashupDetailedElementBundle{
				AuthToken:        "",
				DetailedElements: DetailedElements,
			})

		if genErr != nil {
			log.Fatalf(genErr.Error(), genErr)
		} else {
			go worldApp.MSdkApiHandler.OnResize(&mashupsdk.MashupDisplayHint{Width: 800, Height: 800})
		}
		go worldApp.MSdkApiHandler.OnResize(&mashupsdk.MashupDisplayHint{Width: 800, Height: 800})

		go func() {
			time.Sleep(10 * time.Second)

		}()

	}

	// Initialize the main window.
	go worldApp.InitMainWindow()

	<-worldCompleteChan
}
