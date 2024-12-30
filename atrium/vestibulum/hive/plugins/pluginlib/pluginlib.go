package pluginlib

import (
	"errors"
	"fmt"
	"log"

	tccore "github.com/trimble-oss/tierceron-core/v2/core"
)

func Init(pluginName string,
	properties *map[string]interface{},
	PostInit func(*tccore.ConfigContext)) (*tccore.ConfigContext, error) {
	if properties == nil {
		fmt.Println("Missing initialization components")
		return nil, errors.New("missing initialization component")
	}
	var logger *log.Logger
	if _, ok := (*properties)["log"].(*log.Logger); ok {
		logger = (*properties)["log"].(*log.Logger)
	}

	configContext := &tccore.ConfigContext{
		Config:      properties,
		ConfigCerts: &map[string][]byte{},
		Log:         logger,
	}

	if channels, ok := (*properties)[tccore.PLUGIN_EVENT_CHANNELS_MAP_KEY]; ok {
		if chans, ok := channels.(map[string]interface{}); ok {
			if rchan, ok := chans[tccore.PLUGIN_CHANNEL_EVENT_IN].(map[string]interface{}); ok {
				if cmdreceiver, ok := rchan[tccore.CMD_CHANNEL].(*chan tccore.KernelCmd); ok {
					configContext.CmdReceiverChan = cmdreceiver
					configContext.Log.Println("Command Receiver initialized.")
				} else {
					configContext.Log.Println("Unsupported receiving channel passed")
					goto postinit
				}

				if cr, ok := rchan[tccore.CHAT_CHANNEL].(*chan *tccore.ChatMsg); ok {
					configContext.Log.Println("Chat Receiver initialized.")
					configContext.ChatReceiverChan = cr
					//					go chatHandler(*cr)
				} else {
					configContext.Log.Println("Unsupported chat message receiving channel passed")
					goto postinit
				}

			} else {
				configContext.Log.Println("No event in receiving channel passed")
				goto postinit
			}
			if schan, ok := chans[tccore.PLUGIN_CHANNEL_EVENT_OUT].(map[string]interface{}); ok {
				if cmdsender, ok := schan[tccore.CMD_CHANNEL].(*chan tccore.KernelCmd); ok {
					configContext.CmdSenderChan = cmdsender
					configContext.Log.Println("Command Sender initialized.")
				} else {
					configContext.Log.Println("Unsupported receiving channel passed")
					goto postinit
				}

				if cs, ok := schan[tccore.CHAT_CHANNEL].(*chan *tccore.ChatMsg); ok {
					configContext.Log.Println("Chat Sender initialized.")
					configContext.ChatSenderChan = cs
				} else {
					configContext.Log.Println("Unsupported chat message receiving channel passed")
					goto postinit
				}

				if dfsc, ok := schan[tccore.DATA_FLOW_STAT_CHANNEL].(*chan *tccore.TTDINode); ok {
					configContext.Log.Println("DFS Sender initialized.")
					configContext.DfsChan = dfsc
				} else {
					configContext.Log.Println("Unsupported DFS sending channel passed")
					goto postinit
				}

				if sc, ok := schan[tccore.ERROR_CHANNEL].(*chan error); ok {
					configContext.ErrorChan = sc
				} else {
					configContext.Log.Println("Unsupported sending channel passed")
					goto postinit
				}
			} else {
				configContext.Log.Println("No event out channel passed")
				goto postinit
			}
		} else {
			configContext.Log.Println("No event channels passed")
			goto postinit
		}
	}
postinit:
	PostInit(configContext)
	return configContext, nil
}

func SendDfStat(configContext *tccore.ConfigContext, dfsctx *tccore.DeliverStatCtx, dfstat *tccore.TTDINode) {
	dfstat.Name = configContext.ArgosId
	dfstat.FinishStatistic("", "", "", configContext.Log, true, dfsctx)
	configContext.Log.Printf("Sending dataflow statistic to kernel: %s\n", dfstat.Name)
	dfstatClone := *dfstat
	go func(dsc *tccore.TTDINode) {
		if configContext != nil && *configContext.DfsChan != nil {
			*configContext.DfsChan <- dsc
		}
	}(&dfstatClone)
}
