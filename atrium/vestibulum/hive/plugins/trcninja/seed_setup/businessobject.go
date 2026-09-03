//go:build seedsetup
// +build seedsetup

package seed_setup

import (
	"fmt"
	"strconv"
	"time"

	"github.com/trimble-oss/tierceron/atrium/vestibulum/hive/plugins/trcninja/seed_setup/models"

	etlcore "github.com/trimble-oss/tierceron/atrium/vestibulum/hive/plugins/trcninja/core"
	"github.com/trimble-oss/tierceron/atrium/vestibulum/hive/plugins/trcninja/kafkatesting"
	"github.com/vbauerster/mpb/v8/decor"
)

const (
	boTopic    = "bu_topic"
	boPrefix   = ""
	boRangeMin = 1
	boRangeMax = 9999
)

// BuildJcBusinessObjectMasterMc Builds a BusinessObject object
func BuildBusinessObjectMasterMc() models.JcBusinessObjectMasterMc {
	return models.JcBusinessObjectMasterMc{
		Field1: "Field1",
		Field2: "Field2",
		Field3: "Field3",
	}
}

// AddBusinessObject -- adds a new business object, then deletes when finished adding.
func AddBusinessObject() error {
	currentState := "init"
	currentStateFunc := func(s decor.Statistics) string {
		return currentState
	}
	reader, bar, sociiID, _, spectrumConn, err := kafkatesting.KafkaTestInit(&currentState, BusinessObjectTopic, currentStateFunc)
	if err != nil {
		currentState = "failed setup"
		bar.Abort(false)
		err = fmt.Errorf("setup failure")
		return err
	}

	// 0. Create a new BusinessObject record
	BusinessObject := BuildJcBusinessObjectMasterMc()

	// 1. Setup test requested read record.
	expectedValueIndex := map[string]interface{}{
		"KeyMap.Field1": BusinessObject.Field1,
		"KeyMap.Field2": BusinessObject.Field2,
		"EventType":     "CREATED",
	}
	expectedKey := map[string]interface{}{
		etlcore.SociiKeyField: sociiID,
	}
	bar.IncrBy(25)
	// 2. Lookup existing record from database and exit with error if found
	_, err = models.JcBusinessObjectMasterMcByField1Field2(spectrumConn,
		expectedValueIndex["KeyMap.Field1"].(string), //
		expectedValueIndex["KeyMap.Field2"].(string), //
	)
	if err == nil {
		currentState = "failed missing record"
		bar.Abort(false)
		err = fmt.Errorf("record already exists in database")
		return err
	}

	// 3. Make sure parent exists
	_, err = models.BusinessObjectByField1Field2(spectrumConn,
		BusinessObject.Field1,
		BusinessObject.Field2,
	)
	if err != nil {
		currentState = "failed missing parent record"
		bar.Abort(false)
		etlcore.LogError(fmt.Sprintf("Parent does not exist in database. Error: %v", err))
		return err
	}

	// 4. Update expected value.
	BusinessObject.Comment = strconv.FormatInt(time.Now().UnixNano(), 10)
	expectedValue := map[string]interface{}{
		"Field3": BusinessObject.Field3,
	}

	// 5. Kick off an asynchronous test.
	var resultError error
	if !kafkatesting.GetPlugin() {
		etlcore.LogError(fmt.Sprintf("%s Going to kafka.", sociiID))
	}
	wg := kafkatesting.TestExpected(reader, "Failure to find expected message.", expectedKey, expectedValueIndex, expectedValue, func(err error) {
		if err != nil {
			etlcore.LogError(fmt.Sprintf("%s Kafka error.  %v", sociiID, err))
			currentState = "failed to connect to kafka"
			resultError = err
		}
	})

	// 8. Insert existing record and cleanup.
	etlcore.LogError(fmt.Sprintf("%s Inserting to database", sociiID))
	currentState = "dbinit"
	insertError := BusinessObject.Insert(spectrumConn)
	if insertError != nil {
		currentState = "failed insert record"
		bar.Abort(false)
		etlcore.LogError(fmt.Sprintf("%s Database insert error.", sociiID))
		return insertError
	}

	spectrumConn.Close()
	currentState = "dbupdated"

	bar.IncrBy(25)
	// 7. Clean up.
	var p Pool
	defer p.CleanBusinessObject()
	etlcore.LogError(fmt.Sprintf("%s Database insert complete", sociiID))

	// 8. Wait for result.
	wg.Wait()
	if resultError != nil {
		bar.IncrBy(50)
		currentState = "failed"
		bar.Abort(false)
	} else {
		bar.IncrBy(50)
		currentState = "complete"
		bar.Completed()
	}
	time.Sleep(100 * time.Millisecond)
	return resultError
}

