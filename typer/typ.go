package typer

import "reflect"

// SameType tells whether the type of v1 and v2 is the same, ignoring the pointer.
func SameType(v1, v2 interface{}) bool {
	typ1 := reflect.TypeOf(v1)
	typ2 := reflect.TypeOf(v2)
	if typ1 == typ2 {
		return true
	}

	if typ1.Kind() == reflect.Ptr {
		typ1 = typ1.Elem()
	}

	if typ2.Kind() == reflect.Ptr {
		typ2 = typ2.Elem()
	}

	return typ1 == typ2
}

// ConvertibleTo reports whether data can be converted target type.
func ConvertibleTo(data interface{}, u reflect.Type) (interface{}, bool) {
	v1 := reflect.ValueOf(data)
	t1 := v1.Type()

	if t1.ConvertibleTo(u) {
		return data, true
	}

	if t1.Kind() == reflect.Ptr {
		t1 = t1.Elem()
		v1 = v1.Elem()
		if t1.ConvertibleTo(u) {
			return v1.Interface(), true
		}
	}

	if t1.Kind() == reflect.Struct {
		for i := 0; i < t1.NumField(); i++ {
			if f := t1.Field(i); f.Anonymous && f.Type.Kind() == reflect.Interface {
				if v2, ok := ConvertibleTo(v1.Field(i).Interface(), u); ok {
					return v2, ok
				}
			}
		}
	}

	return nil, false
}
