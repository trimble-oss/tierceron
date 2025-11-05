package kafkatesting

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"log"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
	cmap "github.com/orcaman/concurrent-map/v2"

	tccore "github.com/trimble-oss/tierceron-core/v2/core"
	"github.com/vbauerster/mpb/v8"
	"github.com/vbauerster/mpb/v8/decor"

	// "github.com/trimble-oss/tierceron/pkg/core"
	etlcore "github.com/trimble-oss/tierceron/atrium/vestibulum/hive/plugins/trcninja/core"

	"github.com/segmentio/kafka-go"
	"github.com/segmentio/kafka-go/sasl/plain"
	"github.com/trimble-oss/tierceron/atrium/vestibulum/hive/plugins/trcninja/confighelper"
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
	startTime           time.Time
	TopicName           string
	TopicType           string // json or avro
	kafkaReader         *kafka.Reader
	seededMessage       *kafka.Message
	kafkaTestBundle     map[string]*KafkaTestBundle
	kafkaTestBundleLock sync.RWMutex
	incomingTestChan    chan *KafkaTestBundle
	deleteTestChan      chan *KafkaTestBundle
}

// MultiBarContainer container for our progress bar
type MultiBarContainer struct {
	Mpb *mpb.Progress
	Wg  *sync.WaitGroup
}

var (
	kafkaReaderCache       map[string]*SeededKafkaReader
	openConnectionCache    map[string]*sql.DB
	multiProgressContainer MultiBarContainer
	mapLock                sync.Mutex
	mpbContext             context.Context
	mpbContextCancel       context.CancelFunc
	IsPlugin               bool = true
)

type IndirectDBConnectionFunc func(configContex tccore.ConfigContext, argosId string) (string, string, *sql.DB, error)

var IndirectDBFunc IndirectDBConnectionFunc

var (
	plugin = false
	closed = false
)

