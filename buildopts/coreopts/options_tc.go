//go:build tc
// +build tc

package coreopts

import (
	tcbuildopts "VaultConfig.TenantConfig/util/buildopts"
	tvUtils "VaultConfig.TenantConfig/util/buildtrcprefix"
)

func GetSyncedTables() []string {
	return tcbuildopts.GetSyncedTables()
}

func GetFolderPrefix() string {
	return tvUtils.GetFolderPrefix()
}
