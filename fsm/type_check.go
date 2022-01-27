package fsm

import (
	"errors"
	"reflect"
)

type ReqTypeInfo struct {
	Service   Service
	ReqFields []string
}

func MakeReqTypeInfo(service Service) ReqTypeInfo {
	// get current type fields list
	t := reflect.TypeOf(service.GetReqDataType())

	typeFields := make([]string, t.NumField())
	for i := 0; i < t.NumField(); i++ {
		typeFields[i] = t.Field(i).Name
	}

	return ReqTypeInfo{
		Service:   service,
		ReqFields: typeFields,
	}
}

func getTargetTypeInfo(types []ReqTypeInfo, expectedFields []string) (ReqTypeInfo, error) {
	for _, i := range types {
		if SliceEqual(expectedFields, i.ReqFields) {
			return i, nil
		}
	}

	return ReqTypeInfo{}, errors.New("unknown type!")
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
