package trcxe

import (
	"flag"
	"fmt"
	"log"
	"os"
	"tierceron/buildopts/coreopts"
	eUtils "tierceron/utils"
)

func main() {
	fmt.Println("Version: " + "1.0")
	logFilePtr := flag.String("log", "./"+coreopts.GetFolderPrefix()+"xe.log", "Output path for log files")
	envPtr := flag.String("env", "dev", "Environment for seed file")
	fileAddrPtr := flag.String("indexFilter", "", "Path for seed file")
	fieldsPtr := flag.String("fields", "", "Fields to enter")
	encryptedPtr := flag.String("encrypted", "", "Public app role ID")
	flag.Parse()

	// If logging production directory does not exist and is selected log to local directory
	if _, err := os.Stat("/var/log/"); os.IsNotExist(err) && *logFilePtr == "/var/log/"+coreopts.GetFolderPrefix()+"xe.log" {
		*logFilePtr = "./" + coreopts.GetFolderPrefix() + "xe.log"
	}
	f, err := os.OpenFile(*logFilePtr, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0644)

	logger := log.New(f, "[INIT]", log.LstdFlags)
	config := &eUtils.DriverConfig{Insecure: true, Log: logger, ExitOnFailure: true}
	eUtils.CheckError(config, err, true)

	//TODO: Pull seed file into template structure format
	//Edit it
	//Write it back out where it came from

	//Add encryption for encryptedPtr fields
}
