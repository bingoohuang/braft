package util

import (
	"strings"
)

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
