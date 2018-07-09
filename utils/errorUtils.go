package utils

import (
	"fmt"
	"log"
	"os"
)

// CheckError Simplifies the error checking process
func CheckError(err error, exit bool) {
	if err != nil && exit {
		panic(err)
	}
}

// CheckWarnings Checks warnings returned from various vault relation operations
func CheckWarnings(warnings []string, exit bool) {
	if len(warnings) > 0 {
		for _, w := range warnings {
			fmt.Println(w)
		}
		if exit {
			os.Exit(1)
		}
	}
}

//LogError Writes error to the log file and terminates if an error occurs
func LogError(err error, f *os.File, exit bool) {
	if err != nil {
		_prefix := log.Prefix()
		log.SetOutput(f)
		log.SetPrefix("[ERROR]")
		if exit {
			fmt.Printf("Errors encountered, exiting and writing to log file: %s\n", f.Name())
			log.Fatal(err)
		} else {
			log.Println(err)
			log.SetPrefix(_prefix)
		}
	}
}

//LogWarnings Writes array of warnings to the log file and terminates
func LogWarnings(warnings []string, f *os.File, exit bool) {
	if len(warnings) > 0 {
		_prefix := log.Prefix()
		log.SetOutput(f)
		log.SetPrefix("[WARNS]")
		for _, w := range warnings {
			log.Println(w)
		}
		if exit {
			fmt.Printf("Warnings encountered, exiting and writing to log file: %s\n", f.Name())
			os.Exit(1)
		} else {
			log.SetPrefix(_prefix)
		}
	}
}

//LogErrorObject writes errors to the passed logger object and exits
func LogErrorObject(err error, logger *log.Logger, exit bool) {
	if err != nil {
		_prefix := logger.Prefix()
		logger.SetPrefix("[ERROR]")
		if exit {
			fmt.Println("Errors encountered, exiting and writing to log file")
			logger.Fatal(err)
		} else {
			logger.Println(err)
			logger.SetPrefix(_prefix)
		}
	}
}

//LogWarningsObject writes warnings to the passed logger object and exits
func LogWarningsObject(warnings []string, logger *log.Logger, exit bool) {
	if len(warnings) > 0 {
		_prefix := logger.Prefix()
		logger.SetPrefix("[WARNS]")
		for _, w := range warnings {
			logger.Println(w)
		}
		if exit {
			fmt.Println("Warnings encountered, exiting and writing to log file")
			os.Exit(1)
		} else {
			logger.SetPrefix(_prefix)
		}
	}
}
