package kafkatesting

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"net"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/google/uuid"
	"github.com/trimble-oss/tierceron-core/v2/core"

	"github.com/vbauerster/mpb/v8"
	"github.com/vbauerster/mpb/v8/decor"

	// "github.com/trimble-oss/tierceron/pkg/core"
	etlcore "github.com/trimble-oss/tierceron/atrium/vestibulum/hive/plugins/trcninja/core"

	"github.com/trimble-oss/tierceron/atrium/vestibulum/hive/plugins/trcninja/confighelper"
	"github.com/twmb/franz-go/pkg/kgo"
	"github.com/twmb/franz-go/pkg/sasl/plain"
)

const (
	STATE_INIT                   string = "init                     "
	STATE_DBCONFIG               string = "dbconfig                 "
	STATE_DBINIT                 string = "dbinitted                "
	STATE_DBUPDATED              string = "dbupdated                "
	STATE_FAILED                 string = "failed                   "
	STATE_FAILED_DB_CONN         string = "failed dbconn failure    "
	STATE_FAILED_MISSING_RECORD  string = "failed missing record    "
	STATE_FAILED_UPDATE_RECORD   string = "failed update record     "
	STATE_FAILED_PARENT_RECORD   string = "failed miss parent record"
	STATE_FAILED_INSERT_RECORD   string = "failed insert record     "
	STATE_FAILED_SETUP           string = "failed setup             "
	STATE_RAWTOPIC_ARRIVAL       string = "rawarrive                "
	STATE_AGNOSTIC_TOPIC_ARRIVAL string = "agnostarr                "
	STATE_COMPLETE               string = "completed                "
	STATE_COMPLETE_CLEANED       string = "completed, cleaned       "
	STATE_CLEAN                  string = "clean                    "
	STATE_CLEAN_SETUP_FAILURE    string = "Clean failed setup       "
	STATE_UPDATE_EXPECTED_VAL    string = "Update expected value    "
	STATE_KAFKA_INIT             string = "kafkainit                "
	STATE_FAILED_KAFKA_CONN      string = "failed kafka conn failure"
)

type KafkaErrMessage struct {
	TopicKey   string
	KafkaError error
}

type KafkaTestBundle struct {
	Name               string
	CompletionStatus   string
	Message            string
	ExpectedAvroKey    map[string]interface{}
	ExpectedLogicalKey map[string]interface{}
	ExpectedValue      map[string]interface{}
	SuccessFun         func(error)
	Wg                 *sync.WaitGroup
}

type SeededKafkaReader struct {
	startTime            time.Time
	TopicName            string
	TopicType            string // json or avro
	ConsumerGroupID      string // tracks the consumer group ID for reconnection
	kafkaClient          *kgo.Client
	firstRecordCommitted bool // tracks if we've committed after first fetch
	kafkaTestBundle      map[string]*KafkaTestBundle
	kafkaTestBundleLock  sync.RWMutex
	incomingTestChan     chan *KafkaTestBundle
	deleteTestChan       chan *KafkaTestBundle
	HandleEventFunc      func(k map[string]any, n map[string]any)
	isTestReader         bool           // tracks if this is a test reader or event handler
	ninjaTestClosed      bool           // tracks if this reader's ninja test has been closed
	CacheKey             string         // cached key for reader lookup
	engineRunning        sync.WaitGroup // tracks if the KafkaTestEngine loop is still running
}

// MultiBarContainer container for our progress bar
type MultiBarContainer struct {
	Mpb *mpb.Progress
	Wg  *sync.WaitGroup
}

var (
	kafkaReaderCache       sync.Map // key: string, value: *SeededKafkaReader
	openConnectionCache    map[string]*sql.DB
	multiProgressContainer MultiBarContainer
	multiBarLock           sync.Mutex
	mpbContext             context.Context
	mpbContextCancel       context.CancelFunc
	IsPlugin               bool = true
)

type IndirectDBConnectionFunc func(configContex *core.ConfigContext, tenantID string) (string, string, *sql.DB, error)

var IndirectDbFunc IndirectDBConnectionFunc

var (
	plugin           = false
	flowTopicsClosed = false
)

func init() {
	openConnCacheLock = &sync.Mutex{}
	openConnectionCache = make(map[string]*sql.DB)
}

func ShutdownMPB() {
	etlcore.LogError("Closing context.")
	if mpbContext != nil {
		mpbContext.Done()
	}
	etlcore.LogError("Cancelling context.")
	if mpbContextCancel != nil {
		mpbContextCancel()
	}

	// panic(errors.New("mpb missing good cleanup"))

	defer func() {
		// TODO: mpb has built in cleanup race condition...
		// Always hangs on multiProgressContainer.Mpb.Wait()
		// let vault deal with the problem until mpb fixes it.
		// So, we will not 'recover' for now... just let panic go through...

		// if r := recover(); r != nil {
		// 	etlcore.LogError(fmt.Sprintf("Mpb waitgroup troubles %v", r))
		// 	etlcore.LogError(fmt.Sprintf("Recovered...%v", r))
		// }
		etlcore.LogError("Mpb waitgroup finalized.")
		if multiProgressContainer.Mpb != nil {
			etlcore.LogError("Waiting for mpb.")
			// multiProgressContainer.Mpb.Wait()
			etlcore.LogError("Mpb done.")
		}
		mpbContext = nil
		multiProgressContainer.Mpb = nil
	}()
	etlcore.LogError("Finalizing mpb waitgroup.")
	if multiProgressContainer.Wg != nil {
		// multiProgressContainer.Wg.Done()
		multiProgressContainer.Wg = nil
	}
}

