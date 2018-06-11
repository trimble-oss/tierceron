package utils

import (
	"fmt"
	"log"
	"os"
)

// Performs the same function as CheckError and CheckWarning, but logs to the returned file instead.

//LogError Writes error to the log file and terminates if an error occurs
func LogError(err error, f *os.File) {
	if err != nil {
		fmt.Printf("Errors encountered, exiting and writing to log file: %s\n", f.Name())
		log.SetOutput(f)
		log.SetPrefix("Error: ")
		log.Fatal(err)
	}
}

//LogWarnings Writes array of warnings to the log file and terminates
func LogWarnings(warnings []string, f *os.File) {
	if len(warnings) > 0 {
		fmt.Printf("Warnings encountered, exiting and writing to log file: %s\n", f.Name())
		log.SetOutput(f)
		log.SetPrefix("Warns: ")
		for _, w := range warnings {
			log.Println(w)
		}
		os.Exit(1)
	}
}
