package core

type AskFlumeMessage struct {
	Id      int64
	Message string
}

type AskFlumeContext struct {
	GchatQueries   chan *AskFlumeContext
	ChatGptQueries chan *AskFlumeContext
	ChatGptAnswers chan *AskFlumeContext
	GchatAnswers   chan *AskFlumeContext
	Close          bool
	FlowCase       string
	Query          *AskFlumeMessage
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

	id = 1
	cxt := &AskFlumeContext{
		GchatQueries:   gchat_queries,
		ChatGptQueries: chatgpt_queries,
		ChatGptAnswers: chatgpt_ans,
		GchatAnswers:   gchat_ans,
		Close:          false,
		FlowCase:       "",
		Query:          empty_query,
	}
	return cxt, nil
}

func InitGoogleChat() {
	//Initialize the google chat api here!
}

func InitChatGPT() {
	// Initialize the chat gpt api here and train it!
	// Wonder if it's possible to use chat gpt to train itself? --> tell it to come up with questions about... to populate
	// training data?
}
