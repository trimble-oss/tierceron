package performancetesting

import (
	"flag"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"reflect"
	"regexp"
	"strings"

	"github.com/trimble-oss/tierceron/atrium/vestibulum/hive/plugins/trcninja/confighelper"
	etlcore "github.com/trimble-oss/tierceron/atrium/vestibulum/hive/plugins/trcninja/core"
	"github.com/trimble-oss/tierceron/atrium/vestibulum/hive/plugins/trcninja/kafkatesting"
)

// InitKafka - initialize Kafka
func InitKafka(pool interface{}, idbFn kafkatesting.IndirectDBConnectionFunc) bool {
	var testRegex string
	var testTimeout string
	var testLog string
	var testParallel string
	var testVerbose bool
	var testBenchRegEx string
	var testBenchTime string
	var testBenchmem bool
	var testCount int
	var testList string
	var testClean bool
	var testEnv string
	var testSkipCertCache bool
	confighelper.InitCommon()
	flags := flag.NewFlagSet(os.Args[0], flag.ContinueOnError)
	flags.StringVar(&testRegex, "test.run", "", "testing regular expression")
	flags.StringVar(&testTimeout, "test.timeout", "", "testing timeout")
	flags.StringVar(&testLog, "test.testlogfile", "", "test log")
	flags.StringVar(&testParallel, "test.parallel", "", "test in parallel")
	flags.BoolVar(&testVerbose, "test.v", false, "verbose testing")
	flags.StringVar(&testBenchRegEx, "test.bench", "", "benchmark regular expression")
	flags.StringVar(&testBenchTime, "test.benchtime", "", "benchmark duration")
	flags.BoolVar(&testBenchmem, "test.benchmem", false, "print memory allocation stats")
	flags.IntVar(&testCount, "test.count", 1, "run each test and benchmarks (n) times")
	flags.StringVar(&testList, "test.list", "", "list tests that match regex")
	flags.StringVar(&testList, "test.paniconexit0", "", "Some new default test param.")
	flags.BoolVar(&testClean, "clean", false, "Clean data associated with tests")
	flags.StringVar(&testEnv, "env", "dev", "Environment to test.")
	flags.BoolVar(&testSkipCertCache, "skipCertCache", false, "Cache our configuration files")

	etlcore.LogError(fmt.Sprintf("args %v", os.Args))

	if idbFn != nil {
		kafkatesting.IndirectDbFunc = idbFn
	}

	filteredArgs := filterEmptyElements(os.Args)[1:]
	flags.Parse(filteredArgs)

	if testRegex == "-clean" {
		return true
	}

	if testClean {
		testRegex = strings.Replace(testRegex, "Test", "", 1)
	}

	isBench := len(testBenchRegEx) > 0
	if isBench {
		testRegex = testBenchRegEx
	}

	etlcore.LogError(fmt.Sprintf("Running with timeout %s and log file: %s and parallel: %s verbosity: %v\n", testTimeout, testLog, testParallel, testVerbose))
	set := token.NewFileSet()
	packs, err := parser.ParseDir(set, ".", nil, 0)
	if err != nil {
		etlcore.LogError(fmt.Sprintf("Failed to parse package:%v", err))
	}

	poolCleanerRegex := "^Clean.*"
	compiledPoolRegex, err := regexp.Compile(poolCleanerRegex)
	if err != nil {
		etlcore.LogError(err.Error())
		return true
	}
	compiledTestRegex, err := regexp.Compile(testRegex)
	if err != nil {
		etlcore.LogError(err.Error())
		return true
	}

	for _, pack := range packs {
		processDecls(pool, *pack, testClean, isBench, compiledTestRegex, compiledPoolRegex)
	}

	if testClean {
		os.Exit(0) // This can exit because it is only used from cli
	}

	kafkatesting.StartAllTestEngines(nil, nil)
	return false
}

