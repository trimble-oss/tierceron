// Package kafkatesting provides comprehensive Kafka testing utilities with progress tracking.
//
// This package implements a testing framework for Kafka-based ETL operations, including:
//
//   - Message production and consumption with progress bars
//   - Avro and JSON message validation (avromatching.go, jsonmatching.go)
//   - Dynamic test case discovery and execution
//   - State-based test sequencing (13 state constants)
//   - Database connection pooling for test fixtures
//   - Kafka reader group management
//
// # Message Validation
//
// The package supports two message formats:
//   - Avro: Schema-based validation using Confluent Schema Registry
//   - JSON: Flexible validation with logical key matching
//
// # State Management
//
// Tests progress through 13 defined states:
//   STATE_INIT, STATE_SEED, STATE_CREATE, STATE_UPDATE, STATE_DELETE, etc.
//
// Each state represents a specific phase in the ETL test lifecycle, from initialization
// through cleanup.
//
// # Usage Example
//
//	func TestMain(m *testing.M) {
//	    exitCode := performancetesting.Setup(m, &pool, true, false)
//	    os.Exit(exitCode)
//	}
//
// The package integrates with the performancetesting package for test orchestration
// and automatic cleanup.
package kafkatesting
