package fsm

import (
	"sync"

	"github.com/mitchellh/mapstructure"
)

type OperateType string

const (
	OperateGet OperateType = "get"
	OperateSet OperateType = "set"
	OperateDel OperateType = "del"
)

type MapRequest struct {
	Operate OperateType
	MapName string
	Key     string
	Value   interface{}
}

type MemMapService struct {
	sync.RWMutex
	Maps map[string]*Map
}

func NewMemMapService() *MemMapService {
	return &MemMapService{Maps: map[string]*Map{}}
}

func (m *MemMapService) Name() string { return "in_memory_map" }

func (m *MemMapService) ApplySnapshot(input interface{}) error {
	var svc MemMapService
	if err := mapstructure.Decode(input, &svc); err != nil {
		return err
	}
	m.Maps = svc.Maps
	return nil
}

func (m *MemMapService) NewLog(request map[string]interface{}) interface{} {
	var req MapRequest
	if err := mapstructure.Decode(request, &req); err != nil {
		return err
	}
	return m.Exec(req)
}

func (m *MemMapService) Exec(req MapRequest) interface{} {
	m.Lock()
	defer m.Unlock()

	switch req.Operate {
	case OperateSet:
		m.put(req.MapName, req.Key, req.Value)
	case OperateGet:
		return m.get(req.MapName, req.Key)
	case OperateDel:
		m.del(req.MapName, req.Key)
	}

	return nil
}

func (m *MemMapService) GetReqDataType() interface{} { return MapRequest{} }

func (m *MemMapService) put(mapName string, key string, value interface{}) {
	fMap, found := m.Maps[mapName]
	if !found {
		fMap = &Map{Data: map[string]interface{}{}}
		m.Maps[mapName] = fMap
	}

	fMap.put(key, value)
}

func (m *MemMapService) get(mapName string, key string) interface{} {
	fMap, found := m.Maps[mapName]
	if !found {
		return nil
	}

	return fMap.get(key)
}

func (m *MemMapService) del(mapName string, key string) {
	fMap, found := m.Maps[mapName]
	if !found {
		return
	}

	fMap.del(key)
}

type Map struct {
	sync.RWMutex
	Data map[string]interface{}
}

func (m *Map) del(k string) {
	m.Lock()
	defer m.Unlock()

	delete(m.Data, k)
}

func (m *Map) get(k string) interface{} {
	m.RLock()
	defer m.RUnlock()

	return m.Data[k]
}

func (m *Map) put(k string, v interface{}) {
	m.Lock()
	defer m.Unlock()

	m.Data[k] = v
}
