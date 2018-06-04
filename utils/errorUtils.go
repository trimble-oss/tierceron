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
	for _, w := range warnings {
		fmt.Println(w)
	}
	if len(warnings) > 0 {
		os.Exit(1)
	}
}
