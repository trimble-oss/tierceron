package utils

// CheckError Simplifies the error checking process
func CheckError(e error) {
	if e != nil {
		panic(e)
	}
}

// CheckWarnings Checks warnings returned from various vault relation operations
func CheckWarnings(w []string) {
	if len(w) > 0 {
		panic(w)
	}
}
