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

func Contains(a []string, b string) bool {
	for _, c := range a {
		if b == c {
			return true
		}
	}

	return false
}

// https://stackoverflow.com/questions/36000487/check-for-equality-on-slices-without-order
func SliceEqual(x, y []string) bool {
	if len(x) != len(y) {
		return false
	}

	// create a map of string -> int
	diff := make(map[string]int, len(x))
	for _, _x := range x {
		// 0 value for int is 0, so just increment a counter for the string
		diff[_x]++
	}
	for _, _y := range y {
		// If the string _y is not in diff bail out early
		if _, ok := diff[_y]; !ok {
			return false
		}
		diff[_y] -= 1
		if diff[_y] == 0 {
			delete(diff, _y)
		}
	}
	return len(diff) == 0
}
