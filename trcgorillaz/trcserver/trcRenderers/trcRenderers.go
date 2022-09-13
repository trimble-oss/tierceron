package trcRenderers

// World is a basic gomobile app.
import (
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
	//log.Printf("SearchRenderer PreRender called")
}

func (cr *SearchRenderer) GetPriority() int64 {
	return 1
}

func (cr *SearchRenderer) BuildTabItem(childId int64, concreteElement *mashupsdk.MashupDetailedElement) {
	//log.Printf("BuildTabItem called - SearchRenderer")
	child := cr.CustosWorldApp.MashupDetailedElementLibrary[childId]
	if child != nil && child.Alias != "" {
		//log.Printf("Controller lookup on: %s name: %s\n", child.Alias, child.Name)
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
	//log.Printf("Controller Widget lookup: %s\n", concreteElement.Alias)
	//log.Printf("RenderTabItem called - SearchRenderer")

	if fyneWidgetElement, fyneOk := cr.CustosWorldApp.FyneWidgetElements[concreteElement.Name]; fyneOk {
		//log.Printf("SearchRenderer lookup found: %s\n", concreteElement.Alias)
		if fyneWidgetElement.GuiComponent == nil {
			fyneWidgetElement.GuiComponent = cr.CustosWorldApp.CustomTabItems[concreteElement.Name](cr.CustosWorldApp, concreteElement.Name)
		}
		cr.CustosWorldApp.TabItemMenu.Append(fyneWidgetElement.GuiComponent.(*container.TabItem))
	}
}

func (cr *SearchRenderer) Refresh() {
	//log.Printf("Refresh called - SearchRenderer")
}

type TenantDataRenderer struct {
	CustosWorldApp      *custosworld.CustosWorldApp
	ConcreteElements    []*mashupsdk.MashupDetailedElement
	ClickedElements     []*mashupsdk.MashupDetailedElement
	CurrentListElements []*mashupsdk.MashupDetailedElement
	Elementlist         *widget.List
	DataMenu            *container.AppTabs
	InitialListElements []*mashupsdk.MashupDetailedElement
	DataTabs            []*container.TabItem
}

func (tr *TenantDataRenderer) GetPriority() int64 { //1
	//log.Printf("GetPriority called - TenantDataRenderer")
	return 2
}

func (tr *TenantDataRenderer) BuildTabItem(childId int64, concreteElement *mashupsdk.MashupDetailedElement) { //2
	//log.Printf("BuildTabItem called - TenantDataRenderer")
	child := tr.CustosWorldApp.MashupDetailedElementLibrary[childId]
	if child != nil && child.Alias != "" {
		//log.Printf("TenantDataRenderer.BuildTabItem lookup on: %s name: %s\n", child.Alias, child.Name)
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

func (tr *TenantDataRenderer) PreRender() { //3
	//log.Printf("PreRender called - TenantDataRenderer")
	tr.ConcreteElements = []*mashupsdk.MashupDetailedElement{}
}

func (tr *TenantDataRenderer) renderTabItemHelper(concreteElement *mashupsdk.MashupDetailedElement) {
	//log.Printf("renderTabItemHelper called - TenantDataRenderer")
	//log.Printf("TorusRender Widget lookup: %s\n", concreteElement.Alias)
	//log.Print(spew.Sdump(concreteElement))
	if concreteElement.IsStateSet(mashupsdk.Clicked) {
		//log.Printf("TorusRender Widget looking up: %s\n", concreteElement.Alias)
		if fyneWidgetElement, fyneOk := tr.CustosWorldApp.FyneWidgetElements[concreteElement.Name]; fyneOk {
			//log.Printf("TorusRender Widget lookup found: %s\n", concreteElement.Alias)
			if fyneWidgetElement.GuiComponent == nil {
				fyneWidgetElement.GuiComponent = tr.CustosWorldApp.CustomTabItems[concreteElement.Name](tr.CustosWorldApp, concreteElement.Name)
			}
			tr.CustosWorldApp.TabItemMenu.Append(fyneWidgetElement.GuiComponent.(*container.TabItem))
		}
	} else {
		// Remove it if torus.
		// CUWorldApp.fyneWidgetElements["Inside"].GuiComponent.(*container.TabItem),
		// Remove the formerly clicked elements..
		//log.Printf("TorusRender Widget lookingup for remove: %s\n", concreteElement.Alias)
		if fyneWidgetElement, fyneOk := tr.CustosWorldApp.FyneWidgetElements[concreteElement.Name]; fyneOk {
			//log.Printf("TorusRender Widget lookup found for remove: %s %v\n", concreteElement.Alias, fyneWidgetElement)
			if fyneWidgetElement.GuiComponent != nil {
				tr.CustosWorldApp.TabItemMenu.Remove(fyneWidgetElement.GuiComponent.(*container.TabItem))
			}
		}
	}
	//log.Printf("End TorusRender Widget lookup: %s\n", concreteElement.Alias)
}

func (tr *TenantDataRenderer) RenderTabItem(concreteElement *mashupsdk.MashupDetailedElement) { //4
	//log.Printf("RenderTabItem called - TenantDataRenderer")
	tr.ConcreteElements = append(tr.ConcreteElements, concreteElement)
}

func (tr *TenantDataRenderer) Refresh() { //5
	//log.Printf("Refresh called - TenantDataRenderer")
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
	//log.Printf("OnSelected called - TenantDataRenderer")
	//log.Printf("Selected: %s\n", tabItem.Text)
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
					//log.Printf("Unexpected widget request: %s\n", mashupDetailedElement.Name)
				}
				return
			}
		}
	}
	//CUWorldApp.fyneWidgetElements[tabItem.Text].OnStatusChanged()
}

