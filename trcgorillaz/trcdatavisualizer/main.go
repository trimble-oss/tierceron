//go:build darwin || linux
// +build darwin linux

package main

// World is a basic gomobile app.
import (
	"embed"
	"flag"
	"log"
	"os"
	"strconv"

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

	mashupRenderer.AddRenderer("Sphere", &ttdirender.SphereRenderer{})
	//mashupRenderer.AddRenderer("Lines", &ttdirender.CurveRenderer{})
	mashupRenderer.AddRenderer("Path", &ttdirender.PathRenderer{})

	worldApp := g3nworld.NewWorldApp(*headless, mashupRenderer)

	worldApp.InitServer(*callerCreds, *insecure)

	if *headless {
		DetailedElements := []*mashupsdk.MashupDetailedElement{
			{
				Id:          6,
				State:       &mashupsdk.MashupElementState{Id: 7, State: int64(mashupsdk.Init)},
				Name:        "Outside",
				Alias:       "Outside",
				Description: "",
				Genre:       "Space",
				Subgenre:    "Exo",
				Parentids:   nil,
				Childids:    nil,
			},
			{
				Basisid:     -4,
				State:       &mashupsdk.MashupElementState{Id: -3, State: int64(mashupsdk.Mutable)},
				Name:        "{2}-Path",
				Alias:       "It",
				Description: "",
				Renderer:    "Path",
				Genre:       "Solid",
				Subgenre:    "Ento",
				Parentids:   []int64{},
				Childids:    []int64{5},
			},
			{
				Basisid:     -3,
				State:       &mashupsdk.MashupElementState{Id: -3, State: int64(mashupsdk.Mutable)},
				Name:        "{1}-Sphere",
				Alias:       "It",
				Description: "",
				Renderer:    "Sphere",
				Genre:       "Solid",
				Subgenre:    "Ento",
				Parentids:   nil,
				Childids:    []int64{1},
			},
			{
				Id:          5,
				State:       &mashupsdk.MashupElementState{Id: 6, State: int64(mashupsdk.Init)},
				Name:        "PathEntity-One",
				Description: "",
				Genre:       "Abstract",
				Subgenre:    "",
				Parentids:   nil,         //[]int64{10},
				Childids:    []int64{-4}, // -3 -- generated and replaced by server since it is immutable.
			},
			{
				Id:          9,
				State:       &mashupsdk.MashupElementState{Id: 6, State: int64(mashupsdk.Init)},
				Name:        "PathEntity-Two",
				Description: "",
				Genre:       "Abstract",
				Subgenre:    "",
				Parentids:   nil,         //[]int64{10},
				Childids:    []int64{-4}, // -3 -- generated and replaced by server since it is immutable.
			},
			{
				Id:          7,
				State:       &mashupsdk.MashupElementState{Id: 6, State: int64(mashupsdk.Init)},
				Name:        "PathEntity-Three",
				Description: "",
				Genre:       "Abstract",
				Subgenre:    "",
				Parentids:   nil,         //[]int64{10},
				Childids:    []int64{-4}, // -3 -- generated and replaced by server since it is immutable.
			},
			{
				Id:          8,
				State:       &mashupsdk.MashupElementState{Id: 6, State: int64(mashupsdk.Init)},
				Name:        "PathEntity-Four",
				Description: "",
				Genre:       "Abstract",
				Subgenre:    "",
				Parentids:   nil,         //[]int64{10},
				Childids:    []int64{-4}, // -3 -- generated and replaced by server since it is immutable.
			},
			{
				Id:          4,
				State:       &mashupsdk.MashupElementState{Id: 10, State: int64(mashupsdk.Init)},
				Name:        "PathGroupOne",
				Description: "Paths",
				Genre:       "Collection",
				Subgenre:    "Path",
				Parentids:   []int64{},  //[]int64{},
				Childids:    []int64{5}, //NOTE: If you want to add all children need to include children in for loop!
			},
			//sphere elements

			{
				Id:          1,
				State:       &mashupsdk.MashupElementState{Id: 6, State: int64(mashupsdk.Init)},
				Name:        "SphereEntity-One",
				Description: "",
				Genre:       "Abstract",
				Subgenre:    "",
				Parentids:   nil,         //[]int64{10},
				Childids:    []int64{-3}, // -3 -- generated and replaced by server since it is immutable.
			},
			{
				Id:          2,
				State:       &mashupsdk.MashupElementState{Id: 6, State: int64(mashupsdk.Init)},
				Name:        "SphereEntity-Two",
				Description: "",
				Genre:       "Abstract",
				Subgenre:    "",
				Parentids:   nil,         //[]int64{10},
				Childids:    []int64{-3}, // -3 -- generated and replaced by server since it is immutable.
			},
			{
				Id:          3,
				State:       &mashupsdk.MashupElementState{Id: 6, State: int64(mashupsdk.Init)},
				Name:        "SpheresGroupOne",
				Description: "Spheres",
				Genre:       "Collection",
				Subgenre:    "Sphere",
				Parentids:   nil,        //[]int64{},
				Childids:    []int64{1}, //NOTE: If you want to add all children need to include children in for loop!
			},
		}
		for i := 0; i < 97; i++ {
			DetailedElements = append(DetailedElements, &mashupsdk.MashupDetailedElement{
				Id:          int64(10 + i),
				State:       &mashupsdk.MashupElementState{Id: 6, State: int64(mashupsdk.Init)},
				Name:        "PathEntity-" + strconv.Itoa(10+i),
				Description: "",
				Genre:       "Abstract",
				Subgenre:    "",
				Parentids:   []int64{},
				Childids:    []int64{-4}, // -3 -- generated and replaced by server since it is immutable.
			})
		}

		generatedElements, genErr := worldApp.MSdkApiHandler.UpsertMashupElements(
			&mashupsdk.MashupDetailedElementBundle{
				AuthToken:        "",
				DetailedElements: DetailedElements,
			})

		if genErr != nil {
			log.Fatalf(genErr.Error(), genErr)
		} else { //2-->3
			generatedElements.DetailedElements[2].State.State = int64(mashupsdk.Clicked) //FIND OUT WHAT THIS DOES

			elementStateBundle := mashupsdk.MashupElementStateBundle{
				AuthToken:     "",
				ElementStates: []*mashupsdk.MashupElementState{generatedElements.DetailedElements[2].State},
			}

			worldApp.MSdkApiHandler.UpsertMashupElementsState(&elementStateBundle)

		}

	}

	// Initialize the main window.
	go worldApp.InitMainWindow()

	<-worldCompleteChan
}