func init() {
	openConnCacheLock = &sync.Mutex{}
	kafkaReaderCache = make(map[string]*SeededKafkaReader)
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
	etlcore.LogError("Panicing to trigger vault plugin restart.")

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

// NewKafkaTestReader -- create new kafka test reader for a topic.
func NewKafkaTestReader(topic []string, readerGroupPrefix string, ignoreCacheFail ...bool) (*SeededKafkaReader, error) {
	mapLock.Lock()
	MultiBarInstance()
	defer mapLock.Unlock()
	if reader, readerOk := kafkaReaderCache[topic[0]]; readerOk {
		return reader, nil
	}
	if ignoreCacheFail == nil || !ignoreCacheFail[0] {
		etlcore.LogError("Critical failure.  Attempt to use unregistered reader.")
		return nil, errors.New("Critical failure.  Attempt to use unregistered reader for topic: " + topic[0])
	}

	etlProperties := confighelper.GetProperties()

	// mechanism, err := scram.Mechanism(scram.SHA512,
	// 	(*etlProperties)["kafka_agnostic_username"].(string),
	// 	(*etlProperties)["kafka_agnostic_password"].(string))
	// if err != nil {
	// 	etlcore.LogError("Critical failure.  Cannot scram encode user.")
	// 	return nil, errors.New("Critical failure.  Unable to scram encrypt password: " + err.Error())
	// }
	mechanism := plain.Mechanism{
		Username: (*etlProperties)["kafka_agnostic_username"].(string),
		Password: (*etlProperties)["kafka_agnostic_password"].(string),
	}

	caPool := x509.NewCertPool()
	caPool.AppendCertsFromPEM(confighelper.KafkaCert)
	var tlsConfig *tls.Config

	if (*etlProperties)["env"].(string) == "dev" || (*etlProperties)["env"].(string) == "QA" {
		tlsConfig = &tls.Config{RootCAs: caPool, InsecureSkipVerify: true}
	} else {
		tlsConfig = &tls.Config{RootCAs: caPool}
	}

	dialer := &kafka.Dialer{
		Timeout:       30 * time.Second,
		DualStack:     true,
		SASLMechanism: mechanism,
		KeepAlive:     10 * time.Minute,
		TLS:           tlsConfig,
	}

	// startTime := time.Now().Add(-time.Hour * 24)
	startTime := time.Now()
	id, err := uuid.NewRandom() // machineid.ProtectedID("kafkamonger")
	if err != nil {
		log.Fatal("Could not generate guid", err)
		return nil, errors.New("Could not generate guid")
	}

	// 3. Setup kafka, read and wait for our event.
	r := kafka.NewReader(kafka.ReaderConfig{
		Dialer:           dialer,
		Brokers:          []string{(*etlProperties)["bootstrapServers"].(string)},
		GroupID:          readerGroupPrefix + id.String(),
		Topic:            topic[0],
		MinBytes:         1,    // 10KB
		MaxBytes:         10e6, // 10MB
		MaxWait:          1 * time.Second,
		ReadBackoffMin:   1000 * time.Millisecond, // wait at least a second but not more than 2 seconds.
		ReadBackoffMax:   2000 * time.Millisecond, //
		JoinGroupBackoff: 500 * time.Millisecond,
		IsolationLevel:   kafka.ReadUncommitted,
		Logger:           kafka.LoggerFunc(func(string, ...interface{}) {}), // Turned off logger due to log spam
		ErrorLogger:      kafka.LoggerFunc(GetKafkaErrorLogger()),           // Normally -  kafka.LoggerFunc(confighelper.Logger.Printf)
		StartOffset:      kafka.LastOffset,
	})

	etlcore.LogError(fmt.Sprintf("Starting reader on topic: %s with content type: %s on broker: %s.\n", topic[0], topic[1], (*etlProperties)["bootstrapServers"].(string)))

	reader := &SeededKafkaReader{
		startTime:        startTime,
		TopicName:        topic[0],
		TopicType:        topic[1],
		kafkaReader:      r,
		kafkaTestBundle:  map[string]*KafkaTestBundle{},
		deleteTestChan:   make(chan *KafkaTestBundle, 20),
		incomingTestChan: make(chan *KafkaTestBundle, 3),
	}

	kafkaReaderCache[topic[0]] = reader
	return reader, nil
}

func StartAllEngines(kafkaErrChan chan bool) {
	for _, reader := range kafkaReaderCache {
		go reader.KafkaTestEngine(kafkaErrChan)
	}
}

func CloseAllTests() {
	closed = true
	for _, reader := range kafkaReaderCache {
		for _, testBundle := range reader.kafkaTestBundle {
			if testBundle != nil {
				//				testBundle.Wg.Done()
				reader.DeleteTest(testBundle)
			}
		}
	}
}

func CloseAllEngines() {
	closed = true
	etlcore.LogError("Closing connections...")
	openConnCacheLock.Lock()
	for _, conn := range openConnectionCache {
		if conn != nil {
			conn.Close()
		}
	}
	openConnCacheLock.Unlock()
	etlcore.LogError("Connections closed...")
	etlcore.LogError("Closing readers...")
	for _, reader := range kafkaReaderCache {
		if reader != nil {
			reader.Close()
			reader = nil
		}
	}
	etlcore.LogError("Readers closed...")

	for readerKey := range kafkaReaderCache {
		delete(kafkaReaderCache, readerKey)
	}
}

// PreSeed -- Sets up a waitgroup and kicks off async testhelper.
func (r *SeededKafkaReader) PreSeed() {
	ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(10*time.Second))
	defer cancel()

	for {
		etlcore.LogError("Beginning kafka group setup.")
		m, err := r.kafkaReader.ReadMessage(ctx)
		if err != nil {
			etlcore.LogError(fmt.Sprintf("Failure to parse change order item addon.  Error: %v", err))
			continue
		}

		if m.Time.Before(r.startTime) {
			// Ignore messages before our start time.
			continue
		} else {
			r.seededMessage = &m
			etlcore.LogError("Ended kafka group setup.")
			break
		}
	}
}

// RegisterTest - registers an expected test with the underlying kafka test engine.
func (r *SeededKafkaReader) RegisterTest(testName string) {
	mapLock.Lock()
	r.kafkaTestBundle[testName] = nil
	mapLock.Unlock()
}