func processDecls(pool interface{}, pack ast.Package, testClean bool, isBench bool, compiledTestRegex, compiledPoolRegex *regexp.Regexp) {
	for _, file := range pack.Files {
		if testClean {
			handleTestClean(pool, file, compiledPoolRegex)
			continue
		}
		for _, decl := range file.Decls {
			fn, isFn := decl.(*ast.FuncDecl)
			if !isFn || !compiledTestRegex.MatchString(fn.Name.Name) {
				continue
			}
			registerTest(pool, fn, file, isBench)
		}
	}
}

func handleTestClean(pool interface{}, file *ast.File, compiledPoolRegex *regexp.Regexp) {
	if pool == nil {
		etlcore.LogError("Pool is nil in handleTestClean")
		return
	}

	for _, decl := range file.Decls {
		cfn, isCFn := decl.(*ast.FuncDecl)
		if !isCFn || !compiledPoolRegex.MatchString(cfn.Name.Name) {
			continue
		}
		etlcore.LogError(fmt.Sprintf("Cleaning: %s\n", cfn.Name.Name))

		poolValue := reflect.ValueOf(pool)
		method := poolValue.MethodByName(cfn.Name.Name)
		if !method.IsValid() {
			etlcore.LogError(fmt.Sprintf("Clean method %s not found on pool", cfn.Name.Name))
			continue
		}

		func() {
			defer func() {
				if r := recover(); r != nil {
					etlcore.LogError(fmt.Sprintf("Panic during clean method %s: %v", cfn.Name.Name, r))
				}
			}()
			method.Call([]reflect.Value{})
		}()
	}
}

func registerTest(pool interface{}, fn *ast.FuncDecl, _ *ast.File, isBench bool) {
	testParts := strings.SplitN(fn.Name.Name, "_", 2)
	if len(testParts) != 2 || prefixMatch(isBench, testParts[0]) {
		return
	}

	if pool == nil {
		etlcore.LogError(fmt.Sprintf("Pool is nil for test %s", fn.Name.Name))
		return
	}

	poolValue := reflect.ValueOf(pool)
	method := poolValue.MethodByName(testParts[1])
	if !method.IsValid() {
		etlcore.LogError(fmt.Sprintf("Method %s not found on pool for test %s", testParts[1], testParts[0]))
		return
	}

	var kafkaTopicsRaw []reflect.Value
	func() {
		defer func() {
			if r := recover(); r != nil {
				etlcore.LogError(fmt.Sprintf("Panic during method call %s: %v", testParts[1], r))
				kafkaTopicsRaw = nil
			}
		}()
		kafkaTopicsRaw = method.Call([]reflect.Value{})
	}()

	if len(kafkaTopicsRaw) == 0 {
		etlcore.LogError(fmt.Sprintf("Method %s returned no values", testParts[1]))
		return
	}

	testName := strings.TrimPrefix(strings.TrimPrefix(testParts[0], "Test"), "Benchmark")

	kafkaTopics, ok := kafkaTopicsRaw[0].Interface().([][]string)
	if !ok {
		etlcore.LogError(fmt.Sprintf("Method %s did not return [][]string, got %T", testParts[1], kafkaTopicsRaw[0].Interface()))
		return
	}

	for _, kafkaTopic := range kafkaTopics {
		if len(kafkaTopic) == 0 {
			etlcore.LogError(fmt.Sprintf("Empty kafka topic in sequence for test %s", testName))
			continue
		}
		kafkaTestReader, err := kafkatesting.NewKafkaTestReader(kafkaTopic, nil, true)
		if err != nil {
			etlcore.LogError(fmt.Sprintf("Failed to create KafkaTestReader %v", err))
			continue
		}
		if kafkaTestReader != nil {
			kafkaTestReader.RegisterTest(testName)
		}
	}

	etlcore.LogError(fmt.Sprintf("Registered test %s for %s", testParts[0], testParts[1]))
}

func filterEmptyElements(unfiltered []string) []string {
	filtered := []string{}
	for _, v := range unfiltered {
		if len(v) > 0 {
			filtered = append(filtered, v)
		}
	}
	return filtered
}

func prefixMatch(isBench bool, functionName string) bool {
	if isBench {
		return strings.HasPrefix(functionName, "Benchmark")
	}
	return strings.HasPrefix(functionName, "Test")
}