func SetPlugin(pluginBool bool) {
	plugin = pluginBool
}

func GetPlugin() bool {
	return plugin
}

// MultiBarInstance get function for our multibarcontainer
func MultiBarInstance() *MultiBarContainer {
	if multiProgressContainer.Mpb == nil {
		mpbContext, mpbContextCancel = context.WithCancel(context.Background())
		if IsPlugin && etlcore.GetConfigContext("ninja") != nil && etlcore.GetConfigContext("ninja").Log != nil {
			multiProgressContainer.Mpb = mpb.NewWithContext(mpbContext,
				mpb.WithWidth(64),
				mpb.WithWaitGroup(multiProgressContainer.Wg),
				mpb.WithOutput(etlcore.GetConfigContext("ninja").Log.Writer()),
				mpb.WithDebugOutput(etlcore.GetConfigContext("ninja").Log.Writer()))
		} else if IsPlugin {
			multiProgressContainer.Mpb = mpb.NewWithContext(mpbContext,
				mpb.WithWidth(64),
				mpb.WithWaitGroup(multiProgressContainer.Wg),
				mpb.WithOutput(nil),
				mpb.WithDebugOutput(io.Discard))
		} else {
			multiProgressContainer.Mpb = mpb.NewWithContext(mpbContext,
				mpb.WithWidth(64),
				mpb.WithWaitGroup(multiProgressContainer.Wg))
		}
	}
	return &multiProgressContainer
}

func GetKafkaErrorLogger() func(string, ...interface{}) {
	if !plugin {
		cc := etlcore.GetConfigContext("ninja")
		if cc != nil && cc.Log != nil {
			return cc.Log.Printf
		}
	}
	return func(string, ...interface{}) {}
}

// GetCacheKey generates a unique cache key for a reader based on topic and type
func GetCacheKey(topic string, isTestReader bool) string {
	if isTestReader {
		return topic + ":test"
	}
	return topic + ":handler"
}

// createKafkaReaderConfig creates a new kgo.Client with the given topic and optional consumer group ID
func createKafkaReaderConfig(topicName string, consumerGroupID ...string) (*kgo.Client, string, error) {
	etlProperties := confighelper.GetProperties()
	if etlProperties == nil {
		return nil, "", errors.New("etlProperties is nil")
	}

	kafkaUsername, ok := (*etlProperties)["kafka_agnostic_username"].(string)
	if !ok {
		return nil, "", errors.New("kafka_agnostic_username not found or not a string")
	}
	kafkaPassword, ok := (*etlProperties)["kafka_agnostic_password"].(string)
	if !ok {
		return nil, "", errors.New("kafka_agnostic_password not found or not a string")
	}
	bootstrapServers, ok := (*etlProperties)["bootstrapServers"].(string)
	if !ok {
		return nil, "", errors.New("bootstrapServers not found or not a string")
	}
	envStr, ok := (*etlProperties)["env"].(string)
	if !ok {
		return nil, "", errors.New("env not found or not a string")
	}

	// Setup SASL plain authentication
	mechanism := plain.Auth{
		User: kafkaUsername,
		Pass: kafkaPassword,
	}

	// Setup TLS
	caPool := x509.NewCertPool()
	if len(confighelper.KafkaCert) > 0 {
		caPool.AppendCertsFromPEM(confighelper.KafkaCert)
	}

	var tlsConfig *tls.Config
	if envStr == "dev" || envStr == "QA" {
		tlsConfig = &tls.Config{RootCAs: caPool, InsecureSkipVerify: true}
	} else {
		tlsConfig = &tls.Config{RootCAs: caPool}
	}

	// Use provided consumer group ID or generate a new one
	var groupID string
	if len(consumerGroupID) > 0 && consumerGroupID[0] != "" {
		groupID = consumerGroupID[0]
	} else {
		id, err := uuid.NewRandom()
		if err != nil {
			return nil, "", fmt.Errorf("could not generate guid: %w", err)
		}
		groupID = "spectrum-ninja-" + id.String()
	}

	// Create franz-go client with configuration
	client, err := kgo.NewClient(
		kgo.SeedBrokers(bootstrapServers),
		kgo.ConsumerGroup(groupID),
		kgo.ConsumeTopics(topicName),
		kgo.SASL(mechanism.AsMechanism()),
		kgo.DialTLSConfig(tlsConfig),
		kgo.ConsumeResetOffset(kgo.NewOffset().AtEnd()), // Start from latest
		kgo.FetchMinBytes(1),
		kgo.FetchMaxBytes(10e6),
		kgo.FetchMaxWait(1*time.Second),
		kgo.SessionTimeout(10*time.Minute),
		kgo.WithLogger(kgo.BasicLogger(io.Discard, kgo.LogLevelNone, nil)),
	)
	if err != nil {
		return nil, "", fmt.Errorf("failed to create kafka client: %w", err)
	}

	return client, groupID, nil
}

