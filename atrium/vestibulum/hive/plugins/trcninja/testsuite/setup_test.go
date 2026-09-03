package testsuite

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"reflect"
	"regexp"
	"testing"

	"github.com/trimble-oss/tierceron/atrium/vestibulum/hive/plugins/trcninja/mysqldbutil"

	etlcore "github.com/trimble-oss/tierceron/atrium/vestibulum/hive/plugins/trcninja/core"
	"github.com/trimble-oss/tierceron/atrium/vestibulum/hive/plugins/trcninja/performancetesting"
)

// Variable used by command line run unit tests.
var ninjaTenantID string

func TestMain(m *testing.M) {
	ninjaTenantID = os.Getenv("NINJA_TENANT_ID")

	var pool Pool
	// Write code here to run before tests
	cleanupMode := performancetesting.InitKafka(&pool, mysqldbutil.OpenIndirectConnection)

	if cleanupMode {
		err := PoolCleanup()
		if err != nil {
			etlcore.LogError(err.Error()) // This is a top level function
			return
		}
	}

	// Run tests
	exitVal := m.Run()

	// Write code here to run after tests

	// Exit with exit value from tests
	os.Exit(exitVal)
}

func PoolCleanup() error {
	poolCleanerRegex := "^Clean.*"
	set := token.NewFileSet()
	packs, err := parser.ParseDir(set, ".", nil, 0)
	if err != nil {
		return err
	}
	var pool Pool

	for _, pack := range packs {
		for _, f := range pack.Files {
			for _, d := range f.Decls {
				if fn, isFn := d.(*ast.FuncDecl); isFn {
					if fnMatchOk, _ := regexp.MatchString(poolCleanerRegex, fn.Name.Name); fnMatchOk {
						reflect.ValueOf(&pool).MethodByName(fn.Name.Name).Call([]reflect.Value{})
					}
				}
			}
		}
	}
	return nil
}
