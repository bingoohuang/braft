package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/bingoohuang/braft"
	"github.com/bingoohuang/braft/fsm"
	"github.com/bingoohuang/braft/util"
	"github.com/bingoohuang/gg/pkg/flagparse"
	"github.com/bingoohuang/gg/pkg/v"
	"github.com/bingoohuang/golog"
	"github.com/bingoohuang/golog/pkg/ginlogrus"
	"github.com/bingoohuang/golog/pkg/logfmt"
	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
)

type Arg struct {
	Version bool `flag:",v"`
	Init    bool
}

// Usage is optional for customized show.
func (a Arg) Usage() string {
	return fmt.Sprintf(`
Usage of goup:
  -v    bool   show version
  -init bool   create init ctl shell script`)
}

// VersionInfo is optional for customized version.
func (a Arg) VersionInfo() string { return v.Version() }

func main() {
	c := &Arg{}
	flagparse.Parse(c)

	logfmt.RegisterLevelKey("[DEBUG]", logrus.DebugLevel)
	golog.Setup()

	kvService := fsm.NewMemMapService()
	node, err := braft.NewNode(braft.WithServices(kvService))
	if err != nil {
		log.Fatalf("failed to new node, error: %v", err)
	}
	stoppedCh, err := node.Start()
	if err != nil {
		log.Fatalf("failed to start node, error: %v", err)
	}
	defer node.Stop()

	go startHTTP(node, braft.EnvHport, kvService)

	// wait for interruption/termination
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)

	<-sigs
	<-stoppedCh
}

func startHTTP(node *braft.Node, httpPort int, fsmService *fsm.MemMapService) {
	serveRaft := func(ctx *gin.Context) {
		var nodes []RaftNode
		for _, server := range node.Raft.GetConfiguration().Configuration().Servers {
			if rsp, err := braft.GetPeerDetails(string(server.Address)); err != nil {
				log.Printf("GetPeerDetails error: %v", err)
				nodes = append(nodes, RaftNode{Address: string(server.Address), Error: err.Error()})
			} else {
				nodes = append(nodes, RaftNode{
					Address: string(server.Address), Leader: rsp.Leader,
					ServerID: rsp.ServerId, RaftState: rsp.RaftState,
					RaftPort: rsp.RaftPort, HttpPort: rsp.HttpPort, DiscoveryPort: rsp.DiscoveryPort,
				})
			}
		}
		ctx.JSON(http.StatusOK, map[string]interface{}{
			"Nodes":     nodes,
			"Discovery": node.DiscoveryName(),
		})
	}
	serveKV := func(ctx *gin.Context) {
		req := fsm.MapRequest{
			MapName: util.Or(getQuery(ctx, "map", "m"), "default"),
			Key:     util.Or(getQuery(ctx, "key", "k"), "default"),
		}
		switch ctx.Request.Method {
		case http.MethodPost:
			req.Operate = fsm.OperateSet
			req.Value = getQuery(ctx, "value", "v")
		case http.MethodGet:
			req.Operate = fsm.OperateGet
			ctx.JSON(http.StatusOK, fsmService.Exec(req))
			return
		case http.MethodDelete:
			req.Operate = fsm.OperateDel
		}

		result, err := node.RaftApply(req, time.Second)
		if err != nil {
			ctx.JSON(http.StatusInternalServerError, err.Error())
		} else {
			ctx.JSON(http.StatusOK, result)
		}
	}

	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.Use(ginlogrus.Logger(nil, true), gin.Recovery())
	r.GET("/raft", serveRaft)
	r.GET("/kv", serveKV)
	r.POST("/kv", serveKV)
	r.DELETE("/kv", serveKV)

	if err := r.Run(fmt.Sprintf(":%d", httpPort)); err != nil {
		log.Fatalf("failed to run %d, error: %v", httpPort, err)
	}
}

type RaftNode struct {
	Error         string `json:",omitempty"`
	Leader        string
	ServerID      string
	Address       string
	RaftState     string
	RaftPort      int32
	DiscoveryPort int32
	HttpPort      int32
}

func getQuery(ctx *gin.Context, k ...string) string {
	for _, _k := range k {
		if q, _ := ctx.GetQuery(_k); q != "" {
			return q
		}
	}

	return ""
}
