package initlib

import (
	"errors"
	"fmt"

	eUtils "github.com/trimble-oss/tierceron/utils"
	"github.com/trimble-oss/tierceron/validator"
	helperkv "github.com/trimble-oss/tierceron/vaulthelper/kv"
)

// Runs the verification step from data in the seed file
// v is the data contained under the "verification:" tag
// Service name should match credentials in super-secrets
// to verify
// Example
// SpectrumDB:
// 	type: db
// SendGrid:
//	type: SendGridKey
// KeyStore:
// 	type: KeyStore

func verify(config *eUtils.DriverConfig, mod *helperkv.Modifier, v map[interface{}]interface{}) ([]string, error) {
	var isValid bool
	var path string
	config.Log.SetPrefix("[VERIFY]")

	for service, info := range v {
		vType := info.(map[interface{}]interface{})["type"].(string)
		serviceData, err := mod.ReadData("super-secrets/" + service.(string))
		if err != nil {
			return nil, err
		}
		config.Log.Printf("Verifying %s as type %s\n", service, vType)
		switch vType {
		case "db":
			if url, ok := serviceData["url"].(string); ok {
				if user, ok := serviceData["user"].(string); ok {
					if pass, ok := serviceData["pass"].(string); ok {
						isValid, err = validator.Heartbeat(config, url, user, pass)
						eUtils.LogErrorObject(config, err, false)
					} else {
						eUtils.LogErrorObject(config, fmt.Errorf("password field is not a string value"), false)
					}
				} else {
					eUtils.LogErrorObject(config, fmt.Errorf("username field is not a string value"), false)
				}
			} else {
				eUtils.LogErrorObject(config, fmt.Errorf("URL field is not a string value"), false)
			}
		case "SendGridKey":
			if key, ok := serviceData["SendGridApiKey"].(string); ok {
				isValid, err = validator.ValidateSendGrid(key)
				eUtils.LogErrorObject(config, err, false)
			}
		case "KeyStore":
			// path := serviceData["path"].(string)
			// pass := serviceData["pass"].(string)
			isValid = false
		default:
			return nil, errors.New("Invalid verification type: " + vType)
		}

		// Log verification status and write to vault
		config.Log.Printf("\tverified: %v\n", isValid)
		path = "verification/" + service.(string)
		warn, err := mod.Write(path, map[string]interface{}{
			"type":     vType,
			"verified": isValid,
		}, config.Log)
		if len(warn) > 0 || err != nil {
			return warn, err
		}
	}
	return nil, nil
}
