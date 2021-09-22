package main

import (
	"flag"
	"fmt"

	trcxbase "tierceron/trcxbase"

	"fyne.io/fyne/app"
)

// This executable automates the creation of seed files from template file(s).
// New seed files are written (or overwrite current seed files) to the specified directory.
func main() {
	fmt.Println("Version: " + "1.19")
	envPtr := flag.String("env", "dev", "Environment to get seed data for.")
	app := app.New()
	w := app.NewWindow("Hello")

	trcxbase.CommonMain(envPtr, nil)

	w.ShowAndRun()
}
