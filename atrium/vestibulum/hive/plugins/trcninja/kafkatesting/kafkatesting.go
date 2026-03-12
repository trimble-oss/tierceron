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
	kafkaClientLock      sync.RWMutex // protects kafkaClient from concurrent modification
	firstRecordCommitted atomic.Bool  // tracks if we've committed after first fetch; replaces plain bool
	kafkaTestBundle      map[string]*KafkaTestBundle
	kafkaTestBundleLock  sync.RWMutex
	incomingTestChan     chan *KafkaTestBundle
	deleteTestChan       chan *KafkaTestBundle
	HandleEventFunc      func(k map[string]any, n map[string]any)
	isTestReader         atomic.Bool    // tracks if this is a test reader or event handler; converted to atomic for thread safety
	ninjaTestClosed      atomic.Bool    // tracks if this reader's ninja test has been closed; replaces plain bool
	channelsClosed       atomic.Bool    // tracks if incomingTestChan and deleteTestChan have been closed
	flowTopicsClosed     atomic.Bool    // per-reader flag tracks if this flow topic reader should close
	CacheKey             string         // cached key for reader lookup
	engineActive         atomic.Bool    // tracks if an engine is currently running on this reader; uses CompareAndSwap for atomic activation
	engineRunning        sync.WaitGroup // tracks if the KafkaTestEngine loop is still running
	messageProcessingWg  sync.WaitGroup // tracks in-flight ProcessMessage goroutines to prevent resource leak
	testReadySignaled    atomic.Bool    // tracks if testReadyWG.Done() already called to prevent double-done panic
	// Quota throttling tracking - for graceful backoff when quota exhausted
	quotaErrorCount int32 // counts consecutive quota errors
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
	plugin           atomic.Bool // tracks if we're running in plugin mode; replaces plain bool
	flowTopicsClosed atomic.Bool // tracks if all flow topic readers should close; replaces plain bool
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
	plugin.Store(pluginBool)
}

func GetPlugin() bool {
	return plugin.Load()
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
	if !GetPlugin() {
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

	tlsConfig := &tls.Config{RootCAs: caPool}

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

	// NOTE: Channels are now initialized in newKafkaReaderInternal before returning,
	// eliminating the race condition that existed when channels were nil after return.
	r, err := newKafkaReaderInternal(topic, true, testReadyWG, ignoreCacheFail...)
	return r, err
}

// NewKafkaReader -- create new kafka reader for a topic.
func NewKafkaReader(topic []string, ignoreCacheFail ...bool) (*SeededKafkaReader, error) {
	flowTopicsClosed.Store(false)
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
		// For non-test (handler) readers, don't check global flowTopicsClosed during cache reuse.
		// The global flag is a signal to stop reading at engine level, not a cache invalidation.
		// Handlers should remain reusable as long as they're not explicitly closed.
		var canReuse bool
		if isTestReader {
			// Test readers: use standard closing check (ninjaTestClosed)
			canReuse = !reader.isReaderClosing() && !reader.engineActive.Load()
		} else {
			// Non-test (handler) readers: only check if actually closed, not global flowTopicsClosed
			canReuse = !reader.channelsClosed.Load() && !reader.engineActive.Load()
		}

		if canReuse {
			// Reader exists, is healthy, and no engine is currently running - safe to reuse
			// NOTE: Do NOT call testReadyWG.Add() here. testReadyWG is only for initial reader setup.
			// Once initial readers are created and engines start, testReadyWG is "spent" and
			// should not be modified. Reuse happens after initial setup phase.

			// Channels might be closed from a previous test - reopen them for sequential reuse
			if reader.channelsClosed.Load() {
				reader.incomingTestChan = make(chan *KafkaTestBundle, 3)
				reader.deleteTestChan = make(chan *KafkaTestBundle, 20)
				reader.channelsClosed.Store(false)
			}

			// DO NOT clear kafkaTestBundle map here - it contains RegisterTest entries
			// that were populated during the Init phase and need to survive until testExpected runs.
			// Only the individual bundle entries (non-nil values) get deleted when tests complete.
			// IMPORTANT: The RegisterTest(testName) entry (with nil value) MUST persist across
			// the Init→Test transition, even on cache reuse.

			// Reset state flags for fresh test
			reader.testReadySignaled.Store(false)
			reader.engineActive.Store(false)
			reader.ninjaTestClosed.Store(false)

			return reader, nil
		}
		// Reader exists but is closing or has an active engine - don't reuse, create new one instead
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
		startTime:       startTime,
		TopicName:       topic[0],
		TopicType:       topicType,
		ConsumerGroupID: groupID,
		kafkaClient:     r,
		CacheKey:        cacheKey,
	}

	// Initialize atomic fields
	reader.isTestReader.Store(isTestReader)
	reader.channelsClosed.Store(false)
	reader.flowTopicsClosed.Store(false) // initialize per-reader flag

	// Initialize channels BEFORE storing in cache to prevent race condition
	// (moved from NewKafkaTestReader to prevent nil channel access)
	reader.kafkaTestBundleLock = sync.RWMutex{}
	reader.kafkaTestBundle = make(map[string]*KafkaTestBundle)
	reader.incomingTestChan = make(chan *KafkaTestBundle, 3)
	reader.deleteTestChan = make(chan *KafkaTestBundle, 20)

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
		if reader.isTestReader.Load() {
			go reader.KafkaTestEngine(kafkaErrChan, testReadyWG)
		}
		return true
	})
}

