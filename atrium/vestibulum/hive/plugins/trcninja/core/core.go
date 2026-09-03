package core

import (
	"fmt"
	"log"
	"os"
	"sync"

	"github.com/google/uuid"
	cmap "github.com/orcaman/concurrent-map/v2"
	tccore "github.com/trimble-oss/tierceron-core/v2/core"
)

const (
	CONFLUENT_CLIENT_CERT        = "Common/kafka-confluent.pem.mf.tmpl"
	CONFLUENT_SCHEMA_CLIENT_CERT = "Common/schema-confluent.pem.mf.tmpl"
	SERVICE_CLIENT_ROOT_CERT     = "Common/serviceclientcert.pem.mf.tmpl"
	COMMON_PATH                  = "/local_config/kafkaconsumer"
)

var (
	chatMsgHookCtx             *cmap.ConcurrentMap[string, tccore.ChatHookFunc]
	TenantStatusMap            *cmap.ConcurrentMap[string, TenantStatus]
	configContext              *tccore.ConfigContext
	defaultConsumerGroupID     string
	defaultConsumerGroupMu     sync.RWMutex
	defaultConsumerGroupIDFunc ConsumerGroupIDFunc
	flowLogTagFunc             FlowLogTagFunc
)

// SociiKeyField is the key field name used for enterprise/socii identification.
var SociiKeyField = "sociiId"

type ConsumerGroupIDFunc func() (string, error)

type FlowLogTagFunc func(configContext *tccore.ConfigContext, topicName string) string

type TenantStatus int

const (
	TenantStatusUntested TenantStatus = iota
	TenantStatusDBConnFailure
	TenantStatusKafkaFailure
	TenantStatusFailure
	TenantStatusRetry
	TenantStatusRetried
)

func GetChatMsgHookCtx() *cmap.ConcurrentMap[string, tccore.ChatHookFunc] { return chatMsgHookCtx }

func SetChatMsgHookCtx(ctx *cmap.ConcurrentMap[string, tccore.ChatHookFunc]) { chatMsgHookCtx = ctx }

func SetConfigContext(cc *tccore.ConfigContext) {
	configContext = cc
}

func GetConfigContext(pluginName ...string) *tccore.ConfigContext {
	return configContext
}

func LogError(errMsg string) {
	if configContext != nil {
		if configContext.Log != nil {
			func() {
				defer func() {
					if r := recover(); r != nil {
						fmt.Fprintf(os.Stderr, "Panic in logger: %v, original message: %s\n", r, errMsg)
					}
				}()
				configContext.Log.Println(errMsg)
			}()
			return
		}
	}
	fmt.Fprintln(os.Stderr, errMsg)
}

func SetLogger(logger *log.Logger) {
	if configContext != nil {
		configContext.Log = logger
	} else {
		configContext = &tccore.ConfigContext{Log: logger}
	}
}

func SetSociiKey(keyName string) {
	SociiKeyField = keyName
}

func SetDefaultConsumerGroupID(consumerGroupID string) {
	defaultConsumerGroupMu.Lock()
	defer defaultConsumerGroupMu.Unlock()
	defaultConsumerGroupID = consumerGroupID
	defaultConsumerGroupIDFunc = nil
}

func SetDefaultConsumerGroupIDFunc(consumerGroupIDFunc ConsumerGroupIDFunc) {
	defaultConsumerGroupMu.Lock()
	defer defaultConsumerGroupMu.Unlock()
	defaultConsumerGroupID = ""
	defaultConsumerGroupIDFunc = consumerGroupIDFunc
}

func GetDefaultConsumerGroupID() (string, ConsumerGroupIDFunc) {
	defaultConsumerGroupMu.RLock()
	defer defaultConsumerGroupMu.RUnlock()
	return defaultConsumerGroupID, defaultConsumerGroupIDFunc
}

func SetFlowLogTagFunc(logTagFunc FlowLogTagFunc) {
	flowLogTagFunc = logTagFunc
}

func GetFlowLogTagFunc() FlowLogTagFunc {
	return flowLogTagFunc
}

func defaultKafkaConsumerGroupID() (string, error) {
	id, err := uuid.NewRandom()
	if err != nil {
		return "", fmt.Errorf("could not generate guid: %w", err)
	}
	return "kafka-ninja-" + id.String(), nil
}

func Init(pluginName string, properties *map[string]interface{},
	startHandler func(string),
	receiverHandler func(chan tccore.KernelCmd),
	chatReceiverHandler func(chan *tccore.ChatMsg),
) {
	// Initialize chat message hook context
	if chatMsgHookCtx == nil {
		cm := cmap.New[tccore.ChatHookFunc]()
		chatMsgHookCtx = &cm
	}

	var err error
	configContext, err := tccore.Init(
		properties,
		CONFLUENT_CLIENT_CERT,        // confluent client cert
		CONFLUENT_SCHEMA_CLIENT_CERT, // confluent schema client cert
		COMMON_PATH,
		"hiveplugin",
		startHandler,
		receiverHandler,
		chatReceiverHandler,
	)
	if err != nil {
		if configContext != nil {
			configContext.Log.Println("Some trouble initializing ninja.")
			configContext.Log.Println(err.Error())
		} else {
			fmt.Fprintln(os.Stderr, err.Error())
			return
		}
	}

	if configContext != nil && configContext.Config != nil {
		if configContext.ConfigCerts == nil {
			configContext.ConfigCerts = &map[string][]byte{}
		}
		if certBytes, ok := (*properties)[tccore.TRCSHHIVEK_CERT].([]byte); ok {
			(*configContext.Config)[tccore.TRCSHHIVEK_CERT] = certBytes
			(*configContext.ConfigCerts)[tccore.TRCSHHIVEK_CERT] = certBytes
		} else {
			configContext.Log.Println("No cert provided for database connection.")
		}
		if rootCertBytes, ok := (*properties)[SERVICE_CLIENT_ROOT_CERT].([]byte); ok {
			(*configContext.Config)[SERVICE_CLIENT_ROOT_CERT] = rootCertBytes
			(*configContext.ConfigCerts)[SERVICE_CLIENT_ROOT_CERT] = rootCertBytes
		}
	}
	// Change logging context
	configContext.Log = log.New(configContext.Log.Writer(), "[ninja]", log.LstdFlags)
	SetDefaultConsumerGroupIDFunc(defaultKafkaConsumerGroupID)
	SetConfigContext(configContext)

	configContext.Log.Println("Successfully initialized ninja.")
}
