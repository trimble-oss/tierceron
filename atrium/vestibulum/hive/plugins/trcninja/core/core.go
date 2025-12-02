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

func GetConfigContext() *core.ConfigContext {
	return configContext
}

func LogError(err_msg string) {
	if configContext != nil {
		if configContext.Log != nil {
			func() {
				defer func() {
					if r := recover(); r != nil {
						fmt.Fprintf(os.Stderr, "Panic in logger: %v, original message: %s\n", r, err_msg)
					}
				}()
				configContext.Log.Println(err_msg)
			}()
			return
		}
	}
	fmt.Fprintln(os.Stderr, err_msg)
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
