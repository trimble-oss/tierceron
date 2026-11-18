// Package memprotectopts selects per-value memory protection for host processes.
// Initialization never locks current or future process mappings. Call MemProtect
// for each selected value and check its error before publishing that value.
package memprotectopts

import (
	"log"
	"runtime"

	corememprotect "github.com/trimble-oss/tierceron-core/v2/buildopts/memprotectopts"
)

func MemProtectInit(logger *log.Logger) error {
	if runtime.GOOS == "windows" {
		return corememprotect.MemProtectInit(logger)
	}
	return nil
}

func MemProtect(logger *log.Logger, sensitive *string) error {
	return corememprotect.MemProtect(logger, sensitive)
}
