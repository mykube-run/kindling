package utils

import (
	"fmt"
	"net"
	"strings"
)

// LocalIP returns local loopback IP
func LocalIP() (ip net.IP, err error) {
	// Get all interfaces
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return nil, err
	}
	// Get first non-loopback ip
	for _, addr := range addrs {
		if ipNet, ok := addr.(*net.IPNet); ok && !ipNet.IP.IsLoopback() {
			if ipNet.IP.To4() != nil {
				ip = ipNet.IP
				return
			}
		}
	}

	return nil, fmt.Errorf("no local ip found")
}

// LocalIPString returns local loopback IP address in string format
func LocalIPString() (ip string, err error) {
	ipv, err := LocalIP()
	if err == nil {
		return ipv.String(), nil
	}
	return "", err
}

// IsIPv4 returns whether v is IPv4
func IsIPv4(v string) bool {
	spl := strings.Split(v, ":")
	return len(spl) <= 2
}
