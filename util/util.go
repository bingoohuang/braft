package util

import (
	"fmt"
	"github.com/spf13/viper"
	"log"
	"net"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/bingoohuang/gg/pkg/thinktime"
)

// ErrorString returns the error string, err can be nil.
func ErrorString(err error) string {
	if err != nil {
		return err.Error()
	}

	return ""
}

func OrSlice(a, b map[string]string) map[string]string {
	if len(a) > 0 {
		return a
	}

	return b
}

// Cut cuts the s into two ones.
func Cut(s, sep string) (a, b string) {
	ret := strings.Split(s, sep)
	if len(ret) >= 2 {
		return ret[0], ret[1]
	} else if len(ret) >= 1 {
		return ret[0], ""
	} else {
		return "", ""
	}
}

// Think sleeps for a duration by envValue.
func Think(thinkTime string) {
	if tt, _ := thinktime.ParseThinkTime(thinkTime); tt != nil {
		if sleeping := tt.Think(false); sleeping > 0 {
			log.Printf("sleeping for thinkTime %s", sleeping)
			time.Sleep(sleeping)
		}
	}
}

func EnvDuration(name string, defaultValue time.Duration) time.Duration {
	env := Env(name)
	if env == "" {
		return defaultValue
	}

	d, err := time.ParseDuration(env)
	if err != nil {
		log.Printf("E! parse env $%s = %s failed: %v", name, env, err)
		return defaultValue
	}
	return d
}

func Env(name ...string) string {
	for _, n := range name {
		if v := os.Getenv(n); v != "" {
			return v
		}
	}

	for _, n := range name {
		if v := viper.GetString(n); v != "" {
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

// RemoveCreateDir - create a directory structure, if still exist -> delete it before
func RemoveCreateDir(folderPath string) error {
	if IsDir(folderPath) {
		if err := os.RemoveAll(folderPath); err != nil {
			return err
		}
	}
	return os.MkdirAll(folderPath, os.ModePerm)
}

// IsDir - Check if input path is a directory
func IsDir(dirInput string) bool {
	fi, err := os.Stat(dirInput)
	if err != nil {
		return false
	}
	switch mode := fi.Mode(); {
	case mode.IsDir():
		return true
	case mode.IsRegular():
		return false
	}

	return false
}

func FileExists(name string) bool {
	if _, err := os.Stat(name); err != nil {
		if os.IsNotExist(err) {
			return false
		}
	}
	return true
}
