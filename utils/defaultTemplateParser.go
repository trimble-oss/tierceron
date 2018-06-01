package utils

import (
	"io/ioutil"
	"regexp"
	"strings"
)

const pattern string = "{{or .+ .+}}"

// Parse Extracts default values as key-value pairs from template files
func Parse(filename string) (map[string]interface{}, error) {
	workingSet := make(map[string]interface{})
	file, err := ioutil.ReadFile(filename)
	if err != nil {
		return nil, err
	}

	regex, err := regexp.Compile(pattern)

	if err != nil {
		return nil, err
	}

	matched := regex.FindAllString(string(file), -1)

	for _, match := range matched {
		match = strings.Trim(match, "{}")
		match = match[4:] // Remove the "or ."

		kv := strings.SplitN(match, " ", 2)
		// Split and add to map
		//fmt.Println(match)
		workingSet[kv[0]] = strings.Trim(kv[1], "\"")
	}

	return workingSet, nil
}
