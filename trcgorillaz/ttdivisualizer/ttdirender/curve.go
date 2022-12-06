package ttdirender

import (
	"encoding/json"
	"fmt"
	"log"
	"math"
	"strconv"
	"strings"

	"github.com/g3n/engine/core"
	"github.com/g3n/engine/graphic"
	"github.com/g3n/engine/material"
	"github.com/g3n/engine/math32"
	"github.com/mrjrieke/nute/g3nd/g3nmash"
	"github.com/mrjrieke/nute/g3nd/g3nworld"
	"github.com/mrjrieke/nute/mashupsdk"

	"github.com/mrjrieke/nute/g3nd/worldg3n/g3nrender"

	"github.com/g3n/engine/geometry"
)

var sqrtfive float64 = float64(math.Sqrt(float64(5.0)))
var goldenRatio float64 = (float64(1.0) + sqrtfive) / (float64(2.0))
var ctrlel *g3nmash.G3nDetailedElement

type CurveRenderer struct {
	g3nrender.GenericRenderer
	er                    *ElementRenderer
	CollaboratingRenderer g3nrender.IG3nRenderer
	totalElements         int
	clickedPaths          []*CurveMesh
	maxTime               int
	quartiles             []float64
	avg                   float64
	isCtrl                bool
}

type CurveMesh struct {
	path       *graphic.Mesh
	g3nElement *g3nmash.G3nDetailedElement
}

// Returns true if length of cr.clickedPaths stack is 0 and false otherwise
func (cr *CurveRenderer) isEmpty() bool {
	return len(cr.clickedPaths) == 0
}

// Returns size of cr.clickedPaths stack
func (cr *CurveRenderer) length() int {
	return len(cr.clickedPaths)
}

// Adds given element and location to the cr.clickedPaths stack
func (cr *CurveRenderer) push(spiralPath *graphic.Mesh, g3nDetailedElement *g3nmash.G3nDetailedElement) {
	element := CurveMesh{
		path:       spiralPath,
		g3nElement: g3nDetailedElement,
	}
	cr.clickedPaths = append(cr.clickedPaths, &element)
}

// Removes and returns top element in cr.clickedPaths stack
func (cr *CurveRenderer) pop() *CurveMesh {
	size := len(cr.clickedPaths)
	element := cr.clickedPaths[size-1]
	cr.clickedPaths = cr.clickedPaths[:size-1]
	return element
}

// Returns top element in cr.clickedPaths stack
func (cr *CurveRenderer) top() *CurveMesh {
	return cr.clickedPaths[cr.length()-1]
}

// Calculates real and imaginary parts of Binet's Formula with given input and returns the value
func binetFormula(n float64) complex128 {
	real := (float64(math.Pow(goldenRatio, n)) - float64(math.Cos(float64(math.Pi)*n)*math.Pow(goldenRatio, -n))) / sqrtfive
	imag := (float64(-1.0) * float64(math.Sin(math.Pi*n)) * float64(math.Pow(goldenRatio, -n))) / sqrtfive
	return complex(real, imag)
}

