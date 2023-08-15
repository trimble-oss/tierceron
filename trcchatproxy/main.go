package main

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"embed"
	"encoding/pem"
	"log"

	"github.com/trimble-oss/tierceron-nute/mashupsdk"
	"github.com/trimble-oss/tierceron-nute/mashupsdk/client"
	"github.com/trimble-oss/tierceron-nute/mashupsdk/server"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"

	// trccontext "github.com/trimble-oss/tierceron/trcchatproxy/context"
	askflumeserver "github.com/trimble-oss/tierceron/trcflow/core/askflumeserver"
)

// var connectionConfigs *mashupsdk.MashupConnectionConfigs
// var clientConnectionConfigs *mashupsdk.MashupConnectionConfigs
var serverConnectionConfigs *mashupsdk.MashupConnectionConfigs

// var mashupContext *mashupsdk.MashupContext
var insecure *bool

//go:embed tls/mashup.crt
var mashupCert embed.FS

//go:embed tls/mashup.key
var mashupKey embed.FS

// var gchatApp GChatApp
var flumeworld askflumeserver.FlumeWorldApp

var ctx *mashupsdk.MashupContext

type GoogleChatContext struct {
	MashupContext *mashupsdk.MashupContext // Needed for callbacks to other mashups
}

type GChatApp struct {
	MashupSdkApiHandler          *GoogleChatHandler
	GoogleChatContext            *GoogleChatContext //*FlumeWorldContext
	mashupDisplayContext         *mashupsdk.MashupDisplayContext
	WClientInitHandler           *WorldClientInitHandler
	DetailedElements             []*mashupsdk.MashupDetailedElement
	MashupDetailedElementLibrary map[int64]*mashupsdk.MashupDetailedElement
	ElementLoaderIndex           map[string]int64 // mashup indexes by Name
}

func (w *GChatApp) InitServer(callerCreds string, insecure bool, maxMessageLength int) {
	if callerCreds != "" {
		server.InitServer(callerCreds, insecure, maxMessageLength, w.MashupSdkApiHandler, w.WClientInitHandler)
	}
}

