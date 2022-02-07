package fsm

import (
	"sync"
)

type KvOperate string

const (
	KvGet KvOperate = "get"
	KvSet KvOperate = "set"
	KvDel KvOperate = "del"
)

type KvRequest struct {
	KvOperate KvOperate
	MapName   string
	Key       string
	Value     interface{}
}

type MemKvService struct {
	sync.RWMutex
	Maps map[string]*Map
}

func NewMemKvService() *MemKvService {
	return &MemKvService{Maps: map[string]*Map{}}
}

func (m *MemKvService) Name() string { return "mem_kv" }

func (m *MemKvService) ApplySnapshot(input interface{}) error {
	m.Maps = input.(MemKvService).Maps
	return nil
}

func (m *MemKvService) NewLog(req interface{}) interface{} {
	return m.Exec(req.(KvRequest))
}

func (m *MemKvService) Exec(req KvRequest) interface{} {
	m.Lock()
	defer m.Unlock()

	switch req.KvOperate {
	case KvSet:
		m.put(req.MapName, req.Key, req.Value)
	case KvGet:
		return m.get(req.MapName, req.Key)
	case KvDel:
		m.del(req.MapName, req.Key)
	}

	return nil
}

func (m *MemKvService) GetReqDataType() interface{} { return KvRequest{} }

func (m *MemKvService) put(mapName string, key string, value interface{}) {
	fMap, found := m.Maps[mapName]
	if !found {
		fMap = &Map{Data: map[string]interface{}{}}
		m.Maps[mapName] = fMap
	}

	fMap.put(key, value)
}

func (m *MemKvService) get(mapName string, key string) interface{} {
	fMap, found := m.Maps[mapName]
	if !found {
		return nil
	}

	return fMap.get(key)
}

func (m *MemKvService) del(mapName string, key string) {
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
