package util

import (
	"strings"
)

func Or(a, b string) string {
	if a == "" {
		return b
	}

	return a
}

func ParseStringToMap(s, kkSep, kvSep string) map[string]string {
	entries := strings.Split(s, kkSep)

	m := make(map[string]string)
	for _, e := range entries {
		parts := strings.Split(e, kvSep)
		m[parts[0]] = parts[1]
	}

	return m
}

// Cut cut the s into two ones.
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
