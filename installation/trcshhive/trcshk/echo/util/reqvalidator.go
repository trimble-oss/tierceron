package util

import (
	"strings"
)

func ValidateRequest(message string) bool {
	return strings.Contains(strings.ToUpper(message), strings.ToUpper("run"))
}