// NewKafkaTestReader -- create new kafka test reader for a topic.
func NewKafkaTestReader(topic []string, testReadyWG *sync.WaitGroup, ignoreCacheFail ...bool) (*SeededKafkaReader, error) {
	// Defensive: Validate input
	if len(topic) == 0 {
		return nil, errors.New("topic array is empty")
	}
	if topic[0] == "" {
		return nil, errors.New("topic name is empty")
	}

	r, err := newKafkaReaderInternal(topic, true, testReadyWG, ignoreCacheFail...)
	if r != nil && r.kafkaTestBundle == nil && r.deleteTestChan == nil && r.incomingTestChan == nil {
		r.kafkaTestBundle = map[string]*KafkaTestBundle{}
		r.deleteTestChan = make(chan *KafkaTestBundle, 20)
		r.incomingTestChan = make(chan *KafkaTestBundle, 3)
	}
	return r, err
}

// NewKafkaReader -- create new kafka reader for a topic.
func NewKafkaReader(topic []string, ignoreCacheFail ...bool) (*SeededKafkaReader, error) {
	flowTopicsClosed = false
	return newKafkaReaderInternal(topic, false, nil, ignoreCacheFail...)
}

// newKafkaReaderInternal -- internal function to create kafka readers with specific cache keys
func newKafkaReaderInternal(topic []string, isTestReader bool, testReadyWG *sync.WaitGroup, ignoreCacheFail ...bool) (*SeededKafkaReader, error) {
	// Defensive: Validate input
	if len(topic) == 0 {
		return nil, errors.New("topic array is empty")
	}
	if topic[0] == "" {
		return nil, errors.New("topic name is empty")
	}

	multiBarLock.Lock()
	MultiBarInstance()
	multiBarLock.Unlock()

	cacheKey := GetCacheKey(topic[0], isTestReader)
	if cached, ok := kafkaReaderCache.Load(cacheKey); ok {
		reader := cached.(*SeededKafkaReader)
		if !reader.isReaderClosing() {
			// Reader exists and is healthy - return it
			return reader, nil
		}
		// Reader exists but is closing - delete it and create a new one
		kafkaReaderCache.Delete(reader.CacheKey)
	}

	// No valid cached reader - check if we're allowed to create a new one
	if ignoreCacheFail == nil || !ignoreCacheFail[0] {
		etlcore.LogError("Critical failure.  Attempt to use unregistered reader.")
		return nil, errors.New("Critical failure.  Attempt to use unregistered reader for topic: " + topic[0])
	}

	// Defensive: Ensure topic has at least 2 elements for TopicType
	topicType := "json" // default
	if len(topic) > 1 && topic[1] != "" {
		topicType = topic[1]
	}

	// Create kafka reader using shared helper
	r, groupID, err := createKafkaReaderConfig(topic[0])
	if err != nil {
		return nil, err
	}

	etlcore.LogError(fmt.Sprintf("Starting reader on topic: %s with content type: %s\n", topic[0], topicType))

	startTime := time.Now()

	reader := &SeededKafkaReader{
		startTime:            startTime,
		TopicName:            topic[0],
		TopicType:            topicType,
		ConsumerGroupID:      groupID,
		kafkaClient:          r,
		firstRecordCommitted: false,
		isTestReader:         isTestReader,
		CacheKey:             cacheKey,
	}

	// Increment wait group when creating a new reader
	if isTestReader && testReadyWG != nil {
		testReadyWG.Add(1)
	}

	kafkaReaderCache.Store(cacheKey, reader)
	return reader, nil
}

func StartAllTestEngines(kafkaErrChan chan KafkaErrMessage, testReadyWG *sync.WaitGroup) {
	kafkaReaderCache.Range(func(key, value interface{}) bool {
		reader := value.(*SeededKafkaReader)
		if reader.isTestReader {
			go reader.KafkaTestEngine(kafkaErrChan, testReadyWG)
		}
		return true
	})
}

func StartAllFlowTopicEngines(kafkaErrChan chan KafkaErrMessage) {
	kafkaReaderCache.Range(func(key, value interface{}) bool {
		reader := value.(*SeededKafkaReader)
		if !reader.isTestReader {
			go reader.KafkaTestEngine(kafkaErrChan, nil)
		}
		return true
	})
}

func CloseAllTests() {
	kafkaReaderCache.Range(func(key, value interface{}) bool {
		reader := value.(*SeededKafkaReader)
		if reader.isTestReader {
			reader.ninjaTestClosed = true
		}
		if len(reader.kafkaTestBundle) > 0 {
			for _, testBundle := range reader.kafkaTestBundle {
				if testBundle != nil {
					//				testBundle.Wg.Done()
					reader.DeleteTest(testBundle)
				}
			}
		}
		return true
	})
}

