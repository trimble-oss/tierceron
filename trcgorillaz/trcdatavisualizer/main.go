//go:build darwin || linux
// +build darwin linux

package main

// World is a basic gomobile app.
import (
	"embed"
	"flag"
	"log"
	"os"

	//"strconv"

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
	worldLog, err := os.OpenFile("world.log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0666)
	if err != nil {
		log.Fatal(err)
	}
	log.SetOutput(worldLog)

	mashupsdk.InitCertKeyPair(mashupCert, mashupKey)

	mashupRenderer := &g3nrender.MashupRenderer{}

	mashupRenderer.AddRenderer("Curve", &ttdirender.CurveRenderer{})
	mashupRenderer.AddRenderer("Background", &ttdirender.BackgroundRenderer{})
	mashupRenderer.AddRenderer("Path", &ttdirender.PathRenderer{})

	worldApp := g3nworld.NewWorldApp(*headless, mashupRenderer)

	worldApp.InitServer(*callerCreds, *insecure)

	if *headless {
		ArgosyFleet := argosyopts.BuildFleet(nil)
		DetailedElements := []*mashupsdk.MashupDetailedElement{}
		for _, argosy := range ArgosyFleet.Fleet {
			DetailedElements = append(DetailedElements, &argosy.MashupDetailedElement)
		}
		generatedElements, genErr := worldApp.MSdkApiHandler.UpsertMashupElements(
			&mashupsdk.MashupDetailedElementBundle{
				AuthToken:        "",
				DetailedElements: DetailedElements,
			})

		if genErr != nil {
			log.Fatalf(genErr.Error(), genErr)
		} else {
			generatedElements.DetailedElements[2].State.State = int64(mashupsdk.Clicked)

			elementStateBundle := mashupsdk.MashupElementStateBundle{
				AuthToken:     "",
				ElementStates: []*mashupsdk.MashupElementState{generatedElements.DetailedElements[2].State},
			}

			worldApp.MSdkApiHandler.UpsertMashupElementsState(&elementStateBundle)

		}
		go worldApp.MSdkApiHandler.OnResize(&mashupsdk.MashupDisplayHint{Width: 1600, Height: 800})

	}
	//worldApp.MSdkApiHandler.OnResize(&mashupsdk.MashupDisplayHint{Xpos: 0, Ypos: 0, Width: 600, Height: 800})
	// Initialize the main window.
	go worldApp.InitMainWindow()

	<-worldCompleteChan
}
