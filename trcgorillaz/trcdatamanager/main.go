package main

import (
	"embed"
	"flag"
	"log"
	"math"
	"os"
	"sort"
	"tierceron/buildopts/argosyopts"
	"tierceron/trcgorillaz/trcdatavisualizer/ttdirender"

	eUtils "tierceron/utils"
	helperkv "tierceron/vaulthelper/kv"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"

	//"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"
	"github.com/mrjrieke/nute/g3nd/worldg3n/g3nrender"
	"github.com/mrjrieke/nute/mashupsdk"
	"github.com/mrjrieke/nute/mashupsdk/client"
	"github.com/mrjrieke/nute/mashupsdk/guiboot"
)

type HelloContext struct {
	mashupContext *mashupsdk.MashupContext // Needed for callbacks to other mashups
}

type fyneMashupApiHandler struct {
}

var helloContext HelloContext

type FyneWidgetBundle struct {
	mashupsdk.GuiWidgetBundle
	Elements []*mashupsdk.MashupDetailedElement
}

type HelloApp struct {
	fyneMashupApiHandler         *fyneMashupApiHandler
	HelloContext                 *HelloContext
	mainWin                      fyne.Window
	mashupDisplayContext         *mashupsdk.MashupDisplayContext
	mashupDetailedElementLibrary map[int64]*mashupsdk.MashupDetailedElement
	elementLoaderIndex           map[string]int64 // mashup indexes by Name
	fyneWidgetElements           map[string][]*FyneWidgetBundle
}

func (fwb *FyneWidgetBundle) OnStatusChanged() {
	selectedDetailedElement := fwb.MashupDetailedElement
	if helloApp.HelloContext.mashupContext == nil {
		return
	}

	elementStateBundle := mashupsdk.MashupElementStateBundle{
		AuthToken:     client.GetServerAuthToken(),
		ElementStates: []*mashupsdk.MashupElementState{selectedDetailedElement.State},
	}
	helloApp.HelloContext.mashupContext.Client.ResetG3NDetailedElementStates(helloApp.HelloContext.mashupContext, &mashupsdk.MashupEmpty{AuthToken: client.GetServerAuthToken()})

	log.Printf("Display fields set to: %d", selectedDetailedElement.State.State)
	helloApp.HelloContext.mashupContext.Client.UpsertMashupElementsState(helloApp.HelloContext.mashupContext, &elementStateBundle)
}

func (ha *HelloApp) OnResize(displayHint *mashupsdk.MashupDisplayHint) {
	resize := ha.mashupDisplayContext.OnResize(displayHint)

	if ha.HelloContext.mashupContext == nil {
		return
	}

	if resize || !ha.mashupDisplayContext.MainWinDisplay.Focused {
		ha.mashupDisplayContext.MainWinDisplay.Focused = true
		ha.HelloContext.mashupContext.Client.OnResize(ha.HelloContext.mashupContext,
			&mashupsdk.MashupDisplayBundle{
				AuthToken:         client.GetServerAuthToken(),
				MashupDisplayHint: ha.mashupDisplayContext.MainWinDisplay,
			})
	}
}

// func (ha *HelloApp) TorusParser(childId int64) {
// 	child := helloApp.mashupDetailedElementLibrary[childId]
// 	if child.Alias != "" {
// 		helloApp.fyneWidgetElements[child.Alias].MashupDetailedElement.Copy(child)
// 		helloApp.fyneWidgetElements[child.Alias].GuiComponent.(*container.TabItem).Text = child.Name
// 	}

// 	if len(child.GetChildids()) > 0 {
// 		for _, cId := range child.GetChildids() {
// 			ha.TorusParser(cId)
// 		}

// 	}
// }

var helloApp HelloApp

//go:embed logo.png
var logo embed.FS

//go:embed tls/mashup.crt
var mashupCert embed.FS

//go:embed tls/mashup.key
var mashupKey embed.FS

