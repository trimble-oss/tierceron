package utils

import (
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

func RefRefEquals(src *string, dest *string) bool {
	if src == nil && dest == nil {
		return true
	} else if src == nil || dest == nil {
		return false
	}
	return *src == *dest
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

func RefSliceLength(src *[]string) int {
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

func IToString(src interface{}) string {
	if src == nil {
		return ""
	}
	if strPtr, ok := src.(*string); ok {
		return *strPtr
	} else if str, ok := src.(string); ok {
		return str
	}
	return ""
}

func RefMap(m map[string]interface{}, key string) *string {
	v, ok := m[key]
	if !ok {
		return nil
	}

	if _, ok := v.(*string); ok {
		return v.(*string)
	} else if _, ok := v.(**string); ok {
		refStr := v.(**string)
		return *refStr
	} else if _, ok := v.(string); ok {
		str := v.(string)
		return &str
	} else {
		return nil
	}
}

func EmptyStringRef() *string { return &EMPTY_STRING }
