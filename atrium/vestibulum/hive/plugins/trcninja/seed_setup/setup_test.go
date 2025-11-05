//go:build seedsetup
// +build seedsetup

package seed_setup

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"log"
	"os"
	"reflect"
	"regexp"
	"testing"

	// "github.com/trimble-oss/tierceron/atrium/vestibulum/trcdb/opts/insecure"

	etlcore "github.com/trimble-oss/tierceron/atrium/vestibulum/hive/plugins/trcninja/core"
	"github.com/trimble-oss/tierceron/atrium/vestibulum/hive/plugins/trcninja/performancetesting"
	// eUtils "github.com/trimble-oss/tierceron/pkg/utils"
)

func TestMain(m *testing.M) {
	var pool Pool
	// Write code here to run before tests
	cleanupMode := performancetesting.InitKafka(&pool, nil)

	if cleanupMode {
		PoolCleanup()
		return
	}

	// Run tests
	exitVal := m.Run()

	// Write code here to run after tests

	// Exit with exit value from tests
	os.Exit(exitVal)
}

func PoolCleanup() {
	poolCleanerRegex := "^Clean.*"
	set := token.NewFileSet()
	packs, err := parser.ParseDir(set, ".", nil, 0)
	if err != nil {
		etlcore.LogError(fmt.Sprintf("Failed to parse package: %v", err))
		f, err := os.OpenFile("./etlninja.log", os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0644)
		if err != nil {
			fmt.Fprintln(os.Stderr, "Log setup failure.")
		}
		etlcore.SetLogger(log.New(f, "[etlninja]", log.LstdFlags))
		// eUtils.CheckError(&eUtils.DriverConfig{Insecure: insecure.IsInsecure(), Log: logger}, err, false)
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
}
