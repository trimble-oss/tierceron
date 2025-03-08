//go:build darwin || linux
// +build darwin linux

package main

// World is a basic gomobile app.
import (
	"embed"
	"flag"

	"github.com/trimble-oss/tierceron/atrium/speculatio/spiralis/spiralisbase"
)

var worldCompleteChan chan bool

//go:embed tls/mashup.crt
var mashupCert embed.FS

//go:embed tls/mashup.key
var mashupKey embed.FS

func main() {
	callerCreds := flag.String("CREDS", "", "Credentials of caller")
	insecure := flag.Bool("tls-skip-validation", false, "Skip server validation")
	custos := flag.Bool("custos", false, "Run in guardian mode.")
	headless := flag.Bool("headless", false, "Run headless")
	flag.Parse()

	spiralisbase.CommonMain(mashupCert,
		mashupKey,
		callerCreds,
		insecure,
		custos,
		headless)

	<-worldCompleteChan
}
