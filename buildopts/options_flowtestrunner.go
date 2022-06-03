//go:build testrunner
// +build testrunner

package buildopts

import (
	flowcore "tierceron/trcflow/core"

	testtcutil "VaultConfig.Test/util"
)

func GetTestConfig(token string, wantPluginPaths bool) map[string]interface{} {
	return tcutil.GetTestConfig(token, wantPluginPaths)
}
