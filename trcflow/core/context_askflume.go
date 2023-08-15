//go:build darwin || linux
// +build darwin linux

package core

import (
	"context"

	"github.com/trimble-oss/tierceron-nute/mashupsdk"

	chat "google.golang.org/api/chat/v1"
)

// var GChatHandler trccontext.MashupSdkApiHandler

var clientConnectionConfigs *mashupsdk.MashupConnectionConfigs
var serverConnectionConfigs *mashupsdk.MashupConnectionConfigs



type AskFlumeMessage struct {
	Id      int64
	Message string
}

type GoogleChatCxt struct {
	Context context.Context
	Service *chat.Service
}

type AskFlumeContext struct {
	GchatQueries   chan *AskFlumeContext
	ChatGptQueries chan *AskFlumeContext
	ChatGptAnswers chan *AskFlumeContext
	GchatAnswers   chan *AskFlumeContext
	GoogleChatCxt  *GoogleChatCxt
	Close          bool
	FlowCase       string
	Query          *AskFlumeMessage
	Queries        []*AskFlumeMessage
}

var id int64

func GetId() int64 {
	id += 1
	return id - 1
}

func InitAskFlume() (*AskFlumeContext, error) {
	gchat_queries := make(chan *AskFlumeContext)
	chatgpt_queries := make(chan *AskFlumeContext)
	chatgpt_ans := make(chan *AskFlumeContext)
	gchat_ans := make(chan *AskFlumeContext)
	empty_query := &AskFlumeMessage{
		Id:      0,
		Message: "",
	}
	// gchat_cxt, err := InitGoogleChat()
	// if e}rr != nil {
	// 	fmt.Println("Find way to log err")

	id = 1
	cxt := &AskFlumeContext{
		GchatQueries:   gchat_queries,
		ChatGptQueries: chatgpt_queries,
		ChatGptAnswers: chatgpt_ans,
		GchatAnswers:   gchat_ans,
		GoogleChatCxt:  nil,
		Close:          false,
		FlowCase:       "",
		Query:          empty_query,
	}
	return cxt, nil
}

func InitChatGPT() {
	// Initialize the chat gpt api here and train it!
	// Wonder if it's possible to use chat gpt to train itself? --> tell it to come up with questions about... to populate
	// training data?
}
