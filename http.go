package braft

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/hashicorp/raft"

	"github.com/bingoohuang/gg/pkg/ss"

	"github.com/bingoohuang/gg/pkg/fn"

	"github.com/bingoohuang/braft/fsm"
	"github.com/bingoohuang/gg/pkg/v"
	"github.com/bingoohuang/golog/pkg/ginlogrus"
	"github.com/gin-gonic/gin"
)

// HTTPConfig is configuration for HTTP service.
type HTTPConfig struct {
	EnableKv bool
	Handlers []pathHalder
}

// HandlerFunc defines the handler used by gin middleware as return value.
type HandlerFunc func(ctx *gin.Context, n *Node)

type pathHalder struct {
	method  string
	path    string
	handler HandlerFunc
}

// HTTPConfigFn is function options for HTTPConfig.
type HTTPConfigFn func(*HTTPConfig)

// WithEnableKV enables or disables KV service on HTTP.
func WithEnableKV(b bool) HTTPConfigFn { return func(c *HTTPConfig) { c.EnableKv = b } }

// WithHandler defines the http handler.
func WithHandler(method, path string, handler HandlerFunc) HTTPConfigFn {
	return func(c *HTTPConfig) {
		c.Handlers = append(c.Handlers, pathHalder{method: method, path: path, handler: handler})
	}
}

// runHTTP run http service on block.
func (n *Node) runHTTP(fs ...HTTPConfigFn) {
	c := &HTTPConfig{EnableKv: true}
	for _, f := range fs {
		f(c)
	}

	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.Use(ginlogrus.Logger(nil, true), gin.Recovery())
	r.GET("/raft", n.ServeRaft)

	if c.EnableKv {
		n.RegisterServeKV(r, "/kv")
	}

	for _, h := range c.Handlers {
		hh := h.handler
		log.Printf("register method: %s path: %s handler: %s", h.method, h.path, fn.GetFuncName(hh))
		r.Handle(h.method, h.path, func(ctx *gin.Context) {
			hh(ctx, n)
		})
	}

	n.httpServer = &http.Server{
		Addr:    fmt.Sprintf(":%d", EnvHport),
		Handler: r,
	}

	go func() {
		if err := n.httpServer.ListenAndServe(); err != nil {
			log.Printf("E! listen: %s\n", err)
		}
	}()
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
	Error          string `json:",omitempty"`
	Leader         string
	ServerID       string
	Address        string
	RaftState      string
	RaftID         RaftID
	DiscoveryNodes []string
	StartTime      string
	Duration       string

	Rss        uint64
	RaftLogSum uint64
	Pid        uint64

	GitCommit  string
	BuildTime  string
	GoVersion  string
	AppVersion string
	Pcpu       float32

	BizData json.RawMessage

	Addr []string
}

// ServeRaft services the raft http api.
func (n *Node) ServeRaft(ctx *gin.Context) {
	raftServers := n.GetRaftServers()
	nodes := n.GetRaftNodes(raftServers)
	leaderAddr, leaderID := n.Raft.LeaderWithID()
	ctx.JSON(http.StatusOK, gin.H{
		"Leader":        leaderAddr,
		"LeaderID":      leaderID,
		"Nodes":         nodes,
		"NodeNum":       len(nodes),
		"Discovery":     n.DiscoveryName(),
		"CurrentLeader": n.IsLeader(),
		"RaftServers":   raftServers,
	})
}

// GetRaftNodesInfo return the raft nodes information.
func (n *Node) GetRaftNodesInfo() (nodes []RaftNode) {
	raftServers := n.GetRaftServers()
	return n.GetRaftNodes(raftServers)
}

// GetRaftNodes return the raft nodes information.
func (n *Node) GetRaftNodes(raftServers []raft.Server) (nodes []RaftNode) {
	for _, server := range raftServers {
		rsp, err := GetPeerDetails(string(server.Address), 3*time.Second)
		if err != nil {
			log.Printf("E! GetPeerDetails error: %v", err)
			nodes = append(nodes, RaftNode{Address: string(server.Address), Error: err.Error()})
			continue
		}

		rid := ParseRaftID(rsp.ServerId)
		nodes = append(nodes, RaftNode{
			RaftID:  rid,
			Address: string(server.Address), Leader: rsp.Leader,
			ServerID: rsp.ServerId, RaftState: rsp.RaftState,
			Error:          rsp.Error,
			DiscoveryNodes: rsp.DiscoveryNodes,
			StartTime:      rsp.StartTime,
			Duration:       rsp.Duration,

			Rss:        rsp.Rss,
			Pcpu:       rsp.Pcpu,
			RaftLogSum: rsp.RaftLogSum,
			Pid:        rsp.Pid,

			GitCommit:  v.GitCommit,
			BuildTime:  v.BuildTime,
			GoVersion:  v.GoVersion,
			AppVersion: v.AppVersion,

			BizData: func() json.RawMessage {
				if rsp.BizData == "" {
					return nil
				}
				return json.RawMessage(rsp.BizData)
			}(),

			Addr: rsp.Addr,
		})
	}
	return
}

func (n *Node) GetRaftServers() []raft.Server {
	return n.Raft.GetConfiguration().Configuration().Servers
}

// RegisterServeKV register kv service for the gin route.
func (n *Node) RegisterServeKV(r gin.IRoutes, path string) {
	r.GET(path, n.ServeKV)
	r.POST(path, n.ServeKV)
	r.DELETE(path, n.ServeKV)
}

// ServeKV services the kv set/get http api.
func (n *Node) ServeKV(ctx *gin.Context) {
	req := fsm.KvRequest{
		MapName: ss.Or(getQuery(ctx, "map", "m"), "default"),
		Key:     ss.Or(getQuery(ctx, "key", "k"), "default"),
	}
	ctx.Header("Braft-IP", strings.Join(n.RaftID.IP, ","))
	ctx.Header("Braft-ID", n.RaftID.ID)
	ctx.Header("Braft-Host", n.RaftID.Hostname)
	switch ctx.Request.Method {
	case http.MethodPost:
		req.KvOperate = fsm.KvSet
		req.Value = getQuery(ctx, "value", "v")
	case http.MethodGet:
		req.KvOperate = fsm.KvGet
		for _, service := range n.Conf.Services {
			if m, ok := service.(fsm.KvExectable); ok {
				ctx.JSON(http.StatusOK, m.Exec(req))
				return
			}
		}
	case http.MethodDelete:
		req.KvOperate = fsm.KvDel
	}

	result, err := n.RaftApply(req, time.Second)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, err.Error())
	} else {
		ctx.JSON(http.StatusOK, result)
	}
}
