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
	if schemaSource == "" {
		return nil
	}

	var tlsConfig *tls.Config
	if len(schemaCert) > 0 {
		caCertPool := x509.NewCertPool()
		if ok := caCertPool.AppendCertsFromPEM(schemaCert); !ok {
			// Log warning - certificates may be invalid but continue
		}
		tlsConfig = &tls.Config{RootCAs: caCertPool, MinVersion: tls.VersionTLS12}
	} else {
		// Use system root CAs
		tlsConfig = &tls.Config{MinVersion: tls.VersionTLS12}
	}

	httpsClientTransport := &http.Transport{
		TLSClientConfig: tlsConfig,
	}

	httpsClient := &http.Client{
		Transport: httpsClientTransport,
	}

	schemaClient, err := schemaregistry.NewClientWithBasicAuth(schemaSource, schemaUser, schemaPassword, schemaregistry.UsingClient(httpsClient))
	if err != nil {
		return nil
	}

	return &SchemaManager{SchemaClient: schemaClient}
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
