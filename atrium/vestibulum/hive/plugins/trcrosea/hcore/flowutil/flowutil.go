package flowutil

import (
	cmap "github.com/orcaman/concurrent-map/v2"
	tccore "github.com/trimble-oss/tierceron-core/v2/core"
)

var chatMsgHookCtx *cmap.ConcurrentMap[string, tccore.ChatHookFunc]

var chatSenderChan *chan *tccore.ChatMsg

func InitChatSenderChan(csc *chan *tccore.ChatMsg) {
	chatSenderChan = csc
}

func GetChatSenderChan() *chan *tccore.ChatMsg {
	return chatSenderChan
}

func GetChatMsgHookCtx() **cmap.ConcurrentMap[string, tccore.ChatHookFunc] { return &chatMsgHookCtx }
