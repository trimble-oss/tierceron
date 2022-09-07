//go:build darwin || linux
// +build darwin linux

package main

// World is a basic gomobile app.
import (
	"embed"
	"flag"
	"log"
	"math"
	"os"
	"sort"
	"tierceron/buildopts/argosyopts"
	"tierceron/trcgorillaz/trcdatavisualizer/ttdirender"
	"tierceron/trcgorillaz/trcserver/ttdisupport"

	eUtils "tierceron/utils"
	helperkv "tierceron/vaulthelper/kv"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"
	"github.com/mrjrieke/nute/custos/custosworld"
	"github.com/mrjrieke/nute/g3nd/worldg3n/g3nrender"
	"github.com/mrjrieke/nute/mashupsdk"
)

var worldCompleteChan chan bool

//go:embed tls/mashup.crt
var mashupCert embed.FS

//go:embed tls/mashup.key
var mashupKey embed.FS

func TorusParser(custosWorldApp *custosworld.CustosWorldApp, childId int64, concreteElement *mashupsdk.MashupDetailedElement) {
	child := custosWorldApp.MashupDetailedElementLibrary[childId]
	if child != nil && child.Alias != "" {
		log.Printf("TorusParser lookup on: %s\n", child.Alias)
		if fwb, fwbOk := custosWorldApp.FyneWidgetElements[child.Alias]; fwbOk {
			if fwb.MashupDetailedElement != nil && fwb.GuiComponent != nil {
				fwb.MashupDetailedElement.Copy(child)
				fwb.GuiComponent.(*container.TabItem).Text = child.Name
			}
		}
	}

	if child != nil && len(child.GetChildids()) > 0 {
		for _, cId := range child.GetChildids() {
			TorusParser(custosWorldApp, cId, concreteElement)
		}
	}
}

func OutsideClone(custosWorldApp *custosworld.CustosWorldApp, childId int64, concreteElement *mashupsdk.MashupDetailedElement) {
	custosWorldApp.FyneWidgetElements["Outside"].MashupDetailedElement.Copy(concreteElement)
}

func DetailMappedTabItemFyneComponent(custosWorldApp *ttdisupport.CustosWorldApp, id string) *container.TabItem {
	de := custosWorldApp.FyneWidgetElements[id][0].MashupDetailedElement

	tabLabel := widget.NewLabel(de.Description)
	tabLabel.Wrapping = fyne.TextWrapWord
	tabItem := container.NewTabItem(id, container.NewBorder(nil, nil, layout.NewSpacer(), nil, container.NewVBox(tabLabel, container.NewAdaptiveGrid(2,
		widget.NewButton("Show", func() {
			// Workaround... mashupdetailedelement points at wrong element sometimes, but shouldn't!
			if len(custosWorldApp.ElementLoaderIndex) > 0 {
				mashupIndex := custosWorldApp.ElementLoaderIndex[custosWorldApp.FyneWidgetElements[de.Alias][0].GuiComponent.(*container.TabItem).Text]
				custosWorldApp.FyneWidgetElements[de.Alias][0].MashupDetailedElement = custosWorldApp.MashupDetailedElementLibrary[mashupIndex]

				custosWorldApp.FyneWidgetElements[de.Alias][0].MashupDetailedElement.ApplyState(mashupsdk.Hidden, false)
				if custosWorldApp.FyneWidgetElements[de.Alias][0].MashupDetailedElement.Genre == "Collection" {
					custosWorldApp.FyneWidgetElements[de.Alias][0].MashupDetailedElement.ApplyState(mashupsdk.Recursive, true)
				}
				custosWorldApp.FyneWidgetElements[de.Alias][0].OnStatusChanged()
			}
		}), widget.NewButton("Hide", func() {
			if len(custosWorldApp.ElementLoaderIndex) > 0 {
				// Workaround... mashupdetailedelement points at wrong element sometimes, but shouldn't!
				mashupIndex := custosWorldApp.ElementLoaderIndex[custosWorldApp.FyneWidgetElements[de.Alias][0].GuiComponent.(*container.TabItem).Text]
				custosWorldApp.FyneWidgetElements[de.Alias][0].MashupDetailedElement = custosWorldApp.MashupDetailedElementLibrary[mashupIndex]

				custosWorldApp.FyneWidgetElements[de.Alias][0].MashupDetailedElement.ApplyState(mashupsdk.Hidden, true)
				if custosWorldApp.FyneWidgetElements[de.Alias][0].MashupDetailedElement.Genre == "Collection" {
					custosWorldApp.FyneWidgetElements[de.Alias][0].MashupDetailedElement.ApplyState(mashupsdk.Recursive, true)
				}
				custosWorldApp.FyneWidgetElements[de.Alias][0].OnStatusChanged()
			}
		})))),
	)
	return tabItem
}

