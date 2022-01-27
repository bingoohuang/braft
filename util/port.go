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

func RandPort(defaultPort int) int {
	l, err := net.Listen("tcp", ":0")
	if err != nil {
		return defaultPort
	}

	p := l.Addr().(*net.TCPAddr).Port
	l.Close()
	return p
}

func IsPortFree(port int) bool {
	ln, err := net.Listen("tcp", fmt.Sprintf(":%d", +port))
	if err != nil {
		return false
	}

	ln.Close()
	return true
}

func FindFreePort(port int) int {
	if IsPortFree(port) {
		return port
	}

	return RandPort(port)
}
