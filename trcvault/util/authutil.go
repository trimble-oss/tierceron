package util

import (
	"fmt"
	"os"
	"os/user"
	"strconv"
)

// CheckNotSudo -- checks if current user is sudoer and exits if they are.
func CheckNotSudo() {
	sudoer, sudoErr := user.LookupGroup("sudo")
	if sudoErr != nil {
		fmt.Println("Trcsh unable to definitively identify sudoers.")
		os.Exit(-1)
	}
	sudoerGid, sudoConvErr := strconv.Atoi(sudoer.Gid)
	if sudoConvErr != nil {
		fmt.Println("Trcsh unable to definitively identify sudoers.  Conversion error.")
		os.Exit(-1)
	}
	groups, groupErr := os.Getgroups()
	if groupErr != nil {
		fmt.Println("Trcsh unable to definitively identify sudoers.  Missing groups.")
		os.Exit(-1)
	}
	for _, groupId := range groups {
		if groupId == sudoerGid {
			fmt.Println("Trcsh cannot be run with user having sudo privileges.")
			os.Exit(-1)
		}
	}

}
