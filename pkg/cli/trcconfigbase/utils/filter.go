package utils

import (
	"strings"
)

// FilterPaths -- filters based on provided fileFilter
func FilterPaths(templatePaths []string, endPaths []string, fileFilter []string, prefix bool) ([]string, []string) {
	fileFilterIndex := make([]int, len(templatePaths))
	fileFilterCounter := 0

	if len(fileFilter) != 0 && fileFilter[0] != "" {
		for _, FileFilter := range fileFilter {
			for i, templatePath := range templatePaths {
				if !prefix {
					ii := -1
					if strings.Contains(templatePath, FileFilter) {
						ii = i
					}
					if len(fileFilterIndex) > fileFilterCounter {
						fileFilterIndex[fileFilterCounter] = ii
						fileFilterCounter++
					} else {
						break
					}
				} else {
					if strings.HasPrefix(templatePath, FileFilter) {
						if len(fileFilterIndex) > fileFilterCounter {
							fileFilterIndex[fileFilterCounter] = i
							fileFilterCounter++
						} else {
							break
						}
					}

				}
			}
		}

		fileTemplatePaths := []string{}
		fileEndPaths := []string{}
		for _, index := range fileFilterIndex {
			if index >= 0 {
				fileTemplatePaths = append(fileTemplatePaths, templatePaths[index])
				fileEndPaths = append(fileEndPaths, endPaths[index])
			}
		}

		templatePaths = fileTemplatePaths
		endPaths = fileEndPaths
	}
	return templatePaths, endPaths
}
