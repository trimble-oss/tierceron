package ttdirender

import (
	"fmt"
	"math"

	"github.com/g3n/engine/graphic"
	"github.com/g3n/engine/material"
	"github.com/g3n/engine/math32"
	"github.com/mrjrieke/nute/g3nd/g3nmash"
	"github.com/mrjrieke/nute/g3nd/g3nworld"

	"github.com/mrjrieke/nute/g3nd/worldg3n/g3nrender"

	"github.com/g3n/engine/geometry"
)

type CurveRenderer struct {
	g3nrender.GenericRenderer
}

func binetFormula(n float64) complex128 {
	goldenRatio := (float64(1.0) + float64(math.Sqrt(float64(5.0)))) / (float64(2.0))
	real := (float64(math.Pow(goldenRatio, n)) - float64(math.Cos(float64(math.Pi)*n)*math.Pow(goldenRatio, -n))) / (float64(math.Sqrt(float64(5.0))))
	imag := (float64(-1.0) * float64(math.Sin(math.Pi*n)) * float64(math.Pow(goldenRatio, -n))) / (math.Sqrt(float64(5.0)))
	return complex(real, imag)
}

func (sp *CurveRenderer) NewSolidAtPosition(g3n *g3nmash.G3nDetailedElement, vpos *math32.Vector3) *graphic.Mesh {
	var path []math32.Vector3
	var i float64
	for i = -20.0; i <= -0.1; i = i + 0.1 {
		c := binetFormula(i)
		x := real(c)
		y := imag(c)
		z := -i
		path = append(path, *math32.NewVector3(float32(-x), float32(y), float32(z)))
	}
	path = append(path, *math32.NewVector3(float32(0.0), float32(0.0), float32(0.0)))
	fmt.Println(binetFormula(-20.0))
	fmt.Println(binetFormula(0.0))
	fmt.Println(i)
	fmt.Println(binetFormula(i))
	tubeGeometry := geometry.NewTube(path, .007, 32, true)
	color := math32.NewColor("darkmagenta")
	mat := material.NewStandard(color.Set(float32(148)/255.0, float32(120)/255.0, float32(42)/255.0))
	tubeMesh := graphic.NewMesh(tubeGeometry, mat)
	fmt.Printf("LoaderID: %s\n", g3n.GetDisplayName())
	tubeMesh.SetLoaderID(g3n.GetDisplayName())
	tubeMesh.SetPositionVec(vpos)
	return tubeMesh
}

func (sp *CurveRenderer) NewInternalMeshAtPosition(g3n *g3nmash.G3nDetailedElement, vpos *math32.Vector3) *graphic.Mesh {
	return nil
}

func (sp *CurveRenderer) NextCoordinate(g3n *g3nmash.G3nDetailedElement, totalElements int) (*g3nmash.G3nDetailedElement, *math32.Vector3) {
	return g3n, math32.NewVector3(0.0, 0.0, 0.0)
}

func (sp *CurveRenderer) Layout(worldApp *g3nworld.WorldApp,
	g3nRenderableElements []*g3nmash.G3nDetailedElement) {
	sp.GenericRenderer.LayoutBase(worldApp, sp, g3nRenderableElements)
}
