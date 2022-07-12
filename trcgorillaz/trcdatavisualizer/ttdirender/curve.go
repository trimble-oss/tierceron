package ttdirender

import (
	"fmt"
	"math"

	"github.com/g3n/engine/graphic"
	"github.com/g3n/engine/material"
	"github.com/g3n/engine/math32"
	"github.com/mrjrieke/nute/g3nd/g3nmash"
	"github.com/mrjrieke/nute/g3nd/g3nworld"

	//g3ndpalette "github.com/mrjrieke/nute/g3nd/palette"
	"github.com/mrjrieke/nute/g3nd/worldg3n/g3nrender"

	"github.com/g3n/engine/geometry"
	//"tierceron/trcgorillaz/trcdatavisualizer/ttdiGeometry"
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
	//var path *[]math32.Vector3
	/*path := make([]math32.Vector3, 3)
	path[0] = *math32.NewVector3(1.0, 0.0, 0.0)
	path[1] = *math32.NewVector3(2.0, 0.0, 0.0)
	path[2] = *math32.NewVector3(3.0, 0.0, 0.0)
	tubeGeometry := geometry.NewTube(path, 1, 10, true)*/
	var path []math32.Vector3
	var i float64
	for i = -20.0; i <= -0.1; i = i + 0.1 {
		c := binetFormula(i)
		x := real(c)
		y := imag(c)
		//x := math.Cos(math.Pi*2*i/10) * 3
		//y := i / 3
		//z := math.Sin(math.Pi*2*i/10) * 3
		z := -i
		path = append(path, *math32.NewVector3(float32(-x), float32(y), float32(z)))
	}
	path = append(path, *math32.NewVector3(float32(0.0), float32(0.0), float32(0.0)))
	fmt.Println(binetFormula(-20.0))
	fmt.Println(binetFormula(0.0))
	fmt.Println(i)
	fmt.Println(binetFormula(i))
	tubeGeometry := geometry.NewTube(path, .09, 32, true)

	/*lineGeom := geometry.NewGeometry()
	vertices := math32.NewArrayF32(0, 16)
	vertices.Append(make([]math32.Vector3, l)
		-1.0, 0.0, 0.0,lineGeom
		0.5, 0.0, 0.0,
		0.0, -0.25, 0.0,
		0.0, 0.5, 0.0,
		0.0, 0.0, -0.5,
		0.0, 0.0, 0.5,
	)
	colors := math32.NewArrayF32(0, 16)
	colors.Append(
		1.0, 0.0, 0.0,
		1.0, 0.0, 0.0,
		0.0, 1.0, 0.0,
		0.0, 1.0, 0.0,
		0.0, 0.0, 1.0,
		0.0, 0.0, 1.0,
	)
	lineGeom.AddVBO(gls.NewVBO(vertices).AddAttrib(gls.VertexPosition))
	lineGeom.AddVBO(gls.NewVBO(colors).AddAttrib(gls.VertexColor))*/

	//sphereGeom := geometry.NewSphere(.5, 100, 100)

	mat := material.NewStandard(math32.NewColor("hotpink"))
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

	/*if sp.iOffset == 0 {
		sp.iOffset = 1
		sp.counter = 0
		sp.fib1 = float64(0.0)
		sp.fib2 = float64(1.0)
		sp.location.SetX(float32(0.0))
		sp.location.SetY(float32(0.0))
		sp.location.SetZ(float32(0.0))
		return g3n, math32.NewVector3(float32(0.0), float32(0.0), float32(0.0))
	} else {
		//Finding new x coordinate
		powX := math32.Floor(float32(float32(sp.counter)-2.0) / float32(2.0))
		newX := sp.location.X + float32(float64(sp.fib2)*(math.Pow(-1, float64(powX))))

		//Finding new y coordinate
		powY := math32.Floor(float32(float32(sp.counter)-3.0) / float32(2.0)) //(sp.counter - 3) / 2
		newY := sp.location.Y + float32(float64(sp.fib2)*(math.Pow(-1, float64(powY))))

		//Later find new z coordinate

		//Updating counter, fib1, fib2 and location
		sp.counter = sp.counter + 1
		oldfib2 := sp.fib2
		sp.fib2 = sp.fib1 + sp.fib2
		sp.fib1 = oldfib2
		sp.location.SetX(newX)
		sp.location.SetY(newY)

		return g3n, math32.NewVector3(newX, newY, 0.0)
	}*/

}

func (sp *CurveRenderer) Layout(worldApp *g3nworld.WorldApp,
	g3nRenderableElements []*g3nmash.G3nDetailedElement) {
	sp.GenericRenderer.LayoutBase(worldApp, sp, g3nRenderableElements)
}

