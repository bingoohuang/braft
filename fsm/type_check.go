package fsm

import (
	"errors"
	"reflect"
)

func MatchAll(a []string, f func(int, string) bool) bool {
	for i, c := range a {
		if !f(i, c) {
			return false
		}
	}

	return true
}

func Contains(a []string, b string) bool {
	for _, c := range a {
		if b == c {
			return true
		}
	}

	return false
}

func getTargetType(types []interface{}, expectedFields []string) (interface{}, error) {
	for _, i := range types {
		// get current type fields list
		v := reflect.ValueOf(i)
		typeOfS := v.Type()
		currentTypeFields := make([]string, v.NumField())
		for i := 0; i < v.NumField(); i++ {
			currentTypeFields[i] = typeOfS.Field(i).Name
		}

		// compare current vs expected fields
		foundAllFields := MatchAll(expectedFields, func(i int, s string) bool {
			return Contains(currentTypeFields, s)
		})

		if foundAllFields && len(expectedFields) == len(currentTypeFields) {
			return i, nil
		}
	}

	return nil, errors.New("unknown type!")
}