// UpdateBusinessObject -- updates and reads an BusinessObject.
func UpdateBusinessObject() error {
	currentState := "init"
	currentStateFunc := func(s decor.Statistics) string {
		return currentState
	}
	reader, bar, sociiID, _, spectrumConn, err := kafkatesting.KafkaTestInit(&currentState, BusinessObjectTopic, currentStateFunc)
	if err != nil {
		currentState = "failed setup"
		bar.Abort(false)
		err = fmt.Errorf("setup failure")
		return err
	}
	// expectedValueIndex["KeyMap.Field1"].(string), //
	// expectedValueIndex["KeyMap.Field2"].(string), //

	// 1. Setup test requested read record.
	expectedValueIndex := map[string]interface{}{
		"KeyMap.Field1": "Field1",
		"KeyMap.Field2": "Field2",
		"EventType":     "UPDATED",
	}
	expectedKey := map[string]interface{}{
		etlcore.SociiKeyField: sociiID,
	}

	// 2. Lookup existing record from database.
	BusinessObject, err := models.BusinessObjectByField1Field2(spectrumConn,
		expectedValueIndex["KeyMap.Field1"].(string), // Field1 string,
		expectedValueIndex["KeyMap.Field2"].(string), // Field2 string,
	)
	if err != nil {
		currentState = "failed missing record"
		bar.Abort(false)
		return err
	}

	// 3. Make a change
	BusinessObject.Description = strconv.FormatInt(time.Now().UnixNano(), 10)
	bar.IncrBy(25)

	// 4. Update expected value.
	expectedValue := map[string]interface{}{
		"BusinessObjectDescription": BusinessObject.Description,
	}

	// 5. Kick off an asynchronous test.
	var resultError error
	if !kafkatesting.GetPlugin() {
		etlcore.LogError(fmt.Sprintf("%s Going to kafka.", sociiID))
	}
	wg := kafkatesting.TestExpected(reader, "Failure to find expected BusinessObject.", expectedKey, expectedValueIndex, expectedValue, func(err error) {
		if err != nil {
			etlcore.LogError(fmt.Sprintf("%s Kafka error.  %v", sociiID, err))
			resultError = err
		} else {
			etlcore.LogError(fmt.Sprintf("%s Kafka successful test result.", sociiID))
		}
	})

	// 6. Update existing record in database and cleanup.
	etlcore.LogError(fmt.Sprintf("%s Updating database", sociiID))
	currentState = "dbinit"
	updateError := BusinessObject.Update(spectrumConn)
	spectrumConn.Close()
	if updateError != nil {
		currentState = "failed update record"
		bar.Abort(false)
		etlcore.LogError(fmt.Sprintf("%s Database update error.", sociiID))
		return updateError
	}
	etlcore.LogError(fmt.Sprintf("%s Database update complete", sociiID))

	// 7. Wait for result
	currentState = "dbupdated"
	bar.IncrBy(25)
	wg.Wait()
	if resultError != nil {
		bar.IncrBy(50)
		currentState = "failed"
		bar.Abort(false)
	} else {
		bar.IncrBy(50)
		currentState = "complete"
		bar.Completed()
	}
	time.Sleep(100 * time.Millisecond)
	return resultError
}

// CleanBusinessObject is used to clean up BusinessObject records made from our tests
func (p *Pool) CleanBusinessObject() error {
	currentState := "clean"
	currentStateFunc := func(s decor.Statistics) string {
		return currentState
	}
	// 1. Setup connections to database and kafka.
	_, bar, sociiID, _, spectrumConn, err := kafkatesting.KafkaTestInit(&currentState, "", currentStateFunc)
	if err != nil {
		currentState = "Clean failed setup"
		bar.Abort(false)
		err = fmt.Errorf("clean setup failure")
		return err
	}

	// 2. Delete added record
	etlcore.LogError(fmt.Sprintf("%s Deleting database records", sociiID))
	rangeLength := strconv.Itoa(len(strconv.Itoa(BusinessObjectRangeMax)))
	from := fmt.Sprintf(BusinessObjectPrefix+"%0"+rangeLength+"d", BusinessObjectRangeMin)
	to := fmt.Sprintf(BusinessObjectPrefix+"%0"+rangeLength+"d", BusinessObjectRangeMax)
	BusinessObject := BuildJcBusinessObjectMasterMc()
	deleteError := BusinessObject.DeleteRange(spectrumConn, from, to)
	spectrumConn.Close()
	if deleteError != nil {
		etlcore.LogError(fmt.Sprintf("%s Database delete error.", sociiID))
		bar.IncrBy(50)
		return deleteError
	}
	etlcore.LogError(fmt.Sprintf("%s Delete database records complete", sociiID))
	bar.IncrBy(75)
	bar.Completed()
	time.Sleep(100 * time.Millisecond)
	return nil
}
