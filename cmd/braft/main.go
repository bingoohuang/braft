package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"reflect"
	"strconv"
	"time"

	"github.com/bingoohuang/braft"
	"github.com/bingoohuang/braft/fsm"
	"github.com/bingoohuang/braft/marshal"
	"github.com/bingoohuang/braft/ticker"
	"github.com/bingoohuang/gg/pkg/codec"
	"github.com/bingoohuang/gg/pkg/flagparse"
	"github.com/bingoohuang/gg/pkg/randx"
	"github.com/bingoohuang/gg/pkg/sigx"
	"github.com/bingoohuang/gg/pkg/v"
	"github.com/bingoohuang/golog"
	"github.com/gin-gonic/gin"
	"github.com/hashicorp/raft"
	"github.com/segmentio/ksuid"
	"github.com/thoas/go-funk"
)

func main() {
	dh := &DemoPicker{}
	braft.DefaultMdnsService = "_braft._tcp,_demo"
	t := ticker.New(10 * time.Second)

	node, err := braft.NewNode(
		braft.WithServices(fsm.NewMemKvService(), fsm.NewDistributeService(dh)),
		braft.WithLeaderChange(func(n *braft.Node, s raft.RaftState) {
			log.Printf("nodeState: %s", s)
			if s == raft.Leader {
				t.Start(func() {
					log.Printf("ticker ticker, I'm %s, nodeIds: %v", n.Raft.State(), n.ShortNodeIds())
				})
			} else {
				t.Stop()
			}
		}),
		braft.WithHTTPFns(
			braft.WithHandler(http.MethodPost, "/distribute", dh.distributePost),
			braft.WithHandler(http.MethodGet, "/distribute", dh.distributeGet),
		))
	if err != nil {
		log.Fatalf("failed to new node, error: %v", err)
	}
	if err := node.Start(); err != nil {
		log.Fatalf("failed to start node, error: %v", err)
	}
}

type DemoItem struct {
	ID     string
	NodeID string
}

var _ fsm.DistributableItem = (*DemoItem)(nil)

func (d *DemoItem) GetItemID() string       { return d.ID }
func (d *DemoItem) SetNodeID(nodeID string) { d.NodeID = nodeID }

type DemoDist struct {
	Common string
	Items  []DemoItem
}

func (d *DemoDist) GetDistributableItems() any { return d.Items }

type DemoPicker struct{ DD *DemoDist }

func (d *DemoPicker) PickForNode(nodeID string, request any) {
	dd := request.(*DemoDist)
	dd.Items = funk.Filter(dd.Items, func(item DemoItem) bool {
		return item.NodeID == nodeID
	}).([]DemoItem)
	d.DD = dd
	log.Printf("got %d items: %s", len(dd.Items), codec.Json(dd))
}

func (d *DemoPicker) RegisterMarshalTypes(reg *marshal.TypeRegister) {
	reg.RegisterType(reflect.TypeOf(DemoDist{}))
}

func (d *DemoPicker) distributeGet(ctx *gin.Context, _ *braft.Node) {
	ctx.JSON(http.StatusOK, d.DD)
}

func (d *DemoPicker) distributePost(ctx *gin.Context, n *braft.Node) {
	dd := &DemoDist{Items: makeRandItems(ctx.Query("n")), Common: ksuid.New().String()}
	if result, err := n.Distribute(dd); err != nil {
		ctx.JSON(http.StatusInternalServerError, err.Error())
	} else {
		ctx.JSON(http.StatusOK, result)
	}
}

func makeRandItems(q string) (ret []DemoItem) {
	n, _ := strconv.Atoi(q)
	if n <= 0 {
		n = randx.IntN(20) + 1
	}

	for i := 0; i < n; i++ {
		ret = append(ret, DemoItem{ID: fmt.Sprintf("%d", i)})
	}

	return
}

func init() {
	flagparse.Parse(&arg)

	golog.Setup()

	// 注册性能采集信号，用法:
	// 第一步，通知开始采集：touch jj.cpu; kill -USR1 `pidof dsvs2`;
	// 第二部，压力测试开始（或者其他手工测试，等待程序运行一段时间，比如5分钟）
	// 第三步，通知结束采集，生成 cpu.profile 文件，命令与第一步相同
	// 第四步，下载 cpu.profile 文件，`go tool pprof -http :9402 cpu.profile` 开启浏览器查看
	sigx.RegisterSignalProfile()
}

var arg Arg

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

// VersionInfo is optional for a customized version.
func (a Arg) VersionInfo() string { return v.Version() }