func CloseAllTestEngines() {
	etlcore.LogError("Closing connections...")

	// Wrap in recovery to ensure cleanup continues even if panic occurs
	defer func() {
		if r := recover(); r != nil {
			etlcore.LogError(fmt.Sprintf("Panic during CloseAllEngines: %v", r))
		}
	}()

	if openConnCacheLock != nil {
		openConnCacheLock.Lock()
		for _, conn := range openConnectionCache {
			if conn != nil {
				func() {
					defer func() {
						if r := recover(); r != nil {
							etlcore.LogError(fmt.Sprintf("Panic closing connection: %v", r))
						}
					}()
					conn.Close()
				}()
			}
		}
		openConnCacheLock.Unlock()
	}
	etlcore.LogError("Connections closed...")
	etlcore.LogError("Closing readers...")

	// Close readers and wait for their engine loops to fully exit
	kafkaReaderCache.Range(func(key, value interface{}) bool {
		reader := value.(*SeededKafkaReader)
		if reader != nil && reader.isTestReader {
			reader.ninjaTestClosed = true
			reader.Close()
			reader.engineRunning.Wait() // Block until this reader's message loop exits
		}
		return true
	})

	etlcore.LogError("Readers closed...")
}

func CloseFlowTopicEngines(topicReaderKeys ...string) {
	flowTopicsClosed = true
	etlcore.LogError("Closing Flow Topic connections...")

	// Wrap in recovery to ensure cleanup continues even if panic occurs
	defer func() {
		if r := recover(); r != nil {
			etlcore.LogError(fmt.Sprintf("Panic during CloseFlowTopicEngines: %v", r))
		}
	}()

	if len(topicReaderKeys) == 0 && openConnCacheLock != nil {
		openConnCacheLock.Lock()
		for _, conn := range openConnectionCache {
			if conn != nil {
				func() {
					defer func() {
						if r := recover(); r != nil {
							etlcore.LogError(fmt.Sprintf("Panic closing flow topic connection: %v", r))
						}
					}()
					conn.Close()
				}()
			}
		}
		openConnCacheLock.Unlock()
		etlcore.LogError("Flow Topic Connections closed...")
	}
	etlcore.LogError("Closing Flow Topic readers...")
	if len(topicReaderKeys) > 0 {
		for _, topicReaderKey := range topicReaderKeys {
			if cached, ok := kafkaReaderCache.Load(topicReaderKey); ok {
				reader := cached.(*SeededKafkaReader)
				if reader != nil && !reader.isTestReader {
					reader.Close()
				}
			}
		}

		etlcore.LogError(fmt.Sprintf("Flow Topic readers closed: %d", len(topicReaderKeys)))
	} else {
		// If none specified, assume it's all of them.
		kafkaReaderCache.Range(func(key, value interface{}) bool {
			reader := value.(*SeededKafkaReader)
			if reader != nil && !reader.isTestReader {
				reader.Close()
				reader.engineRunning.Wait() // Block until this reader's message loop exits
			}
			return true
		})

		etlcore.LogError("All flow Topic readers closed...")
	}
}

// PreSeed -- Sets up a waitgroup and kicks off async testhelper.
func (r *SeededKafkaReader) PreSeed() {
	ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(10*time.Second))
	defer cancel()

	for {
		etlcore.LogError("Beginning kafka group setup.")
		fetches := r.kafkaClient.PollFetches(ctx)
		if fetches.IsClientClosed() {
			etlcore.LogError("Kafka client closed during PreSeed")
			return
		}
		if err := fetches.Err(); err != nil {
			etlcore.LogError(fmt.Sprintf("Failure to parse change order item addon.  Error: %v", err))
			continue
		}

		iter := fetches.RecordIter()
		for !iter.Done() {
			rec := iter.Next()
			if rec.Timestamp.Before(r.startTime) {
				// Ignore messages before our start time.
				continue
			} else {
				// Found a message after start time - position established
				etlcore.LogError("Ended kafka group setup.")
				return
			}
		}
	}
}

// RegisterTest - registers an expected test with the underlying kafka test engine.
func (r *SeededKafkaReader) RegisterTest(testName string) {
	if r == nil {
		etlcore.LogError("Cannot register test on nil reader")
		return
	}
	if testName == "" {
		etlcore.LogError("Cannot register test with empty name")
		return
	}
	r.kafkaTestBundleLock.Lock()
	if r.kafkaTestBundle == nil {
		r.kafkaTestBundle = make(map[string]*KafkaTestBundle)
	}
	r.kafkaTestBundle[testName] = nil
	r.kafkaTestBundleLock.Unlock()
}

// DeleteTest -- remove test bundle.
func (r *SeededKafkaReader) DeleteTest(incomingTest *KafkaTestBundle) {
	if r == nil {
		etlcore.LogError("Cannot delete test on nil reader")
		return
	}
	if r.deleteTestChan == nil {
		etlcore.LogError("deleteTestChan is nil")
		return
	}
	if incomingTest == nil {
		etlcore.LogError("Cannot delete nil test bundle")
		return
	}
	r.deleteTestChan <- incomingTest
}

