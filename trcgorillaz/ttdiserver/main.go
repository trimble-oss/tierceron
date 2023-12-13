//go:build darwin || linux
// +build darwin linux

package main

// World is a basic gomobile app.
import (
	"embed"
	"flag"
	"log"
	"os"

	"github.com/trimble-oss/tierceron/trcgorillaz/ttdiserver/data"
	"github.com/trimble-oss/tierceron/trcgorillaz/ttdiserver/trcRenderers"

	"fyne.io/fyne/v2"
	"github.com/trimble-oss/tierceron-nute/custos/custosworld"
	"github.com/trimble-oss/tierceron-nute/mashupsdk"
)

var worldCompleteChan chan bool

//go:embed tls/mashup.crt
var mashupCert embed.FS

//go:embed tls/mashup.key
var mashupKey embed.FS

func OutsideClone(custosWorldApp *custosworld.CustosWorldApp, childId int64, concreteElement *mashupsdk.MashupDetailedElement) {
	custosWorldApp.FyneWidgetElements["Outside"].MashupDetailedElement.Copy(concreteElement)
}

//go:embed logo.png
var logoIcon embed.FS

func main() {
	callerCreds := flag.String("CREDS", "", "Credentials of caller")
	insecure := flag.Bool("tls-skip-validation", false, "Skip server validation")
	headless := flag.Bool("headless", false, "Run headless")
	serverheadless := flag.Bool("serverheadless", false, "Run server completely headless")
	envPtr := flag.String("env", "QA", "Environment to configure")
	flag.Parse()

	helloLog, err := os.OpenFile("ttdiserver.log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0666)
	if err != nil {
		log.Fatalf(err.Error(), err)
	}
	logger := log.New(helloLog, "[ttdiserver]", log.LstdFlags)
	log.SetOutput(helloLog)

	mashupsdk.InitCertKeyPair(mashupCert, mashupKey)
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
	logoIconBytes, _ := logoIcon.ReadFile("logo.png")
	custosWorld.Icon = fyne.NewStaticResource("Logo", logoIconBytes)

	if !custosWorld.Headless {
		custosWorld.InitServer(*callerCreds, *insecure, 500*1024*1024)
	}

	// Initialize the main window.
	custosWorld.InitMainWindow()

	<-worldCompleteChan
}
