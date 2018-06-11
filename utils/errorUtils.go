package utils

import (
	"fmt"
	"os"
)

// CheckError Simplifies the error checking process
func CheckError(err error) {
	if err != nil {
		panic(err)
	}
}

// CheckWarnings Checks warnings returned from various vault relation operations
func CheckWarnings(warnings []string) {
	if len(warnings) > 0 {
		for _, w := range warnings {
			fmt.Println(w)
		}
		os.Exit(1)
	}
}
