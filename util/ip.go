package util

import (
	"fmt"
	"log"
	"net"
	"slices"
	"strings"
)

func MustHostIP() string {
	ips, err := HostIP()
	if err != nil {
		panic(err)
	}
	if len(ips) == 0 {
		panic("no host ip")
	}

	log.Printf("host ip: %v", ips)
	return ips[0]
}

func HostIP() ([]string, error) {
	list, err := net.Interfaces()
	if err != nil {
		return nil, fmt.Errorf("failed to get interfaces, err: %w", err)
	}

	var ips []string

	for _, i := range list {
		f := i.Flags
		if i.HardwareAddr == nil ||
			f&net.FlagUp != net.FlagUp ||
			f&net.FlagLoopback == net.FlagLoopback {
			continue
		}

		addrs, err := i.Addrs()
		if err != nil {
			continue
		}

		for _, a := range addrs {
			var ip net.IP
			switch v := a.(type) {
			case *net.IPAddr:
				ip = v.IP
			case *net.IPNet:
				ip = v.IP
			default:
				continue
			}

			ipStr := ip.String()
			if strings.Index(ipStr, ":") < 0 && !slices.Contains(ips, ipStr) {
				ips = append(ips, ipStr)
			}
		}
	}

	return ips, nil
}
