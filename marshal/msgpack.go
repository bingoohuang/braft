package marshal

import (
	"github.com/vmihailenco/msgpack/v5"
)

func NewMsgPackSerializer() Marshaler { return &MsgPackSerializer{} }

type MsgPackSerializer struct{}

func (s *MsgPackSerializer) Marshal(data interface{}) ([]byte, error) {
	return msgpack.Marshal(data)
}

func (s *MsgPackSerializer) Unmarshal(data []byte, v interface{}) error {
	return msgpack.Unmarshal(data, v)
}
