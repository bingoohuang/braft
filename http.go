package braft

import (
	"encoding/base64"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/bingoohuang/braft/fsm"
	"github.com/bingoohuang/braft/util"
	"github.com/bingoohuang/gg/pkg/v"
	"github.com/bingoohuang/golog/pkg/ginlogrus"
	"github.com/gin-gonic/gin"
	"github.com/vmihailenco/msgpack/v5"
)

// HTTPConfig is configuration for HTTP service.
type HTTPConfig struct {
	EnableKv bool
	Handlers map[string]pathHalder
}

// HandlerFunc defines the handler used by gin middleware as return value.
type HandlerFunc func(ctx *gin.Context, n *Node)

type pathHalder struct {
	path    string
	handler HandlerFunc
}

// HTTPConfigFn is function options for HTTPConfig.
type HTTPConfigFn func(*HTTPConfig)

// WithEnableKV enables or disables KV service on HTTP.
func WithEnableKV(b bool) HTTPConfigFn { return func(c *HTTPConfig) { c.EnableKv = b } }

// WithHandler defines the http handler.
func WithHandler(method, path string, handler HandlerFunc) HTTPConfigFn {
	return func(c *HTTPConfig) { c.Handlers[method] = pathHalder{path: path, handler: handler} }
}

// RunHTTP run http service on block.
func (n *Node) RunHTTP(fs ...HTTPConfigFn) {
	c := &HTTPConfig{EnableKv: true, Handlers: map[string]pathHalder{}}
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

	for method, handler := range c.Handlers {
		r.Handle(method, handler.path, func(ctx *gin.Context) {
			handler.handler(ctx, n)
		})
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
	Error          string `json:",omitempty"`
	Leader         string
	ServerID       string
	Address        string
	RaftState      string
	RaftID         RaftID
	DiscoveryNodes []string
	StartTime      string
	Duration       string
	GitCommit      string
	BuildTime      string
	GoVersion      string
	AppVersion     string
}

// ServeRaft services the the raft http api.
func (n *Node) ServeRaft(ctx *gin.Context) {
	var nodes []RaftNode
	for _, server := range n.Raft.GetConfiguration().Configuration().Servers {
		if rsp, err := GetPeerDetails(string(server.Address)); err != nil {
			log.Printf("GetPeerDetails error: %v", err)
			nodes = append(nodes, RaftNode{Address: string(server.Address), Error: err.Error()})
		} else {
			var rid RaftID
			data, _ := base64.RawURLEncoding.DecodeString(rsp.ServerId)
			msgpack.Unmarshal(data, &rid)
			nodes = append(nodes, RaftNode{
				RaftID:  rid,
				Address: string(server.Address), Leader: rsp.Leader,
				ServerID: rsp.ServerId, RaftState: rsp.RaftState,
				Error:          rsp.Error,
				DiscoveryNodes: rsp.DiscoveryNodes,
				StartTime:      rsp.StartTime,
				Duration:       rsp.Duration,
				GitCommit:      v.GitCommit,
				BuildTime:      v.BuildTime,
				GoVersion:      v.GoVersion,
				AppVersion:     v.AppVersion,
			})
		}
	}

	ctx.JSON(http.StatusOK, map[string]interface{}{
		"Leader":        n.Raft.Leader(),
		"Nodes":         nodes,
		"NodeNum":       len(nodes),
		"Discovery":     n.DiscoveryName(),
		"CurrentLeader": n.IsLeader(),
	})
}

// ServeKV services the kv set/get http api.
func (n *Node) ServeKV(ctx *gin.Context) {
	req := fsm.KvRequest{
		MapName: util.Or(getQuery(ctx, "map", "m"), "default"),
		Key:     util.Or(getQuery(ctx, "key", "k"), "default"),
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
