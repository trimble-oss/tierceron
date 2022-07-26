/*
NOTE: Creating own geometry seems to be ok besides having to comment
out the bounding box and sphere. There is a problem with creating a unique
mesh in the layout method when calling layoutbase --> requires graphic.Mesh
*/

package ttdirender

import (
	"fmt"

	//"github.com/g3n/engine/geometry"
	"github.com/g3n/engine/graphic"
	"github.com/g3n/engine/material"
	"github.com/g3n/engine/math32"
	"github.com/mrjrieke/nute/g3nd/g3nmash"
	"github.com/mrjrieke/nute/g3nd/g3nworld"
	g3ndpalette "github.com/mrjrieke/nute/g3nd/palette"
	"github.com/mrjrieke/nute/g3nd/worldg3n/g3nrender"
)

type SubSpiralRenderer struct {
	g3nrender.GenericRenderer
	//CollaboratingRenderer g3nrender.G3nRenderer
	iOffset       int
	counter       float64
	locnCounter   float64
	totalElements int
	activeSet     map[int64]*math32.Vector3
}

func (sp *SubSpiralRenderer) NewSubSpiral(vpos *math32.Vector3) *SubSpiralRenderer {

	return sp
}

func (sp *SubSpiralRenderer) NewSolidAtPosition(g3n *g3nmash.G3nDetailedElement, vpos *math32.Vector3) *Mesh {
	//sphereGeom := geometry.NewSphere(.1, 100, 100)
	spiralGeom := NewSphere(.1, 100, 100) //new geometry seems to be ok but had to comment out bounding box and sphere for it to work
	color := g3ndpalette.DARK_BLUE
	mat := material.NewStandard(color.Set(0, 0.349, 0.643))
	sphereMesh := NewMesh(spiralGeom, mat)
	fmt.Printf("LoaderID: %s\n", g3n.GetDisplayName())
	sphereMesh.SetLoaderID(g3n.GetDisplayName())
	sphereMesh.SetPositionVec(vpos)
	return sphereMesh
}

func (sp *SubSpiralRenderer) NewInternalMeshAtPosition(g3n *g3nmash.G3nDetailedElement, vpos *math32.Vector3) *graphic.Mesh {
	return nil
}

func (sp *SubSpiralRenderer) NextCoordinate(g3n *g3nmash.G3nDetailedElement, totalElements int) (*g3nmash.G3nDetailedElement, *math32.Vector3) {
	sp.totalElements = totalElements
	if sp.iOffset == 0 {
		sp.iOffset = 1
		sp.counter = -0.1 * float64(15.0)
		sp.locnCounter = -0.1 * float64(totalElements/15)
		return g3n, math32.NewVector3(float32(0.0), float32(0.0), float32(0.0))
	} else if sp.iOffset%15 == 0 {
		sp.counter = -0.1 * float64(15.0)
		sp.iOffset++
		sp.locnCounter = 0.1 + sp.locnCounter
		complex := binetFormula(sp.locnCounter)
		return g3n, math32.NewVector3(float32(-real(complex)), float32(imag(complex)), -float32(sp.locnCounter))
	} else {
		sp.iOffset++
		sp.counter = sp.counter + 0.1
		complex := binetFormula(sp.locnCounter)
		complex2 := binetFormula(sp.counter)
		return g3n, math32.NewVector3(float32(-real(complex)+-real(complex2)), float32(imag(complex)+imag(complex2)), -float32(sp.locnCounter))
	}
}

func (sp *SubSpiralRenderer) Layout(worldApp *g3nworld.WorldApp,
	g3nRenderableElements []*g3nmash.G3nDetailedElement) {
	sp.GenericRenderer.LayoutBase(worldApp, sp, g3nRenderableElements) //Doesn't accept new mesh type in call to layoutbase
}

func (sp *SubSpiralRenderer) HandleStateChange(worldApp *g3nworld.WorldApp, g3nDetailedElement *g3nmash.G3nDetailedElement) bool {
	var g3nColor *math32.Color

	if g3nDetailedElement.IsItemActive() {
		g3nColor = math32.NewColor("darkred")
		mesh := g3nDetailedElement.GetNamedMesh(g3nDetailedElement.GetDisplayName())
		if sp.activeSet == nil {
			sp.activeSet = map[int64]*math32.Vector3{}
		}
		activePosition := mesh.GetGraphic().Position()
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

	return g3nDetailedElement.SetColor(g3nColor)
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
