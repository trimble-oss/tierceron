package utils

import (
	"fmt"
	"log"
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

//LogErrorObject writes errors to the passed logger object and exits
func LogErrorObject(err error, logger *log.Logger) {
	if err != nil {
		fmt.Println("Errors encountered, exiting and writing to log file")
		logger.SetPrefix("[ERROR]")
		logger.Fatal(err)
	}
}

//LogWarningsObject writes warnings to the passed logger object and exits
func LogWarningsObject(warnings []string, logger *log.Logger) {
	if len(warnings) > 0 {
		fmt.Println("Warnings encountered, exiting and writing to log file")
		logger.SetPrefix("[WARNS]")
		for _, w := range warnings {
			logger.Println(w)
		}
		os.Exit(1)
	}
}
