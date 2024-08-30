package trcnet

import (
	"fmt"
	"net"
	"strings"
)

func LocalIp() (string, error) {

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
