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
	"github.com/gin-gonic/gin"
)

func main() {
	log.Printf("Starting, rport:%d, dport:%d, hport:%d, discovery:%s",
		braft.EnvRport, braft.EnvDport, braft.EnvHport, braft.EnvDiscoveryMethod.Name())
	fsmService := fsm.NewMemMapService()
	node, err := braft.NewNode(braft.WithServices(fsmService))
	if err != nil {
		log.Fatalf("failed to new node, error: %v", err)
	}
	stoppedCh, err := node.Start()
	if err != nil {
		log.Fatalf("failed to start node, error: %v", err)
	}
	defer node.Stop()

	go startHTTP(node, braft.EnvHport, fsmService)

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

	r := gin.Default()
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