// CountTest -- returns count of number of expected values tests.
func (r *SeededKafkaReader) CountTest() int {
	var expectedValuesCnt int
	r.kafkaTestBundleLock.RLock()
	expectedValuesCnt = len(r.kafkaTestBundle)
	r.kafkaTestBundleLock.RUnlock()
	return expectedValuesCnt
}

func (r *SeededKafkaReader) HasEmptyTest(wg *sync.WaitGroup) bool {
	hasEmpty := false

	r.kafkaTestBundleLock.RLock()
	for _, v := range r.kafkaTestBundle {
		if v == nil {
			hasEmpty = true
			break
		}
	}

	r.kafkaTestBundleLock.RUnlock()

	if !hasEmpty {
		if wg != nil {
			wg.Done()
		}
	}
	return hasEmpty
}

func (r *SeededKafkaReader) ScanTests(wg *sync.WaitGroup) {
	for {
		hasEmpty := false
		timeout := make(chan bool, 1)
		go func() {
			time.Sleep(1 * time.Second)
			timeout <- true
		}()

		select {
		case deleteTest := <-r.deleteTestChan:

			r.kafkaTestBundleLock.Lock()
			if deleteTest != nil {
				delete(r.kafkaTestBundle, deleteTest.Name)
			}

			r.kafkaTestBundleLock.Unlock()
			hasEmpty = r.HasEmptyTest(wg)
		case incomingTest := <-r.incomingTestChan:

			r.kafkaTestBundleLock.Lock()
			r.kafkaTestBundle[incomingTest.Name] = incomingTest

			r.kafkaTestBundleLock.Unlock()
			hasEmpty = r.HasEmptyTest(wg)
		case <-timeout:
			if r.ninjaTestClosed && (r == nil || r.isTestReader) {
				if wg != nil {
					wg.Done()
				}
				return
			}
			hasEmpty = r.HasEmptyTest(wg)
		}
		if !hasEmpty {
			wg = nil
		}
	}
}

func (r *SeededKafkaReader) ProcessMessage(m *kgo.Record) {
	if r == nil {
		etlcore.LogError("Cannot process message on nil reader")
		return
	}
	if m == nil {
		etlcore.LogError("Cannot process nil message")
		return
	}

	// Wrap in recovery to prevent panic from crashing the entire engine
	defer func() {
		if rec := recover(); rec != nil {
			etlcore.LogError(fmt.Sprintf("Panic in ProcessMessage for topic %s: %v", r.TopicName, rec))
		}
	}()

	switch r.TopicType {
	case "avro":
		r.ProcessMessageAvro(m)
	case "json":
		fallthrough
	default:
		r.ProcessMessageJSON(m)
	}
}

func (r *SeededKafkaReader) isReaderClosing() bool {
	return (r.ninjaTestClosed && r.isTestReader) || (!r.isTestReader && flowTopicsClosed)
}

// recreateKafkaReader recreates the underlying kgo.Client connection
func (r *SeededKafkaReader) recreateKafkaReader() error {
	if r == nil {
		return errors.New("cannot recreate reader on nil SeededKafkaReader")
	}

	// Close old client if it exists
	if r.kafkaClient != nil {
		func() {
			defer func() {
				if rec := recover(); rec != nil {
					etlcore.LogError(fmt.Sprintf("Panic closing old kafka client: %v", rec))
				}
			}()
			r.kafkaClient.Close()
		}()
	}

	// Create new client using shared helper with the same consumer group ID
	newClient, _, err := createKafkaReaderConfig(r.TopicName, r.ConsumerGroupID)
	if err != nil {
		return fmt.Errorf("failed to create kafka client config: %w", err)
	}

	r.kafkaClient = newClient
	r.firstRecordCommitted = false // Reset so we commit after first fetch on reconnect
	etlcore.LogError(fmt.Sprintf("Successfully recreated kafka client for topic: %s with group ID: %s", r.TopicName, r.ConsumerGroupID))
	return nil
}

