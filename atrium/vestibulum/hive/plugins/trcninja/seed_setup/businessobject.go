package seed_setup

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
	"github.com/trimble-oss/tierceron/atrium/vestibulum/hive/plugins/trcninja/seed_setup/models"
	"github.com/trimble-oss/tierceron/atrium/vestibulum/hive/plugins/trcninja/util"

	etlcore "github.com/trimble-oss/tierceron/atrium/vestibulum/hive/plugins/trcninja/core"
	"github.com/trimble-oss/tierceron/atrium/vestibulum/hive/plugins/trcninja/kafkatesting"
	"github.com/vbauerster/mpb/v8"
	"github.com/vbauerster/mpb/v8/decor"
)

var pluginLog = false

const (
// boTopic    = "bu_topic"
// boPrefix   = ""
// boRangeMin = 1
// boRangeMax = 9999
)

// BuildBusinessObject - Builds a BusinessObject object
func BuildBusinessObject() models.BusinessObject {
	return models.BusinessObject{
		Description: "Description",
		Field1:      "Field1",
		Field2:      "Field2",
		Field3:      "Field3",
	}
}

var (
	BusinessObjectStateMapLock sync.Mutex
	BusinessObjectStateMap     map[string]interface{}
)

var BusinessObjectTopicSequence = [][]string{
	{"kafkatopicone", "json"},
	{"kafkatopictwo", "avro"},
}

type Pool struct{}

// AddBusinessObject -- adds a new business object, then deletes when finished adding.
func AddBusinessObject(readerGroupPrefix string) error {
	var currentState atomic.Value
	argosID := "recordId"
	start := time.Now()
	currentState.Store(kafkatesting.STATE_INIT)
	currentStateFunc := func(s decor.Statistics) string {
		return currentState.Load().(string)
	}
	readerSequence, bar, sociiID, _, dbConn, err := kafkatesting.KafkaTestInit(argosID, readerGroupPrefix, etlcore.GetConfigContext("ninja"), &currentState, BusinessObjectTopicSequence, currentStateFunc, BusinessObjectStateMap, start)
	if err != nil {
		currentState.Store(kafkatesting.STATE_FAILED_SETUP)
		BusinessObjectStateMapLock.Lock()
		BusinessObjectStateMap[currentState.Load().(string)] = time.Since(start)
		BusinessObjectStateMapLock.Unlock()
		if bar != nil {
			bar.Abort(false)
		}
		if strings.Contains(err.Error(), "somePipelineError") {
			BusinessObjectStateMapLock.Lock()
			BusinessObjectStateMap["Pipeline is not in a Testable State"] = time.Since(start)
			BusinessObjectStateMapLock.Unlock()
		} else {
			err = fmt.Errorf("setup failure")
		}
		return err
	}

	// 0. Create a new BusinessObject record
	BusinessObject := BuildBusinessObject()

	var resultError error
	kafkaTestSequence := util.TestSequenceBundleBuilder(sociiID,
		BusinessObjectTopicSequence,
		readerSequence,
		map[string]interface{}{
			"Field1":    BusinessObject.Field1,
			"Field2":    BusinessObject.Field2,
			"Field3":    BusinessObject.Field3,
			"EventType": "UPDATED",
		},
		map[string]interface{}{
			"Description": strings.TrimSpace(BusinessObject.Description),
		},
		func(err error) {
			if err != nil {
				etlcore.LogError(fmt.Sprintf("%s Kafka error.  %v", sociiID, err))
				currentState.Store(kafkatesting.STATE_FAILED)
				BusinessObjectStateMapLock.Lock()
				BusinessObjectStateMap[currentState.Load().(string)] = time.Since(start)
				BusinessObjectStateMapLock.Unlock()
				resultError = err
			}
		},
	)

	bar.IncrBy(25)
	// 2. Lookup existing record from database and exit with error if found
	_, err = models.BusinessObjectByField1Field2(context.Background(),
		dbConn,
		kafkaTestSequence[1].ExpectedLogicalKey["KeyMap.Field1"].(string), // CompanyCode string,
		kafkaTestSequence[1].ExpectedLogicalKey["KeyMap.Field2"].(string), // CustomerCode string,
	)
	if err == nil {
		currentState.Store(kafkatesting.STATE_FAILED_MISSING_RECORD)
		BusinessObjectStateMapLock.Lock()
		BusinessObjectStateMap[currentState.Load().(string)] = time.Since(start)
		BusinessObjectStateMapLock.Unlock()
		bar.Abort(pluginLog)
		err = fmt.Errorf("record already exists in database")
		return err
	}

	// 3. Make sure parent exists
	_, err = models.BusinessObjectByField1Field2(context.Background(),
		dbConn,
		BusinessObject.Field1,
		BusinessObject.Field2,
	)
	if err != nil {
		currentState.Store(kafkatesting.STATE_FAILED_PARENT_RECORD)
		BusinessObjectStateMapLock.Lock()
		BusinessObjectStateMap[currentState.Load().(string)] = time.Since(start)
		BusinessObjectStateMapLock.Unlock()
		bar.Abort(pluginLog)
		etlcore.LogError(fmt.Sprintf("Business Object"+"Parent does not exist in database. Error: %v", err))
		return err
	}

	// 5. Kick off an asynchronous test.
	kafkatesting.TestSequenceExpected(sociiID, readerSequence, kafkaTestSequence)

	// 6. Insert existing record and cleanup.
	if !pluginLog {
		etlcore.LogError(fmt.Sprintf("%s Inserting into database", sociiID))
	}
	currentState.Store(kafkatesting.STATE_DBINIT)
	BusinessObjectStateMapLock.Lock()
	BusinessObjectStateMap[currentState.Load().(string)] = time.Since(start)
	BusinessObjectStateMapLock.Unlock()
	insertError := BusinessObject.Insert(context.Background(), dbConn)
	if insertError != nil {
		currentState.Store(kafkatesting.STATE_FAILED_INSERT_RECORD)
		BusinessObjectStateMapLock.Lock()
		BusinessObjectStateMap[currentState.Load().(string)] = time.Since(start)
		BusinessObjectStateMapLock.Unlock()
		bar.Abort(pluginLog)
		etlcore.LogError(fmt.Sprintf("%s Database insert error.", sociiID))
		return insertError
	}

	bar.IncrBy(25)
	// Delete added record
	var p Pool
	defer p.cleanBusinessObjectHelper(argosID, readerGroupPrefix, &currentState, bar, sociiID, dbConn)

	// 7. Wait for results.
	kafkatesting.TestWait(&currentState, kafkaTestSequence, bar, resultError, BusinessObjectStateMap, start, &BusinessObjectStateMapLock)

	return resultError
}

