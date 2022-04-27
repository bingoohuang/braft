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
	Picker
}

var _ Service = (*DistributeService)(nil)

func NewDistributeService(picker Picker) *DistributeService {
	return &DistributeService{Picker: picker}
}

// RegisterMarshalTypes registers the types for marshaling and unmarshaling.
func (m *DistributeService) RegisterMarshalTypes(reg *marshal.TypeRegister) {
	reg.RegisterType(reflect.TypeOf(DistributeRequest{}))
	m.Picker.RegisterMarshalTypes(reg)
}

func (m *DistributeService) ApplySnapshot(nodeID string, input interface{}) error {
	log.Printf("DistributeService ApplySnapshot req: %+v", input)
	return nil
}

func (m *DistributeService) NewLog(nodeID string, request interface{}) interface{} {
	log.Printf("DistributeService NewLog req: %+v", request)

	req := request.(DistributeRequest)
	m.PickForNode(nodeID, req.Data)

	return nil
}

func (m *DistributeService) GetReqDataType() interface{} { return DistributeRequest{} }

type DistributeRequest struct {
	Data interface{}
}

var _ marshal.TypeRegisterMarshalerAdapter = (*DistributeRequest)(nil)

func (s DistributeRequest) Marshal(t *marshal.TypeRegister) ([]byte, error) { return t.Marshal(s.Data) }
func (s DistributeRequest) GetSubData() interface{}                         { return s.Data }
func (s *DistributeRequest) Unmarshal(t *marshal.TypeRegister, d []byte) (err error) {
	s.Data, err = t.Unmarshal(d)
	return
}

type Picker interface {
	PickForNode(node string, request interface{})
	MarshalTypesRegister
}

type PickerFn func(node string, request interface{})

func (f PickerFn) PickForNode(node string, request interface{}) { f(node, request) }
