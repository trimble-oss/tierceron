package capauth

import (
	"errors"
	"fmt"
	"net"
	"strings"
)

func LocalIp(env string) (string, error) {

	interfaces, err := net.Interfaces()
	if err != nil {
		return "", err
	}

	for _, iface := range interfaces {
		if strings.HasPrefix(iface.Name, "eth0") {
			addrs, err := iface.Addrs()
			if err != nil {
				fmt.Println("Error getting addresses for", iface.Name, ":", err)
				continue
			}

			for _, address := range addrs {
				// Check if address belongs to eth0
				if ipnet, ok := address.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
					if ipnet.IP.To4() != nil {
						return ipnet.IP.String(), nil
					}
				}
			}
		}
	}
	return "", err
}

func LocalAddr(env string) (string, error) {
	localIP, err := LocalIp(env)
	if err != nil {
		return "", err
	}
	addrs, hostErr := net.LookupAddr(localIP)
	if hostErr != nil {
		return "", hostErr
	}
	localHost := ""
	if len(addrs) > 0 {
		if len(addrs) > 20 {
			return "", errors.New("unsupported hosts")
		}
		for _, addr := range addrs {
			localHost = strings.TrimRight(addr, ".")
			if validErr := ValidateVhost(localHost, ""); validErr != nil {
				localHost = ""
				continue
			} else {
				break
			}
		}
	} else {
		return "", errors.New("invalid host")
	}

	return localHost, nil
}
