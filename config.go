package braft

import (
	"io/ioutil"

	"github.com/bingoohuang/braft/discovery"
	"github.com/bingoohuang/braft/fsm"
	"github.com/bingoohuang/braft/marshal"
	"github.com/bingoohuang/braft/util"
)

// ConfigFn is the function option pattern for the NodeConfig.
type ConfigFn func(*Config)

// LeaderChanger defines the leader change callback func prototype.
type LeaderChanger func(becameLeader bool)

// WithLeaderChange specifies the leader change callback.
func WithLeaderChange(s LeaderChanger) ConfigFn { return func(c *Config) { c.LeaderChange = s } }

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
		dir, err := ioutil.TempDir("", "braft")
		if err != nil {
			return nil, err
		}
		conf.DataDir = dir
	} else {
		// stable/log/snapshot store config
		if !util.IsDir(conf.DataDir) {
			if err := util.RemoveCreateDir(conf.DataDir); err != nil {
				return nil, err
			}
		}
	}
	return conf, nil
}
