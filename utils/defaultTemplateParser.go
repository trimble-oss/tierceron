package utils

import (
	"os"
	"regexp"
	"strings"
)

// {{or .<key> "<value>"}}
const pattern string = `{{or \.([^"]+) "([^"]+)"}}`

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
