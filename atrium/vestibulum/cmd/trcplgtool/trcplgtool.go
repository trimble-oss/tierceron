package main

import (
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/trimble-oss/tierceron/pkg/utils/config"

	"github.com/trimble-oss/tierceron/atrium/buildopts/flowcoreopts"
	plgtbase "github.com/trimble-oss/tierceron/atrium/vestibulum/trcdb/trcplgtoolbase"
	"github.com/trimble-oss/tierceron/buildopts"
	"github.com/trimble-oss/tierceron/buildopts/coreopts"
	"github.com/trimble-oss/tierceron/buildopts/cursoropts"
	"github.com/trimble-oss/tierceron/buildopts/deployopts"
	"github.com/trimble-oss/tierceron/buildopts/harbingeropts"
	"github.com/trimble-oss/tierceron/buildopts/memonly"
	"github.com/trimble-oss/tierceron/buildopts/memprotectopts"
	"github.com/trimble-oss/tierceron/buildopts/tcopts"
	"github.com/trimble-oss/tierceron/buildopts/xencryptopts"
	"github.com/trimble-oss/tierceron/pkg/capauth"
	"github.com/trimble-oss/tierceron/pkg/core"
)

// This executable automates the cerification of a plugin docker image.
func main() {
	if memonly.IsMemonly() {
		memprotectopts.MemProtectInit(nil)
	}
	buildopts.NewOptionsBuilder(buildopts.LoadOptions())
	coreopts.NewOptionsBuilder(coreopts.LoadOptions())
	deployopts.NewOptionsBuilder(deployopts.LoadOptions())
	flowcoreopts.NewOptionsBuilder(flowcoreopts.LoadOptions())
	harbingeropts.NewOptionsBuilder(harbingeropts.LoadOptions())
	tcopts.NewOptionsBuilder(tcopts.LoadOptions())
	xencryptopts.NewOptionsBuilder(xencryptopts.LoadOptions())
	cursoropts.NewOptionsBuilder(cursoropts.LoadOptions())

	fmt.Println("Version: " + "1.05")

	flagset := flag.NewFlagSet(os.Args[0], flag.ExitOnError)
	flagset.Usage = func() {
		fmt.Fprintf(flagset.Output(), "Usage of %s:\n", os.Args[0])
		flagset.PrintDefaults()
	}
	envPtr := flagset.String("env", "dev", "Environment to configure")
	addrPtr := flagset.String("addr", "", "API endpoint for the vault")
	regionPtr := flagset.String("region", "", "Region to be processed") //If this is blank -> use context otherwise override context.
	secretIDPtr := flagset.String("secretID", "", "Secret for app role ID")
	appRoleIDPtr := flagset.String("appRoleID", "", "Public app role ID")
	tokenNamePtr := flagset.String("tokenName", "", "Token name used by this"+coreopts.BuildOptions.GetFolderPrefix(nil)+"config to access the vault")
	logFilePtr := flagset.String("log", "./"+coreopts.BuildOptions.GetFolderPrefix(nil)+"plgtool.log", "Output path for log files")

	if _, err := os.Stat("/var/log/"); os.IsNotExist(err) && *logFilePtr == "/var/log/"+coreopts.BuildOptions.GetFolderPrefix(nil)+"plgtool.log" {
		*logFilePtr = "./" + coreopts.BuildOptions.GetFolderPrefix(nil) + "plgtool.log"
	}
	f, errLog := os.OpenFile(*logFilePtr, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0644)
	if errLog != nil {
		fmt.Printf("Could not open logfile: %s\n", *logFilePtr)
		os.Exit(1)
	}

	logger := log.New(f, "[INIT]", log.LstdFlags)

	trcshDriveConfigPtr := &capauth.TrcshDriverConfig{
		DriverConfig: &config.DriverConfig{
			CoreConfig: &core.CoreConfig{
				ExitOnFailure:    true,
				Insecure:         false,
				Log:              logger,
				AppRoleConfigPtr: new(string),
			},
			IsShellSubProcess: false,
			StartDir:          []string{""},
		},
	}

	err := plgtbase.CommonMain(envPtr, addrPtr, nil, secretIDPtr, appRoleIDPtr, tokenNamePtr, regionPtr, flagset, os.Args, trcshDriveConfigPtr)
	if err != nil {
		os.Exit(1)
	}

}
