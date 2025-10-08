//go:build darwin || linux
// +build darwin linux

package main

// World is a basic gomobile app.
import (
	"embed"
	"flag"

	"github.com/trimble-oss/tierceron/atrium/speculatio/fenestra/fenestrabase"
)

var worldCompleteChan chan bool

//go:embed tls/mashup.crt
var mashupCert embed.FS

//go:embed tls/mashup.key
var mashupKey embed.FS

//go:embed logo.png
var logoIcon embed.FS

func main() {
	callerCreds := flag.String("CREDS", "", "Credentials of caller")
	insecure := flag.Bool("tls-skip-validation", false, "Skip server validation")
	headless := flag.Bool("headless", false, "Run headless")
	serverheadless := flag.Bool("serverheadless", false, "Run server completely headless")
	envPtr := flag.String("env", "QA", "Environment to configure")
	flag.Parse()

	logoIconBytes, _ := logoIcon.ReadFile("logo.png")
	mashupCertBytes, _ := mashupCert.ReadFile("tls/mashup.crt")
	mashupKeyBytes, _ := mashupKey.ReadFile("tls/mashup.key")

	fenestrabase.CommonMain(logoIconBytes,
		mashupCertBytes,
		mashupKeyBytes,
		callerCreds,
		insecure,
		headless,
		serverheadless,
		envPtr)

	<-worldCompleteChan
}
