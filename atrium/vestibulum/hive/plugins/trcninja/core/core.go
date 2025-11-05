package core

import (
	"fmt"
	"log"
	"os"

	tccore "github.com/trimble-oss/tierceron-core/v2/core"
)

// SociiKeyField is the key field name used for enterprise/socii identification
var SociiKeyField = "sociiId"

// SetSociiKey sets the key field name to be used for enterprise/socii identification
func SetSociiKey(keyName string) {
	SociiKeyField = keyName
}

var configContext *tccore.ConfigContext

func SetConfigContext(cc *tccore.ConfigContext) {
	configContext = cc
}

func GetConfigContext(pluginName string) *tccore.ConfigContext {
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
		configContext = &tccore.ConfigContext{
			Log: logger,
		}
	}
}
