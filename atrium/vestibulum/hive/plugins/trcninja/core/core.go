package core

import (
	"fmt"
	"log"

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

func LogError(err_msg string) {
	if configContext != nil && configContext.Log != nil {
		configContext.Log.Println(err_msg)
		return
	}
	fmt.Println(err_msg)
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
