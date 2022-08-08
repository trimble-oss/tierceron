/*
NOTE: Creating own geometry seems to be ok besides having to comment
out the bounding box and sphere. There is a problem with creating a unique
mesh in the layout method when calling layoutbase --> requires graphic.Mesh
*/

package ttdirender

import (
	"fmt"
	"log"
	"strconv"

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

type SubSpiralRenderer struct {
	g3nrender.GenericRenderer
	iOffset       int
	counter       float64
	locnCounter   *math32.Vector3
	totalElements int
	activeSet     map[int64]*math32.Vector3
	compoundMesh  *CompoundMesh
}

func (sp *SubSpiralRenderer) NewSolidAtPosition(g3n *g3nmash.G3nDetailedElement, vpos *math32.Vector3) core.INode {
	meshes := make([]*graphic.Mesh, 25)
	for i := 0; i < 25; i++ {
		geom := geometry.NewSphere(.1, 100, 100) //new geometry seems to be ok but had to comment out bounding box and sphere for it to work
		color := math32.NewColor("darkred")
		mat := material.NewStandard(color.Set(0.278, 0.529, 0.741))
		sphereMesh := graphic.NewMesh(geom, mat)
		fmt.Printf("LoaderID: %s\n", g3n.GetDisplayName()+strconv.Itoa(int(g3n.GetDisplayId())+i)) //+strconv.Itoa(int(g3n.GetDisplayId()))
		sphereMesh.SetLoaderID(g3n.GetDisplayName() + strconv.Itoa(int(g3n.GetDisplayId())+i))
		sphereMesh.SetPositionVec(math32.NewVector3(float32(i), 0.0, 0.0))
		meshes[i] = sphereMesh
	}
	compoundMesh := NewCompoundMesh(meshes)
	compoundMesh.SetLoaderID(g3n.GetDisplayName())
	sp.compoundMesh = compoundMesh
	g3n.SetNamedMesh(g3n.GetDisplayName(), compoundMesh) //compoundMesh.LoaderID(), compoundMesh)
	if g3n.IsStateSet(mashupsdk.Hidden) {
		sp.compoundMesh = nil
		return nil
	}
	return compoundMesh
}

func (sp *SubSpiralRenderer) NewInternalMeshAtPosition(g3n *g3nmash.G3nDetailedElement, vpos *math32.Vector3) core.INode {
	return nil
}

func (sp *SubSpiralRenderer) NextCoordinate(g3n *g3nmash.G3nDetailedElement, totalElements int) (*g3nmash.G3nDetailedElement, *math32.Vector3) {
	sp.totalElements = totalElements
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

func (sp *SubSpiralRenderer) Layout(worldApp *g3nworld.WorldApp,
	g3nRenderableElements []*g3nmash.G3nDetailedElement) {
	sp.LayoutBase(worldApp, sp, g3nRenderableElements) //Doesn't accept new mesh type in call to layoutbase
}

func (sp *SubSpiralRenderer) HandleStateChange(worldApp *g3nworld.WorldApp, g3nDetailedElement *g3nmash.G3nDetailedElement) bool {
	var g3nColor *math32.Color

	if g3nDetailedElement.IsStateSet(mashupsdk.Hidden) {
		if g3nDetailedElement.GetDetailedElement().Genre == "Collection" && g3nDetailedElement.GetDetailedElement().Subgenre == "SubSpiral" {
			//sp.TorusParser(worldApp, g3nDetailedElement.GetDetailedElement().Id)
		} else {
			log.Printf("Item removed %s: %v", g3nDetailedElement.GetDisplayName(), worldApp.RemoveFromScene(g3nDetailedElement.GetNamedMesh(g3nDetailedElement.GetDisplayName())))
		}
		return true
	} else {
		//name := g3nDetailedElement.GetDisplayName()
		//worldApp.UpsertToScene(g3nDetailedElement.GetNamedMesh(name))
	}

	if g3nDetailedElement.IsItemActive() {
		if !g3nDetailedElement.IsStateSet(mashupsdk.Hidden) {
			name := g3nDetailedElement.GetDisplayName()
			compoundMesh := g3nDetailedElement.GetNamedMesh(name)
			//meshes := compoundMesh.(*CompoundMesh).GetMeshes()
			sp.compoundMesh = compoundMesh.(*CompoundMesh)
			sp.LayoutMesh(worldApp)
			// for _, mesh := range meshes {
			// 	worldApp.AddToScene(mesh)
			// }
			//worldApp.UpsertToScene()
		}
		g3nColor = math32.NewColor("darkred")
		// mesh := g3nDetailedElement.GetNamedMesh(g3nDetailedElement.GetDisplayName())
		// if sp.activeSet == nil {
		// 	sp.activeSet = map[int64]*math32.Vector3{}
		// }
		// activePosition := mesh.(*graphic.Mesh).GetGraphic().Position()
		// sp.activeSet[g3nDetailedElement.GetDetailedElement().GetId()] = &activePosition
		// fmt.Printf("Active element centered at %v\n", activePosition)
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

func (sp *SubSpiralRenderer) LayoutMesh(worldApp *g3nworld.WorldApp) {
	if sp.compoundMesh != nil {
		counter := -float64(0.1) * float64(len(sp.compoundMesh.meshes))

		for i := 0; i < len(sp.compoundMesh.meshes); i += 1 {
			complex := binetFormula(counter) //+nextPos.X  nextPos.Z+
			sp.compoundMesh.meshes[i].SetPositionVec(math32.NewVector3(float32(-real(-complex))+sp.locnCounter.X, float32(imag(complex))+sp.locnCounter.Y, float32(-counter)+sp.locnCounter.Z))
			counter += 0.1
			worldApp.AddToScene(sp.compoundMesh.meshes[i])
			//concreteG3nRenderableElement.GetDisplayName()
		}
		//concreteG3nRenderableElement.SetNamedMesh(gr.compoundMesh.LoaderID(), gr.compoundMesh)
	}
}

func (gr *SubSpiralRenderer) LayoutBase(worldApp *g3nworld.WorldApp,
	g3Renderer *SubSpiralRenderer,
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

		prevSolidPos = nextPos
		_, nextPos = g3Renderer.NextCoordinate(concreteG3nRenderableElement, totalElements)
		gr.locnCounter = nextPos
		g3Renderer.NewSolidAtPosition(concreteG3nRenderableElement, nextPos)
		counter := 0.0
		if gr.compoundMesh != nil {
			counter = -float64(0.1) * float64(len(gr.compoundMesh.meshes))
		}

		if gr.compoundMesh != nil {
			for i := 0; i < len(gr.compoundMesh.meshes); i += 1 {
				complex := binetFormula(counter) //+nextPos.X  nextPos.Z+
				gr.compoundMesh.meshes[i].SetPositionVec(math32.NewVector3(float32(-real(-complex))+nextPos.X, float32(imag(complex))+nextPos.Y, float32(-counter)+nextPos.Z))
				counter += 0.1
				worldApp.AddToScene(gr.compoundMesh.meshes[i])
				//concreteG3nRenderableElement.GetDisplayName()
			}
			//concreteG3nRenderableElement.SetNamedMesh(gr.compoundMesh.LoaderID(), gr.compoundMesh)
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

/*func (sp *SubSpiralRenderer) GetRenderer(rendererName string) g3nrender.G3nRenderer {
	if sp.CollaboratingRenderer != nil {
		return sp.CollaboratingRenderer
	}
	return nil
}

func (sp *SubSpiralRenderer) Collaborate(worldApp *g3nworld.WorldApp, collaboratingRenderer interface{}) {
	sp.CollaboratingRenderer.Collaborate(worldApp, sp)
}*/
