package utils

import (
	"reflect"
	"runtime"
)

func IsWindows() bool {
	return runtime.GOOS == "windows"
}

var (
	EMPTY_STRING string = ""
)

func RefEquals(src *string, dest string) bool {
	if src == nil {
		return false
	}
	return *src == dest
}

func RefEqualsAny(src *string, dest []string) bool {
	if src == nil {
		return false
	}
	for _, d := range dest {
		if *src == d {
			return true
		}
	}
	return false
}

func RefLength(src *string) int {
	if src == nil {
		return 0
	}
	return len(*src)
}

func RefString(src *string) *string {
	if src == nil {
		return nil
	}
	return src
}

func RefMap(m map[string]interface{}, key string) *string {
	v, ok := m[key]
	if !ok {
		return nil
	}

	rv := reflect.ValueOf(v)
	if rv.Kind() == reflect.Ptr {
		rv = rv.Elem()
	}
	if rv.Kind() != reflect.String {
		return nil
	}
	strPtr := rv.String()

	return &strPtr
}

func EmptyStringRef() *string { return &EMPTY_STRING }