// DeleteTest -- remove test bundle.
func (r *SeededKafkaReader) DeleteTest(incomingTest *KafkaTestBundle) {
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
			if closed {
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

func (r *SeededKafkaReader) ProcessMessage(m *kafka.Message) {
	switch r.TopicType {
	case "avro":
		r.ProcessMessageAvro(m)
	case "json":
		fallthrough
	default:
		r.ProcessMessageJSON(m)
	}
}

func (r *SeededKafkaReader) KafkaTestEngine(kafkaErrChan chan bool) {
	var wg sync.WaitGroup
	wg.Add(1)

	//
	// Block until all tests are set up.
	//
	go r.ScanTests(&wg)

	wg.Wait()

	etlcore.LogError("All tests have registered.  Engine starting to read from kafka.")

	// All tests loaded and ready to go.
	for {
		var m kafka.Message
		var err error
		if closed {
			r.seededMessage = nil
		}
		if r.seededMessage != nil {
			m = *r.seededMessage
			err = nil
			r.seededMessage = nil
		} else {
			// etlcore.LogError("Waiting for message from kafka.")
			if closed {
				ctx, cancel := context.WithCancel(context.Background())
				_, err := r.kafkaReader.ReadMessage(ctx)
				if err != nil {
					etlcore.LogError(err.Error())
				}
				kafkaErrChan <- true
				cancel()
				r.Close()
				break
			} else {
				m, err = r.kafkaReader.ReadMessage(context.Background())
			}

			// etlcore.LogError("Message received from kafka.")
			if !plugin {
				etlcore.LogError(fmt.Sprintf("message at topic/partition/offset %v/%v/%v: %s = %s\n", m.Topic, m.Partition, m.Offset, string(m.Key), string(m.Value)))
			}
		}

		var kerr kafka.Error
		if errors.As(err, &kerr) {
			if kerr == kafka.GroupAuthorizationFailed {
				etlcore.LogError(fmt.Sprintf("Group authorization failed: %v", err))
				kafkaErrChan <- true
				defer r.Close()
				break
			}
		}

		// After a test completes, if there are no more tests, then close the reader and exit.
		if r.CountTest() == 0 {
			// NOTE: May gobble an extra message here, but that is o.k.
			// All done.  Don't process messages further.
			defer r.Close()
			break
		}

		if err != nil {
			etlcore.LogError(fmt.Sprintf("Failure to read message.  Error: %v", err))
			continue
		}

		if !closed {
			go r.ProcessMessage(&m)
		}
	}
}

// Close -- closes reader and cleans up.
func (r *SeededKafkaReader) Close() {
	r.kafkaReader.Close()
}

func TestSequenceExpected(sociiID string, readerSequence []*SeededKafkaReader, kafkaTestSequence []*KafkaTestBundle) {
	if !plugin {
		etlcore.LogError(fmt.Sprintf("%s Going to kafka.", sociiID))
	}
	for i, reader := range readerSequence {
		testExpected(reader, kafkaTestSequence[i])
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

var chatMsgHookCtx *cmap.ConcurrentMap[string, tccore.ChatHookFunc]

func GetChatMsgHookCtx() *cmap.ConcurrentMap[string, tccore.ChatHookFunc] { return chatMsgHookCtx }

// KafkaTestInit - obtains mpb, kafka reader, and connection for use in kafka testing.
func KafkaTestInit(argosID string,
	readerGroupPrefix string,
	configContext tccore.ConfigContext,
	currentState *atomic.Value,
	kafkaTopicSequence [][]string,
	currentStateFunc decor.DecorFunc,
	stateMap map[string]interface{},
	start time.Time,
) ([]*SeededKafkaReader, *mpb.Bar, string, string, *sql.DB, error) {
	etlcore.LogError("KafkaTestInit")
	closed = false
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

	mapLock.Lock()
	multibar := MultiBarInstance()

	bar := multibar.Mpb.AddBar(int64(100),
		mpb.PrependDecorators(
			decor.Name(testName, decor.WCSyncSpace),
			decor.Name(" "),
			decor.Any(currentStateFunc),
			decor.Elapsed(decor.ET_STYLE_MMSS, decor.WCSyncSpace),
		),
	)
	mapLock.Unlock()
	var reader *SeededKafkaReader = nil
	var err error
	var readerSequence []*SeededKafkaReader

	for _, kafkaTopic := range kafkaTopicSequence {
		if kafkaTopic[0] != "" {
			reader, err = NewKafkaTestReader(kafkaTopic, readerGroupPrefix)
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

	if IndirectDBFunc != nil {
		argosIDIndirect, region, dbConn, err := IndirectDBFunc(configContext, argosID)
		if err != nil {
			(*currentState).Store(STATE_FAILED_DB_CONN)
			stateMap[currentState.Load().(string)] = time.Since(start)
			bar.Abort(plugin)
			return nil, bar, "", "", nil, err
		}
		etlcore.LogError("KafkaTestInit indirect db conn obtained.  Obtaining direct connection.")

		bar.IncrBy(25)
		return readerSequence, bar, argosIDIndirect, region, dbConn, nil
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
		if closed {
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