func main() {
	secure := true
	insecure = &secure
	flumeworld = askflumeserver.FlumeWorldApp{
		MashupSdkApiHandler: &askflumeserver.FlumeChat{},
		FlumeWorldContext:   &mashupsdk.MashupContext{},
		WClientInitHandler:  &askflumeserver.WorldClientInitHandler{},
	}

	// Initialize local server.
	mashupsdk.InitCertKeyPair(mashupCert, mashupKey)

	// params := []string{"ChatHandler"} //ChatHandler alerts bootstrapinit to follow chat connection route
	// New:
	// rpcCreds := oauth.NewOauthAccess(&oauth2.Token{AccessToken: "c5376ccf9edc2a02499716c7e4f5599e8a96747e8a762c8ebed7a45074ad192a"})

	mashupCertBytes, err := mashupsdk.MashupCert.ReadFile("tls/mashup.crt")
	if err != nil {
		log.Fatalf("Couldn't load cert: %v", err)
	}

	// mashupKeyBytes, err := mashupsdk.MashupKey.ReadFile("tls/mashup.key")
	// if err != nil {
	// 	log.Fatalf("Couldn't load key: %v", err)
	// }

	// serverCert, err := tls.X509KeyPair(mashupCertBytes, mashupKeyBytes)
	// if err != nil {
	// 	log.Fatalf("failed to serve: %v", err)
	// }
	// creds := credentials.NewServerTLSFromCert(&serverCert)
	// opts := []grpc.DialOption{
	// 	grpc.WithTransportCredentials(creds),
	// 	// grpc.WithPerRPCCredentials(rpcCreds),
	// }
	// opts = append(opts, grpc.WithBlock())
	serverConnectionConfigs := &mashupsdk.MashupConnectionConfigs{
		AuthToken: "c5376ccf9edc2a02499716c7e4f5599e8a96747e8a762c8ebed7a45074ad192a", // server token.
		Port:      8080,
	}
	client.SetServerConfigs(serverConnectionConfigs)
	server.SetServerConfigs(serverConnectionConfigs)
	mashupCertPool := x509.NewCertPool()
	mashupBlock, _ := pem.Decode([]byte(mashupCertBytes))
	mashupClientCert, err := x509.ParseCertificate(mashupBlock.Bytes)
	if err != nil {
		log.Fatalf("failed to serve: %v", err)
	}
	mashupCertPool.AddCert(mashupClientCert)
	conn, err := grpc.Dial("localhost:8080", grpc.EmptyDialOption{}, grpc.WithTransportCredentials(credentials.NewTLS(&tls.Config{ServerName: "", RootCAs: mashupCertPool, InsecureSkipVerify: *insecure})))
	if err != nil {
		log.Fatalf("did not connect: %v", err)
	}
	c := mashupsdk.NewMashupServerClient(conn)
	mashupCtx := &mashupsdk.MashupContext{Context: context.Background(), Client: c}
	flumeworld.FlumeWorldContext = mashupCtx
	//Before:
	// flumeworld.FlumeWorldContext = client.BootstrapInit("trcchatproxy", flumeworld.MashupSdkApiHandler, nil, params, insecure)

	ctx = flumeworld.FlumeWorldContext
	var upsertErr error

	element := mashupsdk.MashupDetailedElement{
		Name: "GChatQuery",
		Data: "Hello Server!",
	}
	DetailedElements := []*mashupsdk.MashupDetailedElement{}
	DetailedElements = append(DetailedElements, &element)
	// for _, detailedElement := range gchatApp.MashupDetailedElementLibrary {
	// 	DetailedElements = append(DetailedElements, detailedElement)
	// }
	log.Printf("Delivering mashup elements.\n")

	// Need to make client Before:::
	// serverConnectionConfigs = client.GetServerConfigs()
	// client.SetServerConfigs(serverConnectionConfigs)

	// Connection with mashup fully established.  Initialize mashup elements.
	// trccontext.SetContext(flumeworld.FlumeWorldContext)
	_, upsertErr = flumeworld.FlumeWorldContext.Client.UpsertElements(flumeworld.FlumeWorldContext,
		&mashupsdk.MashupDetailedElementBundle{
			AuthToken:        "c5376ccf9edc2a02499716c7e4f5599e8a96747e8a762c8ebed7a45074ad192a", //client.GetServerAuthToken()
			DetailedElements: DetailedElements,
		})

	if upsertErr != nil {
		log.Printf("Element state initialization failure: %s\n", upsertErr.Error())
	}

	log.Printf("Mashup elements delivered.\n")
}

// func RegisterFromFlume(elements *mashupsdk.MashupDetailedElementBundle) {
// 	log.Println("Received elements from flume flow: ", elements)
// }

// chatApp.InitConnection(mashupctx)
// Should call initialization of server -->
//Initialize the google chat api here!
// callerCreds := flag.String("CREDS", "", "Credentials of caller")
// insecure := flag.Bool("tls-skip-validation", false, "Skip server validation")
//flag.Parse()
// worldLog, err := os.OpenFile("world.log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0666)
// if err != nil {
// 	log.Fatalf(err.Error(), err)
// }
// log.SetOutput(worldLog)

//c := GChatApp{}

// add api handler
// server.InitServer(*callerCreds, *insecure, 0, c.mashupSdkApiHandler, nil)
// ctx := context.Background()
// //Need to update file path with path to key file --> Find secure place to store it!
// service, err := chat.NewService(ctx, option.WithCredentialsFile("/home/mbailey/workspace/Github/tierceron/trcflow/core/tls/askflume-cdad4e96aa2f.json"))
//
//	gchat_ctx := &GoogleChatCxt{
//		Context: ctx,
//		Service: service,
//	}
//
//	if err != nil {
//		fmt.Println(err)
//		return gchat_ctx, err
//	} else {
//
//		return gchat_ctx, nil
//	}
