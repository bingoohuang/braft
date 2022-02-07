package fsm

import (
	"errors"
	"io"
	"io/ioutil"
	"log"
	"reflect"
	"sync"

	"github.com/bingoohuang/braft/marshal"
	"github.com/hashicorp/raft"
)

type FSM struct {
	ser          *marshal.TypeRegister
	services     []Service
	reqDataTypes []ReqTypeInfo
}

func NewRoutingFSM(services []Service, ser *marshal.TypeRegister) raft.FSM {
	i := &FSM{
		services: services,
		ser:      ser,
	}
	for _, service := range services {
		i.reqDataTypes = append(i.reqDataTypes, MakeReqTypeInfo(service))
	}

	return i
}

func (i *FSM) Apply(raftlog *raft.Log) interface{} {
	switch raftlog.Type {
	case raft.LogCommand:
		payload, err := i.ser.Unmarshal(raftlog.Data)
		if err != nil {
			return err
		}

		// routing request to service
		if target, err := getTargetTypeInfo(i.reqDataTypes, payload); err == nil {
			return target.Service.NewLog(payload)
		}

		return errors.New("unknown request data type")
	}

	return nil
}

func (i *FSM) Snapshot() (raft.FSMSnapshot, error) { return newFSMSnapshot(i), nil }

func (i *FSM) Restore(closer io.ReadCloser) error {
	snapData, err := ioutil.ReadAll(closer)
	if err != nil {
		return err
	}
	servicesData, err := i.ser.Unmarshal(snapData)
	if err != nil {
		return err
	}

	serviceType := reflect.TypeOf(servicesData)
	for _, service := range i.services {
		if reflect.TypeOf(service) == serviceType {
			if err = service.ApplySnapshot(servicesData); err != nil {
				log.Printf("Failed to apply snapshot to service %q!", service.Name())
			}
		}
	}
	return nil
}

type ReqTypeInfo struct {
	Service Service
	ReqType reflect.Type
}

func MakeReqTypeInfo(service Service) ReqTypeInfo {
	// get current type fields list
	t := reflect.TypeOf(service.GetReqDataType())

	return ReqTypeInfo{
		Service: service,
		ReqType: t,
	}
}

func getTargetTypeInfo(types []ReqTypeInfo, result interface{}) (ReqTypeInfo, error) {
	resultType := reflect.TypeOf(result)
	for _, typ := range types {
		if typ.ReqType == resultType {
			return typ, nil
		}
	}

	return ReqTypeInfo{}, marshal.ErrUnknownType
}

type FSMSnapshot struct {
	sync.Mutex
	fsm *FSM
}

func newFSMSnapshot(fsm *FSM) raft.FSMSnapshot { return &FSMSnapshot{fsm: fsm} }

func (i *FSMSnapshot) Persist(sink raft.SnapshotSink) error {
	i.Lock()
	snapshotData, err := i.fsm.ser.Marshal(i.fsm.services)
	if err != nil {
		return err
	}
	_, err = sink.Write(snapshotData)
	return err
}

func (i *FSMSnapshot) Release() { i.Unlock() }
