package util

import (
	"fmt"
	"os"
	"os/user"
	"strconv"
)

// CheckNotSudo -- checks if current user is sudoer and exits if they are.
func CheckNotSudo() {
	return
	sudoer, sudoErr := user.LookupGroup("sudo")
	if sudoErr != nil {
		fmt.Fprintln(os.Stderr, "Trcsh unable to definitively identify sudoers.")
		os.Exit(-1)
	}
	sudoerGid, sudoConvErr := strconv.Atoi(sudoer.Gid)
	if sudoConvErr != nil {
		fmt.Fprintln(os.Stderr, "Trcsh unable to definitively identify sudoers.  Conversion error.")
		os.Exit(-1)
	}
	groups, groupErr := os.Getgroups()
	if groupErr != nil {
		fmt.Fprintln(os.Stderr, "Trcsh unable to definitively identify sudoers.  Missing groups.")
		os.Exit(-1)
	}
	for _, groupId := range groups {
		if groupId == sudoerGid {
			fmt.Fprintln(os.Stderr, "Trcsh cannot be run with user having sudo privileges.")
			os.Exit(-1)
		}
	}
}
