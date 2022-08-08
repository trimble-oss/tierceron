package ttdirender

import (
	"fmt"

	//"sort"
	//"strings"

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

type PathRenderer struct {
	g3nrender.GenericRenderer
	iOffset       int
	counter       float64
	totalElements int
	activeSet     map[int64]*math32.Vector3
}

/*type TTDICollection g3nrender.G3nCollection

func (a TTDICollection) Len() int { return len(a) }
func (a TTDICollection) Less(i, j int) bool {
	return strings.Compare(a[i].GetDisplayName(), a[j].GetDisplayName()) < 0
}
func (a TTDICollection) Swap(i, j int) { a[i], a[j] = a[j], a[i] }

func (sp *SphereRenderer) Sort(worldApp *g3nworld.WorldApp, g3nRenderableElements g3nrender.G3nCollection) g3nrender.G3nCollection {
	sort.Sort(TTDICollection(g3nRenderableElements))
	return g3nRenderableElements
}*/

func (sp *PathRenderer) NewSolidAtPosition(g3n *g3nmash.G3nDetailedElement, vpos *math32.Vector3) core.INode {
	sphereGeom := geometry.NewSphere(.1, 100, 100)
	color := g3ndpalette.DARK_BLUE
	mat := material.NewStandard(color.Set(0, 0.349, 0.643))
	sphereMesh := graphic.NewMesh(sphereGeom, mat)
	fmt.Printf("LoaderID: %s\n", g3n.GetDisplayName())
	sphereMesh.SetLoaderID(g3n.GetDisplayName())
	sphereMesh.SetPositionVec(vpos)
	return sphereMesh
}

func (sp *PathRenderer) NewInternalMeshAtPosition(g3n *g3nmash.G3nDetailedElement, vpos *math32.Vector3) core.INode {
	return nil
}

func (sp *PathRenderer) NextCoordinate(g3n *g3nmash.G3nDetailedElement, totalElements int) (*g3nmash.G3nDetailedElement, *math32.Vector3) {
	sp.totalElements = totalElements //totalElements parameter increases as it is rendered?
	if sp.iOffset == 0 {
		sp.iOffset = 1
		sp.counter = -0.1 * float64(totalElements)
		return g3n, math32.NewVector3(float32(0.0), float32(0.0), float32(0.0))
	} else {
		sp.counter = sp.counter + 0.1
		complex := binetFormula(sp.counter)
		return g3n, math32.NewVector3(float32(-real(complex)), float32(imag(complex)), float32(-sp.counter))
	}
}

func (sp *PathRenderer) Layout(worldApp *g3nworld.WorldApp,
	g3nRenderableElements []*g3nmash.G3nDetailedElement) {
	sp.GenericRenderer.LayoutBase(worldApp, sp, g3nRenderableElements)
}

func (sp *PathRenderer) HandleStateChange(worldApp *g3nworld.WorldApp, g3nDetailedElement *g3nmash.G3nDetailedElement) bool {
	var g3nColor *math32.Color

	if g3nDetailedElement.IsItemActive() {
		for _, childId := range g3nDetailedElement.GetChildElementIds() {
			//worldApp.ConcreteElements[int64(childId)].GetDetailedElement().State.State = 4
			e := worldApp.ConcreteElements[int64(childId)]
			for _, childId := range e.GetChildElementIds() {
				for _, subChildID := range worldApp.ConcreteElements[childId].GetChildElementIds() {
					if worldApp.ConcreteElements[subChildID].GetNamedMesh(worldApp.ConcreteElements[subChildID].GetDisplayName()) != nil {
						// for _, mesh := range worldApp.ConcreteElements[subChildID].GetNamedMesh(worldApp.ConcreteElements[subChildID].GetDisplayName()).meshes {

						// }
						element := worldApp.ConcreteElements[subChildID]
						element.SetElementState(mashupsdk.Init)
						for _, mesh := range element.MeshComposite {
							worldApp.AddToScene(mesh)
						}
						// meshcomp.GetNode().meshComposite
						worldApp.AddToScene(worldApp.ConcreteElements[subChildID].GetNamedMesh(worldApp.ConcreteElements[subChildID].GetDisplayName()))
					}
				}
			}
			// e.SetNamedMesh(e.GetDisplayName()+strconv.Itoa(int(e.GetDisplayId())), e.GetNamedMesh(e.GetDisplayName()+strconv.Itoa(int(e.GetDisplayId()))))
			// element := e.GetNamedMesh(e.GetDisplayName() + strconv.Itoa(int(e.GetDisplayId())))
			// worldApp.UpsertToScene(element)
			// worldApp.AddToScene(worldApp.ConcreteElements[int64(childId)].GetNamedMesh(worldApp.ConcreteElements[int64(childId)].GetDisplayName()))
		}
		g3nColor = math32.NewColor("darkred")
		mesh := g3nDetailedElement.GetNamedMesh(g3nDetailedElement.GetDisplayName())
		if sp.activeSet == nil {
			sp.activeSet = map[int64]*math32.Vector3{}
		}
		activePosition := mesh.(*graphic.Mesh).GetGraphic().Position()
		sp.activeSet[g3nDetailedElement.GetDetailedElement().GetId()] = &activePosition

		fmt.Printf("Active element centered at %v\n", activePosition)
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
}

func (sp *PathRenderer) Collaborate(worldApp *g3nworld.WorldApp, collaboratingRenderer g3nrender.IG3nRenderer) {
	curveRenderer := collaboratingRenderer.(*CurveRenderer)
	curveRenderer.totalElements = sp.totalElements
}
