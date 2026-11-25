package kafka

import (
	"errors"
	"reflect"
	"testing"

	"github.com/twmb/franz-go/pkg/kgo"
)

func TestRawReaderRejectsCachedConsumerOptions(t *testing.T) {
	topic := "raw-reader-cache-test"
	cacheKey := GetCacheKey(topic, false)
	reader := &SeededKafkaReader{}
	kafkaReaderCache.Store(cacheKey, reader)
	t.Cleanup(func() { kafkaReaderCache.Delete(cacheKey) })
	_, err := NewKafkaRecordReaderWithConfig(nil, []string{topic, "avro"}, "new-group", nil, kgo.NewOffset().AtStart(), func(*kgo.Record) error {
		return nil
	})
	if err == nil {
		t.Fatal("cached reader silently retained old consumer options")
	}
	if reader.HandleRawRecordFunc != nil {
		t.Fatal("rejected registration replaced the existing reader handler")
	}
}

func TestRawRecordsPreserveOrderAndDoNotCommitFailures(t *testing.T) {
	records := []*kgo.Record{{Offset: 1}, {Offset: 2}, {Offset: 3}}
	var operations []string
	failure := errors.New("response publication failed")
	err := processRawRecordsBeforeCommit(records, func(record *kgo.Record) error {
		switch record.Offset {
		case 1:
			operations = append(operations, "process-1")
			return nil
		case 2:
			operations = append(operations, "process-2")
			return failure
		default:
			t.Fatal("a later record overtook a failed record")
			return nil
		}
	}, func(records ...*kgo.Record) error {
		if len(records) != 1 || records[0].Offset != 1 {
			t.Fatal("a failed record was committed")
		}
		operations = append(operations, "commit-1")
		return nil
	})
	if !errors.Is(err, failure) || !reflect.DeepEqual(operations, []string{"process-1", "commit-1", "process-2"}) {
		t.Fatalf("operations = %v, error = %v", operations, err)
	}
}
