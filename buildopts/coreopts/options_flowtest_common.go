//go:build !testrunner

package coreopts

func IsTestRunner() bool {
	return false
}
