package ttdirender

import (
	//"fmt"
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
	//"github.com/mrjrieke/nute/mashupsdk"
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
	activeSet     map[int64]*math32.Vector3
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

// Add into different group based on g3n element
// Sort group using mashupdetailedelement.Alias
func (er *ElementRenderer) NewSolidAtPosition(g3n *g3nmash.G3nDetailedElement, vpos *math32.Vector3) core.INode {
	sphereGeom := geometry.NewSphere(.1, 100, 100)
	color := g3ndpalette.DARK_BLUE
	mat := material.NewStandard(color.Set(0, 0.349, 0.643))
	sphereMesh := graphic.NewMesh(sphereGeom, mat)
	sphereMesh.SetLoaderID(g3n.GetDisplayName())
	sphereMesh.SetPositionVec(vpos)
	//g3n.SetNamedMesh(g3n.GetDisplayName(), sphereMesh)
	return sphereMesh
}

func (er *ElementRenderer) NewInternalMeshAtPosition(g3n *g3nmash.G3nDetailedElement, vpos *math32.Vector3) core.INode {
	return nil
}

// Add multiple next coordinate methods?? Base it on grouping? How to edit groups --> need it based on argosy struct --> use alias
func (er *ElementRenderer) NextCoordinate(g3n *g3nmash.G3nDetailedElement, totalElements int) (*g3nmash.G3nDetailedElement, *math32.Vector3) {
	er.totalElements = totalElements //totalElements parameter increases as it is rendered?
	if er.iOffset == 0 {
		er.iOffset = 1
		er.counter = 0.0
		er.Push(g3n, math32.NewVector3(float32(0.0), float32(0.0), float32(0.0)))
		return g3n, math32.NewVector3(float32(0.0), float32(0.0), float32(0.0))
	} else if er.iOffset == 1 {
		er.iOffset = 2
		er.counter = 0.0
		return g3n, math32.NewVector3(er.Top().center.X, er.Top().center.Y, er.Top().center.Z)
	} else {
		er.counter = er.counter - 0.1
		complex := binetFormula(er.counter)
		return g3n, math32.NewVector3(float32(-real(complex))+er.Top().center.X, float32(imag(complex))+er.Top().center.Y, float32(-er.counter)+er.Top().center.Z)
	}
}

func (er *ElementRenderer) Layout(worldApp *g3nworld.WorldApp,
	g3nRenderableElements []*g3nmash.G3nDetailedElement) {
	er.LayoutBase(worldApp, er, g3nRenderableElements) //Doesn't accept new mesh type in call to layoutbase
}

func (er *ElementRenderer) InitRenderLoop(worldApp *g3nworld.WorldApp) bool {
	// TODO: noop
	return true
}

func (er *ElementRenderer) RenderElement(worldApp *g3nworld.WorldApp, g3n *g3nmash.G3nDetailedElement) bool {
	//Look at clicked elements and apply state changes to child Ids
	//Keep this method ignorant (only changes look and feel of objects and state)
	if g3n == worldApp.ClickedElements[len(worldApp.ClickedElements)-1] {
		if g3n.GetNamedMesh(g3n.GetDisplayName()) != nil {
			location := g3n.GetNamedMesh(g3n.GetDisplayName()).Position()
			er.Push(g3n, &location)
			er.iOffset = 1
		}
		for _, childId := range g3n.GetChildElementIds() {
			//for _, child := range worldApp.ConcreteElements[childId].GetChildElementIds() {
			element := worldApp.ConcreteElements[childId]
			element.ApplyState(mashupsdk.Hidden, false)
			element.ApplyState(mashupsdk.Clicked, true)
			_, nextPos := er.NextCoordinate(element, er.totalElements)
			solidMesh := er.NewSolidAtPosition(g3n, nextPos)
			if solidMesh != nil {
				log.Printf("Adding %s\n", solidMesh.GetNode().LoaderID())
				//if !g3nRenderableElement.IsStateSet(mashupsdk.Hidden) {\
				//graphic.Mesh(solidMesh).SetPositionVec(math32.NewVector3(float32(1), 1.0, 1.0))
				//solidMesh.Position().X = float32(1.0)

				//math32.NewVector3(float32(1), 1.0, 1.0))
				solidMesh.GetNode().SetPositionVec(math32.NewVector3(1.0, 1.0, 1.0))
				worldApp.AddToScene(solidMesh)
				//}
				g3n.SetNamedMesh(g3n.GetDisplayName(), solidMesh)
			}
			//worldApp.AddToScene(element.GetNamedMesh(element.GetDisplayName()))
			//}
		}
		return true
	}
	return true
	// if g3nDetailedElement.GetDetailedElement().Alias == "Argosy" {
	// 	return er.HandleArgosyStateChange(worldApp, g3nDetailedElement)
	// } else if g3nDetailedElement.GetDetailedElement().Alias == "DataFlowGroup" {
	// 	return er.HandleDFGStateChange(worldApp, g3nDetailedElement)
	// } else {
	// 	return true
	// }
}

func (er *ElementRenderer) HelpStateChange() {}

func (er *ElementRenderer) HandleArgosyStateChange(worldApp *g3nworld.WorldApp, g3nDetailedElement *g3nmash.G3nDetailedElement) bool {
	var g3nColor *math32.Color

	if g3nDetailedElement.IsItemActive() {
		//er.Push(g3nDetailedElement, g3nDetailedElement.Position())
		// for _, childId := range g3nDetailedElement.GetChildElementIds() {
		// 	e := worldApp.ConcreteElements[int64(childId)]
		// 	for _, childId := range e.GetChildElementIds() {
		// 		for _, subChildID := range worldApp.ConcreteElements[childId].GetChildElementIds() {
		// 			element := worldApp.ConcreteElements[subChildID]
		// 			name := element.GetDisplayName()
		// 			compoundMesh := element.GetNamedMesh(name)
		// 			if compoundMesh != nil {
		// 				element.ApplyState(mashupsdk.Init, false)
		// 				element.ApplyState(mashupsdk.Hidden, false)
		// 				element.ApplyState(mashupsdk.Clicked, true)
		// 			}
		// 		}
		// 	}
		// }
		g3nColor = math32.NewColor("darkred")
		mesh := g3nDetailedElement.GetNamedMesh(g3nDetailedElement.GetDisplayName())
		if er.activeSet == nil {
			er.activeSet = map[int64]*math32.Vector3{}
		}
		activePosition := mesh.(*graphic.Mesh).GetGraphic().Position()
		er.activeSet[g3nDetailedElement.GetDetailedElement().GetId()] = &activePosition
		return g3nDetailedElement.SetColor(g3nColor, 1.0)
		//fmt.Printf("Active element centered at %v\n", activePosition)
	} else {
		if g3nDetailedElement.IsBackgroundElement() {
			// Axial circle
			g3nColor = g3ndpalette.GREY
		} else {
			if !worldApp.Sticky {
				g3nColor = g3ndpalette.DARK_BLUE
			} else {
				g3nColor = g3nDetailedElement.GetColor()
				if g3nColor == nil {
					g3nColor = g3ndpalette.DARK_BLUE
				}
			}
		}
	}

	return g3nDetailedElement.SetColor(g3nColor, 1.0)
	//return true
}

func (er *ElementRenderer) HandleDFGStateChange(worldApp *g3nworld.WorldApp, g3nDetailedElement *g3nmash.G3nDetailedElement) bool {
	var g3nColor *math32.Color

	if g3nDetailedElement.IsItemActive() {
		//er.Push(g3nDetailedElement)
		// for _, childId := range g3nDetailedElement.GetChildElementIds() {
		// 	e := worldApp.ConcreteElements[int64(childId)]
		// 	for _, childId := range e.GetChildElementIds() {
		// 		for _, subChildID := range worldApp.ConcreteElements[childId].GetChildElementIds() {
		// 			element := worldApp.ConcreteElements[subChildID]
		// 			name := element.GetDisplayName()
		// 			compoundMesh := element.GetNamedMesh(name)
		// 			if compoundMesh != nil {
		// 				element.ApplyState(mashupsdk.Init, false)
		// 				element.ApplyState(mashupsdk.Hidden, false)
		// 				element.ApplyState(mashupsdk.Clicked, true)
		// 			}
		// 		}
		// 	}
		// }
		g3nColor = math32.NewColor("darkred")
		mesh := g3nDetailedElement.GetNamedMesh(g3nDetailedElement.GetDisplayName())
		if er.activeSet == nil {
			er.activeSet = map[int64]*math32.Vector3{}
		}
		activePosition := mesh.(*graphic.Mesh).GetGraphic().Position()
		er.activeSet[g3nDetailedElement.GetDetailedElement().GetId()] = &activePosition
		return g3nDetailedElement.SetColor(g3nColor, 1.0)
		//fmt.Printf("Active element centered at %v\n", activePosition)
	} else {
		if g3nDetailedElement.IsBackgroundElement() {
			// Axial circle
			g3nColor = g3ndpalette.GREY
		} else {
			if !worldApp.Sticky {
				g3nColor = g3ndpalette.DARK_BLUE
			} else {
				g3nColor = g3nDetailedElement.GetColor()
				if g3nColor == nil {
					g3nColor = g3ndpalette.DARK_BLUE
				}
			}
		}
	}

	return g3nDetailedElement.SetColor(g3nColor, 1.0)
	//return true
}

func (er *ElementRenderer) LayoutBase(worldApp *g3nworld.WorldApp,
	g3Renderer *ElementRenderer,
	g3nRenderableElements []*g3nmash.G3nDetailedElement) {
	var nextPos *math32.Vector3
	var prevSolidPos *math32.Vector3

	totalElements := len(g3nRenderableElements)

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
				//if !g3nRenderableElement.IsStateSet(mashupsdk.Hidden) {
				worldApp.AddToScene(solidMesh)
				//}
				concreteG3nRenderableElement.SetNamedMesh(concreteG3nRenderableElement.GetDisplayName(), solidMesh)
			}

			for _, relatedG3n := range worldApp.GetG3nDetailedChildElementsByGenre(concreteG3nRenderableElement, "Related") {
				relatedMesh := g3Renderer.NewRelatedMeshAtPosition(concreteG3nRenderableElement, nextPos, prevSolidPos)
				if relatedMesh != nil {
					worldApp.AddToScene(relatedMesh)
					concreteG3nRenderableElement.SetNamedMesh(relatedG3n.GetDisplayName(), relatedMesh)
				}
			}

			for _, innerG3n := range worldApp.GetG3nDetailedChildElementsByGenre(concreteG3nRenderableElement, "Space") {
				negativeMesh := g3Renderer.NewInternalMeshAtPosition(innerG3n, nextPos)
				if negativeMesh != nil {
					worldApp.AddToScene(negativeMesh)
					innerG3n.SetNamedMesh(innerG3n.GetDisplayName(), negativeMesh)
				}
			}
		}

	}
}