func StartAllFlowTopicEngines(kafkaErrChan chan KafkaErrMessage) {
	kafkaReaderCache.Range(func(key, value interface{}) bool {
		reader := value.(*SeededKafkaReader)
		if !reader.isTestReader.Load() {
			go reader.KafkaTestEngine(kafkaErrChan, nil)
		}
		return true
	})
}

func CloseAllTests() {
	kafkaReaderCache.Range(func(key, value interface{}) bool {
		reader := value.(*SeededKafkaReader)
		if reader.isTestReader.Load() {
			reader.ninjaTestClosed.Store(true)
		}
		if len(reader.kafkaTestBundle) > 0 {
			reader.kafkaTestBundleLock.Lock()
			for _, testBundle := range reader.kafkaTestBundle {
				if testBundle != nil {
					//				testBundle.Wg.Done()
					reader.DeleteTest(testBundle)
				}
			}
			reader.kafkaTestBundleLock.Unlock()
		}
		return true
	})
}

// CloseAllTestEngines closes all test reader engines and waits for them to finish.
// DEPRECATED: Use CloseAllEngines() instead for complete cleanup including flow topics.
func CloseAllTestEngines() {
	CloseAllEngines()
}

// CloseAllEngines closes all reader engines (both test and flow topic) and waits for them to finish.
func CloseAllEngines() {
	etlcore.LogError("Closing all connections...")

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
	etlcore.LogError("All connections closed...")
	etlcore.LogError("Closing all readers (test and flow topic)...")

	// Close all readers (both test and flow topic) and wait for their engine loops to fully exit
	kafkaReaderCache.Range(func(key, value interface{}) bool {
		reader := value.(*SeededKafkaReader)
		if reader != nil && reader.isTestReader.Load() {
			// Mark channels as closed first to prevent new sends
			reader.channelsClosed.Store(true)

			// Mark both test and flow topic readers as closing
			if reader.isTestReader.Load() {
				reader.ninjaTestClosed.Store(true)
			} else {
				// flowTopicsClosed.Store(true)
			}

			// Close channels to signal ScanTests to exit
			if reader.incomingTestChan != nil {
				close(reader.incomingTestChan)
			}
			if reader.deleteTestChan != nil {
				close(reader.deleteTestChan)
			}

			reader.Close()
			reader.engineRunning.Wait() // Block until this reader's message loop exits
		}
		return true
	})

	etlcore.LogError("All readers closed...")
}