func (r *SeededKafkaReader) KafkaTestEngine(kafkaErrChan chan KafkaErrMessage, testReadyWG *sync.WaitGroup) {
	// Signal that the engine loop is running
	r.engineRunning.Add(1)
	defer r.engineRunning.Done() // Signal completion when loop exits

	var wg sync.WaitGroup
	wg.Add(1)

	//
	// Block until all tests are set up.
	//
	go r.ScanTests(&wg)

	wg.Wait()

	etlcore.LogError("All tests have registered.  Engine starting to read from kafka.")

	// Signal that this engine is ready to process messages
	if r.isTestReader && testReadyWG != nil {
		testReadyWG.Done()
		etlcore.LogError("Test reader signaled engine ready.")
	}

	// All tests loaded and ready to go.
	for {
		// After a test completes, if there are no more tests, then close the reader and exit.
		if r.isTestReader && r.CountTest() == 0 {
			// NOTE: May gobble an extra message here, but that is o.k.
			// All done.  Don't process messages further.
			etlcore.LogError("All tests completed.  Closing reader, exiting reader loop.")
			defer r.Close()
			break
		}

		// Check if we're closing
		if r.isReaderClosing() {
			r.Close()
			etlcore.LogError("Reader is closed, exiting reader loop.")
			break
		}

		// Poll for new messages using franz-go
		ctx := context.Background()
		fetches := r.kafkaClient.PollFetches(ctx)

		// Check if client was closed using franz-go's built-in method
		if fetches.IsClientClosed() && !r.isReaderClosing() {
			etlcore.LogError("Kafka client closed")
			if kafkaErrChan != nil {
				kafkaErrChan <- KafkaErrMessage{
					TopicKey:   r.CacheKey,
					KafkaError: errors.New("Kafka client closed"),
				}
			}
			defer r.Close()
			break
		}

		// Check for errors in fetches
		if err := fetches.Err(); err != nil {
			// Check for authorization errors - these are fatal and should not retry
			errStr := err.Error()
			if strings.Contains(errStr, "TOPIC_AUTHORIZATION_FAILED") ||
				strings.Contains(errStr, "Not authorized to access topics") {
				etlcore.LogError(fmt.Sprintf("Authorization error for topic %s: %v - failing permanently", r.TopicName, err))
				if kafkaErrChan != nil {
					kafkaErrChan <- KafkaErrMessage{
						TopicKey:   r.CacheKey,
						KafkaError: errors.New("authorization error"),
					}
				}
				defer r.Close()
				break
			}

			// Check if this is the ErrClientClosed error
			if errors.Is(err, kgo.ErrClientClosed) {
				if !r.isTestReader || (r.isTestReader && r.CountTest() > 0) {
					etlcore.LogError("Kafka client closed error")
				}
				if kafkaErrChan != nil {
					kafkaErrChan <- KafkaErrMessage{
						TopicKey:   r.CacheKey,
						KafkaError: errors.New("Kafka client closed error"),
					}
				}
				defer r.Close()
				break
			}

			// Check if context was canceled
			if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
				etlcore.LogError(fmt.Sprintf("Context error: %v", err))
				if r.isReaderClosing() {
					defer r.Close()
					break
				}
				continue
			}

			// Check for transient network errors using standard Go error types
			var netErr *net.OpError
			var syscallErr syscall.Errno
			isTransientError := errors.Is(err, io.EOF) ||
				errors.Is(err, io.ErrUnexpectedEOF) ||
				errors.Is(err, syscall.EPIPE) ||
				errors.Is(err, syscall.ECONNRESET) ||
				(errors.As(err, &netErr) && netErr.Temporary()) ||
				(errors.As(err, &syscallErr) && (syscallErr == syscall.EPIPE || syscallErr == syscall.ECONNRESET))

			if isTransientError {
				// Check if we're intentionally shutting down
				if r.isReaderClosing() {
					// Intentional shutdown - exit gracefully
					etlcore.LogError("Kafka connection closed during shutdown, exiting reader loop gracefully")
					defer r.Close()
					break
				} else {
					// Unexpected connection loss - attempt to recreate
					etlcore.LogError(fmt.Sprintf("Transient network error (%v), attempting to recreate client", err))
					if recreateErr := r.recreateKafkaReader(); recreateErr != nil {
						etlcore.LogError(fmt.Sprintf("Failed to recreate kafka client: %v", recreateErr))
						defer r.Close()
						break
					}
					// Add a short delay before retrying
					time.Sleep(2 * time.Second)
					continue
				}
			}

			// Check for non-recoverable infrastructure errors using standard Go error types
			isNonRecoverable := errors.Is(err, syscall.ECONNREFUSED) ||
				errors.Is(err, syscall.EHOSTUNREACH) ||
				errors.Is(err, syscall.ENETUNREACH) ||
				errors.Is(err, syscall.ETIMEDOUT) ||
				(errors.As(err, &netErr) && (netErr.Timeout() || netErr.Err == syscall.ECONNREFUSED))

			if isNonRecoverable {
				etlcore.LogError(fmt.Sprintf("Non-recoverable network error: %v", err))
				if kafkaErrChan != nil {
					kafkaErrChan <- KafkaErrMessage{
						TopicKey:   r.CacheKey,
						KafkaError: err,
					}
				}
				defer r.Close()
				break
			}

			// Other errors - log and retry (could be kafka protocol errors)
			etlcore.LogError(fmt.Sprintf("Kafka error, retrying: %v", err))
			continue
		}

		// If this is the first successful fetch, commit immediately to establish position
		if !r.firstRecordCommitted {
			commitCtx, commitCancel := context.WithTimeout(context.Background(), 5*time.Second)
			if err := r.kafkaClient.CommitUncommittedOffsets(commitCtx); err != nil {
				etlcore.LogError(fmt.Sprintf("First fetch commit failed: %v", err))
			} else {
				r.firstRecordCommitted = true
				etlcore.LogError("Successfully committed position after first fetch")
			}
			commitCancel()
			etlcore.LogError("First fetch processing complete.")
		}

		// Process all records in the fetch
		iter := fetches.RecordIter()
		for !iter.Done() {
			rec := iter.Next()

			if !plugin {
				etlcore.LogError(fmt.Sprintf("message at topic/partition/offset %v/%v/%v: %s = %s\n",
					rec.Topic, rec.Partition, rec.Offset, string(rec.Key), string(rec.Value)))
			}

			if (!r.ninjaTestClosed && r.isTestReader) || (!flowTopicsClosed && !r.isTestReader) {
				go r.ProcessMessage(rec)
			}
		}
	}
}

