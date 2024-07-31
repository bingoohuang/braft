package braft

import (
	"os"

	"github.com/bingoohuang/braft/discovery"
	"github.com/bingoohuang/braft/fsm"
	"github.com/bingoohuang/braft/marshal"
	"github.com/bingoohuang/braft/util"
	"github.com/hashicorp/raft"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// ConfigFn is the function option pattern for the NodeConfig.
type ConfigFn func(*Config)

// NodeStateChanger defines the leader change callback func prototype.
type NodeStateChanger func(n *Node, nodeState raft.RaftState)

// WithShutdownExitCode specifies the program should exit or not when the raft cluster shutdown.
func WithShutdownExitCode(shutdownExit bool, shutdownExitCode int) ConfigFn {
	return func(c *Config) {
		c.ShutdownExit = shutdownExit
		c.ShutdownExitCode = shutdownExitCode
	}
}

// WithLeaderChange specifies the leader change callback.
func WithLeaderChange(s NodeStateChanger) ConfigFn { return func(c *Config) { c.LeaderChange = s } }

// WithBizData specifies the biz data of current node for the node for /raft api .
func WithBizData(s func() any) ConfigFn { return func(c *Config) { c.BizData = s } }

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

func WithHostIP(v string) ConfigFn {
	return func(c *Config) {
		c.HostIP = v
	}
}

// WithHTTPFns specifies the http service.
func WithHTTPFns(s ...HTTPConfigFn) ConfigFn {
	return func(c *Config) {
		c.HTTPConfigFns = append(c.HTTPConfigFns, s...)
		if !c.EnableHTTP && len(c.HTTPConfigFns) > 0 {
			c.EnableHTTP = true
		}
	}
}

// WithGrpcDialOptions specifies the grpc options.
func WithGrpcDialOptions(options ...grpc.DialOption) ConfigFn {
	return func(c *Config) {
		c.GrpcDialOptions = options
	}
}

// WithRaftPort specifies the raft port.
func WithRaftPort(port int) ConfigFn {
	return func(c *Config) {
		c.Rport = port
	}
}

// WithDiscoveryPort specifies the discovery port.
func WithDiscoveryPort(port int) ConfigFn {
	return func(c *Config) {
		c.Dport = port
	}
}

// WithHttpPort specifies the http port.
func WithHttpPort(port int) ConfigFn {
	return func(c *Config) {
		c.Hport = port
	}
}

// WithServerID specifies the RaftID.
func WithServerID(serverID string) ConfigFn {
	return func(c *Config) {
		c.ServerID = serverID
	}
}

func createConfig(fns []ConfigFn) (*Config, error) {
	conf := &Config{
		GrpcDialOptions: []grpc.DialOption{
			grpc.WithTransportCredentials(insecure.NewCredentials()),
		},
		HostIP: EnvIP,
		Rport:  EnvRport,
		Dport:  EnvDport,
		Hport:  EnvHport,
	}
	for _, f := range fns {
		f(conf)
	}
	if conf.TypeRegister == nil {
		conf.TypeRegister = marshal.NewTypeRegister(marshal.NewMsgPacker())
	}
	if conf.Discovery == nil {
		conf.Discovery = CreateDiscovery(DefaultDiscovery, conf.Dport)
	}
	if len(conf.Services) == 0 {
		conf.Services = []fsm.Service{fsm.NewMemKvService()}
	}
	if conf.LeaderChange == nil {
		conf.LeaderChange = func(*Node, raft.RaftState) {}
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
