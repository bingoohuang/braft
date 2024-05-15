package braft

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/bingoohuang/braft/fsm"
	"github.com/bingoohuang/braft/util"
	"github.com/bingoohuang/gg/pkg/fn"
	"github.com/bingoohuang/gg/pkg/ss"
	"github.com/bingoohuang/gg/pkg/v"
	"github.com/bingoohuang/golog/pkg/ginlogrus"
	"github.com/gin-gonic/gin"
	"github.com/hashicorp/raft"
	"github.com/samber/lo"
	"github.com/sqids/sqids-go"
)

// HTTPConfig is configuration for HTTP service.
type HTTPConfig struct {
	Handlers []pathHalder
	EnableKv bool
}

// HandlerFunc defines the handler used by gin middleware as return value.
type HandlerFunc func(ctx *gin.Context, n *Node)

type pathHalder struct {
	handler HandlerFunc
	method  string
	path    string
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
	ServerID  string `json:"serverID"`
	BuildTime string `json:"buildTime"`
	Duration  string `json:"duration"`

	Address    string `json:"address"`
	RaftState  string `json:"raftState"`
	Leader     string `json:"leader"`
	AppVersion string `json:"appVersion"`
	StartTime  string `json:"startTime"`
	Error      string `json:"error,omitempty"`
	GoVersion  string `json:"goVersion"`

	GitCommit      string   `json:"gitCommit"`
	DiscoveryNodes []string `json:"discoveryNodes"`

	Addr []string `json:"addr"`

	BizData json.RawMessage `json:"bizData,omitempty"`

	RaftID RaftID `json:"raftID"`

	RaftLogSum uint64 `json:"raftLogSum"`
	Pid        uint64 `json:"pid"`

	Rss  uint64  `json:"rss"`
	Pcpu float32 `json:"pcpu"`

	Rport int `json:"rport"`
	Dport int `json:"dport"`
	Hport int `json:"hport"`
}

// raftServer tracks the information about a single server in a configuration.
type raftServer struct {
	// Suffrage determines whether the server gets a vote.
	Suffrage raft.ServerSuffrage `json:"suffrage"`
	// ID is a unique string identifying this server for all time.
	ID raft.ServerID `json:"id"`
	// Address is its network address that a transport can contact.
	Address raft.ServerAddress `json:"address"`
}

// ServeRaft services the raft http api.
func (n *Node) ServeRaft(ctx *gin.Context) {
	raftServers := n.GetRaftServers()
	nodes := n.GetRaftNodes(raftServers)
	leaderAddr, leaderID := n.Raft.LeaderWithID()
	ctx.JSON(http.StatusOK, gin.H{
		"leaderAddr":    leaderAddr,
		"leaderID":      leaderID,
		"nodes":         nodes,
		"nodeNum":       len(nodes),
		"discovery":     n.DiscoveryName(),
		"currentLeader": n.IsLeader(),
		"raftServers": lo.Map(raftServers, func(r raft.Server, index int) raftServer {
			return raftServer{
				Suffrage: r.Suffrage,
				ID:       r.ID,
				Address:  r.Address,
			}
		}),
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
		ports := util.Pick1(sqids.New()).Decode(rid.Sqid)

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

			BizData: json.RawMessage(rsp.BizData),

			Addr: rsp.Addr,

			Rport: int(ports[0]),
			Dport: int(ports[1]),
			Hport: int(ports[2]),
		})
	}
	return
}

// GetRaftServers 获得 Raft 节点服务器列表.
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
	ctx.Header("Braft-IP", n.RaftID.IP)
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
