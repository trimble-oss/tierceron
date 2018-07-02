package utils

import (
	"io/ioutil"
	"regexp"
)

const pattern string = `{{or \.([^"]+) "([^"]+)"}}`

// Parse Extracts default values as key-value pairs from template files
func Parse(filepath string, service string, filename string) (map[string]interface{}, error) {
	workingSet := make(map[string]interface{})
	file, err := ioutil.ReadFile(filepath)
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
		// fmt.Println(match)
		kv[1] = service + "_" + filename + "_" + kv[1]
		workingSet[kv[1]] = kv[2]
	}

	return workingSet, nil
}
