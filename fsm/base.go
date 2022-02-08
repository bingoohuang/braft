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
	shortNodeID  string
}

func NewRoutingFSM(shortNodeID string, services []Service, ser *marshal.TypeRegister) raft.FSM {
	i := &FSM{
		services:    services,
		ser:         ser,
		shortNodeID: shortNodeID,
	}
	for _, service := range services {
		i.reqDataTypes = append(i.reqDataTypes, MakeReqTypeInfo(service))
	}

	return i
}

func (i *FSM) Apply(raftlog *raft.Log) interface{} {
	log.Printf("FSM Apply LogType: %s", raftlog.Type)

	switch raftlog.Type {
	case raft.LogCommand:
		payload, err := i.ser.Unmarshal(raftlog.Data)
		if err != nil {
			log.Printf("E! Unmarshal, error: %v", err)
			return err
		}

		// routing request to service
		if target, err := getTargetTypeInfo(i.reqDataTypes, payload); err == nil {
			return target.Service.NewLog(i.shortNodeID, payload)
		} else {
			log.Printf("E! unknown request data type, error: %v", err)
		}

		return errors.New("unknown request data type")
	}

	return nil
}

func (i *FSM) Snapshot() (raft.FSMSnapshot, error) { return newFSMSnapshot(i), nil }

func (i *FSM) Restore(closer io.ReadCloser) error {
	log.Printf("FSM Restore")

	snapData, err := ioutil.ReadAll(closer)
	if err != nil {
		return err
	}
	servicesData, err := i.ser.Unmarshal(snapData)
	if err != nil {
		return err
	}

	services := servicesData.([]Service)
	for _, a := range i.services {
		for _, b := range services {
			if aType := reflect.TypeOf(a); aType == reflect.TypeOf(b) {
				if err := a.ApplySnapshot(i.shortNodeID, b); err != nil {
					log.Printf("Failed to apply snapshot to service %s, error: %v!", aType, err)
				}

				break
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
