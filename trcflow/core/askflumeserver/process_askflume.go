package askflumeserver

import (
	"embed"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"strconv"
	"sync"

	"github.com/trimble-oss/tierceron-nute/mashupsdk"
	"github.com/trimble-oss/tierceron-nute/mashupsdk/server"

	// "github.com/trimble-oss/tierceron-nute/mashupsdk/server"

	// trccontext "github.com/trimble-oss/tierceron/trcchatproxy/context"

	// trcchat "github.com/trimble-oss/tierceron/trcchatproxy"
	"VaultConfig.TenantConfig/util"
	flowcore "github.com/trimble-oss/tierceron/trcflow/core"
)

var askFlumeContext *flowcore.AskFlumeContext
var msg string

//go:embed tls/mashup.crt
var mashupCert embed.FS

//go:embed tls/mashup.key
var mashupKey embed.FS

var flumeworld FlumeWorldApp

var id int64

func ProcessAskFlumeController(tfmContext *flowcore.TrcFlowMachineContext, trcFlowContext *flowcore.TrcFlowContext) error {
	var err error
	askFlumeContext, err = flowcore.InitAskFlume()
	// id = flowcore.GetId()
	if err != nil {
		fmt.Printf("Error initializing AskFlume %v", err)
		return err
	}

	if err != nil {
		fmt.Printf("Error initializing Google Chat %v", err)
		return err
	}
	var askFlumeWg sync.WaitGroup
	askFlumeWg.Add(1)

	go askFlumeFlowReceiver(askFlumeContext, &askFlumeWg)
	go askFlumeFlowSender(askFlumeContext)
	go InitGoogleChat()
	//Previous:
	// if msg == "" {
	// 	askFlumeContext.GoogleChatCxt, err = InitGoogleChat()
	// }

	if msg != "" {
		askFlumeContext.Query.Id = id
		askFlumeContext.Query.Message = msg
		askFlumeContext.FlowCase = "GChatQuery"
	}
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
				// askFlumeContext.FlowCase = ""
			}(askFlumeContext)
			askFlumeContext.FlowCase = ""
		case askFlumeContext.FlowCase == "GChatAnswer":
			go func(askFlumeContext *flowcore.AskFlumeContext) {
				askFlumeContext.GchatAnswers <- askFlumeContext
				// askFlumeContext.FlowCase = ""
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

func GetQuery(message *mashupsdk.MashupDetailedElementBundle) {
	msg = message.DetailedElements[0].Data
	msg_type := message.DetailedElements[0].Name

	if (msg_type != "ChatGptQuery" && msg_type != "ChatGptAnswer" && msg_type != "GChatQuery" && msg_type != "GChatAnswer") || msg_type == "" {
		log.Printf("Message does not correspond to any channel")
	} else {
		askFlumeContext.Query.Id = message.DetailedElements[0].Id
		askFlumeContext.Query.Message = msg
		askFlumeContext.FlowCase = msg_type
	}
}

func handleGChatQuery(askFlumeContext *flowcore.AskFlumeContext) error {
	if askFlumeContext.Query.Message != "" {
		askFlumeContext.Queries = append(askFlumeContext.Queries, askFlumeContext.Query)
		fmt.Println("Received query from google chat channel in Flume: ", askFlumeContext.Query.Message, " with ID: ", askFlumeContext.Query.Id)
		Flumeworld.DetailedElements[0].Name = "DialogFlow"
		// Send query back to DialogFlow to process it after it has been recorded in the flume queries
		_, upsertErr := Flumeworld.FlumeWorldContext.Client.UpsertElements(Flumeworld.FlumeWorldContext, &mashupsdk.MashupDetailedElementBundle{
			AuthToken:        " ",
			DetailedElements: Flumeworld.DetailedElements,
		})
		if upsertErr != nil {
			log.Printf("Failed to upsert google chat query to client: %v", upsertErr)
		}
	}

	return nil
}

func handleChatGptQuery(askFlumeContext *flowcore.AskFlumeContext) error {
	if askFlumeContext.Query.Message != "" {
		askFlumeContext.Queries = append(askFlumeContext.Queries, askFlumeContext.Query)
		fmt.Println("Received query from ChatGptQueries channel in Flume: ", askFlumeContext.Query.Message, " with ID: ", askFlumeContext.Query.Id)
		Flumeworld.DetailedElements[0].Name = "DialogFlow"
		fmt.Println("Processing query and accessing database...")
		response := util.ProcessAskFlumeEventMapper(askFlumeContext, askFlumeContext.Query)
		// Send query back to DialogFlow to process it after it has been recorded in the flume queries
		element := mashupsdk.MashupDetailedElement{
			Name: "DialogFlowResponse",
			Id:   askFlumeContext.Query.Id,
			Data: response.Message,
		}
		DetailedElements := []*mashupsdk.MashupDetailedElement{}
		DetailedElements = append(DetailedElements, &element)
		fmt.Println("Sending response back to DialogFlow for formatting")
		_, upsertErr := Flumeworld.FlumeWorldContext.Client.UpsertElements(Flumeworld.FlumeWorldContext, &mashupsdk.MashupDetailedElementBundle{
			AuthToken:        " ",
			DetailedElements: DetailedElements,
		})
		if upsertErr != nil {
			log.Printf("Failed to upsert google chat query to client: %v", upsertErr)
		}
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
	// Need to make client
	element := mashupsdk.MashupDetailedElement{
		Data: msg,
	}
	DetailedElements := []*mashupsdk.MashupDetailedElement{}
	DetailedElements = append(DetailedElements, &element)

	// serverConnectionConfigs := client.GetServerConfigs()
	// client.SetServerConfigs(serverConnectionConfigs)

	// Connection with mashup fully established.  Initialize mashup elements.
	// ctx := mashupsdk.MashupContext{Context: context.Background()} //trccontext.GetContext()

	// gchat_world_handler := &FlumeChat{}
	// s := server.MashupServer{}
	// s.SetHandler(gchat_world_handler
	// s := server.GetServer()
	// server.SetServerConfigs(serverConnectionConfigs) //may neeed to be in server instead
	// _, upsertErr := s.UpsertElements(ctx.Context, &mashupsdk.MashupDetailedElementBundle{
	// 	AuthToken:        client.GetServerAuthToken(),
	// 	DetailedElements: DetailedElements,
	// })

	// if upsertErr != nil {
	// 	log.Printf("Element state initialization failure: %s\n", upsertErr.Error())
	// }
	askFlumeContext.Close = true
	return nil
}

func InitGoogleChat() (*flowcore.GoogleChatCxt, error) {
	// Initialize gRPC server
	// callerCreds := flag.String("CREDS", "", "Credentials of caller")

	// Figure out how to set up log file
	// worldLog, err := os.OpenFile("custos.log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0666)

	//Attempt 2:
	// mashupsdk.InitCertKeyPair(mashupCert, mashupKey)

	// mashupCertBytes, err := mashupsdk.MashupCert.ReadFile("tls/mashup.crt")
	// if err != nil {
	// 	log.Fatalf("Couldn't load cert: %v", err)
	// }

	// mashupKeyBytes, err := mashupsdk.MashupKey.ReadFile("tls/mashup.key")
	// if err != nil {
	// 	log.Fatalf("Couldn't load key: %v", err)
	// }

	// cert, err := tls.X509KeyPair(mashupCertBytes, mashupKeyBytes)
	// if err != nil {
	// 	log.Fatalf("Couldn't construct key pair: %v", err)
	// }
	// creds := credentials.NewServerTLSFromCert(&cert)
	// s := grpc.NewServer(grpc.Creds(creds))

	// flumeworld := FlumeWorldApp{}

	// port := os.Getenv("PORT")
	// if port == "" {
	// 	port = "8080"
	// }
	// lis, err := net.Listen("tcp", ":"+port)
	// if err != nil {
	// 	log.Fatalf("failed to listen: %v", err)
	// }
	// // myInvoicerServer := &myGRPCServer{}
	// // myPkgName.RegisterInvoicerServer(s, myInvoicerServer)
	// // log.Printf("server listening at %v", lis.Addr())
	// // if err := s.Serve(lis); err != nil {
	// // 	log.Fatalf("failed to serve: %v", err)
	// // }

	// serverConnectionConfigs := &mashupsdk.MashupConnectionConfigs{
	// 	AuthToken: "c5376ccf9edc2a02499716c7e4f5599e8a96747e8a762c8ebed7a45074ad192a", // server token.
	// 	Port:      int64(lis.Addr().(*net.TCPAddr).Port),
	// }
	// log.Println(serverConnectionConfigs)
	// client.SetServerConfigs(serverConnectionConfigs)
	// server.SetServerConfigs(serverConnectionConfigs)
	// mashupCertPool := x509.NewCertPool()
	// mashupBlock, _ := pem.Decode([]byte(mashupCertBytes))
	// mashupClientCert, err := x509.ParseCertificate(mashupBlock.Bytes)
	// if err != nil {
	// 	log.Fatalf("failed to serve: %v", err)
	// }
	// mashupCertPool.AddCert(mashupClientCert)

	// defaultDialOpt := grpc.EmptyDialOption{}
	// conn, err := grpc.Dial("localhost:"+port, defaultDialOpt, grpc.WithTransportCredentials(credentials.NewTLS(&tls.Config{ServerName: "", RootCAs: mashupCertPool, InsecureSkipVerify: true})))
	// if err != nil {
	// 	log.Fatalf("did not connect: %v", err)
	// }
	// mashupContext := &mashupsdk.MashupContext{Context: context.Background(), MashupGoodies: nil}
	// mashupContext.Client = mashupsdk.NewMashupServerClient(conn)

	// log.Printf("Start Registering server.\n")
	// serv := &server.MashupServer{}
	// serv.SetHandler(flumeworld.MashupSdkApiHandler)
	// mashupsdk.RegisterMashupServerServer(s, serv)
	// log.Printf("server listening at %v", lis.Addr())
	// log.Printf("My Starting service.\n")
	// if err := s.Serve(lis); err != nil {
	// 	log.Fatalf("failed to serve: %v", err)
	// }

	//Attempt 1:

	mashupsdk.InitCertKeyPair(mashupCert, mashupKey)

	flumeworld = FlumeWorldApp{
		MashupSdkApiHandler: &FlumeHandler{},
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	configPort, err := strconv.ParseInt(port, 10, 64)
	if err != nil {
		fmt.Println(err)
	}
	// token := mashupsdk.GenAuthToken()
	configs := mashupsdk.MashupConnectionConfigs{
		AuthToken:   " ", //AuthToken provided by user
		CallerToken: " ",
		Server:      "localhost",
		Port:        configPort,
	}
	encoding, err := json.Marshal(&configs)
	if err != nil {
		fmt.Println(err)
	}

	callerCreds := flag.String("CREDS", string(encoding), "Credentials of caller")
	flag.Parse()
	// mashupsdk.InitCertKeyPair(mashupCert, mashupKey)
	// client.SetHandler(flumeworld.MashupSdkApiHandler)
	server.RemoteInitServer(*callerCreds, true, -2, flumeworld.MashupSdkApiHandler, flumeworld.WClientInitHandler) //Problem: Doesn't return back --> find out why
	// s := server.GetServer()
	// log.Println(s)

	return nil, nil
}
