package ttdirender

import (
	"strings"

	"github.com/g3n/engine/core"
	"github.com/g3n/engine/gui"
	"github.com/g3n/engine/math32"

	"github.com/trimble-oss/tierceron-nute/g3nd/g3nmash"
	"github.com/trimble-oss/tierceron-nute/g3nd/g3nworld"
	"github.com/trimble-oss/tierceron-nute/g3nd/worldg3n/g3nrender"
)

type GuiRenderer struct {
	g3nrender.GenericRenderer
	GuiNodeMap map[string]interface{}
}

func (br *GuiRenderer) NewSolidAtPosition(g3n *g3nmash.G3nDetailedElement, vpos *math32.Vector3) core.INode {
	return nil
}

func (br *GuiRenderer) NewInternalMeshAtPosition(g3n *g3nmash.G3nDetailedElement, vpos *math32.Vector3) core.INode {
	return nil
}

func (br *GuiRenderer) NewRelatedMeshAtPosition(g3n *g3nmash.G3nDetailedElement, vpos *math32.Vector3, vprevpos *math32.Vector3) core.INode {
	return nil
}

func (br *GuiRenderer) NextCoordinate(g3n *g3nmash.G3nDetailedElement, totalElements int) (*g3nmash.G3nDetailedElement, *math32.Vector3) {
	return nil, nil
}

func (br *GuiRenderer) Layout(worldApp *g3nworld.WorldApp,
	g3nRenderableElements []*g3nmash.G3nDetailedElement) {
	//return
}

func (br *GuiRenderer) InitRenderLoop(worldApp *g3nworld.WorldApp) bool {
	// TODO: noop
	return true
}

type ColorDialog struct {
	gui.Panel
	msg *gui.ImageLabel
}

// //go:embed trimblelogo.png
// var trimbleLogo embed.FS

// //go:embed legend.png
// var legend embed.FS

// Renders GUI elements for Legend and NodeLabel
// Legend and Logo are commented out because still need to finalize UX color choices for Visualizer
func (br *GuiRenderer) RenderElement(worldApp *g3nworld.WorldApp, g3n *g3nmash.G3nDetailedElement) bool {

	//clickedElement := worldApp.ClickedElements[len(worldApp.ClickedElements)-1]
	if g3n.GetDetailedElement().Name == "Legend" {

		// imageNode := br.GuiNodeMap["Logo"]
		// if imageNode != nil {
		// 	imageNode.(*gui.Image).SetPosition(float32(10), float32(10))
		// 	worldApp.UpsertToScene(imageNode.(*gui.Image))
		// } else {
		// 	trimbleImageFile, tErr := trimbleLogo.Open("trimblelogo.png")
		// 	// Decodes image

		// 	if tErr == nil {

		// 		// Converts image to RGBA format
		// 		trimbleImage, _, _ := image.Decode(trimbleImageFile)
		// 		rgba := image.NewRGBA(trimbleImage.Bounds())
		// 		draw.Draw(rgba, rgba.Bounds(), trimbleImage, image.Point{0, 0}, draw.Src)
		// 		//trimbleImage.ImageFillOriginal = canvas.ImageFillOriginal
		// 		imageNode := gui.NewImageFromRGBA(rgba)
		// 		imageNode.SetLoaderID("Logo")
		// 		br.GuiNodeMap["Logo"] = imageNode

		// 		//winWidth, _ := worldApp.MainWin.GetSize()

		// 		imageNode.SetPosition(float32(10), float32(10))
		// 		worldApp.UpsertToScene(imageNode)
		// 	}
		// }

		// legendImage := br.GuiNodeMap["Legend"]
		// if legendImage != nil {
		// 	winWidth, _ := worldApp.MainWin.GetSize()
		// 	legendImage.(*gui.Image).SetPosition(float32(winWidth-250), float32(800))
		// 	worldApp.UpsertToScene(legendImage.(*gui.Image))
		// } else {
		// 	legendImageFile, legendErr := legend.Open("legend.png")
		// 	// Decodes image
		// 	if legendErr == nil {

		// 		// Converts image to RGBA format
		// 		legendImage, _, _ := image.Decode(legendImageFile)
		// 		rgba := image.NewRGBA(legendImage.Bounds())
		// 		draw.Draw(rgba, rgba.Bounds(), legendImage, image.Point{0, 0}, draw.Src)
		// 		//trimbleImage.ImageFillOriginal = canvas.ImageFillOriginal
		// 		imageNode := gui.NewImageFromRGBA(rgba)
		// 		imageNode.SetLoaderID("Legend")
		// 		br.GuiNodeMap["Legend"] = imageNode
		// 		winWidth, _ := worldApp.MainWin.GetSize()
		// 		imageNode.SetPosition(float32(winWidth-250), float32(800))
		// 		worldApp.UpsertToScene(imageNode)
		// 	}
		// }

		// Create table with colors.
		// // Get Name of this.
		/* colorDialog := new(ColorDialog)

		colorVBoxLayout := gui.NewVBoxLayout()
		colorVBoxLayout.SetSpacing(4)
		 colorDialog.SetLayout(colorVBoxLayout)

		 colorDialog.msg = gui.NewImageLabel("")
		 colorDialog.msg.SetColor(math32.NewColor("red"))
		 winWidth, _ := worldApp.MainWin.GetSize()
		colorDialog.msg.SetLayoutParams(&gui.VBoxLayoutParams{Expand: 2, AlignH: gui.AlignWidth})
		colorDialog.Add(colorDialog.msg)
		colorDialog.SetPosition(float32(winWidth-1000), float32(100))
		worldApp.UpsertToScene(colorDialog) */

		//Creates error message label
		//nodeLabel.SetPosition(float32(winWidth-100), float32(10))
		//worldApp.UpsertToScene(nodeLabel)
	} else if g3n.GetDetailedElement().Name == "NodeLabel" {

		// Create a label.
		for _, clickedElement := range worldApp.ClickedElements {
			if clickedElement.GetDetailedElement().Genre != "Space" && clickedElement.GetDetailedElement().Genre != "Collection" {
				nodeLabel := br.GuiNodeMap["NodeLabel"]
				// Get Name of this.
				if nodeLabel != nil {
					scrubbedNames := strings.Split(clickedElement.GetDetailedElement().Name, "-")
					nodeLabel.(*gui.Label).SetText(scrubbedNames[0])
				} else {
					scrubbedNames := strings.Split(clickedElement.GetDetailedElement().Name, "-")
					nodeLabel = gui.NewLabel(scrubbedNames[0])
					nodeLabel.(*gui.Label).SetLoaderID("NodeLabel")
					br.GuiNodeMap["NodeLabel"] = nodeLabel
				}

				winWidth, _ := worldApp.MainWin.GetSize()
				nodeLabel.(*gui.Label).SetPosition(float32(winWidth-250), float32(30))
				worldApp.UpsertToScene(nodeLabel.(*gui.Label))
			}
		}

	}

	return false
}
