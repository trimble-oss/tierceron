package utils

import (
	"flag"
	"fmt"
	"os"
	"strings"
)

var (
	SubPathFilter        *string
	ServiceNameFilterPtr *string
	ServiceFilterPtr     *string
	IndexNameFilterPtr   *string
	IndexValueFilterPtr  *string
	IndexedPtr           *string
	RestrictedPtr        *string
	ProtectedPtr         *string
	BasePtr              *bool
	OnlyBasePtr          = false
)

func initglobals(flagset *flag.FlagSet) {
	SubPathFilter = flagset.String("subPathFilter", "", "Specifies subpath templates to filter")                           // a sub path filter.
	ServiceNameFilterPtr = flagset.String("serviceExtFilter", "", "Specifies which nested services (or tables) to filter") // offset or database
	ServiceFilterPtr = flagset.String("serviceFilter", "", "Specifies which services (or tables) to filter")               // Table names
	IndexNameFilterPtr = flagset.String("indexFilter", "", "Specifies which index names to filter")                        // column index, table to filter.
	IndexValueFilterPtr = flagset.String("indexValueFilter", "", "Specifies which index values to filter")                 // column index value to filter on.
	IndexedPtr = flagset.String("indexed", "", "Specifies which projects are indexed")                                     // Indicates indexed projects...
	RestrictedPtr = flagset.String("restricted", "", "Specifies which projects have restricted access.")
	ProtectedPtr = flagset.String("protected", "", "Specifies which projects have protected access.")
	BasePtr = flagset.Bool("base", false, "Specifies whether the base env seed file will be seeded")
}

func checkInitFlagHelper() {
	if len(*IndexValueFilterPtr) > 0 {
		if len(*IndexNameFilterPtr) > 0 {
			if strings.Contains(*RestrictedPtr, ",") || strings.Contains(*IndexedPtr, ",") {
				fmt.Fprintln(os.Stderr, "Usage of comma delimited -restricted or -indexed is not allowed with additional filters")
				os.Exit(1)
			}
		} else {
			fmt.Fprintln(os.Stderr, "Usage of -indexFilter is required for -indexValueFilter")
			os.Exit(1)
		}
	} else {
		fmt.Fprintln(os.Stderr, "Usage of -indexValueFilter is required")
		os.Exit(1)
	}
}

func CheckInitFlags(flagset *flag.FlagSet, arguments []string) error {
	filtered := false
	initglobals(flagset)
	parseErr := flagset.Parse(arguments)

	// If help flag was used, return early
	if parseErr != nil {
		return parseErr
	}

	// Cannot specify a pathed indexed/restricted seed file while specifying a restricted/indexed section.
	if len(*IndexNameFilterPtr) > 0 || len(*ServiceNameFilterPtr) > 0 || len(*IndexValueFilterPtr) > 0 || len(*ServiceNameFilterPtr) > 0 {
		filtered = true
	}
	if len(*RestrictedPtr) > 0 && len(*IndexedPtr) > 0 && filtered {
		return fmt.Errorf("cannot use -restricted and -indexed at the same time while trying to specify a seed file")
	}

	// Same reason as above, but with protected.
	if len(*ProtectedPtr) > 0 && len(*IndexedPtr) > 0 {
		return fmt.Errorf("cannot use -protected with -indexed")
	}

	if len(*RestrictedPtr) > 0 && len(*ProtectedPtr) > 0 && filtered {
		return fmt.Errorf("cannot use -restricted and -protected at the same time while trying to specify a seed file")
	}

	if (len(*RestrictedPtr) > 0 || len(*IndexedPtr) > 0) && filtered && (strings.Contains(*RestrictedPtr, ",") || strings.Contains(*IndexedPtr, ",")) {
		return fmt.Errorf("cannot use comma delimited lists with filters")
	}

	if len(*RestrictedPtr) > 0 || len(*IndexedPtr) > 0 {
		if len(*ServiceNameFilterPtr) > 0 {
			checkInitFlagHelper()
		} else if len(*IndexValueFilterPtr) > 0 {
			checkInitFlagHelper()
		}
	}

	if len(*RestrictedPtr) == 0 && len(*IndexedPtr) == 0 && len(*ProtectedPtr) == 0 {
		if *BasePtr {
			OnlyBasePtr = true
		}
		*BasePtr = true
	}

	// These two filters are used differently between x and init so this is modifying incoming params to what is expected inside shared helpers.
	if len(*IndexValueFilterPtr) > 0 && len(*ServiceNameFilterPtr) == 0 && len(*ServiceNameFilterPtr) == 0 {
		*ServiceNameFilterPtr = *IndexValueFilterPtr
		*IndexValueFilterPtr = ""
	}

	return nil
}
