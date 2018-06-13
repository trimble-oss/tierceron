package seeder

import (
	"bitbucket.org/dexterchaney/whoville/validator"
	"bitbucket.org/dexterchaney/whoville/vault-helper/kv"
	"errors"
	"log"
	"os"
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

func verify(mod *kv.Modifier, v map[interface{}]interface{}, logFile *os.File) ([]string, error) {
	var isValid bool
	var path string
	log.SetOutput(logFile)
	log.SetPrefix("Verifier: ")

	for service, info := range v {
		vType := info.(map[interface{}]interface{})["type"].(string)
		serviceData, err := mod.ReadData("super-secrets/" + service.(string))
		if err != nil {
			return nil, err
		}
		log.Printf("Verifying %s as type %s\n", service, vType)
		switch vType {
		case "db":
			url := serviceData["url"].(string)
			user := serviceData["user"].(string)
			pass := serviceData["pass"].(string)
			isValid, err = validator.Heartbeat(url, user, pass)
			if err != nil {
				return nil, err
			}
		case "SendGridKey":
			key := serviceData["ApiKey"].(string)
			isValid, err = validator.ValidateSendGrid(key)
			if err != nil {
				return nil, err
			}
		case "KeyStore":
			// path := serviceData["path"].(string)
			// pass := serviceData["pass"].(string)
			isValid = false
		default:
			return nil, errors.New("Invalid verification type: " + vType)
		}

		// Log verification status and write to vault
		log.Printf("\tverified: %v\n", isValid)
		path = "verification/" + service.(string)
		warn, err := mod.Write(path, map[string]interface{}{
			"type":     vType,
			"verified": isValid,
		})
		if len(warn) > 0 || err != nil {
			return warn, err
		}
	}
	return nil, nil
}
