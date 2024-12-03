package main

import (
	"C"
)
import (
	"log"
	"os"

	"github.com/trimble-oss/tierceron/zeroconfiglib/zccommon"
)

//export ConfigTemplateLib
func ConfigTemplateLib(token string, address string, env string, templatePath string, configuredFilePath string, project string, service string) *C.char {
	logger := log.New(os.Stdout, "[ConfigTemplateLib]", log.LstdFlags)
	logger.Println("NCLib Version: " + "1.21")
	configuredTemplate, _, err := zccommon.ConfigCertLibHelper(token,
		address,
		env,
		templatePath,
		configuredFilePath,
		project,
		service,
		false)
	if err != nil {
		return C.CString("")
	}

	return C.CString(configuredTemplate)
}

//export ConfigCertLib
func ConfigCertLib(token string, address string, env string, templatePath string, configuredFilePath string, project string, service string) *C.char {
	logger := log.New(os.Stdout, "[ConfigCertLib]", log.LstdFlags)
	logger.Println("NCLib Version: " + "1.21")
	certBase64, _, err := zccommon.ConfigCertLibHelper(token,
		address,
		env,
		templatePath,
		configuredFilePath,
		project,
		service,
		true)
	if err != nil {
		return C.CString("")
	}

	return C.CString(certBase64)
}

func main() {}