func CloseFlowTopicEngines(topicReaderKeys ...string) {
	flowTopicsClosed.Store(true)
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
				if reader != nil && !reader.isTestReader.Load() {
					reader.Close()
				}
			}
		}

		etlcore.LogError(fmt.Sprintf("Flow Topic readers closed: %d", len(topicReaderKeys)))
	} else {
		// If none specified, assume it's all of them.
		kafkaReaderCache.Range(func(key, value interface{}) bool {
			reader := value.(*SeededKafkaReader)
			if reader != nil && !reader.isTestReader.Load() {
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
	ctx := context.Background()

	for {
		etlcore.LogError("Beginning kafka group setup.")
		r.kafkaClientLock.RLock()
		kafkaClient := r.kafkaClient
		r.kafkaClientLock.RUnlock()

		if kafkaClient == nil {
			etlcore.LogError("Kafka client is nil during PreSeed")
			return
		}

		fetches := kafkaClient.PollFetches(ctx)
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
	// Store nil placeholder - testExpected will replace with real bundle
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
	// Check if channels closed before sending to prevent panic
	if r.channelsClosed.Load() {
		etlcore.LogError("deleteTestChan is closed, cannot delete test")
		return
	}
	// Wrap in panic recovery to handle race where channels close between check and send
	func() {
		defer func() {
			if rec := recover(); rec != nil {
				// Only log if not shutting down to avoid clutter during cleanup
				if !r.channelsClosed.Load() {
					etlcore.LogError(fmt.Sprintf("Panic sending to deleteTestChan: %v", rec))
				}
			}
		}()
		r.deleteTestChan <- incomingTest
	}()
}

// CountTest -- returns count of number of expected values tests.
func (r *SeededKafkaReader) CountTest() int {
	if r == nil {
		return 0
	}
	var expectedValuesCnt int
	r.kafkaTestBundleLock.RLock()
	expectedValuesCnt = len(r.kafkaTestBundle)
	r.kafkaTestBundleLock.RUnlock()
	return expectedValuesCnt
}

func (r *SeededKafkaReader) HasEmptyTest(wg *sync.WaitGroup) bool {
	if r == nil {
		return false
	}
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

		// No timeout - let the test framework control execution timing
		select {
		case deleteTest := <-r.deleteTestChan:
			if deleteTest == nil {
				// Channel was closed, exit ScanTests
				if wg != nil {
					wg.Done()
				}
				return
			}
			r.kafkaTestBundleLock.Lock()
			delete(r.kafkaTestBundle, deleteTest.Name) // Delete by unique key
			r.kafkaTestBundleLock.Unlock()
			hasEmpty = r.HasEmptyTest(wg)
		case incomingTest := <-r.incomingTestChan:
			if incomingTest == nil {
				// Channel was closed, exit ScanTests
				if wg != nil {
					wg.Done()
				}
				return
			}
			// Store the real bundle under its unique key
			r.kafkaTestBundleLock.Lock()
			r.kafkaTestBundle[incomingTest.Name] = incomingTest
			r.kafkaTestBundleLock.Unlock()
			// Check if all tests are now registered (all non-nil)
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
	if r == nil {
		return true // Treat nil reader as closing (safer default)
	}
	return (r.ninjaTestClosed.Load() && r.isTestReader.Load()) || (!r.isTestReader.Load() && flowTopicsClosed.Load())
}

// isQuotaError detects if an error is due to Kafka quota/throttling
// Covers Confluent Cloud quota patterns including explicit error codes and timeout signatures
// From Kafka protocol: code 22 (QUOTA_EXCEEDED), code 31 (THROTTLE_TIME_MS)
func (r *SeededKafkaReader) isQuotaError(err error) bool {
	if err == nil {
		return false
	}
	errStr := strings.ToLower(err.Error())
	return strings.Contains(errStr, "quota") ||
		strings.Contains(errStr, "throttle") ||
		strings.Contains(errStr, "client_quota_exceeded") ||
		strings.Contains(errStr, "quota_exceeded") ||
		strings.Contains(errStr, "429") ||
		strings.Contains(errStr, "broker busy") ||
		strings.Contains(errStr, "broker is busy") ||
		strings.Contains(errStr, "broker not available") ||
		strings.Contains(errStr, "insufficient capacity") ||
		strings.Contains(errStr, "request timed out") ||
		strings.Contains(errStr, "retriable error") || // some quota errors are marked retriable
		strings.Contains(errStr, "throttled") // Kafka protocol THROTTLED_REQUEST_RESPONSE
}

// handleQuotaThrottling implements exponential backoff when quota is exhausted
func (r *SeededKafkaReader) handleQuotaThrottling() time.Duration {
	// Increment quota error count
	count := atomic.AddInt32(&r.quotaErrorCount, 1)

	// Calculate exponential backoff: 1s, 2s, 4s, 8s, 16s, then max 60s
	backoffSeconds := int64(1) << uint(count-1)
	if backoffSeconds > 60 {
		backoffSeconds = 60
	}

	// Log quota issue
	if count == 1 {
		etlcore.LogError(fmt.Sprintf("Quota throttled on reader %s, count=%d, backing off %ds",
			r.TopicName, count, backoffSeconds))
	} else if count%10 == 0 {
		// Log every 10 attempts to avoid log spam
		etlcore.LogError(fmt.Sprintf("Still quota throttled on reader %s, count=%d, backing off %ds",
			r.TopicName, count, backoffSeconds))
	}

	return time.Duration(backoffSeconds) * time.Second
}

// resetQuotaState resets quota error tracking when we successfully read
func (r *SeededKafkaReader) resetQuotaState() {
	if atomic.LoadInt32(&r.quotaErrorCount) > 0 {
		atomic.StoreInt32(&r.quotaErrorCount, 0)
		etlcore.LogError(fmt.Sprintf("Quota recovered on reader %s, resuming normal operation",
			r.TopicName))
	}
}

func (r *SeededKafkaReader) recreateKafkaReader() error {
	if r == nil {
		return errors.New("cannot recreate reader on nil SeededKafkaReader")
	}

	// Acquire write lock to protect kafkaClient replacement
	r.kafkaClientLock.Lock()
	defer r.kafkaClientLock.Unlock()

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
	r.firstRecordCommitted.Store(false) // Reset so we commit after first fetch on reconnect
	etlcore.LogError(fmt.Sprintf("Successfully recreated kafka client for topic: %s with group ID: %s", r.TopicName, r.ConsumerGroupID))
	return nil
}

func (r *SeededKafkaReader) KafkaTestEngine(kafkaErrChan chan KafkaErrMessage, testReadyWG *sync.WaitGroup) {
	// Atomically try to activate this engine. This prevents a race condition where
	// multiple goroutines could all pass the engineActive.Load() check before any of
	// them sets engineActive to true. CompareAndSwap ensures only ONE goroutine succeeds.

	// Recover from any panics to ensure engineActive is released
	defer func() {
		if recErr := recover(); recErr != nil {
			etlcore.LogError(fmt.Sprintf("Panic in KafkaTestEngine: %v", recErr))
			r.engineActive.Store(false) // Ensure engine state is reset on panic
			panic(recErr)               // Re-raise the panic after cleanup
		}
		r.engineActive.Store(false) // Release the engine claim when done normally
	}()

	// Signal that the engine loop is running
	r.engineRunning.Add(1)
	defer r.engineRunning.Done() // Signal completion when loop exits

	if r.isTestReader.Load() {
		var wg sync.WaitGroup
		wg.Add(1)

		//
		// Block until all tests are set up.
		//
		go r.ScanTests(&wg)

		wg.Wait()
		etlcore.LogError("All tests have registered. Test reader engine starting to read from kafka.")
	} else {
		etlcore.LogError("Flow reader engine starting to read from kafka.")
	}
	if !r.engineActive.CompareAndSwap(false, true) {
		// Another engine is already running on this reader - exit immediately
		etlcore.LogError(fmt.Sprintf("Engine already active on reader for topic %s - exiting", r.TopicName))
		if testReadyWG != nil && !r.testReadySignaled.Load() {
			if r.testReadySignaled.CompareAndSwap(false, true) {
				testReadyWG.Done() // Still need to signal done since we were counted in newKafkaReaderInternal
			}
		}
		return
	}
	// Signal that this engine is ready to process messages
	// IMPORTANT: Only signal after ScanTests wg.Done() confirms ScanTests goroutine is fully initialized
	if r.isTestReader.Load() && testReadyWG != nil && !r.testReadySignaled.Load() {
		if r.testReadySignaled.CompareAndSwap(false, true) {
			testReadyWG.Done()
			etlcore.LogError("Test reader signaled engine ready.")
		}
	}

	// All tests loaded and ready to go.
	for {
		// Exit only when explicitly closed by test framework (ninjaTestClosed set by CloseAllTests).
		// DO NOT exit just because CountTest() == 0 - that's normal when tests are deleted between sequential tests.
		// Engine must stay alive to process subsequent test registrations.
		if r.isReaderClosing() {
			etlcore.LogError("Reader is closing, exiting reader loop.")
			break
		}

		// Poll for new messages using franz-go
		ctx := context.Background()
		r.kafkaClientLock.RLock()
		kafkaClient := r.kafkaClient
		r.kafkaClientLock.RUnlock()

		if kafkaClient == nil {
			etlcore.LogError("Kafka client is nil, waiting before retry...")
			time.Sleep(1 * time.Second)
			continue
		}

		fetches := kafkaClient.PollFetches(ctx)

		// Check if client was closed using franz-go's built-in method
		if fetches.IsClientClosed() && !r.isReaderClosing() {
			etlcore.LogError("Kafka client closed")
			if kafkaErrChan != nil {
				kafkaErrChan <- KafkaErrMessage{
					TopicKey:   r.CacheKey,
					KafkaError: errors.New("Kafka client closed"),
				}
			}
			break
		}

		// Check for errors in fetches
		if err := fetches.Err(); err != nil {
			// Check for quota/throttling errors FIRST - these need graceful backoff
			if r.isQuotaError(err) {
				backoffDuration := r.handleQuotaThrottling()
				// Quota errors are handled internally with exponential backoff - do not send to error channel
				// This ensures tests continue gracefully during transient quota throttling
				time.Sleep(backoffDuration)
				continue
			}

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
				break
			}

			// Check if this is the ErrClientClosed error
			if errors.Is(err, kgo.ErrClientClosed) {
				if !r.isTestReader.Load() || (r.isTestReader.Load() && r.CountTest() > 0) {
					etlcore.LogError("Kafka client closed error")
				}
				if kafkaErrChan != nil {
					kafkaErrChan <- KafkaErrMessage{
						TopicKey:   r.CacheKey,
						KafkaError: errors.New("Kafka client closed error"),
					}
				}
				break
			}

			// Check if context was canceled
			if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
				etlcore.LogError(fmt.Sprintf("Context error: %v", err))
				if r.isReaderClosing() {
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
					break
				} else {
					// Unexpected connection loss - attempt to recreate
					etlcore.LogError(fmt.Sprintf("Transient network error (%v), attempting to recreate client", err))
					if recreateErr := r.recreateKafkaReader(); recreateErr != nil {
						etlcore.LogError(fmt.Sprintf("Failed to recreate kafka client: %v", recreateErr))
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
				break
			}

			// Other errors - log and retry (could be kafka protocol errors)
			etlcore.LogError(fmt.Sprintf("Kafka error, retrying: %v", err))
			continue
		}

		// If this is the first successful fetch, commit immediately to establish position
		if !r.firstRecordCommitted.Load() {
			commitCtx, commitCancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer commitCancel() // Release context resources (removed duplicate commitCancel())

			r.kafkaClientLock.RLock()
			kafkaClient := r.kafkaClient
			r.kafkaClientLock.RUnlock()

			if kafkaClient != nil {
				if err := kafkaClient.CommitUncommittedOffsets(commitCtx); err != nil {
					etlcore.LogError(fmt.Sprintf("First fetch commit failed: %v", err))
				} else {
					r.firstRecordCommitted.Store(true)
					etlcore.LogError("Successfully committed position after first fetch")
				}
			}
			etlcore.LogError("First fetch processing complete.")
		}

		// Reset quota throttling state on successful fetch
		r.resetQuotaState()

		// Process all records in the fetch
		iter := fetches.RecordIter()
		for !iter.Done() {
			rec := iter.Next()

			if !GetPlugin() {
				etlcore.LogError(fmt.Sprintf("message at topic/partition/offset %v/%v/%v: %s = %s\n",
					rec.Topic, rec.Partition, rec.Offset, string(rec.Key), string(rec.Value)))
			}

			if (!r.ninjaTestClosed.Load() && r.isTestReader.Load()) || (!flowTopicsClosed.Load() && !r.isTestReader.Load()) {
				r.messageProcessingWg.Add(1)
				go func(record *kgo.Record) {
					defer r.messageProcessingWg.Done()
					r.ProcessMessage(record)
				}(rec)
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

	// Wait for in-flight message processing to complete before fully closing
	if err := r.waitForMessageProcessing(5 * time.Second); err != nil {
		etlcore.LogError(fmt.Sprintf("Warning: %v", err))
	}
}

// waitForMessageProcessing waits for all in-flight message processing goroutines to complete
func (r *SeededKafkaReader) waitForMessageProcessing(timeout time.Duration) error {
	done := make(chan struct{})
	go func() {
		r.messageProcessingWg.Wait()
		close(done)
	}()

	select {
	case <-done:
		return nil
	case <-time.After(timeout):
		return errors.New("timeout waiting for message processing to complete")
	}
}

func TestSequenceExpected(enterpriseID string, readerSequence []*SeededKafkaReader, kafkaTestSequence []*KafkaTestBundle, testReadyWG *sync.WaitGroup) {
	if !GetPlugin() {
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

	// Cleanup expected values BEFORE storing in map to prevent race condition where
	// another goroutine reads the bundle while we're still modifying it.
	// This must complete before the bundle is exposed to message processing goroutines.
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

	// Atomically check, generate unique key, and update map in one critical section
	// to prevent race conditions with concurrent testExpected calls
	r.kafkaTestBundleLock.Lock()
	if _, hasKey := r.kafkaTestBundle[testName]; !hasKey {
		kafkaTestBundle.SuccessFun(fmt.Errorf("invalid and unregisterd test, check testname: %s", testName))
		kafkaTestBundle.Wg.Done()
		r.kafkaTestBundleLock.Unlock()
		return
	}

	// Generate unique ID for this test instance to support multiple simultaneous tests
	testID, _ := uuid.NewRandom()
	uniqueTestKey := testName + "|" + testID.String()
	kafkaTestBundle.Name = uniqueTestKey

	// Store the bundle directly under its unique key to guarantee message matching finds it immediately,
	// even for injected tests where the engine is already running and processing messages.
	// Bundle is fully initialized at this point (fields set before lock), so safe to expose.
	r.kafkaTestBundle[uniqueTestKey] = kafkaTestBundle
	delete(r.kafkaTestBundle, testName) // Remove placeholder - bundle now stored under unique key
	r.kafkaTestBundleLock.Unlock()

	// Send to incomingTestChan to notify ScanTests that bundle is registered (so it checks HasEmptyTest)
	if r.incomingTestChan != nil && !r.channelsClosed.Load() {
		func() {
			defer func() {
				if rec := recover(); rec != nil {
					if !r.channelsClosed.Load() {
						etlcore.LogError(fmt.Sprintf("Panic sending to incomingTestChan in testExpected: %v", rec))
					}
				}
			}()
			// Send the bundle (which is already stored) to trigger ScanTests to check HasEmptyTest
			r.incomingTestChan <- kafkaTestBundle
		}()
	}

	go func() {
		resultErr := TestExpectedHelper(r, kafkaTestBundle)

		if resultErr != nil {
			// Pass nil on success or the error on failure - SuccessFun callback will call Wg.Done()
			kafkaTestBundle.SuccessFun(resultErr)
		} else {
			// Success case: call SuccessFun to trigger Wg.Done() via callback
			kafkaTestBundle.SuccessFun(nil)
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

	testNameFormatted := fmt.Sprintf("%33s", testName)
	etlcore.LogError(fmt.Sprintf("KafkaTestInit setting up mpb for: %s\n", testNameFormatted))

	multiBarLock.Lock()
	multibar := MultiBarInstance()

	bar := multibar.Mpb.AddBar(int64(100),
		mpb.PrependDecorators(
			decor.Name(testNameFormatted, decor.WCSyncSpace),
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
				bar.Abort(GetPlugin())
				return nil, nil, "", "", nil, err
			}
			// Register test as soon as reader is created with base test name
			reader.RegisterTest(testName)
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
			bar.Abort(GetPlugin())
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
	if bar != nil {
		bar.IncrBy(25)
	}
	for _, kafkaTest := range kafkaTestSequence {
		if kafkaTest.Wg == nil {
			kafkaTest.Wg = &sync.WaitGroup{}
		}
		kafkaTest.Wg.Wait()
		// Use flowTopicsClosed global flag like trcdstream, not inverted allTestsClosed logic
		if flowTopicsClosed.Load() {
			resultError = errors.New("timeout signal sent")
		}
		if resultError != nil {
			if bar != nil {
				bar.IncrBy(50)
			}
			(*currentState).Store(STATE_FAILED)
			stateLock.Lock()
			stateMap[currentState.Load().(string)] = time.Since(start)
			stateLock.Unlock()
			if bar != nil {
				bar.Abort(plugin.Load())
			}
			break
		}
		if bar != nil {
			bar.IncrBy(5)
		}
		(*currentState).Store(kafkaTest.CompletionStatus)
		stateLock.Lock()
		stateMap[currentState.Load().(string)] = time.Since(start)
		stateLock.Unlock()
	}
	if bar != nil {
		bar.IncrBy(20)
	}

	if resultError != nil {
		if bar != nil {
			bar.IncrBy(50)
		}
		(*currentState).Store(STATE_FAILED)
		stateLock.Lock()
		stateMap[currentState.Load().(string)] = time.Since(start)
		stateLock.Unlock()
	} else {
		if bar != nil {
			bar.IncrBy(50)
		}
		(*currentState).Store(STATE_COMPLETE)
		stateLock.Lock()
		stateMap[currentState.Load().(string)] = time.Since(start)
		stateLock.Unlock()
	}
	time.Sleep(100 * time.Millisecond)
}

// TestExpectedHelper -- sends the test bundle to ScanTests for registration and parsing.
func TestExpectedHelper(r *SeededKafkaReader, kafkaTestBundle *KafkaTestBundle) error {
	if r == nil {
		return errors.New("cannot send test to nil reader")
	}
	if r.incomingTestChan == nil {
		return errors.New("incomingTestChan is nil")
	}
	if r.channelsClosed.Load() {
		return errors.New("reader channels have been closed")
	}
	// Blocking send - ensures bundle is delivered and stored before returning.
	// This guarantees message matching can find the bundle immediately.
	// Wrap in panic recovery to handle TOCTOU race where channels close between check and send
	done := make(chan bool, 1)
	go func() {
		defer func() {
			if rec := recover(); rec != nil {
				etlcore.LogError(fmt.Sprintf("Panic sending to incomingTestChan in TestExpectedHelper: %v", rec))
			}
			done <- true
		}()
		r.incomingTestChan <- kafkaTestBundle // Blocking send
	}()
	<-done // Wait for send to complete
	return nil
}