func detailMappedFyneComponent(id, description string, de *mashupsdk.MashupDetailedElement) *container.TabItem {
	// tabLabel := widget.NewLabel(description)
	// tabLabel.Wrapping = fyne.TextWrapWord
	// tabItem := container.NewTabItem(id, container.NewBorder(nil, nil, layout.NewSpacer(), nil, container.NewVBox(tabLabel, container.NewAdaptiveGrid(2,
	// 	widget.NewButton("Show", func() {
	// 		// Workaround... mashupdetailedelement points at wrong element sometimes, but shouldn't!
	// 		mashupIndex := helloApp.elementLoaderIndex[helloApp.fyneWidgetElements[de.Alias].GuiComponent.(*container.TabItem).Text]
	// 		helloApp.fyneWidgetElements[de.Alias].MashupDetailedElement = helloApp.mashupDetailedElementLibrary[mashupIndex]

	// 		helloApp.fyneWidgetElements[de.Alias].MashupDetailedElement.ApplyState(mashupsdk.Hidden, false)
	// 		if helloApp.fyneWidgetElements[de.Alias].MashupDetailedElement.Genre == "Collection" {
	// 			helloApp.fyneWidgetElements[de.Alias].MashupDetailedElement.ApplyState(mashupsdk.Recursive, true)
	// 		}
	// 		helloApp.fyneWidgetElements[de.Alias].OnStatusChanged()
	// 	}), widget.NewButton("Hide", func() {
	// 		// Workaround... mashupdetailedelement points at wrong element sometimes, but shouldn't!
	// 		mashupIndex := helloApp.elementLoaderIndex[helloApp.fyneWidgetElements[de.Alias].GuiComponent.(*container.TabItem).Text]
	// 		helloApp.fyneWidgetElements[de.Alias].MashupDetailedElement = helloApp.mashupDetailedElementLibrary[mashupIndex]

	// 		helloApp.fyneWidgetElements[de.Alias].MashupDetailedElement.ApplyState(mashupsdk.Hidden, true)
	// 		if helloApp.fyneWidgetElements[de.Alias].MashupDetailedElement.Genre == "Collection" {
	// 			helloApp.fyneWidgetElements[de.Alias].MashupDetailedElement.ApplyState(mashupsdk.Recursive, true)
	// 		}
	// 		helloApp.fyneWidgetElements[de.Alias].OnStatusChanged()
	// 	})))),
	// )
	return nil //tabItem
}

