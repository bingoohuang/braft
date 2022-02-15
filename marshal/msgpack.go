package marshal

import (
	"github.com/vmihailenco/msgpack/v5"
)

func NewMsgPacker() Marshaler { return &MsgPacker{} }

type MsgPacker struct{}

func (s *MsgPacker) Marshal(d interface{}) ([]byte, error)   { return msgpack.Marshal(d) }
func (s *MsgPacker) Unmarshal(d []byte, v interface{}) error { return msgpack.Unmarshal(d, v) }
