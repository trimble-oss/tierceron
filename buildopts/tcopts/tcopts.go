package tcopts

import (
	"encoding/base64"
	"errors"
	"strings"
)

//	"time"

// Whether to perform additional processing via the CheckFlowDataIncoming function...
// This flow logic section is essentially used to help decide whether a row in a table
// in trcdb has changed or not and requires serialization to the backend secret store.
func CheckIncomingColumnName(col string) bool {
	return false
}

// Provide a means to decrypt and compare a TrcDb table having encoded secrets in backend secret store.
// If the secretValue has the key "TierceronBase64" as a prefix
func CheckFlowDataIncoming(secretColumns map[string]string, secretValue string, dbName string, tableName string) ([]byte, string, string, string, error) {
	var decodeErr error
	var decodedValue []byte
	if strings.HasPrefix(string(secretValue), "TierceronBase64") {
		secretValue = secretValue[len("TierceronBase64"):]
		decodedValue, decodeErr = base64.StdEncoding.DecodeString(string(secretValue))
		if decodeErr != nil {
			return nil, "", "", "", decodeErr
		}
	} else {
		return nil, "", "", "", errors.New("Decoding not implemented.")
	}
	return decodedValue, "", "", "", nil
}

// CheckIncomingAliasColumnName - used to identify if the supplied col column name matches a user defined alias.
func CheckIncomingAliasColumnName(col string) bool {
	return col == "flowAlias"
}

// GetTrcDbUrl - Utilized by speculatio/fenestra to obtain a jdbc compliant connection url to the TrcDb database
// This can be used to perform direct queries against the TrcDb database using the go sql package.
// The data map is provided by the caller as convenience to provide things like dbport, etc...
// The override should return a jdbc compliant connection url to the TrcDb database.
func GetTrcDbUrl(data map[string]interface{}) string {
	return ""
}
