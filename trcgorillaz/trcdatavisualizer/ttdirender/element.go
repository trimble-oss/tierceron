package ttdirender

import (
	"log"

	"github.com/g3n/engine/core"
	"github.com/g3n/engine/geometry"
	"github.com/g3n/engine/graphic"
	"github.com/g3n/engine/material"
	"github.com/g3n/engine/math32"
	"github.com/mrjrieke/nute/g3nd/g3nmash"
	"github.com/mrjrieke/nute/g3nd/g3nworld"
	g3ndpalette "github.com/mrjrieke/nute/g3nd/palette"
	"github.com/mrjrieke/nute/g3nd/worldg3n/g3nrender"
	"github.com/mrjrieke/nute/mashupsdk"
)

type ClickedG3nDetailElement struct {
	clickedElement *g3nmash.G3nDetailedElement
	center         *math32.Vector3
}

type ElementRenderer struct {
	g3nrender.GenericRenderer
	iOffset int
	//subOffset     int
	counter float64
	//subCounter    float64
	//locnCounter   *math32.Vector3
	totalElements int
	locationCache map[int64]*math32.Vector3
	//compoundMesh  *CompoundMesh

	clickedElements []*ClickedG3nDetailElement //ClickedG3nDetailElement // Stack containing clicked spiral (sub as well) g3n elements.
}

func (er *ElementRenderer) IsEmpty() bool {
	return len(er.clickedElements) == 0
}

func (er *ElementRenderer) Length() int {
	return len(er.clickedElements)
}

func (er *ElementRenderer) Push(g3nDetailedElement *g3nmash.G3nDetailedElement, centerLocation *math32.Vector3) {
	clickedElements := ClickedG3nDetailElement{
		clickedElement: g3nDetailedElement,
		center:         centerLocation,
	}
	er.clickedElements = append(er.clickedElements, &clickedElements)
}

func (er *ElementRenderer) Pop() *ClickedG3nDetailElement {
	size := len(er.clickedElements)
	element := er.clickedElements[size-1]
	er.clickedElements = er.clickedElements[:size-1]
	return element
}

func (er *ElementRenderer) Top() *ClickedG3nDetailElement {
	return er.clickedElements[er.Length()-1]
}

func (er *ElementRenderer) NewSolidAtPosition(g3n *g3nmash.G3nDetailedElement, vpos *math32.Vector3) core.INode {
	sphereGeom := geometry.NewSphere(.1, 100, 100)
	color := g3ndpalette.DARK_BLUE
	mat := material.NewStandard(color.Set(0, 0.349, 0.643))
	sphereMesh := graphic.NewMesh(sphereGeom, mat)
	sphereMesh.SetLoaderID(g3n.GetDisplayName())
	sphereMesh.SetPositionVec(vpos)
	return sphereMesh
}

func (er *ElementRenderer) NewInternalMeshAtPosition(g3n *g3nmash.G3nDetailedElement, vpos *math32.Vector3) core.INode {
	return nil
}

func (er *ElementRenderer) NextCoordinate(g3n *g3nmash.G3nDetailedElement, totalElements int) (*g3nmash.G3nDetailedElement, *math32.Vector3) {
	er.totalElements = totalElements // Not sure if this is necessary
	if er.locationCache != nil {
		id := g3n.GetDisplayId()
		return g3n, er.locationCache[id]
	} else { // This is because locn cache is not set until clicked element :(
		if er.iOffset == 0 {
			er.iOffset = 1
			er.counter = 0.0
			return g3n, math32.NewVector3(float32(0.0), float32(0.0), float32(0.0))
		} else {
			er.counter = er.counter - 0.1
			complex := binetFormula(er.counter)
			return g3n, math32.NewVector3(float32(-real(complex)), float32(imag(complex)), float32(-er.counter))
		}
	}
}

func (er *ElementRenderer) Layout(worldApp *g3nworld.WorldApp,
	g3nRenderableElements []*g3nmash.G3nDetailedElement) {
	er.LayoutBase(worldApp, er, g3nRenderableElements)
}

//RECURSIVELY LOOP THROU
func (er *ElementRenderer) loopParentIds(element *g3nmash.G3nDetailedElement) {
	if len(element.GetParentElementIds()) == 0 {

	}
}

