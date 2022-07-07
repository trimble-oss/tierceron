package ttdirender

import (
	"fmt"
	"math"

	//"sort"
	//"strings"

	"github.com/g3n/engine/geometry"
	"github.com/g3n/engine/graphic"
	"github.com/g3n/engine/material"
	"github.com/g3n/engine/math32"
	"github.com/mrjrieke/nute/g3nd/g3nmash"
	"github.com/mrjrieke/nute/g3nd/g3nworld"
	g3ndpalette "github.com/mrjrieke/nute/g3nd/palette"
	"github.com/mrjrieke/nute/g3nd/worldg3n/g3nrender"
)

type PathRenderer struct {
	g3nrender.GenericRenderer
	iOffset int
	counter float64
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

func (sp *PathRenderer) NewSolidAtPosition(g3n *g3nmash.G3nDetailedElement, vpos *math32.Vector3) *graphic.Mesh {
	sphereGeom := geometry.NewSphere(.1, 100, 100)
	mat := material.NewStandard(g3ndpalette.DARK_BLUE)
	sphereMesh := graphic.NewMesh(sphereGeom, mat)
	fmt.Printf("LoaderID: %s\n", g3n.GetDisplayName())
	sphereMesh.SetLoaderID(g3n.GetDisplayName())
	sphereMesh.SetPositionVec(vpos)
	return sphereMesh
}

func (sp *PathRenderer) NewInternalMeshAtPosition(g3n *g3nmash.G3nDetailedElement, vpos *math32.Vector3) *graphic.Mesh {
	return nil
}

func binet(n float64) complex128 {
	goldenRatio := (float64(1.0) + float64(math.Sqrt(float64(5.0)))) / (float64(2.0))
	real := (float64(math.Pow(goldenRatio, n)) - float64(math.Cos(float64(math.Pi)*n)*math.Pow(goldenRatio, -n))) / (float64(math.Sqrt(float64(5.0))))
	imag := (float64(-1.0) * float64(math.Sin(math.Pi*n)) * float64(math.Pow(goldenRatio, -n))) / (math.Sqrt(float64(5.0)))
	return complex(real, imag)
}

func (sp *PathRenderer) NextCoordinate(g3n *g3nmash.G3nDetailedElement, totalElements int) (*g3nmash.G3nDetailedElement, *math32.Vector3) {
	if sp.iOffset == 0 {
		sp.iOffset = 1
		sp.counter = -10.0
		return g3n, math32.NewVector3(float32(0.0), float32(0.0), float32(0.0))
	} else {
		sp.counter = sp.counter + 0.1
		complex := binet(sp.counter)
		fmt.Println(complex)
		return g3n, math32.NewVector3(float32(-real(complex)), float32(imag(complex)), float32(-sp.counter))
	}
}

func (sp *PathRenderer) Layout(worldApp *g3nworld.WorldApp,
	g3nRenderableElements []*g3nmash.G3nDetailedElement) {
	sp.GenericRenderer.LayoutBase(worldApp, sp, g3nRenderableElements)
}
