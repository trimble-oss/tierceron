//go:build darwin || linux
// +build darwin linux

package fenestrabase

// World is a basic gomobile app.
import (
	"log"
	"os"

	"github.com/trimble-oss/tierceron/atrium/speculatio/fenestra/data"
	"github.com/trimble-oss/tierceron/atrium/speculatio/fenestra/trcRenderers"

	"fyne.io/fyne/v2"
	"github.com/trimble-oss/tierceron-nute-core/mashupsdk"
	"github.com/trimble-oss/tierceron-nute/custos/custosworld"
)

func OutsideClone(custosWorldApp *custosworld.CustosWorldApp, childId int64, concreteElement *mashupsdk.MashupDetailedElement) {
	custosWorldApp.FyneWidgetElements["Outside"].MashupDetailedElement.Copy(concreteElement)
}

func CommonMain(logoIconBytes []byte,
	mashupCertBytes []byte,
	mashupKeyBytes []byte,
	callerCreds *string,
	insecure *bool,
	headless *bool,
	serverheadless *bool,
	envPtr *string,
) {
	fenestraLog, err := os.OpenFile("fenestra.log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
	if err != nil {
		log.Fatalf(err.Error(), err)
	}
	logger := log.New(fenestraLog, "[fenestra]", log.LstdFlags)
	log.SetOutput(fenestraLog)

	mashupsdk.InitCertKeyPairBytes(mashupCertBytes, mashupKeyBytes)
	var DetailedElements []*mashupsdk.MashupDetailedElement

	if *headless {
		DetailedElements = data.GetHeadlessData(insecure, logger)
	} else {
		DetailedElements = data.GetData(insecure, logger, envPtr)
	}
	if len(DetailedElements) > 0 {
		logger.Printf("Successfully loaded %d elements.\n", len(DetailedElements))
	} else {
		logger.Printf("Failure to load any enterprises.\n")
	}

	tenantDataRenderer := &trcRenderers.TenantDataRenderer{}
	custosWorld := custosworld.NewCustosWorldApp(*serverheadless, false, DetailedElements, tenantDataRenderer)
	tenantDataRenderer.CustosWorldApp = custosWorld
	custosWorld.CustomTabItemRenderer["TenantDataRenderer"] = tenantDataRenderer
	custosWorld.CustomTabItemRenderer["SearchRenderer"] = &trcRenderers.SearchRenderer{CustosWorldApp: custosWorld}

	custosWorld.Title = "Tierceron Topology Discovery Interface"
	custosWorld.MainWindowSize = fyne.NewSize(800, 100)
	custosWorld.Icon = fyne.NewStaticResource("Logo", logoIconBytes)

	if !custosWorld.Headless {
		custosWorld.InitServer(*callerCreds, *insecure, 500*1024*1024)
	}

	// Initialize the main window.
	custosWorld.InitMainWindow()

}
