package utils

import (
	"errors"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/trimble-oss/tierceron/pkg/core"
)

var headlessService bool

func init() {
	headlessService = false
}

func InitHeadless(headless bool) {
	headlessService = headless
}

// CheckError Simplifies the error checking process
func CheckError(config *core.CoreConfig, err error, exit bool) {
	// If code wants to exit and ExitOnFailure is specified,
	// then we can exit here.
	if err != nil && exit && config.ExitOnFailure {
		log.Fatal(err)
	}
}

// CheckErrorNoStack Simplifies the error checking process
func CheckErrorNoStack(config *core.CoreConfig, err error, exit bool) {
	if err != nil {
		if !headlessService {
			fmt.Println(err)
		}
		if exit && config.ExitOnFailure {
			os.Exit(1)
		}
	}
}

// CheckWarnings Checks warnings returned from various vault relation operations
func CheckWarning(config *core.CoreConfig, warning string, exit bool) {
	if len(warning) > 0 {
		if !headlessService {
			fmt.Println(warning)
		}
		if exit && config.ExitOnFailure {
			os.Exit(1)
		}
	}
}

// CheckWarnings Checks warnings returned from various vault relation operations
func CheckWarnings(config *core.CoreConfig, warnings []string, exit bool) {
	if len(warnings) > 0 {
		if !headlessService {
			for _, w := range warnings {
				fmt.Println(w)
			}
		}
		if exit && config.ExitOnFailure {
			os.Exit(1)
		}
	}
}

// LogError Writes error to the log file and terminates if an error occurs
func LogError(config *core.CoreConfig, err error, f *os.File, exit bool) {
	if err != nil {
		_prefix := log.Prefix()
		log.SetOutput(f)
		log.SetPrefix("[ERROR]")
		if exit && config.ExitOnFailure {
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

// LogWarnings Writes array of warnings to the log file and terminates
func LogWarnings(config *core.CoreConfig, warnings []string, f *os.File, exit bool) {
	if len(warnings) > 0 {
		_prefix := log.Prefix()
		log.SetOutput(f)
		log.SetPrefix("[WARNS]")
		if !headlessService {
			for _, w := range warnings {
				log.Println(w)
			}
		}
		if exit && config.ExitOnFailure {
			if !headlessService {
				fmt.Printf("Warnings encountered, exiting and writing to log file: %s\n", f.Name())
			}
			os.Exit(1)
		} else {
			log.SetPrefix(_prefix)
		}
	}
}

// LogErrorObject writes errors to the passed logger object and exits
func LogWarningMessage(config *core.CoreConfig, errorMessage string, exit bool) {
	_prefix := config.Log.Prefix()
	config.Log.SetPrefix("[WARN]")
	if exit && config.ExitOnFailure {
		if !headlessService {
			fmt.Printf("Errors encountered, exiting and writing to log file\n")
		}
		config.Log.Fatal(errorMessage)
	} else {
		config.Log.Println(errorMessage)
		config.Log.SetPrefix(_prefix)
	}
}

// LogErrorObject writes errors to the passed logger object and exits
func LogMessageErrorObject(config *core.CoreConfig, errorMessage string, err error, exit bool) {
	if err != nil {
		_prefix := config.Log.Prefix()
		config.Log.SetPrefix("[ERROR]")
		if exit && config.ExitOnFailure {
			if !headlessService {
				fmt.Printf("Errors encountered, exiting and writing to log file: %v\n", err)
			}
			config.Log.Fatal(err)
		} else {
			if errorMessage != "" {
				config.Log.Println(errorMessage)
			}
			config.Log.Println(err)
			config.Log.SetPrefix(_prefix)
		}
	}
}

// LogErrorObject writes errors to the passed logger object and exits
func LogErrorMessage(config *core.CoreConfig, errorMessage string, exit bool) {
	_prefix := config.Log.Prefix()
	config.Log.SetPrefix("[ERROR]")
	if exit && config.ExitOnFailure {
		if !headlessService {
			fmt.Printf("Errors encountered, exiting and writing to log file\n")
		}
		config.Log.Fatal(errorMessage)
	} else {
		config.Log.Println(errorMessage)
		config.Log.SetPrefix(_prefix)
	}
}

// LogErrorObject writes errors to the passed logger object and exits
func LogErrorObject(config *core.CoreConfig, err error, exit bool) {
	if err != nil {
		_prefix := config.Log.Prefix()
		config.Log.SetPrefix("[ERROR]")
		if exit && config.ExitOnFailure {
			if !headlessService {
				fmt.Printf("Errors encountered, exiting and writing to log file: %v\n", err)
			}
			config.Log.Fatal(SanitizeForLogging(err.Error()))
		} else {
			config.Log.Println(SanitizeForLogging(err.Error()))
			config.Log.SetPrefix(_prefix)
		}
	}
}

// LogErrorObject writes errors to the passed logger object and exits
func LogInfo(config *core.CoreConfig, msg string) {
	if !headlessService {
		fmt.Println(SanitizeForLogging(msg))
	}
	if config != nil && config.Log != nil {
		_prefix := config.Log.Prefix()
		config.Log.SetPrefix("[INFO]")
		config.Log.Println(SanitizeForLogging(msg))
		config.Log.SetPrefix(_prefix)
	}
}

// LogWarningsObject writes warnings to the passed logger object and exits
func LogWarningsObject(config *core.CoreConfig, warnings []string, exit bool) {
	if len(warnings) > 0 {
		_prefix := config.Log.Prefix()
		config.Log.SetPrefix("[WARNS]")
		for _, w := range warnings {
			config.Log.Println(w)
		}
		if exit && config.ExitOnFailure {
			if !headlessService {
				fmt.Println("Warnings encountered, exiting and writing to log file")
			}
			os.Exit(1)
		} else {
			config.Log.SetPrefix(_prefix)
		}
	}
}

// LogAndSafeExit -- provides isolated location of os.Exit to ensure os.Exit properly processed.
func LogAndSafeExit(config *core.CoreConfig, message string, code int) error {
	if config.Log != nil && message != "" {
		LogInfo(config, message)
	}

	if config.ExitOnFailure {
		os.Exit(code)
	}

	return errors.New(message)
}

// LogErrorAndSafeExit -- provides isolated location of os.Exit to ensure os.Exit properly processed.
func LogErrorAndSafeExit(config *core.CoreConfig, err error, code int) error {
	if config.Log != nil && err != nil {
		LogInfo(config, err.Error())
	}

	if err != nil && config.ExitOnFailure {
		os.Exit(code)
	}

	return err
}

func SanitizeForLogging(errMsg string) string {
	errMsgSanitized := strings.ReplaceAll(errMsg, "\n", "")
	errMsgSanitized = strings.ReplaceAll(errMsgSanitized, "\r", "")
	return errMsgSanitized
}
