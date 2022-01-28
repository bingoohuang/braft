package util

import (
	"fmt"
	"net"
	"os"
	"strconv"
)

func GetEnvInt(name string, defaultValue int) int {
	if s := os.Getenv(name); s != "" {
		if a, err := strconv.Atoi(s); err != nil {
			return defaultValue
		} else {
			return a
		}
	}

	return defaultValue
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
