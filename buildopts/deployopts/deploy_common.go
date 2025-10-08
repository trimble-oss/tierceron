package deployopts

import (
	"fmt"
	"strings"

	"github.com/trimble-oss/tierceron-succinctly/succinctly"
)

// InitSupportedDeployers - initializes a list of supported deployers.  These are the
// plugins defined under: super-secrets/Index/TrcVault/trcplugin/
// where the trctype is defined as trcshservice.
func InitSupportedDeployers(supportedDeployers []string) []string {
	succinctly.Init(supportedDeployers, -1, -1)
	return supportedDeployers
}

// GetDecodedDeployerId - by default provides the decoding utilizing
// the simple succinctly library decoding.
// This code is used in trcsh communications
// Override if you wish to provide a different encoding.
func GetDecodedDeployerId(deployerCode string) (string, bool) {
	deployerIdParts := strings.Split(deployerCode, ".")
	if len(deployerIdParts) != 2 {
		return "", false
	}
	if word, ok := succinctly.QWord(deployerIdParts[0]); ok {
		return fmt.Sprintf("%s.%s", word, deployerIdParts[1]), true
	} else {
		return "", false
	}
}

// GetEncodedDeployerId - by default provides the encoding utilizing
// the simple succinctly library encoding.
// This code is used in trcsh communications
// Override if you wish to provide a different encoding.
func GetEncodedDeployerId(deployer string, env string) (string, bool) {
	if code, ok := succinctly.QCode(deployer); ok {
		env = strings.Split(env, "_")[0]
		return fmt.Sprintf("%s.%s", code, env), true
	} else {
		return "", false
	}
}
