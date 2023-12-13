package utils

import (
	"flag"
	"fmt"
	"os"
	"strings"
)

var SubPathFilter *string
var ServiceNameFilterPtr *string
var ServiceFilterPtr *string
var IndexNameFilterPtr *string
var IndexValueFilterPtr *string
var IndexedPtr *string
var RestrictedPtr *string
var ProtectedPtr *string
var BasePtr *bool
var OnlyBasePtr = false

func initglobals(flagset *flag.FlagSet) {
	SubPathFilter = flagset.String("subPathFilter", "", "Specifies subpath templates to filter")                           // a sub path filter.
	ServiceNameFilterPtr = flagset.String("serviceExtFilter", "", "Specifies which nested services (or tables) to filter") //offset or database
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
				fmt.Println("Usage of comma delimited -restricted or -indexed is not allowed with additional filters")
				os.Exit(1)
			}
		} else {
			fmt.Println("Usage of -indexFilter is required for -indexValueFilter")
			os.Exit(1)
		}
	} else {
		fmt.Println("Usage of -indexValueFilter is required")
		os.Exit(1)
	}
}

func CheckInitFlags(flagset *flag.FlagSet) {
	filtered := false
	initglobals(flagset)
	//Cannot specify a pathed indexed/restricted seed file while specifying a restricted/indexed section.
	if len(*IndexNameFilterPtr) > 0 || len(*ServiceNameFilterPtr) > 0 || len(*IndexValueFilterPtr) > 0 || len(*ServiceNameFilterPtr) > 0 {
		filtered = true
	}
	if len(*RestrictedPtr) > 0 && len(*IndexedPtr) > 0 && filtered {
		fmt.Println("Cannot use -restricted and -indexed at the same time while trying to specify a seed file.")
		os.Exit(1)
	}

	//Same reason as above, but with protected.
	if len(*ProtectedPtr) > 0 && len(*IndexedPtr) > 0 {
		fmt.Println("Cannot use -protected with -indexed.")
		os.Exit(1)
	}

	if len(*RestrictedPtr) > 0 && len(*ProtectedPtr) > 0 && filtered {
		fmt.Println("Cannot use -restricted and -protected at the same time while trying to specify a seed file.")
		os.Exit(1)
	}

	if (len(*RestrictedPtr) > 0 || len(*IndexedPtr) > 0) && filtered && (strings.Contains(*RestrictedPtr, ",") || strings.Contains(*IndexedPtr, ",")) {
		fmt.Println("Cannot use comma delimited lists with filters.")
		os.Exit(1)
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

	//These two filters are used differently between x and init so this is modifying incoming params to what is expected inside shared helpers.
	if len(*IndexValueFilterPtr) > 0 && len(*ServiceNameFilterPtr) == 0 && len(*ServiceNameFilterPtr) == 0 {
		*ServiceNameFilterPtr = *IndexValueFilterPtr
		*IndexValueFilterPtr = ""
	}

}
