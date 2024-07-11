package marshal

import (
	"errors"
	"fmt"
	"reflect"

	"github.com/bingoohuang/gg/pkg/handy"
)

// Marshaler interface is to provide serialize and deserialize methods for BRaft Node
type Marshaler interface {
	// Marshal is used to serialize and data to a []byte
	Marshal(data any) ([]byte, error)

	// Unmarshal is used to deserialize []byte to any
	Unmarshal(data []byte, v any) error
}

type Data struct {
	Type    string
	Payload []byte
	IsPtr   bool
}

type TypeRegister struct {
	Marshaler
	TypeMap map[string]reflect.Type
	lock    handy.Lock
}

func NewTypeRegister(m Marshaler) *TypeRegister {
	return &TypeRegister{TypeMap: map[string]reflect.Type{}, Marshaler: m}
}

func (t *TypeRegister) newType(typ string) (ptr reflect.Value, ok bool) {
	defer t.lock.LockDeferUnlock()()

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

	defer t.lock.LockDeferUnlock()()

	if _, ok := t.TypeMap[typName]; !ok {
		t.TypeMap[typName] = typ
	}

	return typName, isPtr
}

var ErrUnknownType = errors.New("unknown type")

func (t *TypeRegister) Unmarshal(data []byte) (any, error) {
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

	if err := func() error {
		if adapter, ok := pi.(TypeRegisterUnmarshalerAdapter); ok {
			return adapter.Unmarshal(t, wrap.Payload)
		}

		return t.Marshaler.Unmarshal(wrap.Payload, pi)
	}(); err != nil {
		return nil, err
	}

	if wrap.IsPtr {
		return pi, nil
	}

	return ptr.Elem().Interface(), nil
}

type TypeRegisterMarshalerAdapter interface {
	Marshal(*TypeRegister) ([]byte, error)
}

type TypeRegisterUnmarshalerAdapter interface {
	Unmarshal(*TypeRegister, []byte) error
}

func (t *TypeRegister) Marshal(data any) ([]byte, error) {
	payload, err := func() ([]byte, error) {
		if adapter, ok := data.(TypeRegisterMarshalerAdapter); ok {
			return adapter.Marshal(t)
		}

		return t.Marshaler.Marshal(data)
	}()
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
