//go:build trcname
// +build trcname

package util

import (
	tvUtils "VaultConfig.TenantConfig/util/buildtrcprefix"
)

func GetFolderPrefix() string {
	return tvUtils.GetFolderPrefix()
}
