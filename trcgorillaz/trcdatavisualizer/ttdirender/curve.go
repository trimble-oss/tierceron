package ttdirender

import (
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/g3n/engine/core"
	"github.com/g3n/engine/graphic"
	"github.com/g3n/engine/material"
	"github.com/g3n/engine/math32"
	"github.com/mrjrieke/nute/g3nd/g3nmash"
	"github.com/mrjrieke/nute/g3nd/g3nworld"

	"github.com/mrjrieke/nute/g3nd/worldg3n/g3nrender"

	"github.com/g3n/engine/geometry"
)

var sqrtfive float64 = float64(math.Sqrt(float64(5.0)))
var goldenRatio float64 = (float64(1.0) + sqrtfive) / (float64(2.0))

type CurveRenderer struct {
	g3nrender.GenericRenderer
	er                    *ElementRenderer
	CollaboratingRenderer g3nrender.IG3nRenderer
	totalElements         int
	clickedPaths          []*CurveMesh
	Data                  []string
	TimeData              map[string][]float64
}

type CurveMesh struct {
	path       *graphic.Mesh
	g3nElement *g3nmash.G3nDetailedElement
}

func (cr *CurveRenderer) isEmpty() bool {
	return len(cr.clickedPaths) == 0
}

func (cr *CurveRenderer) length() int {
	return len(cr.clickedPaths)
}

func (cr *CurveRenderer) push(spiralPath *graphic.Mesh, g3nDetailedElement *g3nmash.G3nDetailedElement) {
	element := CurveMesh{
		path:       spiralPath,
		g3nElement: g3nDetailedElement,
	}
	cr.clickedPaths = append(cr.clickedPaths, &element)
}

func (cr *CurveRenderer) pop() *CurveMesh {
	size := len(cr.clickedPaths)
	element := cr.clickedPaths[size-1]
	cr.clickedPaths = cr.clickedPaths[:size-1]
	return element
}

func (cr *CurveRenderer) top() *CurveMesh {
	return cr.clickedPaths[cr.length()-1]
}

func binetFormula(n float64) complex128 {
	real := (float64(math.Pow(goldenRatio, n)) - float64(math.Cos(float64(math.Pi)*n)*math.Pow(goldenRatio, -n))) / sqrtfive
	imag := (float64(-1.0) * float64(math.Sin(math.Pi*n)) * float64(math.Pow(goldenRatio, -n))) / sqrtfive
	return complex(real, imag)
}

