package trcconfigbase

import (
	"flag"
	"testing"

	"github.com/trimble-oss/tierceron/pkg/core"
	eUtils "github.com/trimble-oss/tierceron/pkg/utils"
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

	driverConfig := eUtils.DriverConfig{
		CoreConfig: core.CoreConfig{
			TokenPtr:      &tokenPtr,
			ExitOnFailure: true,
		},
	}

	err := CommonMain(&envPtr, &addrPtr, &envCtxPtr, &secretIDPtr, &appRoleIDPtr, &tokenNamePtr, &regionPtr, flagset, argLines, &driverConfig)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	// Add assertions here based on the expected behavior of CommonMain
}
