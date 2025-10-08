package kafkatesting

import (
	"encoding/json"
	"fmt"
	"math/big"
	"strconv"
	"strings"
	"time"

	"github.com/segmentio/kafka-go"
	etlcore "github.com/trimble-oss/tierceron/atrium/vestibulum/hive/plugins/trcninja/core"
)

// FilterByKeyMap - Ensure we only look at messages for a specific key.
func (r *SeededKafkaReader) FilterByKeyMap(kafkaKey map[string]interface{}) bool {
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
					if avi, err := strconv.ParseInt(av.(string), 10, 32); err != nil {
						etlcore.LogError(fmt.Sprintf("Unexpected non int32 expected key: %d\n", ev))
						noMatch = true
						break
					} else {
						if int32(avi) != int32(i) {
							// Enable for debugging
							// etlcore.LogError(fmt.Sprintf("Unexpected match failure got %v: expected value: %v\n", av, i))
							noMatch = true
							break
						}
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
func (r *SeededKafkaReader) FindByJsonKeyIndex(messageTime time.Time, kafkaKey map[string]interface{}, kafkaLogicalKey map[string]interface{}) *KafkaTestBundle {
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
				var i, avi int64
				var err error
				if i, err = strconv.ParseInt(ev.(string), 10, 32); err != nil {
					noMatch = true
					break
				}

				if avi, err = strconv.ParseInt(av.(string), 10, 32); err != nil {
					noMatch = true
					break
				}

				if int32(avi) != int32(i) {
					noMatch = true
					break
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
					etlcore.LogError(fmt.Sprintf("JSON package missing expected logical key: %s\n", eki))
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

func (r *SeededKafkaReader) ProcessMessageJSON(m *kafka.Message) {
	if !plugin {
		etlcore.LogError(fmt.Sprintf("message at topic/partition/offset %v/%v/%v: %s = %s\n", m.Topic, m.Partition, m.Offset, string(m.Key), string(m.Value)))
	}
	// TODO: Implement testExpected for JSON output.
	var kafkaKey map[string]interface{}
	json.Unmarshal(m.Key, &kafkaKey)

	if !r.FilterByKeyMap(kafkaKey) {
		// sociiId mismatch for tests we are running.
		// Enable for debugging
		// etlcore.LogError(fmt.Sprintf("Mismatched socii: %v", kafkaKey))
		return
	}

	var kafkaValue map[string]interface{}
	json.Unmarshal(m.Value, &kafkaValue)

	kafkaTestBundle := r.FindByJsonKeyIndex(m.Time, kafkaKey, kafkaValue)
	if kafkaTestBundle == nil {
		// sociiId mismatch for tests we are running.
		if !plugin {
			etlcore.LogError(fmt.Sprintf("Couldn't find bundle for keyset: %v", kafkaKey))
		}
		return
	}

	expectedValue := kafkaTestBundle.ExpectedValue
	var err error

	// Matching index.  Now compare the changes.
	for evk, evv := range expectedValue {
		if av, aoki := kafkaValue[evk]; aoki {
			if actualDecimal, decimalOk := av.(*big.Rat); decimalOk {
				if evv.(*big.Rat).Cmp(actualDecimal) != 0 {
					etlcore.LogError(fmt.Sprintf("Failure: Key: %s Value: decimal value mismatch expected: %v actual: %v", evk, evv, actualDecimal))
					err = fmt.Errorf("Failure: Key: %s Value: decimal value mismatch expected: %v actual: %v", evk, evv, actualDecimal)
					break
				}
			} else if actualString, stringOk := av.(string); stringOk {
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
			} else if actualTime, timeOk := av.(time.Time); timeOk {
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
		} else {
			err = fmt.Errorf("JSON package missing expected value key %v", evk)
			break
		}
	}

	// etlcore.LogError(fmt.Sprintf("%d %s", kafkaKey[etlcore.SociiKeyField].(int32), m.Time.UTC().Format(time.UnixDate)))

	if err != nil {
		etlcore.LogError(fmt.Sprintf("Failure to parse kafka item.  Ending error: %v", err))
		r.kafkaTestBundleLock.Lock()
		kafkaTestBundle.SuccessFun(err)
		r.kafkaTestBundleLock.Unlock()
		r.DeleteTest(kafkaTestBundle)
		if r.CountTest() == 0 {
			// All done.
			r.Close()
		}
	} else {
		// Actual success.
		r.kafkaTestBundleLock.Lock()
		kafkaTestBundle.SuccessFun(nil)
		r.kafkaTestBundleLock.Unlock()

		r.DeleteTest(kafkaTestBundle)
		if r.CountTest() == 0 {
			// All done.
			r.Close()
		}
	}
}
