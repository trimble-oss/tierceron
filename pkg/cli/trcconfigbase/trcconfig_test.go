package trcconfigbase

import (
	"flag"
	"fmt"
	"testing"

	"github.com/trimble-oss/tierceron/pkg/core"
	trcshcache "github.com/trimble-oss/tierceron/pkg/core/cache"
	eUtils "github.com/trimble-oss/tierceron/pkg/utils"
	"github.com/trimble-oss/tierceron/pkg/utils/config"
)

func TestCommonMain(t *testing.T) {
	envPtr := "test env"
	addrPtr := "test addr"
	tokenPtr := "test token"
	envCtxPtr := "test envCtx"
	secretIDPtr := "test secretID"
	appRoleIDPtr := "test appRoleID"
	tokenNamePtr := "test tokenName"
	regionPtr := "test region"
	flagset := &flag.FlagSet{}
	argLines := []string{"arg1", "arg2"}

	driverConfig := config.DriverConfig{
		CoreConfig: &core.CoreConfig{
			TokenCache:    trcshcache.NewTokenCache(fmt.Sprintf("config_token_%s", envPtr), &tokenPtr, &addrPtr),
			ExitOnFailure: true,
		},
	}

	if eUtils.RefLength(&appRoleIDPtr) > 0 && eUtils.RefLength(&secretIDPtr) > 0 {
		roleSlice := []string{appRoleIDPtr, secretIDPtr}
		driverConfig.CoreConfig.TokenCache.AddRole("bamboo", &roleSlice)
	}

	err := CommonMain(&envPtr, &envCtxPtr, &tokenNamePtr, &regionPtr, flagset, argLines, &driverConfig)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	// Add assertions here based on the expected behavior of CommonMain
}
