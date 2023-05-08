package trcRenderers

// World is a basic gomobile app.
import (
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"

	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"
	"github.com/trimble-oss/tierceron-nute/custos/custosworld"
	"github.com/trimble-oss/tierceron-nute/mashupsdk"
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
	TabCheck            int
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
	if concreteElement.IsStateSet(mashupsdk.Clicked) {
		//log.Printf("TorusRender Widget looking up: %s\n", concreteElement.Alias)
		if fyneWidgetElement, fyneOk := tr.CustosWorldApp.FyneWidgetElements[concreteElement.Name]; fyneOk {
			//log.Printf("TorusRender Widget lookup found: %s\n", concreteElement.Alias)
			fyneWidgetElement.GuiComponent = nil
			if fyneWidgetElement.GuiComponent == nil {
				fyneWidgetElement.GuiComponent = tr.CustosWorldApp.CustomTabItems[concreteElement.Name](tr.CustosWorldApp, concreteElement.Name)
			}
			tr.CustosWorldApp.TabItemMenu.Append(fyneWidgetElement.GuiComponent.(*container.TabItem))
		}
	} else {
		//log.Printf("TorusRender Widget lookingup for remove: %s\n", concreteElement.Alias)
		if fyneWidgetElement, fyneOk := tr.CustosWorldApp.FyneWidgetElements[concreteElement.Name]; fyneOk {
			//log.Printf("TorusRender Widget lookup found for remove: %s %v\n", concreteElement.Alias, fyneWidgetElement)
			if fyneWidgetElement.GuiComponent != nil {
				tr.CustosWorldApp.TabItemMenu.Remove(fyneWidgetElement.GuiComponent.(*container.TabItem))
			}
			fyneWidgetElement.GuiComponent = nil
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
}

func BuildDetailMappedTabItemFyneComponent(CustosWorldApp *custosworld.CustosWorldApp, id string) *container.TabItem {
	de := CustosWorldApp.FyneWidgetElements[id].MashupDetailedElement
	tabLabel := widget.NewLabel(de.Description)
	tabLabel.Wrapping = fyne.TextWrapWord
	if de.Name == "SearchElement" {
		return container.NewTabItem(id, container.NewBorder(nil, nil, layout.NewSpacer(), nil, container.NewVBox(tabLabel, container.NewAdaptiveGrid(2,
			widget.NewLabel("Search: "),
			widget.NewLabel("Results: "),
		))))
	}
	tr := CustosWorldApp.CustomTabItemRenderer["TenantDataRenderer"].(*TenantDataRenderer)
	tr.CurrentListElements = []*mashupsdk.MashupDetailedElement{}
	for i := 0; i < len(de.Childids); i++ {
		if CustosWorldApp.MashupDetailedElementLibrary[de.Childids[i]] != nil {
			tr.CurrentListElements = append(tr.CurrentListElements, CustosWorldApp.MashupDetailedElementLibrary[de.Childids[i]])
		}
	}
	tr.Elementlist = tr.RefreshList(CustosWorldApp, nil)
	tr.Elementlist.Refresh()
	tr.DataTabs = []*container.TabItem{}

	tr.DataTabs = append(tr.DataTabs, container.NewTabItem("Dataflow Groups", tr.Elementlist))
	tr.DataMenu = container.NewAppTabs(tr.DataTabs[0])
	tr.Elementlist.Resize(fyne.NewSize(500, 500))

	tr.DataMenu.OnSelected = func(tab *container.TabItem) {
		if CustosWorldApp.FyneWidgetElements[tab.Text] != nil {
			if tr.TabCheck != 0 {
				newTabs := []*container.TabItem{}
				clickedElement := CustosWorldApp.FyneWidgetElements[tab.Text].MashupDetailedElement
				size := len(tr.ClickedElements) - 1
				for i := size; i >= 0; i-- {
					if tr.ClickedElements[i].Name == CustosWorldApp.MashupDetailedElementLibrary[clickedElement.Parentids[0]].Name {
						break
					} else {
						tr.ClickedElements = tr.ClickedElements[:i]
					}
				}

				tr.CurrentListElements = []*mashupsdk.MashupDetailedElement{}
				parentElement := CustosWorldApp.MashupDetailedElementLibrary[clickedElement.Parentids[0]]
				if parentElement != nil {
					for i := 0; i < len(parentElement.Childids); i++ {
						if CustosWorldApp.MashupDetailedElementLibrary[parentElement.Childids[i]] != nil {
							tr.CurrentListElements = append(tr.CurrentListElements, CustosWorldApp.MashupDetailedElementLibrary[parentElement.Childids[i]])
						}
					}
				}

				tr.Elementlist.Refresh()
				for i := 0; i < len(tr.ClickedElements); i++ {
					newTabs = append(newTabs, container.NewTabItem(tr.ClickedElements[i].Name, tr.Elementlist))
				}
				tab.Content = tr.Elementlist
				if parentElement.Alias == "DataFlowGroup" {
					tab.Text = "Dataflows"
				} else {
					tab.Text = "Dataflow Groups"
				}
				tr.TabCheck = 0
				newTabs = append(newTabs, tab)
				tr.DataTabs = []*container.TabItem{}
				tr.DataTabs = newTabs
				tr.DataMenu.SetItems(tr.DataTabs)
				tr.DataMenu.Select(tr.DataTabs[len(tr.DataTabs)-1])
				tr.TabCheck = 1
			}

		} else if tr.ClickedElements != nil && len(tr.ClickedElements) > 0 {
			clickedElement := tr.ClickedElements[len(tr.ClickedElements)-1]
			tr.CurrentListElements = []*mashupsdk.MashupDetailedElement{}

			for i := 0; i < len(clickedElement.Childids); i++ {
				if CustosWorldApp.MashupDetailedElementLibrary[clickedElement.Childids[i]] != nil {
					tr.CurrentListElements = append(tr.CurrentListElements, CustosWorldApp.MashupDetailedElementLibrary[clickedElement.Childids[i]])
				}
			}
			tr.Elementlist.Refresh()
			tab.Content = tr.Elementlist

			tr.Elementlist.Refresh()
		}

	}
	tabItem := container.NewTabItem(id, container.NewBorder(nil, nil, layout.NewSpacer(), nil, container.New(layout.NewPaddedLayout(), container.NewAdaptiveGrid(1,
		tr.DataMenu,
	))))
	return tabItem
}

func (tr *TenantDataRenderer) RefreshList(CustosWorldApp *custosworld.CustosWorldApp, clickedTab *mashupsdk.MashupDetailedElement) *widget.List {
	updatedList := widget.NewList(
		func() int { return len(tr.CurrentListElements) },
		func() fyne.CanvasObject {
			return widget.NewLabel("")
		},
		func(lii widget.ListItemID, co fyne.CanvasObject) {
			if tr.CurrentListElements[lii].Alias == "DataFlowStatistic" {
				time := tr.CurrentListElements[lii].Data
				if time != "" {
					timeNanoSeconds, err := strconv.Atoi(time)
					if err != nil {
						co.(*widget.Label).SetText(tr.CurrentListElements[lii].Name)
					}
					seconds := (float64(timeNanoSeconds) * math.Pow(10.0, -9.0))
					minutes := seconds / 60.0
					seconds = math.Mod(seconds, 60.0)
					wholeMinutes := int(minutes)
					milliSecondsCheck := int(seconds * 1000)
					fullName := tr.CurrentListElements[lii].Name
					fullName = strings.Split(fullName, "-")[0]
					if wholeMinutes == 0 {
						if milliSecondsCheck/100 != 0 && milliSecondsCheck/1000 == 0 {
							fullName = fullName + ": " + strconv.Itoa(milliSecondsCheck) + "ms"
						} else {
							fullName = fullName + ": " + fmt.Sprintf("%.2f", seconds) + "s"
						}
					} else {
						fullName = fullName + ": " + strconv.Itoa(wholeMinutes) + "m" + fmt.Sprintf("%.2f", seconds) + "s"
					}
					co.(*widget.Label).SetText(fullName)
				}
			} else {
				co.(*widget.Label).SetText(tr.CurrentListElements[lii].Name)

			}
		},
	)
	updatedList.OnSelected = func(id widget.ListItemID) {
		tr.TabCheck = 1
		clickedElement := tr.CurrentListElements[id]

		if len(tr.ClickedElements) == 0 || tr.ClickedElements == nil || (tr.ClickedElements != nil && tr.ClickedElements[len(tr.ClickedElements)-1].Alias != "DataFlow") {
			tr.ClickedElements = append(tr.ClickedElements, clickedElement)

			tr.DataTabs[len(tr.DataTabs)-1].Text = clickedElement.Name
			tr.DataMenu.Refresh()
			var newTab *container.TabItem
			if clickedElement.Alias == "DataFlow" {
				newTab = container.NewTabItem("Dataflow Statistics", tr.Elementlist)
			} else {
				newTab = container.NewTabItem("Dataflows", tr.Elementlist)
			}
			var contains bool
			for _, dataTab := range tr.DataTabs {
				if newTab.Text == dataTab.Text {
					contains = true
					break
				}
				contains = false
			}
			if !contains {
				tr.DataTabs = append(tr.DataTabs, newTab)
				tr.DataMenu.Append(tr.DataTabs[len(tr.DataTabs)-1])
				tr.DataMenu.Select(tr.DataTabs[len(tr.DataTabs)-1])
			}
		}
	}
	return updatedList
}
