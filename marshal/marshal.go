package marshal

import (
	"errors"
	"fmt"
	"reflect"
	"sync"
)

// Marshaler interface is to provide serialize and deserialize methods for BRaft Node
type Marshaler interface {
	// Marshal is used to serialize and data to a []byte
	Marshal(data interface{}) ([]byte, error)

	// Unmarshal is used to deserialize []byte to interface{}
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

func (t *TypeRegister) RegisterType(typ reflect.Type) (typName string, isPtr bool) {
	if typ == nil {
		return "nil", false
	}

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
	if wrap.Type == "nil" {
		return nil, nil
	}

	ptr, ok := t.newType(wrap.Type)
	if !ok {
		return nil, fmt.Errorf("%s: %w", wrap.Type, ErrUnknownType)
	}
	pi := ptr.Interface()

	var err error
	if customMarshal, ok := pi.(TypeRegisterUnmarshaler); ok {
		err = customMarshal.UnmarshalMsgpack(t, wrap.Payload)
	} else {
		err = t.Marshaler.Unmarshal(wrap.Payload, pi)
	}
	if err != nil {
		return nil, err
	}

	if wrap.IsPtr {
		return pi, nil
	}

	return ptr.Elem().Interface(), nil
}

type TypeRegisterMarshaler interface {
	MarshalMsgpack(*TypeRegister) ([]byte, error)
}

type TypeRegisterUnmarshaler interface {
	UnmarshalMsgpack(*TypeRegister, []byte) error
}

func (t *TypeRegister) Marshal(data interface{}) ([]byte, error) {
	var (
		payload []byte
		err     error
	)

	if customMarshal, ok := data.(TypeRegisterMarshaler); ok {
		payload, err = customMarshal.MarshalMsgpack(t)
	} else {
		payload, err = t.Marshaler.Marshal(data)
	}

	if err != nil {
		return nil, err
	}

	typName, isPtr := t.RegisterType(reflect.TypeOf(data))
	wrap := Data{
		Payload: payload,
		Type:    typName,
		IsPtr:   isPtr,
	}

	return t.Marshaler.Marshal(wrap)
}
