package fsm

import "github.com/bingoohuang/braft/marshal"

// Service interface makes it easier to build State Machines
type Service interface {
	// NewLog is called when a new raft log message is committed in the cluster and matched with any of the GetReqDataTypes returned types
	// in this method we can handle what should happen when we got a new raft log regarding our FSM service
	NewLog(shortNodeID string, request interface{}) interface{}

	// GetReqDataTypes returns all the request structs which are used by this FSMService
	GetReqDataType() interface{}

	// ApplySnapshot is used to decode and apply a snapshot to the FSMService
	ApplySnapshot(shortNodeID string, input interface{}) error

	// RegisterMarshalTypes registers the types for marshaling and unmarshaling.
	RegisterMarshalTypes(typeRegister *marshal.TypeRegister)
}
