package kafkautil

import (
	"fmt"
	"sync"

	goavro "github.com/linkedin/goavro/v2"
	etlcore "github.com/trimble-oss/tierceron/atrium/vestibulum/hive/plugins/trcninja/core"
	schemaregistry "github.com/wildbeavers/schema-registry"
)

// GenericObject contains definition for a generic object.  TODO: maybe get rid of?
type GenericObject struct {
	schemaName       string
	schemaVersion    string
	schemaDefinition string
	avroData         []byte
}

type SchemaContainer struct {
	schema *schemaregistry.Schema
	codec  *goavro.Codec
}

type KafkaManager struct {
	schemaManager   *SchemaManager
	schemaCache     map[uint32]*SchemaContainer
	schemaCacheLock sync.RWMutex
}

// InitKafkaManager - initialize kafka with defaults.
func InitKafkaManager(schemaCert []byte, schemaSource string, schemaUser string, schemaPassword string) *KafkaManager {
	var schemaManager *SchemaManager
	schemaManager = InitSchemaManager(schemaCert, schemaSource, schemaUser, schemaPassword)

	kafkaManager := new(KafkaManager)
	kafkaManager.schemaManager = schemaManager

	kafkaManager.schemaCache = make(map[uint32]*SchemaContainer)

	return kafkaManager
}

// LoadAvroCodecByID - loads provided schema codec
func (kafkaManager *KafkaManager) LoadAvroCodecByID(schemaID uint32) (*schemaregistry.Schema, *goavro.Codec, error) {
	if kafkaManager == nil {
		return nil, nil, fmt.Errorf("kafkaManager is nil")
	}

	if kafkaManager.schemaManager == nil {
		return nil, nil, fmt.Errorf("schemaManager is nil")
	}

	if kafkaManager.schemaManager.SchemaClient == nil {
		return nil, nil, fmt.Errorf("schemaClient is nil")
	}

	var schemaSubject schemaregistry.Schema
	var schemaSubjectBody string
	var err error
	if schemaID > 0 {
		kafkaManager.schemaCacheLock.RLock()
		if schemaContainer, ok := kafkaManager.schemaCache[schemaID]; ok {
			kafkaManager.schemaCacheLock.RUnlock()
			// Defensive: validate cached values
			if schemaContainer != nil && schemaContainer.schema != nil && schemaContainer.codec != nil {
				return schemaContainer.schema, schemaContainer.codec, nil
			}
			// Cache had invalid data, continue to reload
		} else {
			kafkaManager.schemaCacheLock.RUnlock()
		}

		schemaSubjectBody, err = kafkaManager.schemaManager.SchemaClient.GetSchemaByID(int(schemaID))
	} else {
		return nil, nil, fmt.Errorf("invalid schemaID: %d", schemaID)
	}

	schemaSubject = schemaregistry.Schema{
		Schema: schemaSubjectBody,
		ID:     int(schemaID),
	}

	if err != nil {
		return nil, nil, err
	}

	codec, codecErr := goavro.NewCodec(string(schemaSubject.Schema))
	if codecErr != nil {
		return nil, nil, codecErr
	}

	var schemaContainer SchemaContainer
	schemaContainer.schema = &schemaSubject
	schemaContainer.codec = codec
	kafkaManager.schemaCacheLock.Lock()
	kafkaManager.schemaCache[schemaID] = &schemaContainer
	kafkaManager.schemaCacheLock.Unlock()

	return &schemaSubject, codec, nil
}

// DeserializeMessage - loads provided schema codec
func (kafkaManager *KafkaManager) DeserializeMessage(schemaID uint32, avroMessage []byte) (*schemaregistry.Schema, *goavro.Codec, map[string]interface{}, error) {
	var valueSchema *schemaregistry.Schema = nil
	var valueSchemaCodec *goavro.Codec = nil
	var valueCodecLoadErr error = nil
	var valueNative interface{}

	if kafkaManager == nil {
		return nil, nil, nil, fmt.Errorf("kafkaManager is nil")
	}

	valueSchema, valueSchemaCodec, valueCodecLoadErr = kafkaManager.LoadAvroCodecByID(schemaID)
	if valueCodecLoadErr != nil {
		etlcore.LogError(fmt.Sprintf("Failure %v", valueCodecLoadErr))
		return nil, nil, nil, valueCodecLoadErr
	}

	if valueSchemaCodec == nil {
		return nil, nil, nil, fmt.Errorf("valueSchemaCodec is nil for schema ID %d", schemaID)
	}

	valueNative, _, valueCodecLoadErr = valueSchemaCodec.NativeFromBinary(avroMessage)
	if valueCodecLoadErr != nil {
		etlcore.LogError(fmt.Sprintf("Falure to parse native from binary. %v", valueCodecLoadErr))
		return nil, nil, nil, valueCodecLoadErr
	}

	if valueNative == nil {
		return nil, nil, nil, fmt.Errorf("valueNative is nil after deserialization")
	}

	// Defensive: Use type assertion with ok pattern
	valueMap, ok := valueNative.(map[string]interface{})
	if !ok {
		return nil, nil, nil, fmt.Errorf("valueNative is not map[string]interface{}, got %T", valueNative)
	}

	return valueSchema, valueSchemaCodec, valueMap, valueCodecLoadErr
}