// Returns and attaches a mesh to provided g3n element at given vector position
func (cr *CurveRenderer) NewSolidAtPosition(g3n *g3nmash.G3nDetailedElement, vpos *math32.Vector3) core.INode {
	if g3n.GetDetailedElement().Genre == "DataFlowStatistic" {
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
	mat.SetOpacity(0.25)
	tubeMesh := graphic.NewMesh(tubeGeometry, mat)
	fmt.Printf("LoaderID: %s\n", g3n.GetDisplayName())
	tubeMesh.SetLoaderID(g3n.GetDisplayName())
	tubeMesh.SetPositionVec(vpos)
	return tubeMesh
}

func (sp *CurveRenderer) NewInternalMeshAtPosition(g3n *g3nmash.G3nDetailedElement, vpos *math32.Vector3) core.INode {
	return nil
}

// Returns the element and location of the given element
func (cr *CurveRenderer) NextCoordinate(g3n *g3nmash.G3nDetailedElement, totalElements int) (*g3nmash.G3nDetailedElement, *math32.Vector3) {
	return g3n, math32.NewVector3(float32(0.0), float32(0.0), float32(0.0))
}

// Calls LayoutBase to render elements in a particular order and location
func (cr *CurveRenderer) Layout(worldApp *g3nworld.WorldApp,
	g3nRenderableElements []*g3nmash.G3nDetailedElement) {
	cr.GenericRenderer.LayoutBase(worldApp, cr, g3nRenderableElements)
}

// Returns the CollaboratingRenderer of the CurveRenderer
// If no collaborating renderer to the CurveRenderer, returns nil
func (cr *CurveRenderer) GetRenderer(rendererName string) g3nrender.IG3nRenderer {
	if cr.CollaboratingRenderer != nil {
		return cr.CollaboratingRenderer
	}
	return nil
}

// Removes elements if they share the same parent id
func (cr *CurveRenderer) removeRelated(worldApp *g3nworld.WorldApp, clickedElement *g3nmash.G3nDetailedElement, element *g3nmash.G3nDetailedElement) {
	if cr.isCtrl {
		cr.ctrlRemoveRelated(worldApp, clickedElement, element)
		cr.isCtrl = false
	} else {
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
}

func (cr *CurveRenderer) ctrlRemoveRelated(worldApp *g3nworld.WorldApp, clickedElement *g3nmash.G3nDetailedElement, element *g3nmash.G3nDetailedElement) {
	//clickedElement := worldApp.ClickedElements[len(worldApp.ClickedElements)-1]
	// if len(clickedElement.GetParentElementIds()) != 0 {
	// 	parent := clickedElement.GetParentElementIds()[0]
	// 	for _, child
	// }
	//clickedElement.GetParentElementIds() != nil &&
	if clickedElement.GetDetailedElement().Genre != "Space" {
		amount := 0
		for amount <= (len(cr.clickedPaths) - 1) { //for i := 0; i < len(er.ctrlElements); i++ {
			el := cr.clickedPaths[amount] //Add check that amount is within array length
			// if el.g3nElement.GetDetailedElement().Genre == "DataFlowGroup" {
			// 	fmt.Println("Check")
			// }
			a := !cr.er.isChildElement(worldApp, el.g3nElement)
			b := len(el.g3nElement.GetParentElementIds()) != 0
			d := len(clickedElement.GetParentElementIds()) != 0
			c := false
			if d && len(el.g3nElement.GetParentElementIds()) != 0 {
				c = el.g3nElement.GetParentElementIds()[0] == clickedElement.GetParentElementIds()[0]
			}
			e := false
			if d {
				e = el.g3nElement.GetDetailedElement().Id != clickedElement.GetDetailedElement().Parentids[0]
			}
			if a && b && ((d && c) || (!d && b)) || (e && ((d && c) || (!d && b))) {
				//mesh := el.GetNamedMesh(el.GetDisplayName())
				worldApp.RemoveFromScene(el.path)
				cr.clickedPaths = append(cr.clickedPaths[:amount], cr.clickedPaths[amount+1:]...)
				// er.pop()
				// 	for _, childID := range prevElement.clickedElement.GetChildElementIds() {
				// 		if !er.isChildElement(worldApp, prevElement.clickedElement) {
				// 			if childElement, childElementOk := worldApp.ConcreteElements[childID]; childElementOk {
				// 				childElement.ApplyState(mashupsdk.Hidden, true)
				// 				er.RemoveAll(worldApp, childID)
				// 			}
				// 		}
				// 	}
				// er.deselectElements(worldApp, prevElement.clickedElement)
				//el.g3nElement.GetDetailedElement().Genre == clickedElement.GetDetailedElement().Genre &&
			} else if b && d && el.g3nElement.GetParentElementIds()[0] != clickedElement.GetParentElementIds()[0] && e {
				worldApp.RemoveFromScene(el.path)
				cr.clickedPaths = append(cr.clickedPaths[:amount], cr.clickedPaths[amount+1:]...)
			} else {
				amount += 1
			}
		} //}
		cr.isCtrl = false
		//cr.ctrlElements = nil
	}

	fmt.Print("checking stack")
}

// Properly sets the elements before rendering new clicked elements
func (cr *CurveRenderer) InitRenderLoop(worldApp *g3nworld.WorldApp) bool {
	// TODO: noop
	if !cr.isEmpty() && worldApp.ClickedElements[len(worldApp.ClickedElements)-1].GetDetailedElement().Genre != "DataFlowStatistic" && !cr.er.isChildElement(worldApp, cr.top().g3nElement) && worldApp.ClickedElements[len(worldApp.ClickedElements)-1].GetDetailedElement().Genre != "Space" {
		cr.removeRelated(worldApp, worldApp.ClickedElements[len(worldApp.ClickedElements)-1], cr.top().g3nElement)
	}
	return true
}

// Returns an array of time splits for given element's child ids in seconds
func (cr *CurveRenderer) getTimeSplits(worldApp *g3nworld.WorldApp, element *g3nmash.G3nDetailedElement) ([]float64, bool) {
	timesplit := []float64{}
	succeeded := false
	for i := 0; i < len(element.GetChildElementIds()); i++ {
		child := worldApp.ConcreteElements[element.GetChildElementIds()[i]]
		if child.GetDetailedElement().Genre != "Solid" {
			if strings.Contains(child.GetDetailedElement().Name, "Successful") {
				succeeded = true
			}
			var decoded interface{}
			err := json.Unmarshal([]byte(child.GetDetailedElement().Data), &decoded)
			if err != nil {
				log.Println("Error decoding data in curve renderer getTimeSplits")
				break
			}
			decodedData := decoded.(map[string]interface{})
			if decodedData["TimeSplit"] != nil {
				timeNanoSeconds := decodedData["TimeSplit"].(float64)
				timeSeconds := float64(timeNanoSeconds) * math.Pow(10.0, -9.0)
				timesplit = append(timesplit, timeSeconds)
			}
			// timeNanoSeconds, err := strconv.ParseInt(child.GetDetailedElement().Data, 10, 64)
			// if err != nil {
			// 	return timesplit, succeeded
			// }
			// timeSeconds := float64(timeNanoSeconds) * math.Pow(10.0, -9.0)
			// timesplit = append(timesplit, timeSeconds)
		}
	}
	return timesplit, succeeded
}

func (cr *CurveRenderer) getMainSpirals(worldApp *g3nworld.WorldApp, currElement *g3nmash.G3nDetailedElement) {
	if len(currElement.GetChildElementIds()) > 0 {
		for _, childID := range currElement.GetChildElementIds() {
			if worldApp.ConcreteElements[childID] != nil {
				childEl := worldApp.ConcreteElements[childID]
				cr.ctrlRenderElement(worldApp, childEl)
				cr.getMainSpirals(worldApp, childEl)
			}
		}
	}
}

func (cr *CurveRenderer) iterateToDF(worldApp *g3nworld.WorldApp, g3n *g3nmash.G3nDetailedElement) {
	if g3n != nil {
		for _, childID := range g3n.GetChildElementIds() {
			element := worldApp.ConcreteElements[childID]
			if element != nil && element.GetDetailedElement().Genre == "DataFlow" {
				cr.ctrlRenderElement(worldApp, element)
			} else {
				cr.iterateToDF(worldApp, element)
			}
		}
	}

	// for _, childID := range g3n.GetChildElementIds() {
	// 	element := worldApp.ConcreteElements[childID]
	// 	if element.GetDetailedElement().Genre == "DataFlowGroup" {
	// 		for _, child := range element.GetChildElementIds() {
	// 			child := worldApp.ConcreteElements[child]
	// 			if child.GetDetailedElement().Genre == "DataFlow" {
	// 				cr.ctrlRenderElement(worldApp, child)
	// 			}
	// 		}
	// 	}
	// }
}

func (cr *CurveRenderer) ctrlRenderElement(worldApp *g3nworld.WorldApp, g3nDetailedElement *g3nmash.G3nDetailedElement) {
	clickedElement := g3nDetailedElement
	var path []math32.Vector3
	cr.isCtrl = true
	if clickedElement != nil && clickedElement.GetDetailedElement().Genre == "DataFlow" && clickedElement.GetNamedMesh(clickedElement.GetDisplayName()) != nil {
		timeSplits, successful := cr.getTimeSplits(worldApp, clickedElement)
		fmt.Println(successful)
		if len(clickedElement.GetChildElementIds()) > 0 && clickedElement.GetDetailedElement().Genre != "Solid" && clickedElement.GetDetailedElement().Genre != "DataFlowStatistic" {
			section := (-0.1 * 20.0) / float64(len(clickedElement.GetChildElementIds()))
			lastLocation := 0.0
			color := math32.NewColor("white")
			diff := 0.0
			maxTotalTime := cr.avg //float64(cr.maxTime) * math.Pow(10.0, -9.0)
			// Can't just average tests have to average overall test time --> what whole spiral would represent
			for j := 0.0; j < float64(len(timeSplits)); j = j + 1.0 {
				if len(timeSplits) > int(j+1) {
					diff = math.Abs(timeSplits[int(j+1)] - timeSplits[int(j)])
					section = ((math.Abs(timeSplits[int(j+1)]-timeSplits[int(j)]) / maxTotalTime) * -2) + lastLocation //total --> maxTotalTime
				}
				if section != 0 && section-lastLocation != 0 {
					for i := section; i < lastLocation; i = i + math.Abs((section-lastLocation)/((section-lastLocation)*100)) {
						c := binetFormula(i)
						x := real(c)
						y := imag(c)
						z := -i
						location := *math32.NewVector3(float32(-x), float32(y), float32(z))
						path = append(path, location)
					}
				}
				if j == float64(len(timeSplits)-1) {
					for i := -2.0; i < lastLocation; i = i + 0.01 {
						c := binetFormula(i)
						x := real(c)
						y := imag(c)
						z := -i
						path = append(path, *math32.NewVector3(float32(-x), float32(y), float32(z)))
					}
				}
				complex := binetFormula(lastLocation)
				path = append(path, *math32.NewVector3(float32(-real(complex)), float32(imag(complex)), -float32(lastLocation)))
				if len(path) > 1 {
					var median float64
					var upperQuartile float64
					var lowerQuartile float64
					if len(cr.quartiles) == 3 {
						median = cr.quartiles[1]
						upperQuartile = cr.quartiles[2]
						lowerQuartile = cr.quartiles[0]
						if diff < lowerQuartile {
							color.Set(0.953, 0.569, 0.125)
						} else if diff < median {
							color.Set(1, 0.682, 0.114)
						} else if diff < upperQuartile {
							color.Set(0, 0.455, 0.737)
						} else {
							color.Set(0.031, 0.227, 0.427)
						}
						if j == float64(len(timeSplits)-1) {
							if successful {
								color = math32.NewColor("black")
							} else {
								color = math32.NewColor("black")
							}
						}
					}
					lastLocation = section
					tubeGeometry := geometry.NewTube(path, .007, 32, true)
					mat := material.NewStandard(color)
					if j == float64(len(timeSplits)-1) {
						mat.SetOpacity(0.1)
					}
					tubeMesh := graphic.NewMesh(tubeGeometry, mat)
					tubeMesh.SetLoaderID(clickedElement.GetDisplayName() + "-Curve" + strconv.Itoa(int(j)))
					locn := clickedElement.GetNamedMesh(clickedElement.GetDisplayName()).Position()
					locn.X = locn.X - 0.005
					locn.Y = locn.Y + 0.0999
					locn.Z = locn.Z - 0.001 //Need to find correct z-component so centered properly
					tubeMesh.SetPositionVec(&locn)
					cr.push(tubeMesh, clickedElement)
					worldApp.UpsertToScene(tubeMesh)
				} else {
					fmt.Println(section)
				}
				path = []math32.Vector3{}
			}
		}
	} else if clickedElement != nil {
		position := math32.NewVector3(1.0, 2.0, 3.0)
		if clickedElement.GetNamedMesh(clickedElement.GetDisplayName()) != nil && clickedElement.GetDetailedElement().Genre != "Solid" {
			locn := clickedElement.GetNamedMesh(clickedElement.GetDisplayName()).Position()
			position = &locn
		}
		if len(clickedElement.GetChildElementIds()) > 0 && clickedElement.GetDetailedElement().Genre != "Solid" && clickedElement.GetDetailedElement().Genre != "DataFlowStatistic" {
			if len(clickedElement.GetChildElementIds()) > 20 {
				for i := -0.1 * float64(len(clickedElement.GetChildElementIds())-1); i < -0.1; i = i + 0.1 {
					c := binetFormula(i)
					x := real(c)
					y := imag(c)
					z := -i
					path = append(path, *math32.NewVector3(float32(-x), float32(y), float32(z)))
				}
			} else {
				for i := -0.1 * 20.0; i < -0.1; i = i + 0.1 {
					c := binetFormula(i)
					x := real(c)
					y := imag(c)
					z := -i
					path = append(path, *math32.NewVector3(float32(-x), float32(y), float32(z)))
				}
			}
			path = append(path, *math32.NewVector3(float32(0.0), float32(0.0), float32(0.0)))
			tubeGeometry := geometry.NewTube(path, .007, 32, true)
			color := math32.NewColor("darkmagenta")
			color.Set(0.435, 0.541, 0.420)
			// if clickedElement.GetDetailedElement().Genre == "Argosy" {
			// 	color.Set(0.435, 0.541, 0.420)
			// } else if clickedElement.GetDetailedElement().Genre == "DataFlowGroup" {
			// 	color.Set(0.675, 0.624, 0.773)
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

// Renders elements based on last clicked element
// Returns true if given element is the last clicked element and false otherwise
func (cr *CurveRenderer) RenderElement(worldApp *g3nworld.WorldApp, g3nDetailedElement *g3nmash.G3nDetailedElement) bool {
	clickedElement := worldApp.ClickedElements[len(worldApp.ClickedElements)-1]
	var path []math32.Vector3
	if g3nDetailedElement.GetDetailedElement().Id == 2 {
		if g3nDetailedElement.GetDetailedElement().Data != "" && cr.quartiles == nil { // Can't add this here --> Put in curve renderer and add data to curve element
			var decoded interface{}
			err := json.Unmarshal([]byte(g3nDetailedElement.GetDetailedElement().Data), &decoded)
			if err != nil {
				log.Println("Error decoding data in curve renderer RenderElement")
			} else {
				decodedData := decoded.(map[string]interface{})
				if decodedData["Quartiles"] != nil && decodedData["MaxTime"] != nil && decodedData["Average"] != nil {
					if interfaceQuartiles, ok := decodedData["Quartiles"].([]interface{}); ok {
						for _, quart := range interfaceQuartiles {
							if floatQuart, ok := quart.(float64); ok {
								cr.quartiles = append(cr.quartiles, floatQuart)
							}
						}
					}

					if decodedMaxTime, ok := decodedData["MaxTime"].(float64); ok {
						cr.maxTime = int(decodedMaxTime)
					}

					if decodedavg, ok := decodedData["Average"].(float64); ok {
						cr.avg = decodedavg
					}
				}
			}

		}
		if clickedElement.IsStateSet(mashupsdk.ControlClicked) {
			cr.getMainSpirals(worldApp, clickedElement)
			cr.iterateToDF(worldApp, clickedElement)
		}
		if !cr.isCtrl && (clickedElement != nil && clickedElement.GetDetailedElement().Genre == "DataFlow" && clickedElement.GetNamedMesh(clickedElement.GetDisplayName()) != nil) {
			timeSplits, successful := cr.getTimeSplits(worldApp, clickedElement)
			fmt.Println(successful)
			if len(clickedElement.GetChildElementIds()) > 0 && clickedElement.GetDetailedElement().Genre != "Solid" && clickedElement.GetDetailedElement().Genre != "DataFlowStatistic" {
				section := (-0.1 * 20.0) / float64(len(clickedElement.GetChildElementIds()))
				lastLocation := 0.0
				color := math32.NewColor("white")
				diff := 0.0
				maxTotalTime := cr.avg //float64(cr.maxTime) * math.Pow(10.0, -9.0)
				for j := 0.0; j < float64(len(timeSplits)); j = j + 1.0 {
					if len(timeSplits) > int(j+1) {
						diff = math.Abs(timeSplits[int(j+1)] - timeSplits[int(j)])
						section = ((math.Abs(timeSplits[int(j+1)]-timeSplits[int(j)]) / maxTotalTime) * -2) + lastLocation //total --> maxTotalTime
					}
					if section != 0 && section-lastLocation != 0 {
						for i := section; i < lastLocation; i = i + math.Abs((section-lastLocation)/((section-lastLocation)*100)) {
							c := binetFormula(i)
							x := real(c)
							y := imag(c)
							z := -i
							location := *math32.NewVector3(float32(-x), float32(y), float32(z))
							path = append(path, location)
						}
					}
					if j == float64(len(timeSplits)-1) {
						for i := -2.0; i < lastLocation; i = i + 0.01 {
							c := binetFormula(i)
							x := real(c)
							y := imag(c)
							z := -i
							path = append(path, *math32.NewVector3(float32(-x), float32(y), float32(z)))
						}
					}
					complex := binetFormula(lastLocation)
					path = append(path, *math32.NewVector3(float32(-real(complex)), float32(imag(complex)), -float32(lastLocation)))
					if len(path) > 1 {
						if len(cr.quartiles) == 3 {
							median := cr.quartiles[1]
							upperQuartile := cr.quartiles[2]
							lowerQuartile := cr.quartiles[0]
							if diff < lowerQuartile {
								color.Set(0.953, 0.569, 0.125)
							} else if diff < median {
								color.Set(1, 0.682, 0.114)
							} else if diff < upperQuartile {
								color.Set(0, 0.455, 0.737)
							} else {
								color.Set(0.031, 0.227, 0.427)
							}
							if j == float64(len(timeSplits)-1) {
								if successful {
									color = math32.NewColor("black")
								} else {
									color = math32.NewColor("black")
								}
							}
						}
						lastLocation = section
						tubeGeometry := geometry.NewTube(path, .007, 32, true)
						mat := material.NewStandard(color)
						if j == float64(len(timeSplits)-1) {
							mat.SetOpacity(0.1)
						}
						tubeMesh := graphic.NewMesh(tubeGeometry, mat)
						tubeMesh.SetLoaderID(clickedElement.GetDisplayName() + "-Curve" + strconv.Itoa(int(j)))
						locn := clickedElement.GetNamedMesh(clickedElement.GetDisplayName()).Position()
						locn.X = locn.X - 0.005
						locn.Y = locn.Y + 0.0999
						locn.Z = locn.Z - 0.001 //Need to find correct z-component so centered properly
						tubeMesh.SetPositionVec(&locn)
						cr.push(tubeMesh, clickedElement)
						worldApp.UpsertToScene(tubeMesh)
					} else {
						fmt.Println(section)
					}
					path = []math32.Vector3{}
				}

			}
		} else if clickedElement != nil {
			position := math32.NewVector3(1.0, 2.0, 3.0)

			if clickedElement.GetNamedMesh(clickedElement.GetDisplayName()) != nil && clickedElement.GetDetailedElement().Genre != "Solid" {
				locn := clickedElement.GetNamedMesh(clickedElement.GetDisplayName()).Position()
				position = &locn
			}
			if len(clickedElement.GetChildElementIds()) > 0 && clickedElement.GetDetailedElement().Genre != "Solid" && clickedElement.GetDetailedElement().Genre != "DataFlowStatistic" {
				if len(clickedElement.GetChildElementIds()) > 20 {
					for i := -0.1 * float64(len(clickedElement.GetChildElementIds())-1); i < -0.1; i = i + 0.1 {
						c := binetFormula(i)
						x := real(c)
						y := imag(c)
						z := -i
						path = append(path, *math32.NewVector3(float32(-x), float32(y), float32(z)))
					}
				} else {
					for i := -0.1 * 20.0; i < -0.1; i = i + 0.1 {
						c := binetFormula(i)
						x := real(c)
						y := imag(c)
						z := -i
						path = append(path, *math32.NewVector3(float32(-x), float32(y), float32(z)))
					}
				}
				path = append(path, *math32.NewVector3(float32(0.0), float32(0.0), float32(0.0)))
				tubeGeometry := geometry.NewTube(path, .007, 32, true)
				color := math32.NewColor("darkmagenta")
				color.Set(0.435, 0.541, 0.420)
				// if clickedElement.GetDetailedElement().Genre == "Argosy" {
				// 	color.Set(0.435, 0.541, 0.420)
				// } else if clickedElement.GetDetailedElement().Genre == "DataFlowGroup" {
				// 	color.Set(0.675, 0.624, 0.773)
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
	return false
}

func (cr *CurveRenderer) Collaborate(worldApp *g3nworld.WorldApp, collaboratingRenderer g3nrender.IG3nRenderer) {
	cr.CollaboratingRenderer.Collaborate(worldApp, cr)
}

// package ttdirender

// import (
// 	"encoding/json"
// 	"fmt"
// 	"log"
// 	"math"
// 	"strconv"
// 	"strings"

// 	"github.com/g3n/engine/core"
// 	"github.com/g3n/engine/graphic"
// 	"github.com/g3n/engine/material"
// 	"github.com/g3n/engine/math32"
// 	"github.com/mrjrieke/nute/g3nd/g3nmash"
// 	"github.com/mrjrieke/nute/g3nd/g3nworld"

// 	"github.com/mrjrieke/nute/g3nd/worldg3n/g3nrender"

// 	"github.com/g3n/engine/geometry"
// )

// var sqrtfive float64 = float64(math.Sqrt(float64(5.0)))
// var goldenRatio float64 = (float64(1.0) + sqrtfive) / (float64(2.0))

// type CurveRenderer struct {
// 	g3nrender.GenericRenderer
// 	er                    *ElementRenderer
// 	CollaboratingRenderer g3nrender.IG3nRenderer
// 	totalElements         int
// 	clickedPaths          []*CurveMesh
// 	maxTime               int
// 	quartiles             []float64
// }

// type CurveMesh struct {
// 	path       *graphic.Mesh
// 	g3nElement *g3nmash.G3nDetailedElement
// }

// // Returns true if length of cr.clickedPaths stack is 0 and false otherwise
// func (cr *CurveRenderer) isEmpty() bool {
// 	return len(cr.clickedPaths) == 0
// }

// // Returns size of cr.clickedPaths stack
// func (cr *CurveRenderer) length() int {
// 	return len(cr.clickedPaths)
// }

// // Adds given element and location to the cr.clickedPaths stack
// func (cr *CurveRenderer) push(spiralPath *graphic.Mesh, g3nDetailedElement *g3nmash.G3nDetailedElement) {
// 	element := CurveMesh{
// 		path:       spiralPath,
// 		g3nElement: g3nDetailedElement,
// 	}
// 	cr.clickedPaths = append(cr.clickedPaths, &element)
// }

// // Removes and returns top element in cr.clickedPaths stack
// func (cr *CurveRenderer) pop() *CurveMesh {
// 	size := len(cr.clickedPaths)
// 	element := cr.clickedPaths[size-1]
// 	cr.clickedPaths = cr.clickedPaths[:size-1]
// 	return element
// }

// // Returns top element in cr.clickedPaths stack
// func (cr *CurveRenderer) top() *CurveMesh {
// 	return cr.clickedPaths[cr.length()-1]
// }

// // Calculates real and imaginary parts of Binet's Formula with given input and returns the value
// func binetFormula(n float64) complex128 {
// 	real := (float64(math.Pow(goldenRatio, n)) - float64(math.Cos(float64(math.Pi)*n)*math.Pow(goldenRatio, -n))) / sqrtfive
// 	imag := (float64(-1.0) * float64(math.Sin(math.Pi*n)) * float64(math.Pow(goldenRatio, -n))) / sqrtfive
// 	return complex(real, imag)
// }

// // Returns and attaches a mesh to provided g3n element at given vector position
// func (cr *CurveRenderer) NewSolidAtPosition(g3n *g3nmash.G3nDetailedElement, vpos *math32.Vector3) core.INode {
// 	if g3n.GetDetailedElement().Genre == "DataFlowStatistic" {
// 		return nil
// 	}
// 	var path []math32.Vector3
// 	var i float64
// 	if cr.totalElements == 0 {
// 		cr.totalElements = 20
// 	}
// 	for i = -0.1 * float64(cr.totalElements-1); i < -0.1; i = i + 0.1 {
// 		c := binetFormula(i)
// 		x := real(c)
// 		y := imag(c)
// 		z := -i
// 		path = append(path, *math32.NewVector3(float32(-x), float32(y), float32(z)))
// 	}
// 	path = append(path, *math32.NewVector3(float32(0.0), float32(0.0), float32(0.0)))
// 	fmt.Println(binetFormula(-20.0))
// 	fmt.Println(binetFormula(0.0))
// 	fmt.Println(i)
// 	fmt.Println(binetFormula(i))
// 	tubeGeometry := geometry.NewTube(path, .007, 32, true)
// 	color := math32.NewColor("darkmagenta")
// 	mat := material.NewStandard(color.Set(float32(148)/255.0, float32(120)/255.0, float32(42)/255.0))
// 	mat.SetOpacity(0.25)
// 	tubeMesh := graphic.NewMesh(tubeGeometry, mat)
// 	fmt.Printf("LoaderID: %s\n", g3n.GetDisplayName())
// 	tubeMesh.SetLoaderID(g3n.GetDisplayName())
// 	tubeMesh.SetPositionVec(vpos)
// 	return tubeMesh
// }

// func (sp *CurveRenderer) NewInternalMeshAtPosition(g3n *g3nmash.G3nDetailedElement, vpos *math32.Vector3) core.INode {
// 	return nil
// }

// // Returns the element and location of the given element
// func (cr *CurveRenderer) NextCoordinate(g3n *g3nmash.G3nDetailedElement, totalElements int) (*g3nmash.G3nDetailedElement, *math32.Vector3) {
// 	return g3n, math32.NewVector3(float32(0.0), float32(0.0), float32(0.0))
// }

// // Calls LayoutBase to render elements in a particular order and location
// func (cr *CurveRenderer) Layout(worldApp *g3nworld.WorldApp,
// 	g3nRenderableElements []*g3nmash.G3nDetailedElement) {
// 	cr.GenericRenderer.LayoutBase(worldApp, cr, g3nRenderableElements)
// }

// // Returns the CollaboratingRenderer of the CurveRenderer
// // If no collaborating renderer to the CurveRenderer, returns nil
// func (cr *CurveRenderer) GetRenderer(rendererName string) g3nrender.IG3nRenderer {
// 	if cr.CollaboratingRenderer != nil {
// 		return cr.CollaboratingRenderer
// 	}
// 	return nil
// }

// // Removes elements if they share the same parent id
// func (cr *CurveRenderer) removeRelated(worldApp *g3nworld.WorldApp, clickedElement *g3nmash.G3nDetailedElement, element *g3nmash.G3nDetailedElement) {
// 	if !cr.isEmpty() && len(element.GetParentElementIds()) != 0 && len(clickedElement.GetParentElementIds()) != 0 && element.GetParentElementIds()[0] == clickedElement.GetParentElementIds()[0] {
// 		toRemove := cr.pop()
// 		worldApp.RemoveFromScene(toRemove.path)
// 		if !cr.isEmpty() && len(cr.top().g3nElement.GetParentElementIds()) != 0 && len(clickedElement.GetParentElementIds()) != 0 && cr.top().g3nElement.GetParentElementIds()[0] == clickedElement.GetParentElementIds()[0] {
// 			cr.removeRelated(worldApp, clickedElement, cr.top().g3nElement)
// 		}
// 	} else if !cr.isEmpty() {
// 		toRemove := cr.pop()
// 		worldApp.RemoveFromScene(toRemove.path)
// 		if !cr.isEmpty() && !(len(element.GetParentElementIds()) != 0 && len(clickedElement.GetParentElementIds()) != 0 && element.GetParentElementIds()[0] == clickedElement.GetParentElementIds()[0]) {
// 			cr.removeRelated(worldApp, clickedElement, cr.top().g3nElement)
// 		}
// 	}
// }

// // Properly sets the elements before rendering new clicked elements
// func (cr *CurveRenderer) InitRenderLoop(worldApp *g3nworld.WorldApp) bool {
// 	// TODO: noop
// 	if !cr.isEmpty() && worldApp.ClickedElements[len(worldApp.ClickedElements)-1].GetDetailedElement().Genre != "DataFlowStatistic" && !cr.er.isChildElement(worldApp, cr.top().g3nElement) && worldApp.ClickedElements[len(worldApp.ClickedElements)-1].GetDetailedElement().Genre != "Space" {
// 		cr.removeRelated(worldApp, worldApp.ClickedElements[len(worldApp.ClickedElements)-1], cr.top().g3nElement)
// 	}
// 	return true
// }

// // Returns an array of time splits for given element's child ids in seconds
// func (cr *CurveRenderer) getTimeSplits(worldApp *g3nworld.WorldApp, element *g3nmash.G3nDetailedElement) ([]float64, bool) {
// 	timesplit := []float64{}
// 	succeeded := false
// 	for i := 0; i < len(element.GetChildElementIds()); i++ {
// 		child := worldApp.ConcreteElements[element.GetChildElementIds()[i]]
// 		if child.GetDetailedElement().Genre != "Solid" {
// 			if strings.Contains(child.GetDetailedElement().Name, "Successful") {
// 				succeeded = true
// 			}
// 			var decoded interface{}
// 			err := json.Unmarshal([]byte(child.GetDetailedElement().Data), &decoded)
// 			if err != nil {
// 				log.Println("Error decoding data in curve renderer getTimeSplits")
// 				break
// 			}
// 			decodedData := decoded.(map[string]interface{})
// 			if decodedData["TimeSplit"] != nil {
// 				timeNanoSeconds := decodedData["TimeSplit"].(float64)
// 				timeSeconds := float64(timeNanoSeconds) * math.Pow(10.0, -9.0)
// 				timesplit = append(timesplit, timeSeconds)
// 			}
// 			// timeNanoSeconds, err := strconv.ParseInt(child.GetDetailedElement().Data, 10, 64)
// 			// if err != nil {
// 			// 	return timesplit, succeeded
// 			// }
// 			// timeSeconds := float64(timeNanoSeconds) * math.Pow(10.0, -9.0)
// 			// timesplit = append(timesplit, timeSeconds)
// 		}
// 	}
// 	return timesplit, succeeded
// }

// // Renders elements based on last clicked element
// // Returns true if given element is the last clicked element and false otherwise
// func (cr *CurveRenderer) RenderElement(worldApp *g3nworld.WorldApp, g3nDetailedElement *g3nmash.G3nDetailedElement) bool {
// 	clickedElement := worldApp.ClickedElements[len(worldApp.ClickedElements)-1]
// 	var path []math32.Vector3
// 	if g3nDetailedElement.GetDetailedElement().Id == 2 {
// 		if clickedElement != nil && clickedElement.GetDetailedElement().Genre == "DataFlow" && clickedElement.GetNamedMesh(clickedElement.GetDisplayName()) != nil {
// 			timeSplits, successful := cr.getTimeSplits(worldApp, clickedElement)
// 			fmt.Println(successful)
// 			if len(clickedElement.GetChildElementIds()) > 0 && clickedElement.GetDetailedElement().Genre != "Solid" && clickedElement.GetDetailedElement().Genre != "DataFlowStatistic" {
// 				section := (-0.1 * 20.0) / float64(len(clickedElement.GetChildElementIds()))
// 				lastLocation := 0.0
// 				color := math32.NewColor("white")
// 				diff := 0.0
// 				maxTotalTime := float64(cr.maxTime) * math.Pow(10.0, -9.0)
// 				for j := 0.0; j < float64(len(timeSplits)); j = j + 1.0 {
// 					if len(timeSplits) > int(j+1) {
// 						diff = timeSplits[int(j+1)] - timeSplits[int(j)]
// 						section = (((timeSplits[int(j+1)] - timeSplits[int(j)]) / maxTotalTime) * -2) + lastLocation //total --> maxTotalTime
// 					}
// 					if section != 0 && section-lastLocation != 0 {
// 						for i := section; i < lastLocation; i = i + math.Abs((section-lastLocation)/((section-lastLocation)*100)) {
// 							c := binetFormula(i)
// 							x := real(c)
// 							y := imag(c)
// 							z := -i
// 							location := *math32.NewVector3(float32(-x), float32(y), float32(z))
// 							path = append(path, location)
// 						}
// 					}
// 					if j == float64(len(timeSplits)-1) {
// 						for i := -2.0; i < lastLocation; i = i + 0.01 {
// 							c := binetFormula(i)
// 							x := real(c)
// 							y := imag(c)
// 							z := -i
// 							path = append(path, *math32.NewVector3(float32(-x), float32(y), float32(z)))
// 						}
// 					}
// 					complex := binetFormula(lastLocation)
// 					path = append(path, *math32.NewVector3(float32(-real(complex)), float32(imag(complex)), -float32(lastLocation)))
// 					if len(path) > 1 {
// 						var decoded interface{}
// 						err := json.Unmarshal([]byte(clickedElement.GetDetailedElement().Data), &decoded)
// 						if err != nil {
// 							log.Println("Error decoding data in RenderElement for curve")
// 							break
// 						}
// 						decodedData := decoded.(map[string]interface{})
// 						var quartiles []float64
// 						if decodedData["Quartiles"] != nil {
// 							if quartileInterfaces, quartileInterfacesOk := decodedData["Quartiles"].([]interface{}); quartileInterfacesOk {
// 								for _, quartileInterface := range quartileInterfaces {
// 									quartiles = append(quartiles, quartileInterface.(float64))
// 								}
// 							}
// 						}
// 						//stringQuartiles := strings.Split(clickedElement.GetDetailedElement().Data, "-")
// 						var median float64
// 						var upperQuartile float64
// 						var lowerQuartile float64
// 						if len(quartiles) == 3 {
// 							median = quartiles[1]        //strconv.ParseFloat(stringQuartiles[1], 64)
// 							upperQuartile = quartiles[2] //strconv.ParseFloat(stringQuartiles[2], 64)
// 							lowerQuartile = quartiles[0] //strconv.ParseFloat(stringQuartiles[0], 64)
// 							if diff < lowerQuartile {
// 								color.Set(0.953, 0.569, 0.125)
// 							} else if diff < median {
// 								color.Set(1, 0.682, 0.114)
// 							} else if diff < upperQuartile {
// 								color.Set(0, 0.455, 0.737)
// 							} else {
// 								color.Set(0.031, 0.227, 0.427)
// 							}
// 							if j == float64(len(timeSplits)-1) {
// 								//color.Set()
// 								if successful {
// 									color = math32.NewColor("black")
// 								} else {
// 									color = math32.NewColor("black")
// 								}
// 							}
// 						}
// 						lastLocation = section
// 						tubeGeometry := geometry.NewTube(path, .007, 32, true)
// 						mat := material.NewStandard(color)
// 						if j == float64(len(timeSplits)-1) {
// 							mat.SetOpacity(0.1)
// 						}
// 						tubeMesh := graphic.NewMesh(tubeGeometry, mat)
// 						tubeMesh.SetLoaderID(clickedElement.GetDisplayName() + "-Curve" + strconv.Itoa(int(j)))
// 						locn := clickedElement.GetNamedMesh(clickedElement.GetDisplayName()).Position()
// 						locn.X = locn.X - 0.005
// 						locn.Y = locn.Y + 0.0999
// 						locn.Z = locn.Z - 0.001 //Need to find correct z-component so centered properly
// 						tubeMesh.SetPositionVec(&locn)
// 						cr.push(tubeMesh, clickedElement)
// 						worldApp.UpsertToScene(tubeMesh)
// 					} else {
// 						fmt.Println(section)
// 					}
// 					path = []math32.Vector3{}
// 				}

// 			}
// 		} else {
// 			position := math32.NewVector3(1.0, 2.0, 3.0)

// 			if clickedElement.GetNamedMesh(clickedElement.GetDisplayName()) != nil && clickedElement.GetDetailedElement().Genre != "Solid" {
// 				locn := clickedElement.GetNamedMesh(clickedElement.GetDisplayName()).Position()
// 				position = &locn
// 			}
// 			if len(clickedElement.GetChildElementIds()) > 0 && clickedElement.GetDetailedElement().Genre != "Solid" && clickedElement.GetDetailedElement().Genre != "DataFlowStatistic" {
// 				if len(clickedElement.GetChildElementIds()) > 20 {
// 					for i := -0.1 * float64(len(clickedElement.GetChildElementIds())-1); i < -0.1; i = i + 0.1 {
// 						c := binetFormula(i)
// 						x := real(c)
// 						y := imag(c)
// 						z := -i
// 						path = append(path, *math32.NewVector3(float32(-x), float32(y), float32(z)))
// 					}
// 				} else {
// 					for i := -0.1 * 20.0; i < -0.1; i = i + 0.1 {
// 						c := binetFormula(i)
// 						x := real(c)
// 						y := imag(c)
// 						z := -i
// 						path = append(path, *math32.NewVector3(float32(-x), float32(y), float32(z)))
// 					}
// 				}
// 				path = append(path, *math32.NewVector3(float32(0.0), float32(0.0), float32(0.0)))
// 				tubeGeometry := geometry.NewTube(path, .007, 32, true)
// 				color := math32.NewColor("darkmagenta")
// 				if clickedElement.GetDetailedElement().Genre == "Argosy" {
// 					color.Set(0.435, 0.541, 0.420)
// 				} else if clickedElement.GetDetailedElement().Genre == "DataFlowGroup" {
// 					color.Set(0.675, 0.624, 0.773)
// 				}
// 				mat := material.NewStandard(color)
// 				mat.SetOpacity(0.1)
// 				tubeMesh := graphic.NewMesh(tubeGeometry, mat)
// 				tubeMesh.SetLoaderID(clickedElement.GetDisplayName() + "-Curve")
// 				tubeMesh.SetPositionVec(position)
// 				cr.push(tubeMesh, clickedElement)
// 				worldApp.UpsertToScene(tubeMesh)
// 			}
// 		}

// 	}
// 	return true
// }

// func (cr *CurveRenderer) Collaborate(worldApp *g3nworld.WorldApp, collaboratingRenderer g3nrender.IG3nRenderer) {
// 	cr.CollaboratingRenderer.Collaborate(worldApp, cr)
// }