func main() {
	insecure := flag.Bool("insecure", false, "Skip server validation")
	envPtr := flag.String("env", "QA", "Environment to configure")
	flag.Parse()

	helloLog, err := os.OpenFile("ttdimanager.log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0666)
	if err != nil {
		log.Fatalf(err.Error(), err)
	}
	logger := log.New(helloLog, "[ttdivisualizer]", log.LstdFlags)
	//log.SetOutput(helloLog)

	mashupsdk.InitCertKeyPair(mashupCert, mashupKey)

	helloApp = HelloApp{
		fyneMashupApiHandler:         &fyneMashupApiHandler{},
		HelloContext:                 &HelloContext{},
		mainWin:                      nil,
		mashupDisplayContext:         &mashupsdk.MashupDisplayContext{MainWinDisplay: &mashupsdk.MashupDisplayHint{}},
		mashupDetailedElementLibrary: map[int64]*mashupsdk.MashupDetailedElement{}, // mashupDetailedElementLibrary,
		elementLoaderIndex:           map[string]int64{},                           // elementLoaderIndex
		fyneWidgetElements: map[string][]*FyneWidgetBundle{
			"Outside": []*FyneWidgetBundle{
				{
					GuiWidgetBundle: mashupsdk.GuiWidgetBundle{
						GuiComponent:          nil,
						MashupDetailedElement: &mashupsdk.MashupDetailedElement{}, //mashupDetailedElementLibrary["Outside"],
					},
				},
			},
			"Argosy": []*FyneWidgetBundle{
				{
					GuiWidgetBundle: mashupsdk.GuiWidgetBundle{
						GuiComponent:          nil,
						MashupDetailedElement: &mashupsdk.MashupDetailedElement{}, //mashupDetailedElementLibrary["{0}-Torus"],
					},
				},
			},
			"DataFlowGroup": []*FyneWidgetBundle{
				{
					GuiWidgetBundle: mashupsdk.GuiWidgetBundle{
						GuiComponent:          nil,
						MashupDetailedElement: &mashupsdk.MashupDetailedElement{}, //mashupDetailedElementLibrary["{0}-SharedAttitude"],
					},
				},
			},
			"DataFlow": []*FyneWidgetBundle{
				{
					GuiWidgetBundle: mashupsdk.GuiWidgetBundle{
						GuiComponent:          nil,
						MashupDetailedElement: &mashupsdk.MashupDetailedElement{}, //mashupDetailedElementLibrary["{0}-SharedAttitude"],
					},
				},
			},
			"DataFlowStatistic": []*FyneWidgetBundle{
				{
					GuiWidgetBundle: mashupsdk.GuiWidgetBundle{
						GuiComponent:          nil,
						MashupDetailedElement: &mashupsdk.MashupDetailedElement{}, //mashupDetailedElementLibrary["{0}-SharedAttitude"],
					},
				},
			},
		},
	}

	// Build G3nDetailedElement cache.

	// Sync initialization.
	initHandler := func(a fyne.App) {
		a.Lifecycle().SetOnExitedForeground(func() {
			log.Printf("OnExitedForeground.\n")
			helloApp.mashupDisplayContext.MainWinDisplay.Focused = false
		})

		a.Lifecycle().SetOnStarted(func() {
			log.Printf("SetOnEnteredForeground: %v\n", helloApp.mashupDisplayContext.MainWinDisplay.Focused)
		})

		a.Lifecycle().SetOnEnteredForeground(func() {
			log.Printf("SetOnEnteredForeground: %v\n", helloApp.mashupDisplayContext.MainWinDisplay.Focused)
			if helloApp.HelloContext.mashupContext == nil {
				helloApp.HelloContext.mashupContext = client.BootstrapInit("ttdivisualizer", helloApp.fyneMashupApiHandler, nil, nil, insecure)

				var upsertErr error
				var concreteElementBundle *mashupsdk.MashupDetailedElementBundle

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

				ArgosyFleet, argosyErr := argosyopts.BuildFleet(mod, logger)
				eUtils.CheckError(&config, argosyErr, true)

				DetailedElements := []*mashupsdk.MashupDetailedElement{}
				dfstatData := map[string]float64{}
				statGroup := []float64{}
				testTimes := []float64{}
				for a := 0; a < len(ArgosyFleet.Argosies); a++ {
					argosyBasis := ArgosyFleet.Argosies[a].MashupDetailedElement
					argosyBasis.Alias = "Argosy"
					argwidgetElement := FyneWidgetBundle{
						GuiWidgetBundle: mashupsdk.GuiWidgetBundle{
							GuiComponent:          widget.NewLabel(argosyBasis.Name),
							MashupDetailedElement: &argosyBasis, //mashupDetailedElementLibrary["{0}-SharedAttitude"],
						},
					}
					helloApp.fyneWidgetElements["Argosy"] = append(helloApp.fyneWidgetElements["Argosy"], &argwidgetElement)
					helloApp.fyneWidgetElements[argosyBasis.Name] = append(helloApp.fyneWidgetElements[argosyBasis.Name], &argwidgetElement)
					DetailedElements = append(DetailedElements, &argosyBasis)

					for i := 0; i < len(ArgosyFleet.Argosies[a].Groups); i++ {
						detailedElement := ArgosyFleet.Argosies[a].Groups[i].MashupDetailedElement
						detailedElement.Alias = "DataFlowGroup"
						dfgwidgetElement := FyneWidgetBundle{
							GuiWidgetBundle: mashupsdk.GuiWidgetBundle{
								GuiComponent:          widget.NewLabel(detailedElement.Name),
								MashupDetailedElement: &detailedElement, //mashupDetailedElementLibrary["{0}-SharedAttitude"],
							},
						}
						helloApp.fyneWidgetElements[argosyBasis.Name] = append(helloApp.fyneWidgetElements[argosyBasis.Name], &dfgwidgetElement)
						// MAKE IT SO HAVE SAME LIST ID AS PREVIOUS ELEMENT TO NAVIGATE THRU AND LINK THEM
						helloApp.fyneWidgetElements["DataFlowGroup"] = append(helloApp.fyneWidgetElements["DataFlowGroup"], &dfgwidgetElement)
						//HAVE TO REDO THIS SO THAT ARRAY ISN'T RESET EA TIME
						// MAKE A FYNE WIDGET ELEMENT WITH NIL GUI COMP AND SET MASHUPEL TO GIVEN EL AT TIME
						DetailedElements = append(DetailedElements, &detailedElement)
						for j := 0; j < len(ArgosyFleet.Argosies[a].Groups[i].Flows); j++ {
							element := ArgosyFleet.Argosies[a].Groups[i].Flows[j].MashupDetailedElement
							element.Alias = "DataFlow"
							dfwidgetElement := FyneWidgetBundle{
								GuiWidgetBundle: mashupsdk.GuiWidgetBundle{
									GuiComponent:          widget.NewLabel(element.Name),
									MashupDetailedElement: &element, //mashupDetailedElementLibrary["{0}-SharedAttitude"],
								},
							}
							helloApp.fyneWidgetElements["DataFlow"] = append(helloApp.fyneWidgetElements["DataFlow"], &dfwidgetElement)
							helloApp.fyneWidgetElements[detailedElement.Name] = append(helloApp.fyneWidgetElements[argosyBasis.Name], &dfwidgetElement)
							DetailedElements = append(DetailedElements, &element)
							for k := 0; k < len(ArgosyFleet.Argosies[a].Groups[i].Flows[j].Statistics); k++ {
								el := ArgosyFleet.Argosies[a].Groups[i].Flows[j].Statistics[k].MashupDetailedElement
								el.Alias = "DataFlowStatistic"
								timeNanoSeconds := int64(ArgosyFleet.Argosies[a].Groups[i].Flows[j].Statistics[k].TimeSplit)
								timeSeconds := float64(timeNanoSeconds) * math.Pow(10.0, -9.0)
								dfstatData[el.Name] = timeSeconds
								statGroup = append(statGroup, timeSeconds)
								statwidgetElement := FyneWidgetBundle{
									GuiWidgetBundle: mashupsdk.GuiWidgetBundle{
										GuiComponent:          widget.NewLabel(el.Name),
										MashupDetailedElement: &el, //mashupDetailedElementLibrary["{0}-SharedAttitude"],
									},
								}
								helloApp.fyneWidgetElements["DataFlowStatistic"] = append(helloApp.fyneWidgetElements["DataFlowStatistic"], &statwidgetElement)
								helloApp.fyneWidgetElements[element.Name] = append(helloApp.fyneWidgetElements[element.Name], &statwidgetElement)
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

				for _, detailedElement := range helloApp.mashupDetailedElementLibrary {
					DetailedElements = append(DetailedElements, detailedElement)
				}
				log.Printf("Delivering mashup elements.\n")

				// Connection with mashup fully established.  Initialize mashup elements.
				concreteElementBundle, upsertErr = helloApp.HelloContext.mashupContext.Client.UpsertMashupElements(helloApp.HelloContext.mashupContext,
					&mashupsdk.MashupDetailedElementBundle{
						AuthToken:        client.GetServerAuthToken(),
						DetailedElements: DetailedElements,
					})

				if upsertErr != nil {
					log.Printf("Element state initialization failure: %s\n", upsertErr.Error())
				}

				for _, concreteElement := range concreteElementBundle.DetailedElements {
					//helloApp.fyneComponentCache[generatedComponent.Basisid]
					helloApp.mashupDetailedElementLibrary[concreteElement.Id] = concreteElement
					helloApp.elementLoaderIndex[concreteElement.Name] = concreteElement.Id

					if concreteElement.GetName() == "Outside" {
						helloApp.fyneWidgetElements["Outside"][0].MashupDetailedElement.Copy(concreteElement) //assuming only 1 outside element!
					}
				}

				// for _, concreteElement := range concreteElementBundle.DetailedElements {
				// 	if concreteElement.GetSubgenre() == "Torus" {
				// 		helloApp.TorusParser(concreteElement.Id)
				// 	}
				// }

				log.Printf("Mashup elements delivered.\n")

				helloApp.mashupDisplayContext.ApplySettled(mashupsdk.AppInitted, false)
			}
			if helloApp.mashupDisplayContext.MainWinDisplay != nil {
				helloApp.OnResize(helloApp.mashupDisplayContext.MainWinDisplay)
				helloApp.mashupDisplayContext.MainWinDisplay.Focused = true
			}
		})

		a.Lifecycle().SetOnResized(func(xpos int, ypos int, yoffset int, width int, height int) {
			log.Printf("Received resize: %d %d %d %d %d\n", xpos, ypos, yoffset, width, height)
			helloApp.mashupDisplayContext.ApplySettled(mashupsdk.Configured|mashupsdk.Position|mashupsdk.Frame, false)

			if helloApp.mashupDisplayContext.GetYoffset() == 0 {
				helloApp.mashupDisplayContext.SetYoffset(yoffset + 3)
			}
			focused := false
			if helloApp.mashupDisplayContext.MainWinDisplay != nil {
				focused = helloApp.mashupDisplayContext.MainWinDisplay.Focused
			}
			helloApp.mashupDisplayContext.MainWinDisplay = &mashupsdk.MashupDisplayHint{
				Focused: focused,
				Xpos:    int64(xpos),
				Ypos:    int64(ypos),
				Width:   int64(width),
				Height:  int64(height),
			}

			helloApp.OnResize(helloApp.mashupDisplayContext.MainWinDisplay)
		})
		helloApp.mainWin = a.NewWindow("Hello Fyne World")
		logoIconBytes, _ := logo.ReadFile("logo.png")

		helloApp.mainWin.SetIcon(fyne.NewStaticResource("Logo", logoIconBytes))
		helloApp.mainWin.Resize(fyne.NewSize(800, 100))
		helloApp.mainWin.SetFixedSize(false)

		argosyList := widget.NewList(
			func() int { return len(helloApp.fyneWidgetElements["Argosy"]) },
			func() fyne.CanvasObject { return widget.NewLabel("") },
			func(lii widget.ListItemID, co fyne.CanvasObject) {
				co.(*widget.Label).SetText(helloApp.fyneWidgetElements["Argosy"][lii].MashupDetailedElement.Name)
			},
		)

		dfgList := widget.NewList(
			func() int { return len(helloApp.fyneWidgetElements["DataFlowGroup"]) },
			func() fyne.CanvasObject { return widget.NewLabel("") },
			func(lii widget.ListItemID, co fyne.CanvasObject) {
				co.(*widget.Label).SetText(helloApp.fyneWidgetElements["DataFlowGroup"][lii].MashupDetailedElement.Name)
			},
		)

		dfList := widget.NewList(
			func() int { return len(helloApp.fyneWidgetElements["DataFlow"]) },
			func() fyne.CanvasObject { return widget.NewLabel("") },
			func(lii widget.ListItemID, co fyne.CanvasObject) {
				co.(*widget.Label).SetText(helloApp.fyneWidgetElements["DataFlow"][lii].MashupDetailedElement.Name)
			},
		)

		dfstatList := widget.NewList(
			func() int { return len(helloApp.fyneWidgetElements["DataFlowStatistic"]) },
			func() fyne.CanvasObject { return widget.NewLabel("") },
			func(lii widget.ListItemID, co fyne.CanvasObject) {
				co.(*widget.Label).SetText(helloApp.fyneWidgetElements["DataFlowStatistic"][lii].MashupDetailedElement.Name)
			},
		)

		helloApp.fyneWidgetElements["Argosy"][0].GuiComponent = container.NewTabItem("Argosy", argosyList)
		helloApp.fyneWidgetElements["DataFlowGroup"][0].GuiComponent = container.NewTabItem("DataFlowGroup", dfgList)
		helloApp.fyneWidgetElements["DataFlow"][0].GuiComponent = container.NewTabItem("DataFlow", dfList)
		helloApp.fyneWidgetElements["DataFlowStatistic"][0].GuiComponent = container.NewTabItem("DataFlowStatistic", dfstatList)
		menu := container.NewAppTabs(
			helloApp.fyneWidgetElements["Argosy"][0].GuiComponent.(*container.TabItem),
			helloApp.fyneWidgetElements["DataFlowGroup"][0].GuiComponent.(*container.TabItem),
			helloApp.fyneWidgetElements["DataFlow"][0].GuiComponent.(*container.TabItem),
			helloApp.fyneWidgetElements["DataFlowStatistic"][0].GuiComponent.(*container.TabItem),
		)

		argosyList.OnSelected = func(id widget.ListItemID) {
			menu.Select(helloApp.fyneWidgetElements["DataFlowGroup"][0].GuiComponent.(*container.TabItem))
			//FIGURE OUT HOW TO MAKE IT CHANGE DATA IN LIST IF OTHER ELEMENT SELECTED
			// dfgList = widget.NewList(
			// 	func() int {
			// 		if helloApp.fyneWidgetElements[helloApp.fyneWidgetElements["Argosy"][id].MashupDetailedElement.Name] != nil {
			// 			return len(helloApp.fyneWidgetElements[helloApp.fyneWidgetElements["Argosy"][id].MashupDetailedElement.Name])
			// 		} else {
			// 			return len(helloApp.fyneWidgetElements["DataFlowGroup"])
			// 		}
			// 	},
			// 	func() fyne.CanvasObject { return widget.NewLabel("") },
			// 	func(lii widget.ListItemID, co fyne.CanvasObject) {
			// 		if helloApp.fyneWidgetElements[helloApp.fyneWidgetElements["Argosy"][lii].MashupDetailedElement.Name] != nil {
			// 			co.(*widget.Label).SetText(helloApp.fyneWidgetElements[helloApp.fyneWidgetElements["Argosy"][id].MashupDetailedElement.Name][id].MashupDetailedElement.Name)
			// 		} else {
			// 			co.(*widget.Label).SetText("hi") //helloApp.fyneWidgetElements["DataFlowGroup"][lii].MashupDetailedElement.Name)
			// 		}
			// 	},
			// )
		}

		dfgList.OnSelected = func(id widget.ListItemID) {
			menu.Select(helloApp.fyneWidgetElements["DataFlow"][0].GuiComponent.(*container.TabItem))
			//FIGURE OUT HOW TO MAKE IT CHANGE DATA IN LIST IF OTHER ELEMENT SELECTED
			// dfgList = widget.NewList(
			// 	func() int {
			// 		if helloApp.fyneWidgetElements[helloApp.fyneWidgetElements["Argosy"][id].MashupDetailedElement.Name] != nil {
			// 			return len(helloApp.fyneWidgetElements[helloApp.fyneWidgetElements["Argosy"][id].MashupDetailedElement.Name])
			// 		} else {
			// 			return len(helloApp.fyneWidgetElements["DataFlowGroup"])
			// 		}
			// 	},
			// 	func() fyne.CanvasObject { return widget.NewLabel("") },
			// 	func(lii widget.ListItemID, co fyne.CanvasObject) {
			// 		if helloApp.fyneWidgetElements[helloApp.fyneWidgetElements["Argosy"][lii].MashupDetailedElement.Name] != nil {
			// 			co.(*widget.Label).SetText(helloApp.fyneWidgetElements[helloApp.fyneWidgetElements["Argosy"][id].MashupDetailedElement.Name][id].MashupDetailedElement.Name)
			// 		} else {
			// 			co.(*widget.Label).SetText("hi") //helloApp.fyneWidgetElements["DataFlowGroup"][lii].MashupDetailedElement.Name)
			// 		}
			// 	},
			// )
		}

		dfList.OnSelected = func(id widget.ListItemID) {
			menu.Select(helloApp.fyneWidgetElements["DataFlowStatistic"][0].GuiComponent.(*container.TabItem))
			//FIGURE OUT HOW TO MAKE IT CHANGE DATA IN LIST IF OTHER ELEMENT SELECTED
			// dfgList = widget.NewList(
			// 	func() int {
			// 		if helloApp.fyneWidgetElements[helloApp.fyneWidgetElements["Argosy"][id].MashupDetailedElement.Name] != nil {
			// 			return len(helloApp.fyneWidgetElements[helloApp.fyneWidgetElements["Argosy"][id].MashupDetailedElement.Name])
			// 		} else {
			// 			return len(helloApp.fyneWidgetElements["DataFlowGroup"])
			// 		}
			// 	},
			// 	func() fyne.CanvasObject { return widget.NewLabel("") },
			// 	func(lii widget.ListItemID, co fyne.CanvasObject) {
			// 		if helloApp.fyneWidgetElements[helloApp.fyneWidgetElements["Argosy"][lii].MashupDetailedElement.Name] != nil {
			// 			co.(*widget.Label).SetText(helloApp.fyneWidgetElements[helloApp.fyneWidgetElements["Argosy"][id].MashupDetailedElement.Name][id].MashupDetailedElement.Name)
			// 		} else {
			// 			co.(*widget.Label).SetText("hi") //helloApp.fyneWidgetElements["DataFlowGroup"][lii].MashupDetailedElement.Name)
			// 		}
			// 	},
			// )
		}

		// menu.OnSelected = func(tabItem *container.TabItem) {
		// 	// Too bad fyne doesn't have the ability for user to assign an id to TabItem...
		// 	// Lookup by name instead and try to keep track of any name changes instead...
		// 	log.Printf("Selected: %s\n", tabItem.Text)
		// 	if mashupItemIndex, miOk := helloApp.elementLoaderIndex[tabItem.Text]; miOk {
		// 		mashupDetailedElement := helloApp.mashupDetailedElementLibrary[mashupItemIndex]
		// 		if mashupDetailedElement.Alias != "" {
		// 			if mashupDetailedElement.Genre != "Collection" {
		// 				mashupDetailedElement.State.State |= int64(mashupsdk.Clicked)
		// 			}
		// 			helloApp.fyneWidgetElements[mashupDetailedElement.Alias].MashupDetailedElement = mashupDetailedElement
		// 			helloApp.fyneWidgetElements[mashupDetailedElement.Alias].OnStatusChanged()
		// 			return
		// 		}
		// 	}
		// 	helloApp.fyneWidgetElements[tabItem.Text].OnStatusChanged()
		// }

		// helloApp.fyneWidgetElements["Outside"].GuiComponent = detailMappedFyneComponent("Inside", "The magnetic field inside a toroid is always tangential to the circular closed path.  These magnetic field lines are concentric circles.", helloApp.fyneWidgetElements["Outside"].MashupDetailedElement)
		// helloApp.fyneWidgetElements["Argosy"].GuiComponent = detailMappedFyneComponent("Outside", "The magnetic field at any point outside the toroid is zero.", helloApp.fyneWidgetElements["Argosy"].MashupDetailedElement)
		// helloApp.fyneWidgetElements["DataFlowGroup"].GuiComponent = detailMappedFyneComponent("It", "The magnetic field inside the empty space surrounded by the toroid is zero.", helloApp.fyneWidgetElements["DataFlowGroup"].MashupDetailedElement)
		// helloApp.fyneWidgetElements["DataFlow"].GuiComponent = detailMappedFyneComponent("Up-Side-Down", "Torus is up-side-down", helloApp.fyneWidgetElements["DataFlow"].MashupDetailedElement)
		// helloApp.fyneWidgetElements["DataFlowStatistic"].GuiComponent = detailMappedFyneComponent("All", "A group of torus or a tori.", helloApp.fyneWidgetElements["DataFlowStatistic"].MashupDetailedElement)

		// torusMenu := container.NewAppTabs(
		// 	helloApp.fyneWidgetElements["Outside"].GuiComponent.(*container.TabItem),
		// 	helloApp.fyneWidgetElements["Argosy"].GuiComponent.(*container.TabItem),
		// 	helloApp.fyneWidgetElements["DataFlowGroup"].GuiComponent.(*container.TabItem),
		// 	helloApp.fyneWidgetElements["DataFlow"].GuiComponent.(*container.TabItem),
		// 	helloApp.fyneWidgetElements["DataFlowStatistic"].GuiComponent.(*container.TabItem),
		// )
		// torusMenu.OnSelected = func(tabItem *container.TabItem) {
		// 	// Too bad fyne doesn't have the ability for user to assign an id to TabItem...
		// 	// Lookup by name instead and try to keep track of any name changes instead...
		// 	log.Printf("Selected: %s\n", tabItem.Text)
		// 	if mashupItemIndex, miOk := helloApp.elementLoaderIndex[tabItem.Text]; miOk {
		// 		mashupDetailedElement := helloApp.mashupDetailedElementLibrary[mashupItemIndex]
		// 		if mashupDetailedElement.Alias != "" {
		// 			if mashupDetailedElement.Genre != "Collection" {
		// 				mashupDetailedElement.State.State |= int64(mashupsdk.Clicked)
		// 			}
		// 			helloApp.fyneWidgetElements[mashupDetailedElement.Alias].MashupDetailedElement = mashupDetailedElement
		// 			helloApp.fyneWidgetElements[mashupDetailedElement.Alias].OnStatusChanged()
		// 			return
		// 		}
		// 	}
		// 	helloApp.fyneWidgetElements[tabItem.Text].OnStatusChanged()
		// }

		// torusMenu.SetTabLocation(container.TabLocationTop)
		menu.SetTabLocation(container.TabLocationTop)
		helloApp.mainWin.SetContent(menu)
		helloApp.mainWin.SetCloseIntercept(func() {
			if helloApp.HelloContext.mashupContext != nil {
				helloApp.HelloContext.mashupContext.Client.Shutdown(helloApp.HelloContext.mashupContext, &mashupsdk.MashupEmpty{AuthToken: client.GetServerAuthToken()})
			}
			os.Exit(0)
		})
	}

	// Async handler.
	runtimeHandler := func() {
		helloApp.mainWin.ShowAndRun()
	}

	guiboot.InitMainWindow(guiboot.Fyne, initHandler, runtimeHandler)
}

func (mSdk *fyneMashupApiHandler) OnResize(displayHint *mashupsdk.MashupDisplayHint) {
	log.Printf("Fyne OnResize - not implemented yet..\n")
	if helloApp.mainWin != nil {
		helloApp.mashupDisplayContext.MainWinDisplay.Focused = displayHint.Focused
		// TODO: Resize without infinite looping....
		// The moment fyne is resized, it'll want to resize g3n...
		// Which then wants to resize fyne ad-infinitum
		//helloApp.mainWin.PosResize(int(displayHint.Xpos), int(displayHint.Ypos), int(displayHint.Width), int(displayHint.Height))
		log.Printf("Fyne Received onResize xpos: %d ypos: %d width: %d height: %d ytranslate: %d\n", int(displayHint.Xpos), int(displayHint.Ypos), int(displayHint.Width), int(displayHint.Height), int(displayHint.Ypos+displayHint.Height))
	} else {
		log.Printf("Fyne Could not apply xpos: %d ypos: %d width: %d height: %d ytranslate: %d\n", int(displayHint.Xpos), int(displayHint.Ypos), int(displayHint.Width), int(displayHint.Height), int(displayHint.Ypos+displayHint.Height))
	}
}

func (mSdk *fyneMashupApiHandler) GetMashupElements() (*mashupsdk.MashupDetailedElementBundle, error) {
	log.Printf("Fyne GetMashupElements - not implemented\n")
	return &mashupsdk.MashupDetailedElementBundle{}, nil
}

func (mSdk *fyneMashupApiHandler) UpsertMashupElements(detailedElementBundle *mashupsdk.MashupDetailedElementBundle) (*mashupsdk.MashupDetailedElementBundle, error) {
	log.Printf("Fyne UpsertMashupElements - not implemented\n")
	return &mashupsdk.MashupDetailedElementBundle{}, nil
}

func (mSdk *fyneMashupApiHandler) ResetG3NDetailedElementStates() {
	log.Printf("Fyne ResetG3NDetailedElementStates - not implemented\n")
}

func (mSdk *fyneMashupApiHandler) UpsertMashupElementsState(elementStateBundle *mashupsdk.MashupElementStateBundle) (*mashupsdk.MashupElementStateBundle, error) {
	// log.Printf("Fyne UpsertMashupElementsState called\n")
	// for _, es := range elementStateBundle.ElementStates {
	// 	detailedElement := helloApp.mashupDetailedElementLibrary[es.GetId()]

	// 	helloApp.fyneWidgetElements[detailedElement.GetAlias()].MashupDetailedElement = detailedElement
	// 	helloApp.fyneWidgetElements[detailedElement.GetAlias()].MashupDetailedElement.State.State = es.State

	// 	if (mashupsdk.DisplayElementState(es.State) & mashupsdk.Clicked) == mashupsdk.Clicked {
	// 		for _, childId := range detailedElement.GetChildids() {
	// 			if childDetailedElement, childDetailOk := helloApp.mashupDetailedElementLibrary[childId]; childDetailOk {
	// 				if childFyneComponent, childFyneOk := helloApp.fyneWidgetElements[childDetailedElement.GetAlias()]; childFyneOk {
	// 					childFyneComponent.MashupDetailedElement = childDetailedElement
	// 					childFyneComponent.GuiComponent.(*container.TabItem).Text = childFyneComponent.MashupDetailedElement.Name
	// 				}
	// 			}
	// 		}
	// 		for _, parentId := range detailedElement.GetParentids() {
	// 			if parentDetailedElement, parentDetailOk := helloApp.mashupDetailedElementLibrary[parentId]; parentDetailOk {
	// 				if parentFyneComponent, parentFyneOk := helloApp.fyneWidgetElements[parentDetailedElement.GetAlias()]; parentFyneOk {
	// 					parentFyneComponent.MashupDetailedElement = parentDetailedElement
	// 					parentFyneComponent.GuiComponent.(*container.TabItem).Text = parentFyneComponent.MashupDetailedElement.Name
	// 				}
	// 			}
	// 		}
	// 		if detailedLookupElement, detailLookupOk := helloApp.mashupDetailedElementLibrary[detailedElement.Id]; detailLookupOk {
	// 			if detailedFyneComponent, detailedFyneOk := helloApp.fyneWidgetElements[detailedLookupElement.GetAlias()]; detailedFyneOk {
	// 				detailedFyneComponent.MashupDetailedElement = detailedLookupElement
	// 				detailedFyneComponent.GuiComponent.(*container.TabItem).Text = detailedFyneComponent.MashupDetailedElement.Name
	// 			}
	// 		}
	// 		torusMenu := helloApp.mainWin.Content().(*container.AppTabs)
	// 		// Select the item.
	// 		helloApp.fyneWidgetElements[detailedElement.GetAlias()].GuiComponent.(*container.TabItem).Text = helloApp.fyneWidgetElements[detailedElement.GetAlias()].MashupDetailedElement.Name
	// 		torusMenu.Select(helloApp.fyneWidgetElements[detailedElement.GetAlias()].GuiComponent.(*container.TabItem))
	// 	}
	// }
	// log.Printf("Fyne UpsertMashupElementsState complete\n")
	return &mashupsdk.MashupElementStateBundle{}, nil
}
