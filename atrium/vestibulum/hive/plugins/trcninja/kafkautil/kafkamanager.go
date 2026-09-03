package kafkautil

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/binary"
	"fmt"
	"io"
	"sync"
	"time"

	etlcore "github.com/trimble-oss/tierceron/atrium/vestibulum/hive/plugins/trcninja/core"
	goavro "github.com/linkedin/goavro/v2"
	"github.com/twmb/franz-go/pkg/kgo"
	"github.com/twmb/franz-go/pkg/sasl/plain"
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
	schemaManager := InitSchemaManager(schemaCert, schemaSource, schemaUser, schemaPassword)
	if schemaManager == nil {
		return nil
	}

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

func (kafkaManager *KafkaManager) LoadSchema(schemaSubject string, version int) (schemaregistry.Schema, error) {
	if kafkaManager == nil {
		return schemaregistry.Schema{}, fmt.Errorf("kafkaManager is nil")
	}
	if kafkaManager.schemaManager == nil {
		return schemaregistry.Schema{}, fmt.Errorf("schemaManager is nil")
	}
	return kafkaManager.schemaManager.LoadSchema(schemaSubject, version)
}

func (kafkaManager *KafkaManager) EncodeSubjectMessage(schemaSubject string, version int, value map[string]interface{}) ([]byte, error) {
	schema, err := kafkaManager.LoadSchema(schemaSubject, version)
	if err != nil {
		return nil, err
	}
	return EncodeConfluentAvroMessage(schema, value)
}

func EncodeConfluentAvroMessage(schema schemaregistry.Schema, value map[string]interface{}) ([]byte, error) {
	codec, err := goavro.NewCodec(schema.Schema)
	if err != nil {
		return nil, err
	}

	binaryValue, err := codec.BinaryFromNative(nil, value)
	if err != nil {
		return nil, err
	}

	payload := make([]byte, 1+4+len(binaryValue))
	binary.BigEndian.PutUint32(payload[1:5], uint32(schema.ID))
	copy(payload[5:], binaryValue)
	return payload, nil
}

func NewProducer(bootstrapServers string, kafkaUsername string, kafkaPassword string, kafkaCert []byte) (*kgo.Client, error) {
	if bootstrapServers == "" {
		return nil, fmt.Errorf("bootstrapServers is empty")
	}
	if kafkaUsername == "" {
		return nil, fmt.Errorf("kafkaUsername is empty")
	}
	if kafkaPassword == "" {
		return nil, fmt.Errorf("kafkaPassword is empty")
	}

	caPool := x509.NewCertPool()
	if len(kafkaCert) > 0 {
		caPool.AppendCertsFromPEM(kafkaCert)
	}

	client, err := kgo.NewClient(
		kgo.SeedBrokers(bootstrapServers),
		kgo.SASL(plain.Auth{
			User: kafkaUsername,
			Pass: kafkaPassword,
		}.AsMechanism()),
		kgo.DialTLSConfig(&tls.Config{RootCAs: caPool, MinVersion: tls.VersionTLS12}),
		kgo.WithLogger(kgo.BasicLogger(io.Discard, kgo.LogLevelNone, nil)),
	)
	if err != nil {
		return nil, err
	}

	return client, nil
}

func ProduceMessage(topic string, key []byte, value []byte, bootstrapServers string, kafkaUsername string, kafkaPassword string, kafkaCert []byte, timeout time.Duration) error {
	if topic == "" {
		return fmt.Errorf("topic is empty")
	}
	producer, err := NewProducer(bootstrapServers, kafkaUsername, kafkaPassword, kafkaCert)
	if err != nil {
		return err
	}
	defer producer.Close()

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	result := producer.ProduceSync(ctx, &kgo.Record{
		Topic: topic,
		Key:   key,
		Value: value,
	})
	if err := result.FirstErr(); err != nil {
		return err
	}

	return nil
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
