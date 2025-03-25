package utils

import (
	"os"
	"regexp"
	"strings"
)

// {{or .<key> "<value>"}}
const pattern string = `{{or \.([^"]+) "([^"]+)"}}`

func TrimLastDotAfterLastSlash(s string) (string, int) {
	if strings.Contains(s, ".tmpl") {
		s = s[0 : len(s)-len(".tmpl")]
	}
	lastSlash := strings.LastIndex(s, "/")
	if lastSlash == -1 {
		lastSlash = 0
	}
	lastDotIndex := strings.LastIndex(s[lastSlash:], ".")
	if lastDotIndex == -1 {
		return s, lastDotIndex
	}
	return s[:lastSlash+lastDotIndex], lastSlash + lastDotIndex
}

// Parse Extracts default values as key-value pairs from template files.
// Before being uploaded, the service and filename will be appended so the uploaded value will be
// <Service>.<Filename>.<Key>
// Underscores in key names will be replaced with periods before being uploaded
func Parse(filepath string, service string, filename string) (map[string]interface{}, error) {
	workingSet := make(map[string]interface{})
	file, err := os.ReadFile(filepath)
	if err != nil {
		return nil, err
	}

	regex, err := regexp.Compile(pattern)

	if err != nil {
		return nil, err
	}

	matched := regex.FindAllString(string(file), -1)

	for _, match := range matched {
		kv := regex.FindStringSubmatch(match)
		// Split and add to map
		kv[1] = strings.Replace(kv[1], "_", ".", -1)
		workingSet[kv[1]] = kv[2]
	}

	return workingSet, nil
}
