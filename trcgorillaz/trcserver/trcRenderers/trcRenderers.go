package trcRenderers

// World is a basic gomobile app.
import (
	"log"
	"sort"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"

	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"
	"github.com/mrjrieke/nute/custos/custosworld"
	"github.com/mrjrieke/nute/mashupsdk"
)

type SearchRenderer struct {
	CustosWorldApp *custosworld.CustosWorldApp
}

func (cr *SearchRenderer) PreRender() {
}

func (cr *SearchRenderer) GetPriority() int64 {
	return 1
}

func (cr *SearchRenderer) BuildTabItem(childId int64, concreteElement *mashupsdk.MashupDetailedElement) {
	child := cr.CustosWorldApp.MashupDetailedElementLibrary[childId]
	if child != nil && child.Alias != "" {
		log.Printf("Controller lookup on: %s name: %s\n", child.Alias, child.Name)
		if fwb, fwbOk := cr.CustosWorldApp.FyneWidgetElements[child.Name]; fwbOk {
			if fwb.MashupDetailedElement != nil && fwb.GuiComponent != nil {
				fwb.MashupDetailedElement.Copy(child)
				fwb.GuiComponent.(*container.TabItem).Text = child.Name
			}
		} else {
			// No widget made yet for this alias...
			cr.CustosWorldApp.DetailFyneComponent(child,
				BuildDetailMappedTabItemFyneComponent)
		}
	}

	if child != nil && len(child.GetChildids()) > 0 {
		for _, cId := range child.GetChildids() {
			cr.BuildTabItem(cId, concreteElement)
		}
	}
}

func (cr *SearchRenderer) RenderTabItem(concreteElement *mashupsdk.MashupDetailedElement) {
	log.Printf("Controller Widget lookup: %s\n", concreteElement.Alias)

	if fyneWidgetElement, fyneOk := cr.CustosWorldApp.FyneWidgetElements[concreteElement.Name]; fyneOk {
		log.Printf("SearchRenderer lookup found: %s\n", concreteElement.Alias)
		if fyneWidgetElement.GuiComponent == nil {
			fyneWidgetElement.GuiComponent = cr.CustosWorldApp.CustomTabItems[concreteElement.Name](cr.CustosWorldApp, concreteElement.Name)
		}
		cr.CustosWorldApp.TabItemMenu.Append(fyneWidgetElement.GuiComponent.(*container.TabItem))
	}
}

func (cr *SearchRenderer) Refresh() {
}

type TenantDataRenderer struct {
	CustosWorldApp   *custosworld.CustosWorldApp
	ConcreteElements []*mashupsdk.MashupDetailedElement
}

func (tr *TenantDataRenderer) PreRender() {
	tr.ConcreteElements = []*mashupsdk.MashupDetailedElement{}
}

func (tr *TenantDataRenderer) GetPriority() int64 {
	return 2
}

func (tr *TenantDataRenderer) BuildTabItem(childId int64, concreteElement *mashupsdk.MashupDetailedElement) {
	child := tr.CustosWorldApp.MashupDetailedElementLibrary[childId]
	if child != nil && child.Alias != "" {
		log.Printf("TenantDataRenderer.BuildTabItem lookup on: %s name: %s\n", child.Alias, child.Name)
		if fwb, fwbOk := tr.CustosWorldApp.FyneWidgetElements[child.Name]; fwbOk {
			if fwb.MashupDetailedElement != nil && fwb.GuiComponent != nil {
				fwb.MashupDetailedElement.Copy(child)
				fwb.GuiComponent.(*container.TabItem).Text = child.Name
			}
		} else {
			// No widget made yet for this alias...
			tr.CustosWorldApp.DetailFyneComponent(child,
				BuildDetailMappedTabItemFyneComponent)
		}
	}

	if child != nil && len(child.GetChildids()) > 0 {
		for _, cId := range child.GetChildids() {
			tr.BuildTabItem(cId, concreteElement)
		}
	}
}

