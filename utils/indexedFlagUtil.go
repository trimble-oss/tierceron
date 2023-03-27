package utils

import (
	"flag"
	"fmt"
	"os"
	"strings"
)

var SubPathFilter = flag.String("subPathFilter", "", "Specifies subpath templates to filter")                           // a sub path filter.
var ServiceNameFilterPtr = flag.String("serviceExtFilter", "", "Specifies which nested services (or tables) to filter") //offset or database
var ServiceFilterPtr = flag.String("serviceFilter", "", "Specifies which services (or tables) to filter")               // Table names
var IndexNameFilterPtr = flag.String("indexFilter", "", "Specifies which index names to filter")                        // column index, table to filter.
var IndexValueFilterPtr = flag.String("indexValueFilter", "", "Specifies which index values to filter")                 // column index value to filter on.
var IndexedPtr = flag.String("indexed", "", "Specifies which projects are indexed")                                     // Indicates indexed projects...
var RestrictedPtr = flag.String("restricted", "", "Specifies which projects have restricted access.")
var ProtectedPtr = flag.String("protected", "", "Specifies which projects have protected access.")
var BasePtr = flag.Bool("base", false, "Specifies whether the base env seed file will be seeded")
var OnlyBasePtr = false

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
func CheckInitFlags() {
	filtered := false
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
