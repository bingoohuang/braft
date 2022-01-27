package serializer

import (
	"github.com/vmihailenco/msgpack/v5"
)

func NewMsgPackSerializer() Serializer {
	return &MsgPackSerializer{}
}

type MsgPackSerializer struct{}

func (s *MsgPackSerializer) Serialize(data interface{}) ([]byte, error) {
	return msgpack.Marshal(data)
}

func (s *MsgPackSerializer) Deserialize(data []byte) (result interface{}, err error) {
	err = msgpack.Unmarshal(data, &result)
	return
}
