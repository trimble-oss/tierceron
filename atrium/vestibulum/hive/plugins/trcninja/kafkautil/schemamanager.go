package kafkautil

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"net/http"

	schemaregistry "github.com/wildbeavers/schema-registry" //github.com/landoop/schema-registry
)

type SchemaManager struct {
	SchemaClient *schemaregistry.Client
}

func InitSchemaManager(schemaCert []byte, schemaSource string, schemaUser string, schemaPassword string) *SchemaManager {
	var schemaManager SchemaManager

	if len(schemaCert) == 0 {
		// Log error but continue with empty cert pool
		// This allows for development/testing scenarios
	}

	caCertPool := x509.NewCertPool()
	if len(schemaCert) > 0 {
		if ok := caCertPool.AppendCertsFromPEM(schemaCert); !ok {
			// Log warning - certificates may be invalid but continue
		}
	}

	tlsConfig := &tls.Config{
		RootCAs:            caCertPool,
		InsecureSkipVerify: true,
	}

	httpsClientTransport := &http.Transport{
		TLSClientConfig: tlsConfig,
	}

	httpsClient := &http.Client{
		Transport: httpsClientTransport,
	}

	// Create the Schema Registry client - now capture error
	var err error
	schemaManager.SchemaClient, err = schemaregistry.NewClientWithBasicAuth(schemaSource, schemaUser, schemaPassword, schemaregistry.UsingClient(httpsClient))
	if err != nil {
		// Return manager with nil client - callers should check before use
		return &schemaManager
	}

	return &schemaManager
}

// LoadSchema - loads provided schema
func (schemaManager *SchemaManager) LoadSchema(schemaSubject string, version int) (schemaregistry.Schema, error) {
	if schemaManager == nil {
		return schemaregistry.Schema{}, fmt.Errorf("schemaManager is nil")
	}

	if schemaManager.SchemaClient == nil {
		return schemaregistry.Schema{}, fmt.Errorf("schemaClient is nil")
	}

	if schemaSubject == "" {
		return schemaregistry.Schema{}, fmt.Errorf("schemaSubject is empty")
	}

	//	subjects, _ := schemaManager.schemaClient.Subjects()
	if version > 1 {
		return schemaManager.SchemaClient.GetSchemaBySubject(schemaSubject, version)
	} else {
		return schemaManager.SchemaClient.GetLatestSchema(schemaSubject)
	}
}
