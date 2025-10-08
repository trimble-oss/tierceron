package kafkatesting

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"math/big"
	"strconv"
	"strings"
	"time"

	"github.com/segmentio/kafka-go"
	"github.com/trimble-oss/tierceron/atrium/vestibulum/hive/plugins/trcninja/confighelper"
	etlcore "github.com/trimble-oss/tierceron/atrium/vestibulum/hive/plugins/trcninja/core"
)

// FilterByAvroKey -- Ensure we only look at messages for a specific key.  Returns true if a test is found, false otherwise.
// Presently only matches on integer index keys.  So, in this case allows
// rejection of further processing of messages based on for instance sociiId.
func (r *SeededKafkaReader) FilterByAvroKeyMap(kafkaKey map[string]interface{}) bool {
	r.kafkaTestBundleLock.RLock()
	testKeys := make([]string, 0, len(r.kafkaTestBundle))
	for k := range r.kafkaTestBundle {
		testKeys = append(testKeys, k)
	}

	r.kafkaTestBundleLock.RUnlock()

	if !plugin {
		etlcore.LogError(fmt.Sprintf("Current topic: %s has match test count: %d\n", r.TopicName, len(testKeys)))
	}

	for _, testKey := range testKeys {
		noMatch := false

		r.kafkaTestBundleLock.RLock()
		kafkaTestBundle, _ := r.kafkaTestBundle[testKey]

		r.kafkaTestBundleLock.RUnlock()

		if kafkaTestBundle == nil {
			continue
		}
		expectedAvroKey := kafkaTestBundle.ExpectedAvroKey
		for ek, ev := range expectedAvroKey {
			if av, aok := kafkaKey[ek]; aok {
				if i, err := strconv.ParseInt(ev.(string), 10, 32); err != nil {
					etlcore.LogError(fmt.Sprintf("Unexpected non int32 expected key: %d\n", ev))
					noMatch = true
					break
				} else {
					if avInt32, avOk := av.(int32); avOk {
						if avInt32 != int32(i) {
							noMatch = true
							break
						}
					} else {
						etlcore.LogError(fmt.Sprintf("Unexpected non int32 expected key value: %v\n", av))
					}
				}
			} else {
				noMatch = true
				break
			}
		}
		if !noMatch {
			return true
		}
	}

	return false
}

// FindByKeyIndex -- finds matching test by index.
func (r *SeededKafkaReader) FindByAvroKeyIndex(messageTime time.Time, kafkaKey map[string]interface{}, kafkaLogicalKey map[string]interface{}) *KafkaTestBundle {
	r.kafkaTestBundleLock.RLock()
	testKeys := make([]string, 0, len(r.kafkaTestBundle))
	for k := range r.kafkaTestBundle {
		testKeys = append(testKeys, k)
	}

	r.kafkaTestBundleLock.RUnlock()

	for _, testKey := range testKeys {
		noMatch := false

		r.kafkaTestBundleLock.RLock()
		kafkaTestBundle, _ := r.kafkaTestBundle[testKey]

		r.kafkaTestBundleLock.RUnlock()

		if kafkaTestBundle == nil {
			// Test not yet setup.
			continue
		}
		expectedAvroKey := kafkaTestBundle.ExpectedAvroKey
		expectedLogicalKey := kafkaTestBundle.ExpectedLogicalKey

		if !plugin {
			etlcore.LogError(fmt.Sprintf("%v %s %v", kafkaKey[etlcore.SociiKeyField], messageTime.UTC().Format(time.UnixDate), kafkaLogicalKey["ErpKeyMapping"]))
		}
		for ek, ev := range expectedAvroKey {
			if av, aok := kafkaKey[ek]; aok {
				if i, err := strconv.ParseInt(ev.(string), 10, 32); err != nil {
					noMatch = true
					break
				} else {
					if av.(int32) != int32(i) {
						noMatch = true
						break
					}
				}
			} else {
				noMatch = true
				break
			}
		}

		if noMatch {
			// Not this test.
			continue
		}

		for eki, evi := range expectedLogicalKey {
			if strings.Index(eki, ".") > 0 {
				ekip := strings.Split(eki, ".")
				if ekp, ekpok := kafkaLogicalKey[ekip[0]]; ekpok {
					ekpString, epkStrOk := ekp.(string)

					if epkStrOk {
						// already deserialized
						jsonMap := map[string]interface{}{}
						err := json.Unmarshal([]byte(ekpString), &jsonMap)
						if err != nil {
							noMatch = true
							break
						}
						kafkaLogicalKey[ekip[0]] = jsonMap
						ekp = jsonMap
					}
					ekpMap := ekp.(map[string]interface{})
					if av, aoki := ekpMap[ekip[1]]; aoki {
						eviString, eviStrOk := evi.(string)

						if eviStrOk {
							if av.(string) != eviString {
								noMatch = true
								break
							} else {
								// We know this key is ok... get next one.
								continue
							}
						}
					} else {
						noMatch = true
						break
					}
				}
			} else {
				if ekp, ekpok := kafkaLogicalKey[eki]; ekpok {
					if ekpMap, ekpMapOk := ekp.(map[string]interface{}); ekpMapOk {
						if ekpString, ekpStringOk := ekpMap["string"].(string); ekpStringOk {
							if evi != ekpString {
								noMatch = true
								break
							}
						} else {
							etlcore.LogError(fmt.Sprintf("Malformatted key data for key: %s\n", eki))
						}
					} else if ekpString, ekpStringOk := ekp.(string); ekpStringOk {
						if evi != ekpString {
							noMatch = true
							break
						}
					} else {
						etlcore.LogError(fmt.Sprintf("Unexpected malformatted key data for key: %s\n", eki))
					}
				} else {
					etlcore.LogError(fmt.Sprintf("Avro missing expected logical key: %s\n", eki))
				}
			}
		}

		if noMatch {
			// Not this test.
		} else {
			return kafkaTestBundle
		}
	}

	return nil
}

