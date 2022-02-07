package fsm

import (
	"errors"
	"io"
	"io/ioutil"
	"log"
	"reflect"
	"sync"

	"github.com/bingoohuang/braft/serializer"
	"github.com/bingoohuang/braft/util"
	"github.com/hashicorp/raft"
)

type FSM struct {
	ser          serializer.Serializer
	services     map[string]Service
	reqDataTypes []ReqTypeInfo
}

func NewRoutingFSM(services []Service, ser serializer.Serializer) raft.FSM {
	servicesMap := map[string]Service{}
	for _, service := range services {
		servicesMap[service.Name()] = service
	}

	i := &FSM{
		services: servicesMap,
		ser:      ser,
	}
	for _, service := range servicesMap {
		i.reqDataTypes = append(i.reqDataTypes, MakeReqTypeInfo(service))
	}

	return i
}

func (i *FSM) Apply(raftlog *raft.Log) interface{} {
	switch raftlog.Type {
	case raft.LogCommand:
		payload, err := i.ser.Deserialize(raftlog.Data)
		if err != nil {
			return err
		}

		payloadMap := payload.(map[string]interface{})

		// check request type
		var fields []string
		for k := range payloadMap {
			fields = append(fields, k)
		}
		// routing request to service
		if target, err := getTargetTypeInfo(i.reqDataTypes, fields); err == nil {
			return target.Service.NewLog(payloadMap)
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
	servicesData, err := i.ser.Deserialize(snapData)
	if err != nil {
		return err
	}
	s := servicesData.(map[string]interface{})
	for key, service := range i.services {
		if err = service.ApplySnapshot(s[key]); err != nil {
			log.Printf("Failed to apply snapshot to %q service!", key)
		}
	}
	return nil
}

type ReqTypeInfo struct {
	Service   Service
	ReqFields []string
}

func MakeReqTypeInfo(service Service) ReqTypeInfo {
	// get current type fields list
	t := reflect.TypeOf(service.GetReqDataType())

	typeFields := make([]string, t.NumField())
	for i := 0; i < t.NumField(); i++ {
		typeFields[i] = t.Field(i).Name
	}

	return ReqTypeInfo{
		Service:   service,
		ReqFields: typeFields,
	}
}

func getTargetTypeInfo(types []ReqTypeInfo, expectedFields []string) (ReqTypeInfo, error) {
	for _, i := range types {
		if util.SliceEqual(expectedFields, i.ReqFields) {
			return i, nil
		}
	}

	return ReqTypeInfo{}, errors.New("unknown type!")
}

type FSMSnapshot struct {
	sync.Mutex
	fsm *FSM
}

func newFSMSnapshot(fsm *FSM) raft.FSMSnapshot { return &FSMSnapshot{fsm: fsm} }

func (i *FSMSnapshot) Persist(sink raft.SnapshotSink) error {
	i.Lock()
	snapshotData, err := i.fsm.ser.Serialize(i.fsm.services)
	if err != nil {
		return err
	}
	_, err = sink.Write(snapshotData)
	return err
}

func (i *FSMSnapshot) Release() { i.Unlock() }
