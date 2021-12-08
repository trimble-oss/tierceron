package util

import (
	"errors"
	"strconv"
	"strings"

	sys "tierceron/vaulthelper/system"

	"github.com/txn2/txeh"
)

func GetLocalVaultHost() (string, error) {
	vaultHost := "https://"
	vaultErr := errors.New("No usable local vault found.")
	hostFileLines, pherr := txeh.ParseHosts("/etc/hosts")
	if pherr != nil {
		return "", pherr
	}

	for _, hostFileLine := range hostFileLines {
		for _, host := range hostFileLine.Hostnames {
			if (strings.Contains(host, "whoboot.org") || strings.Contains(host, "dexchadev.org") || strings.Contains(host, "dexterchaney.com")) && strings.Contains(hostFileLine.Address, "127.0.0.1") {
				vaultHost = vaultHost + host
				break
			}
		}
	}

	// Now, look for vault.
	for i := 8000; i < 9000; i++ {
		_, err := sys.NewVault(true, vaultHost+":"+strconv.Itoa(i), "", false, true)
		if err == nil {
			vaultHost = vaultHost + ":" + strconv.Itoa(i)
			vaultErr = nil
			break
		}
	}

	return vaultHost, vaultErr
}
