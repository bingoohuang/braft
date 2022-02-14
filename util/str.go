package util

import (
	"sort"
	"strings"
)

func Or(a, b string) string {
	if a == "" {
		return b
	}

	return a
}

func MapToString(m map[string]string, kkSep, kvSep string) string {
	var items []string
	var ks []string
	for k := range m {
		ks = append(ks, k)
	}

	sort.Strings(ks)

	for _, k := range ks {
		items = append(items, k+kvSep+m[k])
	}

	return strings.Join(items, kkSep)
}

func ParseStringToMap(s, kkSep, kvSep string) map[string]string {
	entries := strings.Split(s, kkSep)

	m := make(map[string]string)
	for _, e := range entries {
		parts := strings.Split(e, kvSep)
		if parts[0] == "" {
			continue
		}

		if len(parts) > 1 {
			m[parts[0]] = parts[1]
		} else {
			m[parts[0]] = ""
		}

	}

	return m
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