// UpdateBusinessObject -- updates and reads an BusinessObject.
func UpdateBusinessObject(readerGroupPrefix string) error {
	argosID := "recordId"
	var currentState atomic.Value
	start := time.Now()
	currentState.Store(kafkatesting.STATE_INIT)
	currentStateFunc := func(s decor.Statistics) string {
		return currentState.Load().(string)
	}
	readerSequence, bar, sociiID, _, dbConn, err := kafkatesting.KafkaTestInit(argosID, readerGroupPrefix, etlcore.GetConfigContext("ninja"), &currentState, BusinessObjectTopicSequence, currentStateFunc, BusinessObjectStateMap, start)
	BusinessObjectStateMapLock.Lock()
	BusinessObjectStateMap[currentState.Load().(string)] = time.Since(start)
	BusinessObjectStateMapLock.Unlock()
	if err != nil {
		currentState.Store(kafkatesting.STATE_FAILED_SETUP)
		BusinessObjectStateMapLock.Lock()
		BusinessObjectStateMap[currentState.Load().(string)] = time.Since(start)
		BusinessObjectStateMapLock.Unlock()
		if bar != nil {
			bar.Abort(pluginLog)
		}
		if strings.Contains(err.Error(), "snapshotted") {
			BusinessObjectStateMapLock.Lock()
			BusinessObjectStateMap["Pipeline is not in a Testable State"] = time.Since(start)
			BusinessObjectStateMapLock.Unlock()
		} else {
			err = fmt.Errorf("setup failure")
		}
		return err
	}
	var resultError error
	kafkaTestSequence := util.TestSequenceBundleBuilder(sociiID,
		BusinessObjectTopicSequence,
		readerSequence,
		map[string]interface{}{
			"Field1":    "Field1Update",
			"Field2":    "Field2Update",
			"Field3":    "Field3Update",
			"EventType": "UPDATED",
		},
		map[string]interface{}{},
		func(err error) {
			if err != nil {
				etlcore.LogError(fmt.Sprintf("%s Kafka error.  %v", sociiID, err))
				currentState.Store(kafkatesting.STATE_FAILED)
				BusinessObjectStateMapLock.Lock()
				BusinessObjectStateMap[currentState.Load().(string)] = time.Since(start)
				BusinessObjectStateMapLock.Unlock()
				resultError = err
			}
		},
	)

	// 2. Lookup existing record from database.
	businessObject, err := models.BusinessObjectByField1Field2(context.Background(), dbConn,
		kafkaTestSequence[1].ExpectedLogicalKey["KeyMap.Field1"].(string), // Field1 string,
		kafkaTestSequence[1].ExpectedLogicalKey["KeyMap.Field2"].(string), // Field2 string
	)
	if err != nil {
		currentState.Store(kafkatesting.STATE_FAILED_MISSING_RECORD)
		BusinessObjectStateMapLock.Lock()
		BusinessObjectStateMap[currentState.Load().(string)] = time.Since(start)
		BusinessObjectStateMapLock.Unlock()
		bar.Abort(pluginLog)
		return err
	}

	// 3. Make a change
	description, err := uuid.NewRandom()
	if err != nil {
		return err
	}
	businessObject.Description = strings.ToUpper(description.String())
	bar.IncrBy(25)
	// 4. Update expected value.
	kafkaTestSequence[0].ExpectedValue["Description"] = strings.TrimSpace(businessObject.Description)
	kafkaTestSequence[1].ExpectedValue["Description"] = strings.TrimSpace(businessObject.Description)

	// 5. Kick off an asynchronous test.
	kafkatesting.TestSequenceExpected(sociiID, readerSequence, kafkaTestSequence)

	// 6. Update existing record and cleanup.
	if !pluginLog {
		etlcore.LogError(fmt.Sprintf("%s Updating database", sociiID))
	}
	currentState.Store(kafkatesting.STATE_DBINIT)
	BusinessObjectStateMapLock.Lock()
	BusinessObjectStateMap[currentState.Load().(string)] = time.Since(start)
	BusinessObjectStateMapLock.Unlock()
	updateError := businessObject.Update(context.Background(), dbConn)
	dbConn.Close()
	if updateError != nil {
		currentState.Store(kafkatesting.STATE_FAILED_UPDATE_RECORD)
		BusinessObjectStateMapLock.Lock()
		BusinessObjectStateMap[currentState.Load().(string)] = time.Since(start)
		BusinessObjectStateMapLock.Unlock()
		bar.Abort(pluginLog)
		etlcore.LogError(fmt.Sprintf("%s Database update error.", sociiID))
		return updateError
	}
	if !pluginLog {
		etlcore.LogError(fmt.Sprintf("%s Database update complete", sociiID))
	}
	// 7. Wait for results.
	kafkatesting.TestWait(&currentState, kafkaTestSequence, bar, resultError, BusinessObjectStateMap, start, &BusinessObjectStateMapLock)
	bar.Abort(pluginLog)
	return resultError
}