// Close -- closes reader and cleans up.
func (r *SeededKafkaReader) Close() {
	if r == nil {
		etlcore.LogError("Cannot close nil reader")
		return
	}

	// Remove from cache so a new reader can be created on next use
	kafkaReaderCache.Delete(r.CacheKey)

	if r.kafkaClient != nil {
		// Wrap in recovery in case close panics
		func() {
			defer func() {
				if rec := recover(); rec != nil {
					etlcore.LogError(fmt.Sprintf("Panic closing kafka client: %v", rec))
				}
			}()
			r.kafkaClient.LeaveGroup()
			r.kafkaClient.Close()
		}()
	}
}

func TestSequenceExpected(enterpriseID string, readerSequence []*SeededKafkaReader, kafkaTestSequence []*KafkaTestBundle, testReadyWG *sync.WaitGroup) {
	if !plugin {
		etlcore.LogError(fmt.Sprintf("%s Going to kafka.", enterpriseID))
	}
	for i, reader := range readerSequence {
		testExpected(reader, kafkaTestSequence[i])
	}
	// Wait for all tests to be ready
	if testReadyWG != nil {
		testReadyWG.Wait()
		etlcore.LogError("All test engines are ready.")
	}
}

// testExpected -- Sets up a waitgroup and kicks off async testhelper.
func testExpected(r *SeededKafkaReader, kafkaTestBundle *KafkaTestBundle) {
	pc := make([]uintptr, 4)
	runtime.Callers(3, pc)
	testNameFunc := runtime.FuncForPC(pc[0])
	funcParts := strings.Split(testNameFunc.Name(), ".")
	testName := funcParts[len(funcParts)-1]
	testName = strings.Replace(testName, "clean", "", 1)

	kafkaTestBundle.Wg = &sync.WaitGroup{}
	kafkaTestBundle.Wg.Add(1)

	r.kafkaTestBundleLock.Lock()
	if _, hasKey := r.kafkaTestBundle[testName]; !hasKey {
		kafkaTestBundle.SuccessFun(fmt.Errorf("invalid and unregisterd test, check testname: %s", testName))
		kafkaTestBundle.Wg.Done()
		r.kafkaTestBundleLock.Unlock()
		return
	}
	r.kafkaTestBundleLock.Unlock()

	// Override the name.  It should always be based on test function name
	// or registration and setup can fail.
	kafkaTestBundle.Name = testName

	// Cleanup expected values....
	// Because aggregator does this for Databricks.
	for k, v := range kafkaTestBundle.ExpectedLogicalKey {
		kafkaTestBundle.ExpectedLogicalKey[k] = strings.TrimSpace(v.(string))
	}

	for ek, ev := range kafkaTestBundle.ExpectedValue {
		if _, isString := ev.(string); isString {
			kafkaTestBundle.ExpectedValue[ek] = strings.TrimSpace(ev.(string))
		}
	}

	testBundleFun := kafkaTestBundle.SuccessFun
	kafkaTestBundle.SuccessFun = func(err error) {
		if testBundleFun != nil {
			testBundleFun(err)
		}
		kafkaTestBundle.Wg.Done()
	}

	go func() {
		resultErr := TestExpectedHelper(r, kafkaTestBundle)

		if resultErr != nil {
			kafkaTestBundle.SuccessFun(errors.New("test setup failure"))
			kafkaTestBundle.Wg.Done()
		}
	}()
}

var openConnCacheLock *sync.Mutex

