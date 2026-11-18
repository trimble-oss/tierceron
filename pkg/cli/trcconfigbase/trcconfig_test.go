package trcconfigbase

import (
	"flag"
	"fmt"
	"testing"

	"github.com/trimble-oss/tierceron-core/v2/core/coreconfig"
	trcshcache "github.com/trimble-oss/tierceron-core/v2/core/coreconfig/cache"
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

	tokenCache, err := trcshcache.NewTokenCache(fmt.Sprintf("config_token_%s", envPtr), &tokenPtr, &addrPtr)
	if err != nil {
		t.Fatal(err)
	}
	driverConfig := config.DriverConfig{
		CoreConfig: &coreconfig.CoreConfig{
			TokenCache:    tokenCache,
			ExitOnFailure: true,
		},
	}

	if eUtils.RefLength(&appRoleIDPtr) > 0 && eUtils.RefLength(&secretIDPtr) > 0 {
		roleSlice := []string{appRoleIDPtr, secretIDPtr}
		if err := driverConfig.CoreConfig.TokenCache.AddRole("bamboo", &roleSlice); err != nil {
			t.Fatal(err)
		}
	}

	err = CommonMain(&envPtr, &envCtxPtr, &tokenNamePtr, &regionPtr, flagset, argLines, &driverConfig)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	// Add assertions here based on the expected behavior of CommonMain
}
