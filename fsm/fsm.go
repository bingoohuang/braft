package fsm

// Service interface makes it easier to build State Machines
type Service interface {
	// Name returns the unique ID/Name which will identify the FSM Service when it comes to routing incoming messages
	Name() string

	// NewLog is called when a new raft log message is committed in the cluster and matched with any of the GetReqDataTypes returned types
	// in this method we can handle what should happen when we got a new raft log regarding our FSM service
	NewLog(request interface{}) interface{}

	// GetReqDataTypes returns all the request structs which are used by this FSMService
	GetReqDataType() interface{}

	// ApplySnapshot is used to decode and apply a snapshot to the FSMService
	ApplySnapshot(input interface{}) error
}
