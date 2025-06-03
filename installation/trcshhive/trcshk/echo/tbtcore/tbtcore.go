package tbtcore

import (
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/url"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/trimble-oss/tierceron-core/v2/buildopts/plugincoreopts"

	echocore "github.com/trimble-oss/tierceron/installation/trcshhive/trcshk/echo/echocore"
	tbtapi "github.com/trimble-oss/tierceron/installation/trcshhive/trcshk/echo/tbtcore/tbtapi"
	tbtgapi "github.com/trimble-oss/tierceron/installation/trcshhive/trcshk/echo/tbtcore/tbtgapi"
	ttsdk "github.com/trimble-oss/tierceron/installation/trcshhive/trcshk/echo/trcshtalksdk"

	tccore "github.com/trimble-oss/tierceron-core/v2/core"
	"gopkg.in/yaml.v2"
)

var configContext *tccore.ConfigContext

func init() {
	if plugincoreopts.BuildOptions.IsPluginHardwired() {
		return
	}
	peerExe, err := os.Open("/usr/local/trcshk/plugins/trcshtalkback.so")
	if err != nil {
		fmt.Println("Unable to sha256 plugin")
		return
	}

	defer peerExe.Close()

	h := sha256.New()
	if _, err := io.Copy(h, peerExe); err != nil {
		fmt.Printf("Unable to copy file for sha256 of plugin: %s\n", err)
		return
	}
	sha := hex.EncodeToString(h.Sum(nil))
	fmt.Printf("TrcshTalkBack Version: %s\n", sha)
}

func chat_receiver(chat_receive_chan chan *tccore.ChatMsg) {
	for {
		event := <-chat_receive_chan
		switch {
		case event == nil:
			fallthrough
		case event.Name != nil && *event.Name == "SHUTDOWN":
			fmt.Println("trcshtalkback shutting down message receiver")
			return
		case event.Response != nil && *event.Response == "Service unavailable":
			fmt.Println("Healthcheck unable to access chat service.")
			return
		default:
			fmt.Println("trcshtalkback received chat message")
			response := tbtgapi.HelloWorldDiagnostic()
			(*event).Response = &response
			*configContext.ChatSenderChan <- event
		}
	}
}

func receiver(receive_chan chan int) {
	for {
		event := <-receive_chan
		switch {
		case event == tccore.PLUGIN_EVENT_START:
			go start()
		case event == tccore.PLUGIN_EVENT_STOP:
			go stop()
			return
		case event == tccore.PLUGIN_EVENT_STATUS:
			//TODO
		default:
			//TODO
		}
	}
}

func start() {
	if configContext == nil {
		fmt.Println("no config context initialized for trcshtalkback")
		return
	}
	var send_err func(error)
	if echocore.IsKernelPluginMode() {
		send_err, _ = tbtgapi.InitGServer()
	} else {
		send_err = func(err error) {
			configContext.Log.Printf("Error: %s", err.Error())
		}
	}
	tbtapi.InitHttpServer(configContext, send_err)
}

func stop() {
	tbtapi.StopHttpServer()
	if echocore.IsKernelPluginMode() {
		tbtgapi.StopGServer()
	}
	os.Exit(0)
}

func GetConfigContext() *tccore.ConfigContext { return configContext }

func Init(properties *map[string]interface{}) {
	var err error
	configContext, err = tccore.Init(properties,
		tccore.TRCSHHIVEK_CERT,
		tccore.TRCSHHIVEK_KEY,
		tbtgapi.COMMON_PATH,
		"ArgosId",
		start,
		receiver,
		chat_receiver,
	)
	if credentialsInterface, ok := (*properties)[tbtgapi.CREDENTIALS_PATH]; ok {
		if credentialsMap, ok := credentialsInterface.(*map[string]interface{}); ok {
			// Scrub private_key
			cert := (*credentialsMap)["private_key"].(string)
			cert = strings.ReplaceAll(cert, "\\n", "\n")
			(*credentialsMap)["private_key"] = cert

			// Scrub client_x509_cert_url
			certUrl := (*credentialsMap)["client_x509_cert_url"].(string)
			certUrl, err = url.QueryUnescape(certUrl)
			if err == nil {
				(*credentialsMap)["client_x509_cert_url"] = certUrl

				jsonCredentials, err := json.Marshal(*credentialsMap)
				if err == nil {
					(*configContext.Config)[tbtgapi.CREDENTIALS_PATH] = string(jsonCredentials)
				}
			}
		}

	}
	echocore.InitNetwork(configContext)

	stopChan := make(chan os.Signal, 1)
	signal.Notify(stopChan, os.Interrupt, syscall.SIGTERM)

	go func() {
		<-stopChan
		stop()
	}()

	if echocore.IsKernelPluginMode() {
		tbtgapi.SetConfigContext(configContext)
	}
	tbtapi.SetConfigContext(configContext)
	if err != nil {
		if configContext != nil {
			configContext.Log.Println("Successfully initialized trcshtalkback.")
			fmt.Println(err.Error())
		} else {
			fmt.Println(err.Error())
		}
	} else {
		configContext.Log.Println("Successfully initialized trcshtalkback.")
	}
}

func GetConfigPaths() []string {
	return []string{
		tbtgapi.COMMON_PATH,
		tbtgapi.CREDENTIALS_PATH,
		tccore.TRCSHHIVEK_CERT,
		tccore.TRCSHHIVEK_KEY,
	}
}

// Convenience function for starting trcshtalkback outside of the hive.
func EchoRunner(mashupCert *embed.FS, mashupKey *embed.FS, configFile *embed.FS, envPtr *string, logFilePtr *string) *tccore.ChatMsg {
	config := make(map[string]interface{})

	f, err := os.OpenFile(*logFilePtr, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0644)
	if err != nil {
		fmt.Printf("Error opening log file: %v\n", err)
		os.Exit(-1)
	}
	logger := log.New(f, "[trcshtalkback]", log.LstdFlags)
	config["log"] = logger

	// var trcshTalkCertBytes []byte
	var configFileBytes []byte
	if configFile != nil {
		configFileBytes, _ = configFile.ReadFile("local_config/application.yml")
	}

	// Create an empty map for the YAML data
	var configCommon map[string]interface{}

	// Unmarshal the YAML data into the map
	err = yaml.Unmarshal(configFileBytes, &configCommon)
	if err != nil {
		logger.Println("Error unmarshaling YAML:", err)
		os.Exit(-1)
	}
	config["env"] = *envPtr
	config[tbtgapi.COMMON_PATH] = &configCommon

	Init(&config)
	chatSenderChan := make(chan *tccore.ChatMsg, 2)
	configContext.ChatSenderChan = &chatSenderChan

	GetConfigContext().Start()

	// Recreate trcshtalk messaging mechanism to send
	// a healthcheck message to ourselves to ensure
	// the grpc endpoint is alive...
	chat_receive_chan := make(chan *tccore.ChatMsg)
	query := tccore.ChatMsg{}
	chatId := "helloworld"
	query.ChatId = &chatId
	query.Query = &[]string{ttsdk.Diagnostics_name[1]}

	// Off you go...
	go func(crc chan *tccore.ChatMsg, msg *tccore.ChatMsg) {
		crc <- msg
	}(chat_receive_chan, &query)
	go chat_receiver(chat_receive_chan)

	return <-*configContext.ChatSenderChan
}