func (p *Pool) CleanBusinessObject(argosID string, readerGroupPrefix string) error {
	etlcore.LogError("Clean Business Object proxying...")
	return p.cleanBusinessObjectHelper(argosID, readerGroupPrefix, nil, nil, "", nil)
}

func (p *Pool) cleanBusinessObjectHelper(argosID string, readerGroupPrefix string, currentState *atomic.Value, bar *mpb.Bar, sociiID string, dbConn *sql.DB) error {
	etlcore.LogError("Business Object Clean - starting...")
	endState := kafkatesting.STATE_COMPLETE_CLEANED
	start := time.Now()
	if currentState == nil {
		endState = kafkatesting.STATE_COMPLETE
		currentState = &atomic.Value{}
		currentState.Store(kafkatesting.STATE_INIT)
		currentStateFunc := func(s decor.Statistics) string {
			return currentState.Load().(string)
		}
		var err error
		// 1. Setup connections to database and kafka.
		_, bar, sociiID, _, dbConn, err = kafkatesting.KafkaTestInit(argosID, readerGroupPrefix, etlcore.GetConfigContext("ninja"), currentState, [][]string{}, currentStateFunc, nil, start)
		if err != nil {
			currentState.Store(kafkatesting.STATE_CLEAN_SETUP_FAILURE)
			if bar != nil {
				bar.Abort(pluginLog)
			}
			err = fmt.Errorf("clean setup failure")
			return err
		}

	}
	etlcore.LogError("Business Object Clean - Setup Succeeded")

	// 2. Delete added record
	if !pluginLog {
		etlcore.LogError(fmt.Sprintf("%s Deleting database records", sociiID))
	}
	// Create a business object for deletion
	businessObject := BuildBusinessObject()
	coCtx, coDoneChannel := util.CleanupCanceller()
	etlcore.LogError("Business Object Clean - attempting delete query")
	deleteError := businessObject.Delete(coCtx, dbConn)
	coDoneChannel <- true
	dbConn.Close()
	if deleteError != nil {
		etlcore.LogError(fmt.Sprintf("%s Database delete error.", sociiID))
		bar.IncrBy(50)
		return deleteError
	}
	etlcore.LogError("Business Object Clean - delete query succeeded")
	if !pluginLog {
		etlcore.LogError(fmt.Sprintf("%s Delete database records complete", sociiID))
	}
	bar.IncrBy(75)
	currentState.Store(endState)
	bar.Abort(pluginLog)
	time.Sleep(100 * time.Millisecond)
	etlcore.LogError("Business Object Clean - ending clean")
	return nil
}
