package util

import (
	"fmt"
	"net"
	"os"
	"strconv"
)

func Env(name ...string) string {
	for _, n := range name {
		if v := os.Getenv(n); v != "" {
			return v
		}
	}

	return ""
}

func Atoi(v string, defaultValue int) int {
	if a, err := strconv.Atoi(v); err != nil {
		return defaultValue
	} else {
		return a
	}
}

func RandPort(ip string, defaultPort int) int {
	l, err := net.Listen("tcp", ip+":0")
	if err != nil {
		return defaultPort
	}

	p := l.Addr().(*net.TCPAddr).Port
	l.Close()
	return p
}

func IsPortFree(ip string, port int) bool {
	ln, err := net.Listen("tcp", fmt.Sprintf("%s:%d", ip, port))
	if err != nil {
		return false
	}

	ln.Close()
	return true
}

func FindFreePort(ip string, port int) int {
	if IsPortFree(ip, port) {
		return port
	}

	return RandPort(ip, port)
}