func (tr *TenantDataRenderer) renderTabItemHelper(concreteElement *mashupsdk.MashupDetailedElement) {
	log.Printf("TorusRender Widget lookup: %s\n", concreteElement.Alias)

	if concreteElement.IsStateSet(mashupsdk.Clicked) {
		log.Printf("TorusRender Widget looking up: %s\n", concreteElement.Alias)
		if fyneWidgetElement, fyneOk := tr.CustosWorldApp.FyneWidgetElements[concreteElement.Name]; fyneOk {
			log.Printf("TorusRender Widget lookup found: %s\n", concreteElement.Alias)
			if fyneWidgetElement.GuiComponent == nil {
				fyneWidgetElement.GuiComponent = tr.CustosWorldApp.CustomTabItems[concreteElement.Name](tr.CustosWorldApp, concreteElement.Name)
			}
			tr.CustosWorldApp.TabItemMenu.Append(fyneWidgetElement.GuiComponent.(*container.TabItem))
		}
	} else {
		// Remove it if torus.
		// CUWorldApp.fyneWidgetElements["Inside"].GuiComponent.(*container.TabItem),
		// Remove the formerly clicked elements..
		log.Printf("TorusRender Widget lookingup for remove: %s\n", concreteElement.Alias)
		if fyneWidgetElement, fyneOk := tr.CustosWorldApp.FyneWidgetElements[concreteElement.Name]; fyneOk {
			log.Printf("TorusRender Widget lookup found for remove: %s %v\n", concreteElement.Alias, fyneWidgetElement)
			if fyneWidgetElement.GuiComponent != nil {
				tr.CustosWorldApp.TabItemMenu.Remove(fyneWidgetElement.GuiComponent.(*container.TabItem))
			}
		}
	}
	log.Printf("End TorusRender Widget lookup: %s\n", concreteElement.Alias)
}

func (tr *TenantDataRenderer) RenderTabItem(concreteElement *mashupsdk.MashupDetailedElement) {
	tr.ConcreteElements = append(tr.ConcreteElements, concreteElement)
}

func (tr *TenantDataRenderer) Refresh() {
	sort.Slice(tr.ConcreteElements, func(i, j int) bool {
		return strings.Compare(tr.ConcreteElements[i].Name, tr.ConcreteElements[j].Name) == -1
	})
	for _, concreteElement := range tr.ConcreteElements {
		tr.renderTabItemHelper(concreteElement)
	}
}

func (tr *TenantDataRenderer) OnSelected(tabItem *container.TabItem) {
	// Too bad fyne doesn't have the ability for user to assign an id to TabItem...
	// Lookup by name instead and try to keep track of any name changes instead...
	log.Printf("Selected: %s\n", tabItem.Text)
	if mashupItemIndex, miOk := tr.CustosWorldApp.ElementLoaderIndex[tabItem.Text]; miOk {
		if mashupDetailedElement, mOk := tr.CustosWorldApp.MashupDetailedElementLibrary[mashupItemIndex]; mOk {
			if mashupDetailedElement.Name != "" {
				if mashupDetailedElement.Genre != "Collection" {
					mashupDetailedElement.State.State |= int64(mashupsdk.Clicked)
				}
				if fyneWidget, fOk := tr.CustosWorldApp.FyneWidgetElements[mashupDetailedElement.Name]; fOk {
					fyneWidget.MashupDetailedElement = mashupDetailedElement
					fyneWidget.OnStatusChanged()
				} else {
					log.Printf("Unexpected widget request: %s\n", mashupDetailedElement.Name)
				}
				return
			}
		}
	}
	//CUWorldApp.fyneWidgetElements[tabItem.Text].OnStatusChanged()
}

func BuildDetailMappedTabItemFyneComponent(CustosWorldApp *custosworld.CustosWorldApp, id string) *container.TabItem {
	de := CustosWorldApp.FyneWidgetElements[id].MashupDetailedElement
	//build tab item
	tabLabel := widget.NewLabel(de.Description)
	tabLabel.Wrapping = fyne.TextWrapWord
	dfgList := widget.NewList(
		func() int { return len(de.GetChildids()) },
		func() fyne.CanvasObject { return widget.NewLabel("") },
		func(lii widget.ListItemID, co fyne.CanvasObject) {
			co.(*widget.Label).SetText("List argosy dataflowgroups")
		},
	)
	//var dfList widget.List
	// for _, childID := range de.GetChildids() {
	// 	 :=
	// }

	// dfList := widget.NewList(
	// 	func() int { return len(CustosWorldApp.FyneWidgetElements["DataFlow"]) },
	// 	func() fyne.CanvasObject { return widget.NewLabel("") },
	// 	func(lii widget.ListItemID, co fyne.CanvasObject) {
	// 		co.(*widget.Label).SetText("List dfg's dataflows")
	// 	},
	// )

	// dfstatList := widget.NewList(
	// 	func() int { return len(CustosWorldApp.FyneWidgetElements["DataFlowStatistic"]) },
	// 	func() fyne.CanvasObject { return widget.NewLabel("") },
	// 	func(lii widget.ListItemID, co fyne.CanvasObject) {
	// 		co.(*widget.Label).SetText("List df's statistics")
	// 	},
	// )
	argosyMenu := container.NewAppTabs()
	tabItem := container.NewTabItem(id, container.NewBorder(nil, nil, layout.NewSpacer(), nil, container.NewVBox(tabLabel, container.NewAdaptiveGrid(2,
		argosyMenu,
		dfgList,
	))))
	return tabItem
}
