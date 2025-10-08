package utils

import (
	"os"
	"strings"

	eUtils "github.com/trimble-oss/tierceron/pkg/utils"
)

// FilterPaths -- filters based on provided fileFilter
func FilterPaths(templatePaths []string, endPaths []string, fileFilter []string, prefix bool) ([]string, []string) {
	if len(fileFilter) != 0 && fileFilter[0] != "" {
		fileFilterIndex := make([]int, len(templatePaths))
		fileFilterCounter := 0
		for _, FileFilter := range fileFilter {
			if eUtils.IsWindows() {
				FileFilter = strings.ReplaceAll(FileFilter, "/", string(os.PathSeparator))
			} else {
				FileFilter = strings.ReplaceAll(FileFilter, "\\", string(os.PathSeparator))
			}

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
