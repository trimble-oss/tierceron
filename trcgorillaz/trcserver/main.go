//go:build darwin || linux
// +build darwin linux

package main

// World is a basic gomobile app.
import (
	"embed"
	"flag"
	"log"
	"os"
	"tierceron/trcgorillaz/trcserver/data"
	"tierceron/trcgorillaz/trcserver/trcRenderers"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"
	"github.com/mrjrieke/nute/custos/custosworld"
	"github.com/mrjrieke/nute/mashupsdk"
)

var worldCompleteChan chan bool

//go:embed tls/mashup.crt
var mashupCert embed.FS

//go:embed tls/mashup.key
var mashupKey embed.FS

func OutsideClone(custosWorldApp *custosworld.CustosWorldApp, childId int64, concreteElement *mashupsdk.MashupDetailedElement) {
	custosWorldApp.FyneWidgetElements["Outside"].MashupDetailedElement.Copy(concreteElement)
}

//Shouldn't need this method
func DetailMappedTabItemFyneComponent(custosWorldApp *custosworld.CustosWorldApp, id string) *container.TabItem {
	if custosWorldApp.FyneWidgetElements[id] != nil {
		de := custosWorldApp.FyneWidgetElements[id].MashupDetailedElement

		tabLabel := widget.NewLabel(de.Description)
		tabLabel.Wrapping = fyne.TextWrapWord
		tabItem := container.NewTabItem(id, container.NewBorder(nil, nil, layout.NewSpacer(), nil, container.NewVBox(tabLabel, container.NewAdaptiveGrid(2,
			widget.NewButton("Show", func() {
				// Workaround... mashupdetailedelement points at wrong element sometimes, but shouldn't!
				if len(custosWorldApp.ElementLoaderIndex) > 0 {
					mashupIndex := custosWorldApp.ElementLoaderIndex[custosWorldApp.FyneWidgetElements[de.Alias].GuiComponent.(*container.TabItem).Text]
					custosWorldApp.FyneWidgetElements[de.Alias].MashupDetailedElement = custosWorldApp.MashupDetailedElementLibrary[mashupIndex]

					custosWorldApp.FyneWidgetElements[de.Alias].MashupDetailedElement.ApplyState(mashupsdk.Hidden, false)
					if custosWorldApp.FyneWidgetElements[de.Alias].MashupDetailedElement.Genre == "Collection" {
						custosWorldApp.FyneWidgetElements[de.Alias].MashupDetailedElement.ApplyState(mashupsdk.Recursive, true)
					}
					custosWorldApp.FyneWidgetElements[de.Alias].OnStatusChanged()
				}
			}), widget.NewButton("Hide", func() {
				if len(custosWorldApp.ElementLoaderIndex) > 0 {
					// Workaround... mashupdetailedelement points at wrong element sometimes, but shouldn't!
					mashupIndex := custosWorldApp.ElementLoaderIndex[custosWorldApp.FyneWidgetElements[de.Alias].GuiComponent.(*container.TabItem).Text]
					custosWorldApp.FyneWidgetElements[de.Alias].MashupDetailedElement = custosWorldApp.MashupDetailedElementLibrary[mashupIndex]

					custosWorldApp.FyneWidgetElements[de.Alias].MashupDetailedElement.ApplyState(mashupsdk.Hidden, true)
					if custosWorldApp.FyneWidgetElements[de.Alias].MashupDetailedElement.Genre == "Collection" {
						custosWorldApp.FyneWidgetElements[de.Alias].MashupDetailedElement.ApplyState(mashupsdk.Recursive, true)
					}
					custosWorldApp.FyneWidgetElements[de.Alias].OnStatusChanged()
				}
			})))),
		)
		return tabItem
	}
	return nil
}

//go:embed logo.png
var logoIcon embed.FS

func main() {
	callerCreds := flag.String("CREDS", "", "Credentials of caller")
	insecure := flag.Bool("insecure", false, "Skip server validation")
	headless := flag.Bool("headless", false, "Run headless")
	envPtr := flag.String("env", "QA", "Environment to configure")
	flag.Parse()

	helloLog, err := os.OpenFile("ttdiserver.log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0666)
	if err != nil {
		log.Fatalf(err.Error(), err)
	}
	logger := log.New(helloLog, "[ttdiserver]", log.LstdFlags)
	log.SetOutput(helloLog)

	mashupsdk.InitCertKeyPair(mashupCert, mashupKey)
	var DetailedElements []*mashupsdk.MashupDetailedElement

	if *headless {
		DetailedElements = data.GetHeadlessData(insecure, logger)
	} else {
		DetailedElements = data.GetData(insecure, logger, envPtr)

	}

	tenantDataRenderer := &trcRenderers.TenantDataRenderer{}
	custosWorld := custosworld.NewCustosWorldApp(*headless, DetailedElements, tenantDataRenderer)

	// Initialize a tab item renderer
	// This will be called during upsert elements phase.
	// indexed by subgenre
	custosWorld.CustomTabItemRenderer["TabItemRenderer"] = tenantDataRenderer
	custosWorld.CustomTabItemRenderer["ControllerTabItemRenderer"] = &trcRenderers.SearchRenderer{CustosWorldApp: custosWorld}

	custosWorld.Title = "Hello Custos"
	custosWorld.MainWindowSize = fyne.NewSize(800, 100)
	logoIconBytes, _ := logoIcon.ReadFile("logo.png")
	custosWorld.Icon = fyne.NewStaticResource("Logo", logoIconBytes)

	if !custosWorld.Headless {
		custosWorld.InitServer(*callerCreds, *insecure)
	}

	// Initialize the main window.
	custosWorld.InitMainWindow()

	<-worldCompleteChan
}