func (er *ElementRenderer) InitRenderLoop(worldApp *g3nworld.WorldApp) bool {
	// TODO: noop
	//Initialize location cache
	if er.locationCache == nil {
		er.locationCache = make(map[int64]*math32.Vector3)
		for _, mainElement := range worldApp.ConcreteElements {
			if mainElement.GetNamedMesh(mainElement.GetDisplayName()) != nil && mainElement.GetDetailedElement().Renderer == "Element" {
				location := mainElement.GetNamedMesh(mainElement.GetDisplayName()).Position()
				id := mainElement.GetDetailedElement().GetId()
				er.locationCache[id] = &location
				counter := float32(0.0)
				// MAKE THIS RECURSIVE PROCESS OR INSTEAD ITERATE THROUGH ELEMENTS USING PARENT IDS? --> LESS DUPLICATE LOOKUPS
				// LOOK THRU PARENT IDS UNTIL MESH IS COME ACROSS AND IF NO PARENT ID JUST LOOK AT MESH POSITION
				for _, childID := range mainElement.GetChildElementIds() {
					er.locationCache[childID] = er.calculateLocation(&location, counter)
					counter = counter - 0.1
					subCount := float32(0.0)
					for _, dFGchild := range worldApp.ConcreteElements[childID].GetChildElementIds() {
						er.locationCache[dFGchild] = er.calculateLocation(er.locationCache[childID], subCount)
						subCount = subCount - 0.1
					}
					// Is it possible to add to concrete elements thing here?
				}
			}
		}
	}

	// Hide the elements that should be hidden when a different element is clicked
	if !er.IsEmpty() {
		prevElement := er.Pop()
		if !er.isChildElement(worldApp, prevElement.clickedElement) && prevElement != nil && prevElement.clickedElement.GetDetailedElement().Genre != "Solid" {
			//Case 1: Main spiral element: have to remove children nodes
			for _, childID := range prevElement.clickedElement.GetChildElementIds() {
				if !er.isChildElement(worldApp, prevElement.clickedElement) {
					worldApp.ConcreteElements[childID].ApplyState(mashupsdk.Hidden, true)
					er.RemoveAll(worldApp, childID)
				}
			}
			//Case 2: Sub Spiral element and main spiral clicked: have to remove children nodes of parent node
			// CHANGE THIS TO RECURSIVE METHOD TO ITERATE THROUGH PARENT IDS
			for _, parentID := range prevElement.clickedElement.GetParentElementIds() {
				parentElement := worldApp.ConcreteElements[parentID]
				if len(parentElement.GetParentElementIds()) == 0 {
					for _, childID := range parentElement.GetChildElementIds() {
						if !er.isChildElement(worldApp, parentElement) {
							worldApp.ConcreteElements[childID].ApplyState(mashupsdk.Hidden, true)
							er.RemoveAll(worldApp, childID)
						}
					}
				}
				for _, parent := range parentElement.GetParentElementIds() {
					parentElement = worldApp.ConcreteElements[parent]
					for _, childID := range parentElement.GetChildElementIds() {
						if !er.isChildElement(worldApp, parentElement) {
							worldApp.ConcreteElements[childID].ApplyState(mashupsdk.Hidden, true)
							er.RemoveAll(worldApp, childID)
						}
					}
				}

			}
		}
	}

	// Handle state change here and then in render element just render all the elements that are not yet showing but are hidden
	clickedElement := worldApp.ClickedElements[len(worldApp.ClickedElements)-1]
	if clickedElement.GetDetailedElement().Genre != "Solid" && clickedElement.GetDetailedElement().Genre != "Space" {
		center := clickedElement.GetNamedMesh(clickedElement.GetDisplayName()).Position()
		er.Push(clickedElement, &center)
		for _, childID := range clickedElement.GetChildElementIds() {
			childElement := worldApp.ConcreteElements[childID]
			if childElement.GetDetailedElement().Genre != "Solid" {
				childElement.ApplyState(mashupsdk.Hidden, false)
				childElement.ApplyState(mashupsdk.Clicked, true)
			}
			color := g3ndpalette.DARK_BLUE
			childElement.SetColor(color.Set(0, 0.349, 0.643), 1.0)
		}
	}

	return true
}

func (er *ElementRenderer) isChildElement(worldApp *g3nworld.WorldApp, prevElement *g3nmash.G3nDetailedElement) bool {
	clickedElement := worldApp.ClickedElements[len(worldApp.ClickedElements)-1]
	for _, childID := range prevElement.GetChildElementIds() {
		if clickedElement == worldApp.ConcreteElements[childID] {
			return true
		}
	}
	return false
}

func (er *ElementRenderer) calculateLocation(center *math32.Vector3, counter float32) *math32.Vector3 {
	complex := binetFormula(float64(counter))
	return math32.NewVector3(float32(-real(complex))+center.X, float32(imag(complex))+center.Y, float32(-counter)+center.Z)
}