/*package ttdirender

import (
	"fmt"

	//"sort"
	//"strings"

	"github.com/g3n/engine/geometry"
	"github.com/g3n/engine/gls"
	"github.com/g3n/engine/graphic"
	"github.com/g3n/engine/material"
	"github.com/g3n/engine/math32"
	"github.com/mrjrieke/nute/g3nd/g3nmash"
	"github.com/mrjrieke/nute/g3nd/g3nworld"
	g3ndpalette "github.com/mrjrieke/nute/g3nd/palette"
	"github.com/mrjrieke/nute/g3nd/worldg3n/g3nrender"
)

type CurveRenderer struct {
	g3nrender.GenericRenderer
}

/*type TTDICollection g3nrender.G3nCollection

func (a TTDICollection) Len() int { return len(a) }
func (a TTDICollection) Less(i, j int) bool {
	return strings.Compare(a[i].GetDisplayName(), a[j].GetDisplayName()) < 0
}
func (a TTDICollection) Swap(i, j int) { a[i], a[j] = a[j], a[i] }

func (sp *CurveRenderer) Sort(worldApp *g3nworld.WorldApp, g3nRenderableElements g3nrender.G3nCollection) g3nrender.G3nCollection {
	sort.Sort(TTDICollection(g3nRenderableElements))
	return g3nRenderableElements
}

func (sp *CurveRenderer) NewSolidAtPosition(g3n *g3nmash.G3nDetailedElement, vpos *math32.Vector3) *graphic.Mesh {
	//lineGeom := graphic.Ne
	lineGeom := geometry.NewGeometry()
	vertices := math32.NewArrayF32(0, 16)
	vertices.Append(
		-1.0, 0.0, 0.0,
		0.5, 0.0, 0.0,
		0.0, -0.25, 0.0,
		0.0, 0.5, 0.0,
		0.0, 0.0, -0.5,
		0.0, 0.0, 0.5,
	)
	colors := math32.NewArrayF32(0, 16)
	colors.Append(
		1.0, 0.0, 0.0,
		1.0, 0.0, 0.0,
		0.0, 1.0, 0.0,
		0.0, 1.0, 0.0,
		0.0, 0.0, 1.0,
		0.0, 0.0, 1.0,
	)
	lineGeom.AddVBO(gls.NewVBO(vertices).AddAttrib(gls.VertexPosition))
	lineGeom.AddVBO(gls.NewVBO(colors).AddAttrib(gls.VertexColor))

	//sphereGeom := geometry.NewSphere(.5, 100, 100)
	mat := material.NewStandard(g3ndpalette.DARK_BLUE)
	sphereMesh := graphic.NewMesh(lineGeom, mat)
	fmt.Printf("LoaderID: %s\n", g3n.GetDisplayName())
	sphereMesh.SetLoaderID(g3n.GetDisplayName())
	sphereMesh.SetPositionVec(vpos)
	return sphereMesh
}

func (sp *CurveRenderer) NewInternalMeshAtPosition(g3n *g3nmash.G3nDetailedElement, vpos *math32.Vector3) *graphic.Mesh {
	return nil
}

func (sp *CurveRenderer) NextCoordinate(g3n *g3nmash.G3nDetailedElement, totalElements int) (*g3nmash.G3nDetailedElement, *math32.Vector3) {
	return g3n, math32.NewVector3(0.0, 0.0, 0.0)
	/*if sp.iOffset == 0 {
		sp.iOffset = 1
		sp.counter = 0
		sp.fib1 = float64(0.0)
		sp.fib2 = float64(1.0)
		sp.location.SetX(float32(0.0))
		sp.location.SetY(float32(0.0))
		sp.location.SetZ(float32(0.0))
		return g3n, math32.NewVector3(float32(0.0), float32(0.0), float32(0.0))
	} else {
		//Finding new x coordinate
		powX := math32.Floor(float32(float32(sp.counter)-2.0) / float32(2.0))
		newX := sp.location.X + float32(float64(sp.fib2)*(math.Pow(-1, float64(powX))))

		//Finding new y coordinate
		powY := math32.Floor(float32(float32(sp.counter)-3.0) / float32(2.0)) //(sp.counter - 3) / 2
		newY := sp.location.Y + float32(float64(sp.fib2)*(math.Pow(-1, float64(powY))))

		//Later find new z coordinate

		//Updating counter, fib1, fib2 and location
		sp.counter = sp.counter + 1
		oldfib2 := sp.fib2
		sp.fib2 = sp.fib1 + sp.fib2
		sp.fib1 = oldfib2
		sp.location.SetX(newX)
		sp.location.SetY(newY)

		return g3n, math32.NewVector3(newX, newY, 0.0)
	}

}

func (sp *CurveRenderer) Layout(worldApp *g3nworld.WorldApp,
	g3nRenderableElements []*g3nmash.G3nDetailedElement) {
	sp.GenericRenderer.LayoutBase(worldApp, sp, g3nRenderableElements)
}*/
