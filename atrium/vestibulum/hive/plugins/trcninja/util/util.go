package util

import (
	"context"
	"fmt"
	"runtime"
	"strings"
	"time"

	etlcore "github.com/trimble-oss/tierceron/atrium/vestibulum/hive/plugins/trcninja/core"
	"github.com/trimble-oss/tierceron/atrium/vestibulum/hive/plugins/trcninja/kafkatesting"
)

func TestSequenceBundleBuilder(expeditionID string,
	topicSequence [][]string,
	readerSequence []*kafkatesting.SeededKafkaReader,
	expectedLogicalKey map[string]interface{},
	expectedLogicalValue map[string]interface{},
	successFun func(err error),
) []*kafkatesting.KafkaTestBundle {
	kafkaTestSequence := []*kafkatesting.KafkaTestBundle{}

	agnosticExpectedLogicKey := map[string]interface{}{}
	for key, value := range expectedLogicalKey {
		if key != "EventType" {
			agnosticExpectedLogicKey["ErpKeyMapping."+key] = value
		} else {
			agnosticExpectedLogicKey[key] = value
		}
	}

	agnosticExpectedLogicValue := map[string]interface{}{}
	for key, value := range expectedLogicalValue {
		agnosticExpectedLogicValue[key] = value
	}

	baseTestBundle := kafkatesting.KafkaTestBundle{
		Name:             "",
		CompletionStatus: "",
		Message:          "Failure to find expected message.",
		ExpectedAvroKey: map[string]interface{}{
			etlcore.SociiKeyField: expeditionID,
		},
		ExpectedLogicalKey: agnosticExpectedLogicKey,
		ExpectedValue:      agnosticExpectedLogicValue,
		SuccessFun:         successFun,
	}
	//	kafkaTestSequence = append(kafkaTestSequence, &baseTestBundle)

	for _, reader := range readerSequence {
		switch reader.TopicName {
		case topicSequence[0][0]:
			rawTestBundle := baseTestBundle
			rawTestBundle.CompletionStatus = kafkatesting.STATE_RAWTOPIC_ARRIVAL
			rawTestBundle.ExpectedLogicalKey = expectedLogicalKey // Reset and prepare for raw
			switch expectedLogicalKey["EventType"] {
			case "UPDATED":
				rawTestBundle.ExpectedLogicalKey["DebeziumOperation"] = "UPDATE"
			case "CREATED":
				rawTestBundle.ExpectedLogicalKey["DebeziumOperation"] = "CREATE"
			case "DELETED":
				rawTestBundle.ExpectedLogicalKey["DebeziumOperation"] = "DELETE"
			}
			rawTestBundle.ExpectedValue = expectedLogicalValue
			kafkaTestSequence = append(kafkaTestSequence, &rawTestBundle)
		case topicSequence[1][0]:
			agnosticTestBundle := baseTestBundle
			agnosticTestBundle.CompletionStatus = kafkatesting.STATE_AGNOSTIC_TOPIC_ARRIVAL
			kafkaTestSequence = append(kafkaTestSequence, &agnosticTestBundle)
		}
	}
	return kafkaTestSequence
}

// CleanupCanceller - helper that calls cancel function after 30 seconds or exits on a message passed in done channel.
func CleanupCanceller() (context.Context, chan bool) {
	ctx, cancel := context.WithCancel(context.Background())
	pc := make([]uintptr, 2)
	runtime.Callers(2, pc)
	cleanFunc := runtime.FuncForPC(pc[0])

	doneChannel := make(chan bool)
	go func(cleanFuncName string, cf context.CancelFunc, dc chan bool) {
		funcParts := strings.Split(cleanFuncName, ".")
		testName := funcParts[len(funcParts)-1]
		testName = strings.Replace(testName, "clean", "", 1)
		select {
		case <-time.After(time.Second * 30):
			etlcore.LogError(fmt.Sprintf("Timing out test cleanup for test: %s\n", testName))
			cf()
		case <-doneChannel:
		}
	}(cleanFunc.Name(), cancel, doneChannel)

	return ctx, doneChannel
}