func (r *SeededKafkaReader) ProcessMessageAvro(m *kafka.Message) {
	avroKeyData := m.Key[5:]
	keySchemaID := binary.BigEndian.Uint32(m.Key[1:5])

	_, _, kafkaKey, err := confighelper.KafkaManager.DeserializeMessage(keySchemaID, avroKeyData)
	if err != nil {
		etlcore.LogError(fmt.Sprintf("Failure to parse message key, Schema parse error: %v", err))
		return
	}
	// etlcore.LogError(fmt.Sprintf("message at topic/partition/offset %v/%v/%v: %s = %s\n", m.Topic, m.Partition, m.Offset, string(m.Key), string(m.Value)))

	if !r.FilterByAvroKeyMap(kafkaKey) {
		// sociiId mismatch for tests we are running.
		// This is a common occurrence in a topic with a lot of data from different socii...
		// Don't log here because it clutters the logs and makes the test appear to be
		// failing..   You can uncomment for debugging sometimes.
		// etlcore.LogError(fmt.Sprintf("Mismatched socii: %v", kafkaKey))
		return
	}

	avroData := m.Value[5:]
	schemaID := binary.BigEndian.Uint32(m.Value[1:5])
	_, _, kafkaValue, err := confighelper.KafkaManager.DeserializeMessage(schemaID, avroData)
	if err != nil {
		etlcore.LogError(fmt.Sprintf("Failure to parse message value, Schema parse error: %v", err))
		return
	}

	kafkaTestBundle := r.FindByAvroKeyIndex(m.Time, kafkaKey, kafkaValue)
	if kafkaTestBundle == nil {
		// sociiId mismatch for tests we are running.
		if !plugin {
			etlcore.LogError(fmt.Sprintf("Couldn't find bundle for keyset: %v", kafkaKey))
		}
		return
	}

	expectedValue := kafkaTestBundle.ExpectedValue

	// Matching index.  Now compare the changes.
	for evk, evv := range expectedValue {
		if av, aoki := kafkaValue[evk]; aoki {
			if avMap, avOk := av.(map[string]interface{}); avOk {
				if actualDecimal, decimalOk := avMap["bytes.decimal"].(*big.Rat); decimalOk {
					if evv.(*big.Rat).Cmp(actualDecimal) != 0 {
						etlcore.LogError(fmt.Sprintf("Failure: Key: %s Value: decimal value mismatch expected: %v actual: %v", evk, evv, actualDecimal))
						err = fmt.Errorf("Failure: Key: %s Value: decimal value mismatch expected: %v actual: %v", evk, evv, actualDecimal)
						break
					}
				} else if actualString, stringOk := avMap["string"].(string); stringOk {
					if evvString, evvStrOk := evv.(string); evvStrOk {
						if evvString != actualString {
							etlcore.LogError(fmt.Sprintf("Failure: string value mismatch Key: %s Value: expected: %v actual: %v", evk, evvString, actualString))
							err = fmt.Errorf("Failure: string value mismatch Key: %s Value: expected: %v actual: %v", evk, evvString, actualString)
							break
						}
					} else {
						etlcore.LogError(fmt.Sprintf("Failure to parse string value Key: %s Value: %v", evk, evv))
						err = fmt.Errorf("Failure to parse string value Key: %s Value: %v", evk, evv)
						break
					}
				} else if actualTime, timeOk := avMap["int.date"].(time.Time); timeOk {
					if evvTime, evvTimeOk := evv.(time.Time); evvTimeOk {
						utcTime := evvTime.UTC()
						if utcTime != actualTime {
							etlcore.LogError(fmt.Sprintf("Failure: time value mismatch Key: %s Value: expected: %v actual: %v", evk, evvTime, actualTime))
							err = fmt.Errorf("Failure: time value mismatch Key: %s Value: expected: %v actual: %v", evk, evvTime, actualTime)
							break
						}
					} else {
						etlcore.LogError(fmt.Sprintf("Failure to parse Time value Key: %s Value: %v", evk, evv))
						err = fmt.Errorf("Failure to parse Time value Key: %s Value: %v", evk, evv)
						break
					}
				}
			}
		} else {
			err = fmt.Errorf("Skipping event for topic: %s.  Avro package missing expected value key %v", r.TopicName, evk)
			break
		}
	}

	// etlcore.LogError(fmt.Sprintf("%d %s", kafkaKey[etlcore.SociiKeyField].(int32), m.Time.UTC().Format(time.UnixDate)))

	if err != nil {
		etlcore.LogError(fmt.Sprintf("Failure to parse kafka item.  Ending error: %v", err))
		kafkaTestBundle.SuccessFun(err)
		r.DeleteTest(kafkaTestBundle)
		if r.CountTest() == 0 {
			// All done.
			r.Close()
		}
	} else {
		// Actual success.
		kafkaTestBundle.SuccessFun(nil)
		r.DeleteTest(kafkaTestBundle)
		if r.CountTest() == 0 {
			// All done.
			r.Close()
		}
	}
}
