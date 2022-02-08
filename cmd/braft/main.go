package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"reflect"
	"strconv"

	"github.com/bingoohuang/braft"
	"github.com/bingoohuang/braft/fsm"
	"github.com/bingoohuang/gg/pkg/codec"
	"github.com/bingoohuang/gg/pkg/flagparse"
	"github.com/bingoohuang/gg/pkg/randx"
	"github.com/bingoohuang/gg/pkg/v"
	"github.com/bingoohuang/golog"
	"github.com/gin-gonic/gin"
	"github.com/segmentio/ksuid"
	"github.com/thoas/go-funk"
)

type Arg struct {
	Version bool `flag:",v"`
	Init    bool
}

// Usage is optional for customized show.
func (a Arg) Usage() string {
	return fmt.Sprintf(`
Usage of %s:
  -v    bool   show version
  -init bool   create init ctl shell script`, os.Args[0])
}

// VersionInfo is optional for customized version.
func (a Arg) VersionInfo() string { return v.Version() }

func main() {
	c := &Arg{}
	flagparse.Parse(c)

	golog.Setup()

	dh := &DemoHandler{}
	node, err := braft.NewNode(braft.WithServices(
		fsm.NewMemKvService(),
		fsm.NewDistributeService(dh.accept)))
	if err != nil {
		log.Fatalf("failed to new node, error: %v", err)
	}
	node.Conf.TypeRegister.RegisterType(reflect.TypeOf(DemoDistribution{}))
	if err := node.Start(); err != nil {
		log.Fatalf("failed to start node, error: %v", err)
	}

	go func() {
		for becameLeader := range node.Raft.LeaderCh() {
			log.Printf("becameLeader: %v", becameLeader)
		}
	}()
	node.RunHTTP(
		braft.WithHandler(http.MethodPost, "/distribute", dh.distributePost),
		braft.WithHandler(http.MethodGet, "/distribute", dh.distributeGet),
	)
}

type DemoDistributionItem struct {
	ID     string
	NodeID string
}

var _ fsm.DistributableItem = (*DemoDistributionItem)(nil)

func (d *DemoDistributionItem) GetItemID() string       { return d.ID }
func (d *DemoDistributionItem) SetNodeID(nodeID string) { d.NodeID = nodeID }

type DemoDistribution struct {
	Items  []DemoDistributionItem
	Common string
}

func (d *DemoDistribution) GetDistributableItems() interface{} { return d.Items }

type DemoHandler struct {
	DD *DemoDistribution
}

func (d *DemoHandler) accept(nodeID string, request interface{}) {
	dd := request.(*DemoDistribution)
	dd.Items = funk.Filter(dd.Items, func(item DemoDistributionItem) bool {
		return item.NodeID == nodeID
	}).([]DemoDistributionItem)
	d.DD = dd
	log.Printf("got %d items: %s", len(dd.Items), codec.Json(dd))
}

func (d *DemoHandler) distributeGet(ctx *gin.Context, n *braft.Node) {
	ctx.JSON(http.StatusOK, d.DD)
}

func (d *DemoHandler) distributePost(ctx *gin.Context, n *braft.Node) {
	dd := &DemoDistribution{Items: makeRandItems(ctx.Query("n")), Common: ksuid.New().String()}
	if result, err := n.Distribute(dd); err != nil {
		ctx.JSON(http.StatusInternalServerError, err.Error())
	} else {
		ctx.JSON(http.StatusOK, result)
	}
}

func makeRandItems(q string) (ret []DemoDistributionItem) {
	n, _ := strconv.Atoi(q)
	if n <= 0 {
		n = randx.IntN(20)
	}

	for i := 0; i < n; i++ {
		ret = append(ret, DemoDistributionItem{ID: fmt.Sprintf("%d", i)})
	}

	return
}
