package ttdirender

import (
	"github.com/g3n/engine/core"
	"github.com/g3n/engine/math32"
	"github.com/mrjrieke/nute/g3nd/g3nmash"
	"github.com/mrjrieke/nute/g3nd/g3nworld"
	"github.com/mrjrieke/nute/g3nd/worldg3n/g3nrender"
)

type ClickedG3nDetailElement struct {
	*g3nmash.G3nDetailedElement
	location *math32.Vector3
}

type ElementRenderer struct {
	g3nrender.GenericRenderer
	iOffset       int
	counter       float64
	locnCounter   *math32.Vector3
	totalElements int
	activeSet     map[int64]*math32.Vector3
	compoundMesh  *CompoundMesh

	clickedElements []*ClickedG3nDetailElement // Stack containing clicked spiral (sub as well) g3n elements.
}

func (er *ElementRenderer) NewSolidAtPosition(g3n *g3nmash.G3nDetailedElement, vpos *math32.Vector3) core.INode {

	return nil
}

func (er *ElementRenderer) NewInternalMeshAtPosition(g3n *g3nmash.G3nDetailedElement, vpos *math32.Vector3) core.INode {
	return nil
}

func (er *ElementRenderer) NextCoordinate(g3n *g3nmash.G3nDetailedElement, totalElements int) (*g3nmash.G3nDetailedElement, *math32.Vector3) {
	return nil, nil
}

func (er *ElementRenderer) Layout(worldApp *g3nworld.WorldApp,
	g3nRenderableElements []*g3nmash.G3nDetailedElement) {
	er.LayoutBase(worldApp, er, g3nRenderableElements) //Doesn't accept new mesh type in call to layoutbase
}

func (er *ElementRenderer) HandleStateChange(worldApp *g3nworld.WorldApp, g3nDetailedElement *g3nmash.G3nDetailedElement) bool {
	return true
}

func (er *ElementRenderer) LayoutMesh(worldApp *g3nworld.WorldApp) {

}

func (er *ElementRenderer) LayoutBase(worldApp *g3nworld.WorldApp,
	g3Renderer *ElementRenderer,
	g3nRenderableElements []*g3nmash.G3nDetailedElement) {

}
