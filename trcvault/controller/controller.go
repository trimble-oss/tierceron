//go:build trcname
// +build trcname

package controller

import (
	tvUtils "VaultConfig.TenantConfig/util/buildtrcprefix"
)

func GetFolderPrefix() string {
	return tvUtils.GetFolderPrefix()
}
