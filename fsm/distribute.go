package fsm

import (
	"log"
	"reflect"

	"github.com/bingoohuang/braft/marshal"
)

// Distributable enable the distribution of struct.
type Distributable interface {
	GetDistributableItems() interface{}
}

// DistributableItem gives the ID getter.
type DistributableItem interface {
	GetItemID() string
	SetNodeID(string)
}

// Distributor is the role to charge the distribution among the raft cluster nodes.
type Distributor struct {
	// StickyMap sticky item to the nodeID.
	StickyMap map[string]string
}

// NewDistributor makes a new Distributor.
func NewDistributor() *Distributor {
	d := &Distributor{
		StickyMap: map[string]string{},
	}

	return d
}

// Distribute do the distribution.
func (d *Distributor) Distribute(shortNodeIds []string, data interface{}) int {
	d.cleanKeysNotIn(shortNodeIds)

	dv := reflect.ValueOf(data)
	dataLen := dv.Len()
	// 预先计算每个节点可以安放的数量
	nodeNumMap := makeNodeNumMap(shortNodeIds, dataLen)

	var newItems []int
	// 先保持粘滞
	for i := 0; i < dataLen; i++ {
		di, ok := GetDistributable(dv.Index(i))
		if !ok {
			continue
		}

		if shortNodeID := d.StickyMap[di.GetItemID()]; shortNodeID != "" {
			if nodeNumMap[shortNodeID] > 0 {
				nodeNumMap[shortNodeID]--
				di.SetNodeID(shortNodeID)
				continue
			}

			delete(d.StickyMap, di.GetItemID())
		}

		newItems = append(newItems, i)
	}

	// 再分配剩余
	for _, i := range newItems {
		di, ok := GetDistributable(dv.Index(i))
		if !ok {
			continue
		}

		for _, shortNodeID := range shortNodeIds {
			if nodeNumMap[shortNodeID] <= 0 {
				continue
			}

			d.StickTo(di.GetItemID(), shortNodeID)
			nodeNumMap[shortNodeID]--
			di.SetNodeID(shortNodeID)
			break
		}
	}

	return dataLen
}

// StickTo stick a item to a node.
func (d *Distributor) StickTo(itemID, shortNodeID string) {
	d.StickyMap[itemID] = shortNodeID
}

func GetDistributable(dataItem reflect.Value) (DistributableItem, bool) {
	if dataItem.Kind() == reflect.Ptr {
		di := dataItem.Interface()
		v, ok := di.(DistributableItem)
		return v, ok
	}

	di := dataItem.Addr().Interface()
	v, ok := di.(DistributableItem)
	return v, ok
}

func makeNodeNumMap(nodeIds []string, dataLen int) map[string]int {
	numMap := map[string]int{}
	for i := 0; i < dataLen; i++ {
		p := nodeIds[i%len(nodeIds)]
		numMap[p]++
	}

	return numMap
}

// CleanSticky cleans the sticky map state.
func (d *Distributor) CleanSticky() {
	d.StickyMap = map[string]string{}
}

func (d *Distributor) cleanKeysNotIn(nodeIds []string) {
	m := map[string]bool{}
	for _, nodeID := range nodeIds {
		m[nodeID] = true
	}

	for itemID, nodeID := range d.StickyMap {
		if _, ok := m[nodeID]; !ok {
			delete(d.StickyMap, itemID)
		}
	}
}

type DistributeService struct {
	picker NodePicker
}

var _ Service = (*DistributeService)(nil)

func NewDistributeService(picker NodePicker) *DistributeService {
	return &DistributeService{picker: picker}
}

// RegisterMarshalTypes registers the types for marshaling and unmarshaling.
func (m *DistributeService) RegisterMarshalTypes(typeRegister *marshal.TypeRegister) {
	typeRegister.RegisterType(reflect.TypeOf(DistributeRequest{}))
}

func (m *DistributeService) ApplySnapshot(nodeID string, input interface{}) error {
	log.Printf("DistributeService ApplySnapshot req: %+v", input)
	return nil
}

func (m *DistributeService) NewLog(nodeID string, request interface{}) interface{} {
	log.Printf("DistributeService NewLog req: %+v", request)

	req := request.(DistributeRequest)
	m.picker(nodeID, req.Payload)

	return nil
}

func (m *DistributeService) GetReqDataType() interface{} { return DistributeRequest{} }

type DistributeRequest struct {
	Payload interface{}
}

var (
	_ marshal.TypeRegisterMarshaler   = (*DistributeRequest)(nil)
	_ marshal.TypeRegisterUnmarshaler = (*DistributeRequest)(nil)
)

func (s DistributeRequest) MarshalMsgpack(t *marshal.TypeRegister) ([]byte, error) {
	return t.Marshal(s.Payload)
}

func (s *DistributeRequest) UnmarshalMsgpack(t *marshal.TypeRegister, d []byte) (err error) {
	s.Payload, err = t.Unmarshal(d)
	return
}

type NodePicker func(node string, request interface{})