func (cr *CurveRenderer) NewSolidAtPosition(g3n *g3nmash.G3nDetailedElement, vpos *math32.Vector3) core.INode {
	if g3n.GetDetailedElement().Alias == "DataFlowStatistic" {
		return nil
	}
	var path []math32.Vector3
	var i float64
	if cr.totalElements == 0 {
		cr.totalElements = 20
	}
	for i = -0.1 * float64(cr.totalElements-1); i < -0.1; i = i + 0.1 {
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
	mat.SetOpacity(0.1)
	tubeMesh := graphic.NewMesh(tubeGeometry, mat)
	fmt.Printf("LoaderID: %s\n", g3n.GetDisplayName())
	tubeMesh.SetLoaderID(g3n.GetDisplayName())
	tubeMesh.SetPositionVec(vpos)
	return tubeMesh
}

func (sp *CurveRenderer) NewInternalMeshAtPosition(g3n *g3nmash.G3nDetailedElement, vpos *math32.Vector3) core.INode {
	return nil
}

func (cr *CurveRenderer) NextCoordinate(g3n *g3nmash.G3nDetailedElement, totalElements int) (*g3nmash.G3nDetailedElement, *math32.Vector3) {
	return g3n, math32.NewVector3(float32(0.0), float32(0.0), float32(0.0))
}

func (cr *CurveRenderer) Layout(worldApp *g3nworld.WorldApp,
	g3nRenderableElements []*g3nmash.G3nDetailedElement) {
	cr.GenericRenderer.LayoutBase(worldApp, cr, g3nRenderableElements)
}

func (cr *CurveRenderer) GetRenderer(rendererName string) g3nrender.IG3nRenderer {
	if cr.CollaboratingRenderer != nil {
		return cr.CollaboratingRenderer
	}
	return nil
}

func (cr *CurveRenderer) removeRelated(worldApp *g3nworld.WorldApp, clickedElement *g3nmash.G3nDetailedElement, element *g3nmash.G3nDetailedElement) {
	if !cr.isEmpty() && len(element.GetParentElementIds()) != 0 && len(clickedElement.GetParentElementIds()) != 0 && element.GetParentElementIds()[0] == clickedElement.GetParentElementIds()[0] {
		toRemove := cr.pop()
		worldApp.RemoveFromScene(toRemove.path)
		if !cr.isEmpty() && len(cr.top().g3nElement.GetParentElementIds()) != 0 && len(clickedElement.GetParentElementIds()) != 0 && cr.top().g3nElement.GetParentElementIds()[0] == clickedElement.GetParentElementIds()[0] {
			cr.removeRelated(worldApp, clickedElement, cr.top().g3nElement)
		}
	} else if !cr.isEmpty() {
		toRemove := cr.pop()
		worldApp.RemoveFromScene(toRemove.path)
		if !cr.isEmpty() && !(len(element.GetParentElementIds()) != 0 && len(clickedElement.GetParentElementIds()) != 0 && element.GetParentElementIds()[0] == clickedElement.GetParentElementIds()[0]) {
			cr.removeRelated(worldApp, clickedElement, cr.top().g3nElement)
		}
	}
}

func (cr *CurveRenderer) InitRenderLoop(worldApp *g3nworld.WorldApp) bool {
	// TODO: noop
	if !cr.isEmpty() && worldApp.ClickedElements[len(worldApp.ClickedElements)-1].GetDetailedElement().Alias != "DataFlowStatistic" && !cr.er.isChildElement(worldApp, cr.top().g3nElement) {
		cr.removeRelated(worldApp, worldApp.ClickedElements[len(worldApp.ClickedElements)-1], cr.top().g3nElement)
	}
	return true
}

func (cr *CurveRenderer) succeeded(element *g3nmash.G3nDetailedElement) (bool, []float64) {
	for _, stringName := range cr.Data {
		if strings.HasPrefix(element.GetDisplayName(), stringName) {
			return cr.TimeData[stringName][len(cr.TimeData[stringName])-1] == cr.TimeData[stringName][len(cr.TimeData[stringName])-2], cr.TimeData[stringName]
		}
	}
	return false, nil
}

func (cr *CurveRenderer) RenderElement(worldApp *g3nworld.WorldApp, g3nDetailedElement *g3nmash.G3nDetailedElement) bool {
	clickedElement := worldApp.ClickedElements[len(worldApp.ClickedElements)-1]
	var path []math32.Vector3
	//Handle clicking of dataflow first? ignore dfstat until df is clicked?
	if g3nDetailedElement.GetDetailedElement().Id == 2 {
		a := clickedElement != nil
		b := clickedElement.GetDetailedElement().Alias == "DataFlow"
		c := clickedElement.GetNamedMesh(clickedElement.GetDisplayName()) != nil
		//cr.er.LocationCache[clickedElement.GetDetailedElement().Id] != nil
		if a && b && c {
			_, timeSplits := cr.succeeded(clickedElement)
			if len(clickedElement.GetChildElementIds()) > 0 && clickedElement.GetDetailedElement().Genre != "Solid" && clickedElement.GetDetailedElement().Alias != "DataFlowStatistic" {
				section := (-0.1 * 20.0) / float64(len(clickedElement.GetChildElementIds()))
				lastLocation := 0.0
				color := math32.NewColor("white")
				diff := 0.0
				for j := 0.0; j < float64(len(timeSplits)); j = j + 1.0 {
					if len(timeSplits) > int(j+1) {
						diff = timeSplits[int(j+1)] - timeSplits[int(j)]
						section = (((timeSplits[int(j+1)] - timeSplits[int(j)]) / timeSplits[len(timeSplits)-1]) * -2) + lastLocation
					}
					if section != 0 && section-lastLocation != 0 {
						for i := section; i < lastLocation; i = i + math.Abs((section-lastLocation)/((lastLocation-section)*100)) {
							c := binetFormula(i)
							x := real(c)
							y := imag(c)
							z := -i
							location := *math32.NewVector3(float32(-x), float32(y), float32(z))
							path = append(path, location)
						}
					}
					//CHANGE COLOR BASED ON TIME TEST TOOK
					//IF TWO TESTS TOOK SAME AMOUNT OF TIME --> MAKE WHICH TEST HAPPENED FIRST A LIGHTER SHADE??
					if int(j)%2 != 0 {
						color = math32.NewColor("black")
					} else {
						color = math32.NewColor("white")
					}
					complex := binetFormula(lastLocation)
					path = append(path, *math32.NewVector3(float32(-real(complex)), float32(imag(complex)), -float32(lastLocation)))
					if len(path) > 1 {
						if diff < 18.1629166 {
							//change color here
							color.Set(1, 0.808, 0.843)
							color.Set(0.722, 0.2, 0.306)
						} else if diff < 36.315 {
							color.Set(1, 0.243, 0.396)
						} else if diff < 315.65 {
							color.Set(0.968, 0, 0.188)
						} else if diff < 594.99 {
							color.Set(0.761, 0, 0.161)
						}
						lastLocation = section
						tubeGeometry := geometry.NewTube(path, .007, 32, true)
						mat := material.NewStandard(color)
						tubeMesh := graphic.NewMesh(tubeGeometry, mat)
						tubeMesh.SetLoaderID(clickedElement.GetDisplayName() + "-Curve" + strconv.Itoa(int(j)))
						locn := clickedElement.GetNamedMesh(clickedElement.GetDisplayName()).Position()
						tubeMesh.SetPositionVec(&locn) //cr.er.LocationCache[clickedElement.GetDetailedElement().Id])
						cr.push(tubeMesh, clickedElement)
						worldApp.UpsertToScene(tubeMesh)
					} else {
						fmt.Println(section)
					}

					path = []math32.Vector3{}
				}

			}
		} else {
			position := math32.NewVector3(1.0, 2.0, 3.0)

			if clickedElement.GetNamedMesh(clickedElement.GetDisplayName()) != nil && clickedElement.GetDetailedElement().Genre != "Solid" {
				locn := clickedElement.GetNamedMesh(clickedElement.GetDisplayName()).Position()
				position = &locn
			}
			if len(clickedElement.GetChildElementIds()) > 0 && clickedElement.GetDetailedElement().Genre != "Solid" && clickedElement.GetDetailedElement().Alias != "DataFlowStatistic" {
				for i := -0.1 * 20.0; i < -0.1; i = i + 0.1 { //float64(len(clickedElement.GetChildElementIds())-1)
					c := binetFormula(i)
					x := real(c)
					y := imag(c)
					z := -i
					path = append(path, *math32.NewVector3(float32(-x), float32(y), float32(z)))
				}
				path = append(path, *math32.NewVector3(float32(0.0), float32(0.0), float32(0.0)))
				tubeGeometry := geometry.NewTube(path, .007, 32, true)
				color := math32.NewColor("darkmagenta")
				if clickedElement.GetDetailedElement().Alias == "Argosy" {
					color.Set(0.435, 0.541, 0.420)
				} else if clickedElement.GetDetailedElement().Alias == "DataFlowGroup" {
					color.Set(0.675, 0.624, 0.773)
				}
				// else if clickedElement.GetDetailedElement().Alias == "DataFlow" {
				// 	color.Set(0.773, 0.675, 0.624)
				// }
				mat := material.NewStandard(color)
				mat.SetOpacity(0.1)
				tubeMesh := graphic.NewMesh(tubeGeometry, mat)
				tubeMesh.SetLoaderID(clickedElement.GetDisplayName() + "-Curve")
				tubeMesh.SetPositionVec(position)
				cr.push(tubeMesh, clickedElement)
				worldApp.UpsertToScene(tubeMesh)
			}
		}

	}
	return true
}

func (cr *CurveRenderer) Collaborate(worldApp *g3nworld.WorldApp, collaboratingRenderer g3nrender.IG3nRenderer) {
	cr.CollaboratingRenderer.Collaborate(worldApp, cr)
}