// Change color and fade other elements?
func (er *ElementRenderer) RenderElement(worldApp *g3nworld.WorldApp, g3n *g3nmash.G3nDetailedElement) bool {
	if g3n == worldApp.ClickedElements[len(worldApp.ClickedElements)-1] && g3n.GetNamedMesh(g3n.GetDisplayName()) != nil {
		// Changes color
		if g3n.GetDetailedElement().Alias == "Argosy" {
			g3n.SetColor(math32.NewColor("darkred"), 1.0)
		} else if g3n.GetDetailedElement().Alias == "DataFlowGroup" {
			color := math32.NewColor("olive")
			g3n.SetColor(color.Set(1.0, 0.224, 0.0), 1.0)
		} else if g3n.GetDetailedElement().Alias == "DataFlow" {
			g3n.SetColor(math32.NewColor("olive"), 1.0)
		}

		for _, childId := range g3n.GetChildElementIds() {
			element := worldApp.ConcreteElements[childId]
			if element.GetDetailedElement().Genre != "Solid" {
				if element.GetNamedMesh(element.GetDisplayName()) == nil {
					_, nextPos := er.NextCoordinate(element, er.totalElements)
					solidMesh := er.NewSolidAtPosition(element, nextPos)
					if solidMesh != nil {
						log.Printf("Adding %s\n", solidMesh.GetNode().LoaderID())
						worldApp.UpsertToScene(solidMesh)
						element.SetNamedMesh(element.GetDisplayName(), solidMesh)
					}
				} else {
					worldApp.UpsertToScene(element.GetNamedMesh(element.GetDisplayName()))
				}
			}
		}

		return true
	} else {
		// add in fade effect here
		color := g3ndpalette.DARK_BLUE
		g3n.SetColor(color.Set(0, 0.349, 0.643), 1.0)
	}
	return true
}

func (er *ElementRenderer) RemoveAll(worldApp *g3nworld.WorldApp, childId int64) {
	if child, childOk := worldApp.ConcreteElements[childId]; childOk {
		if !child.IsAbstract() && child.GetDetailedElement().Genre != "Solid" {
			if childMesh := child.GetNamedMesh(child.GetDisplayName()); childMesh != nil {
				log.Printf("Child Item removed %s: %v", child.GetDisplayName(), worldApp.RemoveFromScene(childMesh))
			}
		}

		if len(child.GetChildElementIds()) > 0 {
			for _, cId := range child.GetChildElementIds() {
				er.RemoveAll(worldApp, cId)
			}
		}
	}

}

func (er *ElementRenderer) LayoutBase(worldApp *g3nworld.WorldApp,
	g3Renderer *ElementRenderer,
	g3nRenderableElements []*g3nmash.G3nDetailedElement) {
	var nextPos *math32.Vector3
	var prevSolidPos *math32.Vector3

	totalElements := len(g3nRenderableElements)
	//er.totalElements = totalElements

	if totalElements > 0 {
		if g3nRenderableElements[0].GetDetailedElement().Colabrenderer != "" {
			log.Printf("Collab examine: %v\n", g3nRenderableElements[0])
			log.Printf("Renderer name: %s\n", g3nRenderableElements[0].GetDetailedElement().GetRenderer())
			protoRenderer := g3Renderer.GetRenderer(g3nRenderableElements[0].GetDetailedElement().GetRenderer())
			log.Printf("Collaborating %v\n", protoRenderer)
			g3Renderer.Collaborate(worldApp, protoRenderer)
		}
	}

	for _, g3nRenderableElement := range g3nRenderableElements {
		concreteG3nRenderableElement := g3nRenderableElement
		if !g3nRenderableElement.IsStateSet(mashupsdk.Hidden) {
			prevSolidPos = nextPos
			_, nextPos = g3Renderer.NextCoordinate(concreteG3nRenderableElement, totalElements)
			solidMesh := g3Renderer.NewSolidAtPosition(concreteG3nRenderableElement, nextPos)
			if solidMesh != nil {
				log.Printf("Adding %s\n", solidMesh.GetNode().LoaderID())
				worldApp.UpsertToScene(solidMesh)
				concreteG3nRenderableElement.SetNamedMesh(concreteG3nRenderableElement.GetDisplayName(), solidMesh)
			}

			for _, relatedG3n := range worldApp.GetG3nDetailedChildElementsByGenre(concreteG3nRenderableElement, "Related") {
				relatedMesh := g3Renderer.NewRelatedMeshAtPosition(concreteG3nRenderableElement, nextPos, prevSolidPos)
				if relatedMesh != nil {
					worldApp.UpsertToScene(relatedMesh)
					concreteG3nRenderableElement.SetNamedMesh(relatedG3n.GetDisplayName(), relatedMesh)
				}
			}

			for _, innerG3n := range worldApp.GetG3nDetailedChildElementsByGenre(concreteG3nRenderableElement, "Space") {
				negativeMesh := g3Renderer.NewInternalMeshAtPosition(innerG3n, nextPos)
				if negativeMesh != nil {
					worldApp.UpsertToScene(negativeMesh)
					innerG3n.SetNamedMesh(innerG3n.GetDisplayName(), negativeMesh)
				}
			}
		}

	}
}

func (er *ElementRenderer) Collaborate(worldApp *g3nworld.WorldApp, collaboratingRenderer g3nrender.IG3nRenderer) {
	curveRenderer := collaboratingRenderer.(*CurveRenderer)
	curveRenderer.totalElements = int(er.counter) // 50 //er.totalElements
}
