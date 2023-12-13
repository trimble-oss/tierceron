package ttdirender

import (
	"github.com/g3n/engine/core"
	"github.com/g3n/engine/math32"
	"github.com/trimble-oss/tierceron-nute/g3nd/g3nmash"
	"github.com/trimble-oss/tierceron-nute/g3nd/g3nworld"
	g3ndpalette "github.com/trimble-oss/tierceron-nute/g3nd/palette"
	"github.com/trimble-oss/tierceron-nute/g3nd/worldg3n/g3nrender"
)

type BackgroundRenderer struct {
	g3nrender.GenericRenderer
}

func (br *BackgroundRenderer) NewSolidAtPosition(g3n *g3nmash.G3nDetailedElement, vpos *math32.Vector3) core.INode {
	return nil
}

func (br *BackgroundRenderer) NewInternalMeshAtPosition(g3n *g3nmash.G3nDetailedElement, vpos *math32.Vector3) core.INode {
	return nil
}

func (br *BackgroundRenderer) NewRelatedMeshAtPosition(g3n *g3nmash.G3nDetailedElement, vpos *math32.Vector3, vprevpos *math32.Vector3) core.INode {
	return nil
}

func (br *BackgroundRenderer) NextCoordinate(g3n *g3nmash.G3nDetailedElement, totalElements int) (*g3nmash.G3nDetailedElement, *math32.Vector3) {
	return nil, nil
}

func (br *BackgroundRenderer) Layout(worldApp *g3nworld.WorldApp,
	g3nRenderableElements []*g3nmash.G3nDetailedElement) {
	//return
}

func (br *BackgroundRenderer) InitRenderLoop(worldApp *g3nworld.WorldApp) bool {
	// TODO: noop
	return true
}

func (br *BackgroundRenderer) RenderElement(worldApp *g3nworld.WorldApp, g3n *g3nmash.G3nDetailedElement) bool {
	var g3nColor *math32.Color

	if g3n.IsItemActive() {
		g3nColor = g3ndpalette.DARK_RED
		g3nColor.Set(0.266, 0.266, 0.266)
	} else {
		g3nColor = math32.NewColor("black")
		g3nColor.Set(0.266, 0.266, 0.266)
	}

	return g3n.SetColor(g3nColor, 1.0)
}
