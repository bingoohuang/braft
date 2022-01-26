package fsm

import (
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"strings"

	"github.com/bingoohuang/easyraft/serializer"
	"github.com/hashicorp/raft"
)

type RoutingFSM struct {
	services            map[string]FSMService
	ser                 serializer.Serializer
	reqDataTypes        []interface{}
	reqServiceDataTypes map[string]FSMService
}

func NewRoutingFSM(services []FSMService) FSM {
	servicesMap := map[string]FSMService{}
	for _, service := range services {
		servicesMap[service.Name()] = service
	}
	return &RoutingFSM{
		services:            servicesMap,
		reqServiceDataTypes: map[string]FSMService{},
	}
}

func (i *RoutingFSM) Init(ser serializer.Serializer) {
	i.ser = ser
	for _, service := range i.services {
		i.reqDataTypes = append(i.reqDataTypes, service.GetReqDataTypes()...)
		for _, dt := range service.GetReqDataTypes() {
			i.reqServiceDataTypes[fmt.Sprintf("%#v", dt)] = service
		}
	}
}

func (i *RoutingFSM) Apply(log *raft.Log) interface{} {
	switch log.Type {
	case raft.LogCommand:
		// deserialize
		payload, err := i.ser.Deserialize(log.Data)
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
		if foundType, err := getTargetType(i.reqDataTypes, fields); err == nil {
			for typeName, service := range i.reqServiceDataTypes {
				if strings.EqualFold(fmt.Sprintf("%#v", foundType), typeName) {
					return service.NewLog(foundType, payloadMap)
				}
			}
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
