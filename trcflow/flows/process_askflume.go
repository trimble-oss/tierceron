package flows

import (
	"fmt"
	"sync"

	tcutil "VaultConfig.TenantConfig/util"
	flowcore "github.com/trimble-oss/tierceron/trcflow/core"
)

func ProcessAskFlumeController(tfmContext *flowcore.TrcFlowMachineContext, trcFlowContext *flowcore.TrcFlowContext) error {
	// Initialize everything
	askFlumeContext, err := flowcore.InitAskFlume()
	if err != nil {
		return err
	}
	var askFlumeWg sync.WaitGroup
	askFlumeWg.Add(1)

	go askFlumeFlowReceiver(askFlumeContext, &askFlumeWg)
	go askFlumeFlowSender(askFlumeContext)

	// Get user query from google chat api
	askFlumeContext.FlowCase = "GChatQuery"
	askFlumeContext.Query = getGchatQuery(askFlumeContext)

	askFlumeWg.Wait()
	return err
}

func askFlumeFlowReceiver(askFlumeContext *flowcore.AskFlumeContext, askFlumeWg *sync.WaitGroup) {
	for {
		select {
		case gchat_query := <-askFlumeContext.GchatQueries:
			err := handleGChatQuery(askFlumeContext, gchat_query)
			if err != nil {
				return
			}
		case chatgpt_query := <-askFlumeContext.ChatGptQueries:
			err := handleChatGptQuery(askFlumeContext, chatgpt_query)
			if err != nil {
				return
			}
		case gptanswer := <-askFlumeContext.ChatGptAnswers:
			err := handleGptAnswer(askFlumeContext, gptanswer)
			if err != nil {
				return
			}
		case gchatanswer := <-askFlumeContext.GchatAnswers:
			err := handleGchatAnswer(askFlumeContext, gchatanswer)
			if err != nil {
				return
			}
		}
		if askFlumeContext.Close {
			askFlumeWg.Done()
			return
		}
	}
}

func askFlumeFlowSender(askFlumeContext *flowcore.AskFlumeContext) error {
	for {
		switch {
		case askFlumeContext.FlowCase == "GChatQuery":
			askFlumeContext.GchatQueries <- askFlumeContext.Query
			askFlumeContext.FlowCase = ""
			askFlumeContext.Query = &flowcore.AskFlumeMessage{
				Id:      0,
				Message: "",
			}
		case askFlumeContext.FlowCase == "GChatAnswer":
			askFlumeContext.GchatAnswers <- askFlumeContext.Query
			askFlumeContext.FlowCase = ""
			askFlumeContext.Query = &flowcore.AskFlumeMessage{
				Id:      0,
				Message: "",
			}
		case askFlumeContext.FlowCase == "ChatGptQuery":
			askFlumeContext.ChatGptQueries <- askFlumeContext.Query
			askFlumeContext.FlowCase = ""
			askFlumeContext.Query = &flowcore.AskFlumeMessage{
				Id:      0,
				Message: "",
			}
		case askFlumeContext.FlowCase == "ChatGptAnswer":
			askFlumeContext.ChatGptAnswers <- askFlumeContext.Query
			askFlumeContext.FlowCase = ""
			askFlumeContext.Query = &flowcore.AskFlumeMessage{
				Id:      0,
				Message: "",
			}
		default:
			if askFlumeContext.Close {
				return nil
			}
		}
	}
}

func getGchatQuery(askFlumeContext *flowcore.AskFlumeContext) *flowcore.AskFlumeMessage {
	// Hook up to google chat api to get response from user or from chatgpt (maybe have chatgpt send its message directly to gchat chan)
	fmt.Println("Getting input from user...")
	msg := "Hello AskFlume" //Will come from gchat api from user
	return &flowcore.AskFlumeMessage{
		Id:      flowcore.GetId(),
		Message: msg,
	}
}

func handleGChatQuery(askFlumeContext *flowcore.AskFlumeContext, query *flowcore.AskFlumeMessage) error {
	if query.Message != "" {
		fmt.Println("Received query from google chat channel: ", query.Message, " with ID: ", query.Id)
		askFlumeContext.FlowCase = "ChatGptQuery"
		askFlumeContext.Query = query
	}
	return nil
}

func handleChatGptQuery(askFlumeContext *flowcore.AskFlumeContext, query *flowcore.AskFlumeMessage) error {
	if query.Message != "" {
		fmt.Println("Processing query and accessing database...")
		// Make sure chat gpt is trained for mapping and pass any info through this method that
		// event mapper will need
		new_msg := tcutil.ProcessAskFlumeEventMapper(askFlumeContext, query)
		// Send unformatted message that comes from tenantconfig to answer channel to format it!
		askFlumeContext.FlowCase = "ChatGptAnswer"
		askFlumeContext.Query = new_msg
		// The unformatted answer will then be sent on gpt answer channel
	}

	return nil
}

func handleGptAnswer(askFlumeContext *flowcore.AskFlumeContext, gptanswer *flowcore.AskFlumeMessage) error {
	if gptanswer.Message != "" {
		fmt.Println("Formatting response from database...")
		// Will format response and send that to the gchat_response channel and send it out to user
		fmt.Println("Received query from chatgpt channel: ", gptanswer.Message, " with ID: ", gptanswer.Id)

		askFlumeContext.FlowCase = "GChatAnswer"
		askFlumeContext.Query = gptanswer
	}

	return nil
}

func handleGchatAnswer(askFlumeContext *flowcore.AskFlumeContext, gchatanswer *flowcore.AskFlumeMessage) error {
	fmt.Println("Sending response back to user and ending flow")
	// Will send answer to user using google chat api
	askFlumeContext.Close = true
	return nil
}
