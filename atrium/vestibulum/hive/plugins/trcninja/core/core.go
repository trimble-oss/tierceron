package core

import (
	"fmt"
	"log"
	"os"

	cmap "github.com/orcaman/concurrent-map/v2"
	"github.com/trimble-oss/tierceron-core/v2/core"
)

var chatMsgHookCtx *cmap.ConcurrentMap[string, core.ChatHookFunc]

// SociiKeyField is the key field name used for enterprise/socii identification
var SociiKeyField = "sociiId"

// SetSociiKey sets the key field name to be used for enterprise/socii identification
func SetSociiKey(keyName string) {
	SociiKeyField = keyName
}

var configContext *core.ConfigContext

func GetChatMsgHookCtx() *cmap.ConcurrentMap[string, core.ChatHookFunc]    { return chatMsgHookCtx }
func SetChatMsgHookCtx(ctx *cmap.ConcurrentMap[string, core.ChatHookFunc]) { chatMsgHookCtx = ctx }

func SetConfigContext(cc *core.ConfigContext) {
	configContext = cc
}

func GetConfigContext(plugin string) *core.ConfigContext {
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
