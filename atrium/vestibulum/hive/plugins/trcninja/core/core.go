package core

import (
	"fmt"
	"log"
	"os"

	"github.com/trimble-oss/tierceron-core/v2/core"
)

// SociiKeyField is the key field name used for enterprise/socii identification
var SociiKeyField = "sociiId"

// SetSociiKey sets the key field name to be used for enterprise/socii identification
func SetSociiKey(keyName string) {
	SociiKeyField = keyName
}

var configContext *core.ConfigContext

func SetConfigContext(cc *core.ConfigContext) {
	configContext = cc
}

func GetConfigContext(pluginName string) *core.ConfigContext {
	return configContext
}

func LogError(errMsg string) {
	if configContext != nil && configContext.Log != nil {
		configContext.Log.Println(errMsg)
		return
	}
	fmt.Fprintln(os.Stderr, errMsg)
}

func SetLogger(logger *log.Logger) {
	if configContext != nil {
		configContext.Log = logger
	} else {
		configContext = &core.ConfigContext{
			Log: logger,
		}
	}
}