func BuildDetailMappedTabItemFyneComponent(CustosWorldApp *custosworld.CustosWorldApp, id string) *container.TabItem {
	//log.Printf("Started BuildDetailMappedTabItemFyneComponent for " + id)
	de := CustosWorldApp.FyneWidgetElements[id].MashupDetailedElement
	tabLabel := widget.NewLabel(de.Description)
	tabLabel.Wrapping = fyne.TextWrapWord
	// Search tab
	if de.Name == "SearchElement" {
		return container.NewTabItem(id, container.NewBorder(nil, nil, layout.NewSpacer(), nil, container.NewVBox(tabLabel, container.NewAdaptiveGrid(2,
			widget.NewLabel("Search: "),
			widget.NewLabel("Results: "),
		))))
	}
	//log.Printf("Successful for SearchElement")
	tr := CustosWorldApp.CustomTabItemRenderer["TenantDataRenderer"].(*TenantDataRenderer)
	for i := 0; i < len(de.Childids); i++ {
		if CustosWorldApp.MashupDetailedElementLibrary[de.Childids[i]] != nil {
			if CustosWorldApp.MashupDetailedElementLibrary[de.Childids[i]].Genre != "Solid" {
				tr.CurrentListElements = append(tr.CurrentListElements, CustosWorldApp.MashupDetailedElementLibrary[de.Childids[i]])
			}
		}
	}
	tr.InitialListElements = tr.CurrentListElements
	//log.Printf("Successful for initializing tr.CurrentElements")

	tr.Elementlist = widget.NewList(
		func() int { return len(tr.CurrentListElements) },
		func() fyne.CanvasObject {
			return widget.NewLabel("")
		},
		func(lii widget.ListItemID, co fyne.CanvasObject) {
			co.(*widget.Label).SetText(tr.CurrentListElements[lii].Name) //assuming MashupDetailedElementLibrary holds all concrete elements from world
		},
	)
	//log.Printf("Successful for initializing tr.ElementList")
	dfgtab := container.NewTabItem("Dataflow Groups", tr.Elementlist)
	tr.DataTabs = append(tr.DataTabs, dfgtab)
	tr.DataMenu = container.NewAppTabs(tr.DataTabs[0])

	tr.Elementlist.OnSelected = func(id widget.ListItemID) {
		//log.Printf("tr.Elementlist selected")
		//Update ClickedElements
		for i := 0; i < len(tr.DataTabs); i++ {
			tr.DataMenu.Remove(tr.DataTabs[i])
		}
		tr.DataTabs = []*container.TabItem{}
		clickedElement := tr.CurrentListElements[id]
		tr.ClickedElements = append(tr.ClickedElements, clickedElement)
		//Update contents of list and refresh it
		tr.CurrentListElements = []*mashupsdk.MashupDetailedElement{}
		for i := 0; i < len(clickedElement.Childids); i++ {
			if CustosWorldApp.MashupDetailedElementLibrary[clickedElement.Childids[i]] != nil {
				tr.CurrentListElements = append(tr.CurrentListElements, CustosWorldApp.MashupDetailedElementLibrary[clickedElement.Childids[i]])
			}
		}
		tr.Elementlist.Refresh()
		//Set text of old tab to clickedElement.Name
		//tr.DataMenu = container.NewAppTabs() //reset tabs
		for i := 0; i < len(tr.ClickedElements); i++ {
			tr.DataTabs = append(tr.DataTabs, container.NewTabItem(tr.ClickedElements[i].Name, tr.Elementlist))
			tr.DataMenu.Append(tr.DataTabs[i])
		}
		//Create new tab
		var newTab *container.TabItem
		if clickedElement.Alias == "DataFlow" {
			newTab = container.NewTabItem("Dataflow Statistics", tr.Elementlist)
		} else {
			newTab = container.NewTabItem("Dataflows", tr.Elementlist)
		}
		tr.DataTabs = append(tr.DataTabs, newTab)
		tr.DataMenu.Append(tr.DataTabs[len(tr.DataTabs)-1])
		//Select new tab
		tr.DataMenu.Select(newTab)
		//log.Printf("Finished selecting tr.Elementlist")
	}
	//log.Printf("Successful for setting tr.Elementlist.OnSelected")
	tr.DataMenu.OnSelected = func(tab *container.TabItem) {
		//Update ClickedElements
		size := len(tr.ClickedElements) - 1 //not sure if put in for loop condition if it will change when tr.ClickedElements is changed
		for i := size; i >= 0; i-- {
			if tr.ClickedElements[i].Name == tab.Text {
				break
			} else {
				// remove from ClickedElements
				tr.ClickedElements = tr.ClickedElements[:len(tr.ClickedElements)-1] //Don't know if this will work
			}
		}
		//Update list content -- Don't forget to refresh list
		if len(tr.ClickedElements) == 0 {
			//Case 1: list dataflowgroups --> get data from overall tab name
			tr.CurrentListElements = tr.InitialListElements
			tr.Elementlist.Refresh()
			//tr.DataMenu = container.NewAppTabs()
			if len(tr.DataTabs) > 0 {
				tr.DataMenu.Remove(tr.DataTabs[0])
				tr.DataTabs = []*container.TabItem{}
			}
			tr.DataTabs = append(tr.DataTabs, container.NewTabItem("Dataflow Groups", tr.Elementlist))
			tr.DataMenu.Append(tr.DataTabs[0])
		} else {
			//Case 2: Dataflows (middle tab) --> get data from clickedelement childids
			clickedElement := tr.ClickedElements[len(tr.ClickedElements)-1]
			tr.CurrentListElements = []*mashupsdk.MashupDetailedElement{}
			for i := 0; i < len(clickedElement.Childids); i++ {
				if CustosWorldApp.MashupDetailedElementLibrary[clickedElement.Childids[i]] != nil {
					tr.CurrentListElements = append(tr.CurrentListElements, CustosWorldApp.MashupDetailedElementLibrary[clickedElement.Childids[i]])
				}
			}
			tr.Elementlist.Refresh()
			var newTab *container.TabItem
			if clickedElement.Alias == "DataFlow" {
				newTab = container.NewTabItem("Dataflow Statistics", tr.Elementlist)
			} else {
				newTab = container.NewTabItem("Dataflows", tr.Elementlist)
			}
			tr.DataMenu.Append(newTab)
		}
		//Reset tabs -- find index of this tab? then change name then delete all tabs after
		//tr.DataMenu = container.NewAppTabs() //reset tabs

		// for i := 0; i < len(tr.ClickedElements); i++ {
		// 	tr.DataMenu.Append(container.NewTabItem(tr.ClickedElements[i].Name, tr.Elementlist))
		// }

	}
	//log.Printf("Successful for setting tr.ElementTabs.OnSelected")
	tabItem := container.NewTabItem(id, container.NewBorder(nil, nil, layout.NewSpacer(), nil, container.NewVBox(tabLabel, container.NewAdaptiveGrid(2,
		tr.DataMenu,
	))))
	//log.Printf("Finished BuildDetailMappedTabItemFyneComponent")
	return tabItem
}
