//go:build testrunner
// +build testrunner

package testopts

import (
	testtcutil "VaultConfig.TenantConfig/util/core"
)

func GetTestConfig(token string, wantPluginPaths bool) map[string]interface{} {
	return testtcutil.GetTestConfig(token, wantPluginPaths)
}