//go:embed logo.png
var logoIcon embed.FS

func main() {
	callerCreds := flag.String("CREDS", "", "Credentials of caller")
	insecure := flag.Bool("insecure", false, "Skip server validation")
	headless := flag.Bool("headless", false, "Run headless")
	envPtr := flag.String("env", "QA", "Environment to configure")
	flag.Parse()

	helloLog, err := os.OpenFile("ttdimanager.log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0666)
	if err != nil {
		log.Fatalf(err.Error(), err)
	}
	logger := log.New(helloLog, "[ttdivisualizer]", log.LstdFlags)
	//log.SetOutput(helloLog)

	mashupsdk.InitCertKeyPair(mashupCert, mashupKey)

	// callerCreds := flag.String("CREDS", "", "Credentials of caller")
	// insecure := flag.Bool("insecure", false, "Skip server validation")
	// headless := flag.Bool("headless", false, "Run headless")
	// flag.Parse()
	// worldLog, err := os.OpenFile("custos.log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0666)
	// if err != nil {
	// 	log.Fatal(err)
	// }
	// log.SetOutput(worldLog)

	// mashupsdk.InitCertKeyPair(mashupCert, mashupKey)

	DetailedElements := []*mashupsdk.MashupDetailedElement{}
	custosWorld := ttdisupport.NewCustosWorldApp(*headless, DetailedElements, nil)

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

	ArgosyFleet, argosyErr := argosyopts.BuildFleet(mod)
	eUtils.CheckError(&config, argosyErr, true)

	//DetailedElements := []*mashupsdk.MashupDetailedElement{}
	dfstatData := map[string]float64{}
	statGroup := []float64{}
	testTimes := []float64{}
	for a := 0; a < len(ArgosyFleet.Argosies); a++ {
		argosyBasis := ArgosyFleet.Argosies[a].MashupDetailedElement
		argosyBasis.Alias = "Argosy"
		argwidgetElement := ttdisupport.FyneWidgetBundle{
			GuiWidgetBundle: mashupsdk.GuiWidgetBundle{
				GuiComponent:          widget.NewLabel(argosyBasis.Name),
				MashupDetailedElement: &argosyBasis, //mashupDetailedElementLibrary["{0}-SharedAttitude"],
			},
		}
		custosWorld.FyneWidgetElements["Argosy"] = append(custosWorld.FyneWidgetElements["Argosy"], &argwidgetElement)
		custosWorld.FyneWidgetElements[argosyBasis.Name] = append(custosWorld.FyneWidgetElements[argosyBasis.Name], &argwidgetElement)
		DetailedElements = append(DetailedElements, &argosyBasis)

		for i := 0; i < len(ArgosyFleet.Argosies[a].Groups); i++ {
			detailedElement := ArgosyFleet.Argosies[a].Groups[i].MashupDetailedElement
			detailedElement.Alias = "DataFlowGroup"
			dfgwidgetElement := ttdisupport.FyneWidgetBundle{
				GuiWidgetBundle: mashupsdk.GuiWidgetBundle{
					GuiComponent:          widget.NewLabel(detailedElement.Name),
					MashupDetailedElement: &detailedElement, //mashupDetailedElementLibrary["{0}-SharedAttitude"],
				},
			}
			custosWorld.FyneWidgetElements[argosyBasis.Name] = append(custosWorld.FyneWidgetElements[argosyBasis.Name], &dfgwidgetElement)
			// MAKE IT SO HAVE SAME LIST ID AS PREVIOUS ELEMENT TO NAVIGATE THRU AND LINK THEM
			custosWorld.FyneWidgetElements["DataFlowGroup"] = append(custosWorld.FyneWidgetElements["DataFlowGroup"], &dfgwidgetElement)
			//HAVE TO REDO THIS SO THAT ARRAY ISN'T RESET EA TIME
			// MAKE A FYNE WIDGET ELEMENT WITH NIL GUI COMP AND SET MASHUPEL TO GIVEN EL AT TIME
			DetailedElements = append(DetailedElements, &detailedElement)
			for j := 0; j < len(ArgosyFleet.Argosies[a].Groups[i].Flows); j++ {
				element := ArgosyFleet.Argosies[a].Groups[i].Flows[j].MashupDetailedElement
				element.Alias = "DataFlow"
				dfwidgetElement := ttdisupport.FyneWidgetBundle{
					GuiWidgetBundle: mashupsdk.GuiWidgetBundle{
						GuiComponent:          widget.NewLabel(element.Name),
						MashupDetailedElement: &element, //mashupDetailedElementLibrary["{0}-SharedAttitude"],
					},
				}
				custosWorld.FyneWidgetElements["DataFlow"] = append(custosWorld.FyneWidgetElements["DataFlow"], &dfwidgetElement)
				custosWorld.FyneWidgetElements[detailedElement.Name] = append(custosWorld.FyneWidgetElements[argosyBasis.Name], &dfwidgetElement)
				DetailedElements = append(DetailedElements, &element)
				for k := 0; k < len(ArgosyFleet.Argosies[a].Groups[i].Flows[j].Statistics); k++ {
					el := ArgosyFleet.Argosies[a].Groups[i].Flows[j].Statistics[k].MashupDetailedElement
					el.Alias = "DataFlowStatistic"
					timeNanoSeconds := int64(ArgosyFleet.Argosies[a].Groups[i].Flows[j].Statistics[k].TimeSplit)
					timeSeconds := float64(timeNanoSeconds) * math.Pow(10.0, -9.0)
					dfstatData[el.Name] = timeSeconds
					statGroup = append(statGroup, timeSeconds)
					statwidgetElement := ttdisupport.FyneWidgetBundle{
						GuiWidgetBundle: mashupsdk.GuiWidgetBundle{
							GuiComponent:          widget.NewLabel(el.Name),
							MashupDetailedElement: &el, //mashupDetailedElementLibrary["{0}-SharedAttitude"],
						},
					}
					custosWorld.FyneWidgetElements["DataFlowStatistic"] = append(custosWorld.FyneWidgetElements["DataFlowStatistic"], &statwidgetElement)
					custosWorld.FyneWidgetElements[element.Name] = append(custosWorld.FyneWidgetElements[element.Name], &statwidgetElement)
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
	mashupRenderer := &g3nrender.MashupRenderer{}

	curveRenderer := &ttdirender.CurveRenderer{
		CollaboratingRenderer: &ttdirender.ElementRenderer{
			GenericRenderer: g3nrender.GenericRenderer{RendererType: g3nrender.LAYOUT},
		},
		TimeData:    dfstatData,
		SortedTimes: testTimes,
	}
	mashupRenderer.AddRenderer("Background", &ttdirender.BackgroundRenderer{})
	mashupRenderer.AddRenderer("Curve", curveRenderer)
	mashupRenderer.AddRenderer("Element", curveRenderer.CollaboratingRenderer)

	for _, detailedElement := range custosWorld.MashupDetailedElementLibrary {
		DetailedElements = append(DetailedElements, detailedElement)
	}
	log.Printf("Delivering mashup elements.\n")

	custosWorld.DetailedElements = DetailedElements

	custosWorld.Title = "Hello Custos"
	custosWorld.MainWindowSize = fyne.NewSize(800, 100)
	logoIconBytes, _ := logoIcon.ReadFile("logo.png")
	custosWorld.Icon = fyne.NewStaticResource("Logo", logoIconBytes)

	custosWorld.DetailMappedFyneComponent("Outside",
		"The magnetic field at any point outside the toroid is zero.",
		"Outside",
		DetailMappedTabItemFyneComponent)

	custosWorld.DetailMappedFyneComponent("Up-Side-Down",
		"Torus is up-side-down",
		"",
		DetailMappedTabItemFyneComponent)

	custosWorld.DetailMappedFyneComponent("All",
		"A group of torus or a tori.",
		"",
		DetailMappedTabItemFyneComponent)

	if !custosWorld.Headless {
		custosWorld.InitServer(*callerCreds, *insecure)
	}

	// Initialize the main window.
	custosWorld.InitMainWindow()

	<-worldCompleteChan
}
