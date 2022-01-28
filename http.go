package braft

import (
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/bingoohuang/braft/fsm"
	"github.com/bingoohuang/braft/util"
	"github.com/bingoohuang/golog/pkg/ginlogrus"
	"github.com/gin-gonic/gin"
)

// HTTPConfig is configuration for HTTP service.
type HTTPConfig struct {
	EnableKv bool
}

// HTTPConfigFn is function options for HTTPConfig.
type HTTPConfigFn func(*HTTPConfig)

// WithEnableKV enables or disables KV service on HTTP.
func WithEnableKV(b bool) HTTPConfigFn { return func(c *HTTPConfig) { c.EnableKv = b } }

// RunHTTP run http service on block.
func (n *Node) RunHTTP(fs ...HTTPConfigFn) {
	c := &HTTPConfig{EnableKv: true}
	for _, f := range fs {
		f(c)
	}

	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.Use(ginlogrus.Logger(nil, true), gin.Recovery())
	r.GET("/raft", n.ServeRaft)

	if c.EnableKv {
		r.GET("/kv", n.ServeKV)
		r.POST("/kv", n.ServeKV)
		r.DELETE("/kv", n.ServeKV)
	}

	if err := r.Run(fmt.Sprintf(":%d", EnvHport)); err != nil {
		log.Fatalf("failed to run %d, error: %v", EnvHport, err)
	}
}

func getQuery(ctx *gin.Context, k ...string) string {
	for _, _k := range k {
		if q, _ := ctx.GetQuery(_k); q != "" {
			return q
		}
	}

	return ""
}

// RaftNode is a node info of raft cluster.
type RaftNode struct {
	Error         string `json:",omitempty"`
	Leader        string
	ServerID      string
	Address       string
	RaftState     string
	RaftPort      int32
	DiscoveryPort int32
	HTTPPort      int32
}

// ServeRaft services the the raft http api.
func (n *Node) ServeRaft(ctx *gin.Context) {
	var nodes []RaftNode
	for _, server := range n.Raft.GetConfiguration().Configuration().Servers {
		if rsp, err := GetPeerDetails(string(server.Address)); err != nil {
			log.Printf("GetPeerDetails error: %v", err)
			nodes = append(nodes, RaftNode{Address: string(server.Address), Error: err.Error()})
		} else {
			nodes = append(nodes, RaftNode{
				Address: string(server.Address), Leader: rsp.Leader,
				ServerID: rsp.ServerId, RaftState: rsp.RaftState,
				RaftPort: rsp.RaftPort, HTTPPort: rsp.HttpPort, DiscoveryPort: rsp.DiscoveryPort,
			})
		}
	}
	ctx.JSON(http.StatusOK, map[string]interface{}{
		"Leader":        n.Raft.Leader(),
		"Nodes":         nodes,
		"Discovery":     n.DiscoveryName(),
		"CurrentLeader": n.IsLeader(),
	})
}

// ServeKV services the kv set/get http api.
func (n *Node) ServeKV(ctx *gin.Context) {
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
		for _, service := range n.conf.Services {
			if m, ok := service.(interface {
				Exec(req fsm.MapRequest) interface{}
			}); ok {
				ctx.JSON(http.StatusOK, m.Exec(req))
				return
			}
		}
	case http.MethodDelete:
		req.Operate = fsm.OperateDel
	}

	result, err := n.RaftApply(req, time.Second)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, err.Error())
	} else {
		ctx.JSON(http.StatusOK, result)
	}
}
