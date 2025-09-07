package main

// Thin compatibility wrapper preserving legacy exported functions while
// delegating all logic to the refactored ttcore package.

import (
	"flag"
	"fmt"
	"log"
	"os"

	tccore "github.com/trimble-oss/tierceron-core/v2/core"
	"github.com/trimble-oss/tierceron/atrium/vestibulum/hive/plugins/trcshtalk/ttcore"
	"github.com/trimble-oss/tierceron/atrium/vestibulum/hive/plugins/trcshtalk/ttcore/common"
	"gopkg.in/yaml.v2"
)

// Re-export constants via local aliases for backwards compatibility.
const (
	MASHUP_CERT = common.MASHUP_CERT
	COMMON_PATH = common.COMMON_PATH
)

// Exported wrappers (public API surface kept stable)
func GetConfigContext(pluginName string) *tccore.ConfigContext {
	return ttcore.GetConfigContext(pluginName)
}
func GetConfigPaths(pluginName string) []string { return ttcore.GetConfigPaths(pluginName) }
func Init(pluginName string, properties *map[string]interface{}) {
	ttcore.Init(pluginName, properties)
}
func InitCertBytes(cert []byte) { ttcore.InitCertBytes(cert) }

// main remains minimal: load config, initialize, start context, block.
func main() {
	logFilePtr := flag.String("log", "./trcshtalk.log", "Output path for log file")
	flag.Parse()

	config := make(map[string]any)

	f, err := os.OpenFile(*logFilePtr, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0644)
	if err != nil {
		fmt.Printf("Error opening log file: %v\n", err)
		os.Exit(-1)
	}
	logger := log.New(f, "[trcshtalk]", log.LstdFlags)
	config["log"] = logger

	if data, err := os.ReadFile("config.yml"); err == nil {
		var configCommon map[string]any
		if yerr := yaml.Unmarshal(data, &configCommon); yerr == nil {
			config[COMMON_PATH] = &configCommon
		} else {
			logger.Printf("Error unmarshaling YAML: %v\n", yerr)
		}
	}
	if crt, err := os.ReadFile("./local_config/hive.crt"); err == nil {
		config[tccore.TRCSHHIVEK_CERT] = crt
	}
	if key, err := os.ReadFile("./local_config/hivekey.key"); err == nil {
		config[tccore.TRCSHHIVEK_KEY] = key
	}

	Init("trcshtalk", &config)
	if ctx := GetConfigContext("trcshtalk"); ctx != nil {
		ctx.Start("trcshtalk")
	}

	select {} // block forever (plugin lifecycle managed elsewhere)
}
