package fsm

import (
	"errors"
	"io"
	"log"
	"reflect"
	"sync"
	"sync/atomic"

	"github.com/bingoohuang/braft/marshal"
	"github.com/bingoohuang/braft/typer"
	"github.com/hashicorp/raft"
)

type FSM struct {
	ser         *marshal.TypeRegister
	shortNodeID string

	services     []Service
	reqDataTypes []ReqTypeInfo

	RaftLogSum uint64
}

func NewRoutingFSM(shortNodeID string, services []Service, ser *marshal.TypeRegister) *FSM {
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

	if raftlog.Type == raft.LogCommand {
		payload, err := i.ser.Unmarshal(raftlog.Data)
		if err != nil {
			log.Printf("E! Unmarshal, error: %v", err)
			return err
		}

		atomic.AddUint64(&i.RaftLogSum, uint64(len(raftlog.Data)))

		// routing request to service
		if target, err := getTargetTypeInfo(i.reqDataTypes, payload); err == nil {
			return target.Service.NewLog(i.shortNodeID, payload)
		}

		log.Printf("E! unknown request data type, data: %s, error: %v", raftlog.Data, err)
		return errors.New("unknown request data type")
	}

	return nil
}

func (i *FSM) Snapshot() (raft.FSMSnapshot, error) { return newSnapshot(i), nil }

func (i *FSM) Restore(closer io.ReadCloser) error {
	log.Printf("FSM Restore")

	snapData, err := io.ReadAll(closer)
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

	return ReqTypeInfo{Service: service, ReqType: t}
}

func getTargetTypeInfo(types []ReqTypeInfo, result interface{}) (ReqTypeInfo, error) {
	resultType := reflect.TypeOf(result)
	var typs []ReqTypeInfo
	for _, typ := range types {
		if typ.ReqType == resultType {
			typs = append(typs, typ)
		}
	}

	if len(typs) == 1 {
		return typs[0], nil
	}

	for _, typ := range typs {
		if dt, ok1 := typer.ConvertibleTo(typ.Service, SubDataTypeAwareType); ok1 {
			if da, ok2 := typer.ConvertibleTo(result, SubDataAwareType); ok2 {
				if _, ok3 := typer.ConvertibleTo(da.(SubDataAware).GetSubData(), dt.(SubDataTypeAware).GetSubDataType()); ok3 {
					return typ, nil
				}
			}
		}
	}

	return ReqTypeInfo{}, marshal.ErrUnknownType
}

type Snapshot struct {
	fsm *FSM
	sync.Mutex
}

func newSnapshot(fsm *FSM) raft.FSMSnapshot { return &Snapshot{fsm: fsm} }

func (i *Snapshot) Persist(sink raft.SnapshotSink) error {
	i.Lock()
	snapshotData, err := i.fsm.ser.Marshal(i.fsm.services)
	if err != nil {
		return err
	}
	_, err = sink.Write(snapshotData)
	return err
}

func (i *Snapshot) Release() { i.Unlock() }
