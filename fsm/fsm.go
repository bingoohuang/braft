package fsm

import (
	"github.com/bingoohuang/braft/serializer"
	"github.com/hashicorp/raft"
)

// FSM is the interface for using Finite State Machine in Raft Cluster
type FSM interface {
	raft.FSM

	// Init is used to pass the original serializer from BRaft Node to be able to deserialize messages
	// coming from other nodes
	Init(ser serializer.Serializer)
}

// Service interface makes it easier to build State Machines
type Service interface {
	// Name returns the unique ID/Name which will identify the FSM Service when it comes to routing incoming messages
	Name() string

	// NewLog is called when a new raft log message is committed in the cluster and matched with any of the GetReqDataTypes returned types
	// in this method we can handle what should happen when we got a new raft log regarding our FSM service
	NewLog(request map[string]interface{}) interface{}

	// GetReqDataTypes returns all the request structs which are used by this FSMService
	GetReqDataType() interface{}

	// ApplySnapshot is used to decode and apply a snapshot to the FSMService
	ApplySnapshot(input interface{}) error
}