// KafkaTestInit - obtains mpb, kafka reader, and spectrum connection for use in kafka testing.
func KafkaTestInit(argosID string,
	configContext *core.ConfigContext,
	currentState *atomic.Value,
	kafkaTopicSequence [][]string,
	currentStateFunc decor.DecorFunc,
	stateMap map[string]interface{},
	start time.Time,
	testReadyWG *sync.WaitGroup,
) ([]*SeededKafkaReader, *mpb.Bar, string, string, *sql.DB, error) {
	etlcore.LogError("KafkaTestInit")
	if stateMap == nil {
		stateMap = make(map[string]interface{})
	}
	pc := make([]uintptr, 3)
	runtime.Callers(2, pc)
	testNameFunc := runtime.FuncForPC(pc[0])
	funcParts := strings.Split(testNameFunc.Name(), ".")
	testName := funcParts[len(funcParts)-1]
	testName = strings.Replace(testName, "clean", "", 1)

	testName = fmt.Sprintf("%33s", testName)
	etlcore.LogError(fmt.Sprintf("KafkaTestInit setting up mpb for: %s\n", testName))

	multiBarLock.Lock()
	multibar := MultiBarInstance()

	bar := multibar.Mpb.AddBar(int64(100),
		mpb.PrependDecorators(
			decor.Name(testName, decor.WCSyncSpace),
			decor.Name(" "),
			decor.Any(currentStateFunc),
			decor.Elapsed(decor.ET_STYLE_MMSS, decor.WCSyncSpace),
		),
	)
	multiBarLock.Unlock()
	var reader *SeededKafkaReader = nil
	var err error
	var readerSequence []*SeededKafkaReader

	for _, kafkaTopic := range kafkaTopicSequence {
		if kafkaTopic[0] != "" {
			reader, err = NewKafkaTestReader(kafkaTopic, testReadyWG)
			if err != nil {
				(*currentState).Store(STATE_FAILED_KAFKA_CONN)
				stateMap[currentState.Load().(string)] = time.Since(start)
				bar.Abort(plugin)
				return nil, nil, "", "", nil, err
			}
			readerSequence = append(readerSequence, reader)
		}
	}

	if len(kafkaTopicSequence) > 0 {
		(*currentState).Store(STATE_KAFKA_INIT)
		stateMap[currentState.Load().(string)] = time.Since(start)
	} else {
		(*currentState).Store(STATE_DBCONFIG)
		stateMap[currentState.Load().(string)] = time.Since(start)
	}

	// 0. Setup connections to database and kafka.
	etlcore.LogError("KafkaTestInit obtaining db connections...\n")

	if IndirectDbFunc != nil {
		argosIDIndirect, region, spectrumConn, err := IndirectDbFunc(configContext, argosID)
		if err != nil {
			(*currentState).Store(STATE_FAILED_DB_CONN)
			stateMap[currentState.Load().(string)] = time.Since(start)
			bar.Abort(plugin)
			return nil, bar, "", "", nil, err
		}
		etlcore.LogError("KafkaTestInit indirect db conn obtained.  Obtaining direct connection.")

		bar.IncrBy(25)
		return readerSequence, bar, argosIDIndirect, region, spectrumConn, nil
	}
	error := errors.New("sqlType must be either direct or indirect")
	etlcore.LogError("KafkaTestInit complete")

	return nil, nil, "", "", nil, error
}

func TestWait(currentState *atomic.Value, kafkaTestSequence []*KafkaTestBundle, bar *mpb.Bar, resultError error, stateMap map[string]interface{}, start time.Time, stateLock *sync.Mutex) {
	if stateMap == nil {
		stateMap = make(map[string]interface{})
	}
	(*currentState).Store(STATE_DBUPDATED)
	stateLock.Lock()
	stateMap[currentState.Load().(string)] = time.Since(start)
	stateLock.Unlock()
	bar.IncrBy(25)
	for _, kafkaTest := range kafkaTestSequence {
		if kafkaTest.Wg == nil {
			kafkaTest.Wg = &sync.WaitGroup{}
		}
		kafkaTest.Wg.Wait()
		// Check if all test readers are closed
		allTestsClosed := true
		kafkaReaderCache.Range(func(key, value interface{}) bool {
			reader := value.(*SeededKafkaReader)
			if reader != nil && reader.isTestReader && !reader.ninjaTestClosed {
				allTestsClosed = false
				return false // stop iteration - found an open reader
			}
			return true
		})
		if allTestsClosed {
			resultError = errors.New("timeout signal sent")
		}
		if resultError != nil {
			bar.IncrBy(50)
			(*currentState).Store(STATE_FAILED)
			stateLock.Lock()
			stateMap[currentState.Load().(string)] = time.Since(start)
			stateLock.Unlock()
			bar.Abort(plugin)
			break
		}
		bar.IncrBy(5)
		(*currentState).Store(kafkaTest.CompletionStatus)
		stateLock.Lock()
		stateMap[currentState.Load().(string)] = time.Since(start)
		stateLock.Unlock()
	}
	bar.IncrBy(20)

	if resultError != nil {
		bar.IncrBy(50)
		(*currentState).Store(STATE_FAILED)
		stateLock.Lock()
		stateMap[currentState.Load().(string)] = time.Since(start)
		stateLock.Unlock()
	} else {
		bar.IncrBy(50)
		(*currentState).Store(STATE_COMPLETE)
		stateLock.Lock()
		stateMap[currentState.Load().(string)] = time.Since(start)
		stateLock.Unlock()
	}
	time.Sleep(100 * time.Millisecond)
}

// TestExpectedHelper -- reads from kafka waiting for expectedKey and expected value index.  When it finds it, it returns whether expectedValue matched or not.
func TestExpectedHelper(r *SeededKafkaReader, kafkaTestBundle *KafkaTestBundle) error {
	r.incomingTestChan <- kafkaTestBundle
	return nil
}
