package marshal

import (
	"errors"
	"fmt"
	"reflect"
	"sync"
)

// Marshaler interface is to provide serialize and deserialize methods for BRaft Node
type Marshaler interface {
	// Serialize is used to serialize and data to a []byte
	Marshal(data interface{}) ([]byte, error)

	// Deserialize is used to deserialize []byte to interface{}
	Unmarshal(data []byte, v interface{}) error
}

type Data struct {
	Payload []byte
	Type    string
	IsPtr   bool
}

type TypeRegister struct {
	sync.Mutex
	TypeMap map[string]reflect.Type
	Marshaler
}

func NewTypeRegister(ser Marshaler) *TypeRegister {
	return &TypeRegister{
		TypeMap:   map[string]reflect.Type{},
		Marshaler: ser,
	}
}

func (t *TypeRegister) newType(typ string) (ptr reflect.Value, ok bool) {
	t.Lock()
	defer t.Unlock()

	if rt, ok := t.TypeMap[typ]; !ok {
		return reflect.Value{}, false
	} else {
		return reflect.New(rt), true
	}
}

func (t *TypeRegister) registerType(i interface{}) (typName string, isPtr bool) {
	typ := reflect.TypeOf(i)
	if isPtr = typ.Kind() == reflect.Ptr; isPtr {
		typ = typ.Elem()
	}

	typName = typ.String()

	t.Lock()
	defer t.Unlock()

	if _, ok := t.TypeMap[typName]; !ok {
		t.TypeMap[typName] = typ
	}

	return typName, isPtr
}

var ErrUnknownType = errors.New("unknown type")

func (t *TypeRegister) Unmarshal(data []byte) (interface{}, error) {
	wrap := Data{}
	if err := t.Marshaler.Unmarshal(data, &wrap); err != nil {
		return nil, err
	}

	ptr, ok := t.newType(wrap.Type)
	if !ok {
		return nil, fmt.Errorf("%s: %w", wrap.Type, ErrUnknownType)
	}

	if err := t.Marshaler.Unmarshal(wrap.Payload, ptr.Interface()); err != nil {
		return nil, err
	}

	if wrap.IsPtr {
		return ptr.Interface(), nil
	}

	return ptr.Elem().Interface(), nil
}

func (t *TypeRegister) Marshal(data interface{}) ([]byte, error) {
	payload, err := t.Marshaler.Marshal(data)
	if err != nil {
		return nil, err
	}

	typName, isPtr := t.registerType(data)

	wrap := Data{
		Payload: payload,
		Type:    typName,
		IsPtr:   isPtr,
	}

	return t.Marshaler.Marshal(wrap)
}
