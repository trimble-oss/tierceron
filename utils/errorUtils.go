package utils

import (
	"errors"
	"fmt"
	"log"
	"os"
)

var headlessService bool

func init() {
	headlessService = false
}

func InitHeadless(headless bool) {
	headlessService = headless
}

// CheckError Simplifies the error checking process
func CheckError(err error, exit bool) {
	if err != nil && exit {
		panic(err)
	}
}

// CheckErrorNoStack Simplifies the error checking process
func CheckErrorNoStack(err error, exit bool) {
	if err != nil {
		if !headlessService {
			fmt.Println(err)
		}
		if exit {
			os.Exit(1)
		}
	}
}

// CheckWarnings Checks warnings returned from various vault relation operations
func CheckWarning(warning string, exit bool) {
	if len(warning) > 0 {
		if !headlessService {
			fmt.Println(warning)
		}
		if exit {
			os.Exit(1)
		}
	}
}

// CheckWarnings Checks warnings returned from various vault relation operations
func CheckWarnings(warnings []string, exit bool) {
	if len(warnings) > 0 {
		if !headlessService {
			for _, w := range warnings {
				fmt.Println(w)
			}
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
			if !headlessService {
				fmt.Printf("Errors encountered, exiting and writing to log file: %s\n", f.Name())
				log.Fatal(err)
			} else {
				os.Exit(-1)
			}
		} else {
			if !headlessService {
				log.Println(err)
			}
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
		if !headlessService {
			for _, w := range warnings {
				log.Println(w)
			}
		}
		if exit {
			if !headlessService {
				fmt.Printf("Warnings encountered, exiting and writing to log file: %s\n", f.Name())
			}
			os.Exit(1)
		} else {
			log.SetPrefix(_prefix)
		}
	}
}

//LogErrorObject writes errors to the passed logger object and exits
func LogWarningMessage(errorMessage string, logger *log.Logger, exit bool) {
	_prefix := logger.Prefix()
	logger.SetPrefix("[WARN]")
	if exit {
		if !headlessService {
			fmt.Printf("Errors encountered, exiting and writing to log file\n")
		}
		logger.Fatal(errorMessage)
	} else {
		logger.Println(errorMessage)
		logger.SetPrefix(_prefix)
	}
}

//LogErrorObject writes errors to the passed logger object and exits
func LogErrorMessage(errorMessage string, logger *log.Logger, exit bool) {
	_prefix := logger.Prefix()
	logger.SetPrefix("[ERROR]")
	if exit {
		if !headlessService {
			fmt.Printf("Errors encountered, exiting and writing to log file\n")
		}
		logger.Fatal(errorMessage)
	} else {
		logger.Println(errorMessage)
		logger.SetPrefix(_prefix)
	}
}

//LogErrorObject writes errors to the passed logger object and exits
func LogErrorObject(err error, logger *log.Logger, exit bool) {
	if err != nil {
		_prefix := logger.Prefix()
		logger.SetPrefix("[ERROR]")
		if exit {
			if !headlessService {
				fmt.Printf("Errors encountered, exiting and writing to log file: %v\n", err)
			}
			logger.Fatal(err)
		} else {
			if !headlessService {
				logger.Println(err)
			}
			logger.SetPrefix(_prefix)
		}
	}
}

//LogErrorObject writes errors to the passed logger object and exits
func LogInfo(msg string, logger *log.Logger) {
	if !headlessService {
		fmt.Println(msg)
	}
	if logger != nil {
		_prefix := logger.Prefix()
		logger.SetPrefix("[INFO]")
		logger.Println(msg)
		logger.SetPrefix(_prefix)
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
			if !headlessService {
				fmt.Println("Warnings encountered, exiting and writing to log file")
			}
			os.Exit(1)
		} else {
			logger.SetPrefix(_prefix)
		}
	}
}

// LogAndSafeExit -- provides isolated location of os.Exit to ensure os.Exit properly processed.
func LogAndSafeExit(config *DriverConfig, message string, code int) error {
	if config.Log != nil && message != "" {
		LogInfo(message, config.Log)
	}

	if config.ExitOnFailure {
		os.Exit(code)
	}

	return errors.New(message)
}
