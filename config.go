package braft

import (
	"os"

	"github.com/bingoohuang/braft/discovery"
	"github.com/bingoohuang/braft/fsm"
	"github.com/bingoohuang/braft/marshal"
	"github.com/bingoohuang/braft/util"
)

// ConfigFn is the function option pattern for the NodeConfig.
type ConfigFn func(*Config)

// NodeState 节点状态
type NodeState int

const (
	// NodeFollower 表示节点为 Follower 状态
	NodeFollower NodeState = iota
	// NodeLeader 表示节点为 Leader 状态
	NodeLeader
	// NodeShuttingDown 表示节点处于 ShuttingDown 状态
	NodeShuttingDown
)

func (n NodeState) String() string {
	switch n {
	case NodeFollower:
		return "NodeFollower"
	case NodeLeader:
		return "NodeLeader"
	case NodeShuttingDown:
		return "NodeShuttingDown"
	default:
		return "Unknown"
	}
}

// NodeStateChanger defines the leader change callback func prototype.
type NodeStateChanger func(n *Node, nodeState NodeState)

// WithLeaderChange specifies the leader change callback.
func WithLeaderChange(s NodeStateChanger) ConfigFn { return func(c *Config) { c.LeaderChange = s } }

// WithBizData specifies the biz data of current node for the node for /raft api .
func WithBizData(s func() interface{}) ConfigFn { return func(c *Config) { c.BizData = s } }

// WithServices specifies the services for the FSM.
func WithServices(s ...fsm.Service) ConfigFn { return func(c *Config) { c.Services = s } }

// WithDiscovery specifies the discovery method of raft cluster nodes.
func WithDiscovery(s discovery.Discovery) ConfigFn { return func(c *Config) { c.Discovery = s } }

// WithDataDir specifies the data directory.
func WithDataDir(s string) ConfigFn { return func(c *Config) { c.DataDir = s } }

// WithTypeRegister specifies the serializer.TypeRegister of the raft log messages.
func WithTypeRegister(s *marshal.TypeRegister) ConfigFn {
	return func(c *Config) { c.TypeRegister = s }
}

// WithEnableHTTP specifies whether to enable the http service.
func WithEnableHTTP(v bool) ConfigFn { return func(c *Config) { c.EnableHTTP = v } }

// WithHTTPFns specifies the http service.
func WithHTTPFns(s ...HTTPConfigFn) ConfigFn {
	return func(c *Config) {
		c.HTTPConfigFns = append(c.HTTPConfigFns, s...)
		if !c.EnableHTTP && len(c.HTTPConfigFns) > 0 {
			c.EnableHTTP = true
		}
	}
}

func createConfig(fns []ConfigFn) (*Config, error) {
	conf := &Config{}
	for _, f := range fns {
		f(conf)
	}
	if conf.TypeRegister == nil {
		conf.TypeRegister = marshal.NewTypeRegister(marshal.NewMsgPacker())
	}
	if conf.Discovery == nil {
		conf.Discovery = CreateDiscovery(DefaultDiscovery)
	}
	if len(conf.Services) == 0 {
		conf.Services = []fsm.Service{fsm.NewMemKvService()}
	}

	for _, service := range conf.Services {
		service.RegisterMarshalTypes(conf.TypeRegister)
	}

	if conf.DataDir == "" {
		dir, err := os.MkdirTemp("", "braft")
		if err != nil {
			return nil, err
		}
		conf.DataDir = dir
	} else if !util.IsDir(conf.DataDir) {
		// stable/log/snapshot store config
		if err := util.RemoveCreateDir(conf.DataDir); err != nil {
			return nil, err
		}
	}
	return conf, nil
}
