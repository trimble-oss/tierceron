//go:build testrunner
// +build testrunner

package buildopts

import (
	testtcutil "VaultConfig.TenantConfig/util"
)

func GetTestConfig(token string, wantPluginPaths bool) map[string]interface{} {
	return testtcutil.GetTestConfig(token, wantPluginPaths)
}
