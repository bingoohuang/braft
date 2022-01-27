package fsm

import (
	"errors"
	"io"
	"io/ioutil"
	"log"

	"github.com/bingoohuang/braft/serializer"
	"github.com/hashicorp/raft"
)

type RoutingFSM struct {
	services     map[string]Service
	ser          serializer.Serializer
	reqDataTypes []ReqTypeInfo
}

func NewRoutingFSM(services []Service) FSM {
	servicesMap := map[string]Service{}
	for _, service := range services {
		servicesMap[service.Name()] = service
	}
	return &RoutingFSM{
		services: servicesMap,
	}
}

func (i *RoutingFSM) Init(ser serializer.Serializer) {
	i.ser = ser
	for _, service := range i.services {
		i.reqDataTypes = append(i.reqDataTypes, MakeReqTypeInfo(service))
	}
}

func (i *RoutingFSM) Apply(raftlog *raft.Log) interface{} {
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

func (i *RoutingFSM) Snapshot() (raft.FSMSnapshot, error) {
	return NewBaseFSMSnapshot(i), nil
}

func (i *RoutingFSM) Restore(closer io.ReadCloser) error {
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
