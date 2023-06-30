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

	askFlumeContext.FlowCase = "GChatQuery"
	askFlumeContext.Query = getGchatQuery(askFlumeContext)

	askFlumeWg.Wait()
	return err
}

func askFlumeFlowReceiver(askFlumeContext *flowcore.AskFlumeContext, askFlumeWg *sync.WaitGroup) {
	for {
		select {
		case gchat_query := <-askFlumeContext.GchatQueries:
			err := handleGChatQuery(gchat_query)
			if err != nil {
				return
			}
		case chatgpt_query := <-askFlumeContext.ChatGptQueries:
			err := handleChatGptQuery(chatgpt_query)
			if err != nil {
				return
			}
		case gptanswer := <-askFlumeContext.ChatGptAnswers:
			err := handleGptAnswer(gptanswer)
			if err != nil {
				return
			}
		case gchatanswer := <-askFlumeContext.GchatAnswers:
			err := handleGchatAnswer(gchatanswer)
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
			go func(askFlumeContext *flowcore.AskFlumeContext) {
				askFlumeContext.GchatQueries <- askFlumeContext
			}(askFlumeContext)
			askFlumeContext.FlowCase = ""
		case askFlumeContext.FlowCase == "GChatAnswer":
			go func(askFlumeContext *flowcore.AskFlumeContext) {
				askFlumeContext.GchatAnswers <- askFlumeContext
			}(askFlumeContext)
			askFlumeContext.FlowCase = ""
		case askFlumeContext.FlowCase == "ChatGptQuery":
			go func(askFlumeContext *flowcore.AskFlumeContext) {
				askFlumeContext.ChatGptQueries <- askFlumeContext
			}(askFlumeContext)
			askFlumeContext.FlowCase = ""
		case askFlumeContext.FlowCase == "ChatGptAnswer":
			go func(askFlumeContext *flowcore.AskFlumeContext) {
				askFlumeContext.ChatGptAnswers <- askFlumeContext
			}(askFlumeContext)
			askFlumeContext.FlowCase = ""
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

func handleGChatQuery(askFlumeContext *flowcore.AskFlumeContext) error {
	if askFlumeContext.Query.Message != "" {
		fmt.Println("Received query from google chat channel: ", askFlumeContext.Query.Message, " with ID: ", askFlumeContext.Query.Id)
		askFlumeContext.FlowCase = "ChatGptQuery"
	}
	return nil
}

func handleChatGptQuery(askFlumeContext *flowcore.AskFlumeContext) error {
	if askFlumeContext.Query.Message != "" {
		fmt.Println("Processing query and accessing database...")
		// Make sure chat gpt is trained for mapping and pass any info through this method that
		// event mapper will need
		new_msg := tcutil.ProcessAskFlumeEventMapper(askFlumeContext, askFlumeContext.Query)
		// Send unformatted message that comes from tenantconfig to answer channel to format it!
		askFlumeContext.FlowCase = "ChatGptAnswer"
		askFlumeContext.Query = new_msg
		// The unformatted answer will then be sent on gpt answer channel
	}
	return nil
}

func handleGptAnswer(askFlumeContext *flowcore.AskFlumeContext) error {
	if askFlumeContext.Query.Message != "" {
		fmt.Println("Formatting response from database...")
		// Will format response and send that to the gchat_response channel and send it out to user
		fmt.Println("Received query from chatgpt channel: ", askFlumeContext.Query.Message, " with ID: ", askFlumeContext.Query.Id)

		askFlumeContext.FlowCase = "GChatAnswer"
	}

	return nil
}

func handleGchatAnswer(askFlumeContext *flowcore.AskFlumeContext) error {
	fmt.Println("Sending response back to user and ending flow")
	// Will send answer to user using google chat api
	askFlumeContext.Close = true
	return nil
}
