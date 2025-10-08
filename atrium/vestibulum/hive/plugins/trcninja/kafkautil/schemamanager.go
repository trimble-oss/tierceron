package kafkautil

import (
	"crypto/tls"
	"crypto/x509"
	"net/http"

	schemaregistry "github.com/wildbeavers/schema-registry" //github.com/landoop/schema-registry
)

type SchemaManager struct {
	SchemaClient *schemaregistry.Client
}

func InitSchemaManager(schemaCert []byte, schemaSource string, schemaUser string, schemaPassword string) *SchemaManager {
	var schemaManager SchemaManager

	caCertPool := x509.NewCertPool()
	caCertPool.AppendCertsFromPEM(schemaCert)

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

	// Create the Schema Registry client
	schemaManager.SchemaClient, _ = schemaregistry.NewClientWithBasicAuth(schemaSource, schemaUser, schemaPassword, schemaregistry.UsingClient(httpsClient))

	return &schemaManager
}

// LoadSchema - loads provided schema
func (schemaManager *SchemaManager) LoadSchema(schemaSubject string, version int) (schemaregistry.Schema, error) {
	//	subjects, _ := schemaManager.schemaClient.Subjects()
	if version > 1 {
		return schemaManager.SchemaClient.GetSchemaBySubject(schemaSubject, version)
	} else {
		return schemaManager.SchemaClient.GetLatestSchema(schemaSubject)
	}
}
