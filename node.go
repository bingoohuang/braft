package braft

import (
	"encoding/base64"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net"
	"os"
	"path/filepath"
	"sync/atomic"
	"time"

	transport "github.com/Jille/raft-grpc-transport"
	"github.com/bingoohuang/braft/discovery"
	"github.com/bingoohuang/braft/fsm"
	"github.com/bingoohuang/braft/marshal"
	"github.com/bingoohuang/braft/proto"
	"github.com/bingoohuang/braft/util"
	"github.com/bingoohuang/gg/pkg/codec"
	"github.com/bingoohuang/gg/pkg/goip"
	"github.com/bingoohuang/gg/pkg/ss"
	"github.com/bingoohuang/golog/pkg/logfmt"
	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/memberlist"
	"github.com/hashicorp/raft"
	raftboltdb "github.com/hashicorp/raft-boltdb"
	"github.com/segmentio/ksuid"
	"github.com/sirupsen/logrus"
	"github.com/vmihailenco/msgpack/v5"
	ggrpc "google.golang.org/grpc"
)

// Node is the raft cluster node.
type Node struct {
	ID               string
	RaftID           RaftID
	addr             string
	Raft             *raft.Raft
	GrpcServer       *ggrpc.Server
	TransportManager *transport.Manager
	discoveryConfig  *memberlist.Config
	mList            *memberlist.Memberlist
	stopped          uint32
	Conf             *Config

	StartTime   time.Time
	distributor *fsm.Distributor
}

// Config is the configuration of the node.
type Config struct {
	TypeRegister *marshal.TypeRegister
	DataDir      string
	Discovery    discovery.Discovery
	Services     []fsm.Service
}

// ConfigFn is the function option pattern for the NodeConfig.
type ConfigFn func(*Config)

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

// RaftID is the structure of node ID.
type RaftID struct {
	ID                  string
	Rport, Dport, Hport int
	Hostname            string
	IP                  []string
}

// NewNode returns an BRaft node.
func NewNode(fns ...ConfigFn) (*Node, error) {
	nodeConfig := &Config{}
	for _, f := range fns {
		f(nodeConfig)
	}
	if nodeConfig.TypeRegister == nil {
		nodeConfig.TypeRegister = marshal.NewTypeRegister(marshal.NewMsgPackSerializer())
	}

	if nodeConfig.Discovery == nil {
		nodeConfig.Discovery = EnvDiscovery
	}
	if len(nodeConfig.Services) == 0 {
		nodeConfig.Services = []fsm.Service{fsm.NewMemKvService()}
	}

	for _, service := range nodeConfig.Services {
		service.RegisterMarshalTypes(nodeConfig.TypeRegister)
	}

	if nodeConfig.DataDir == "" {
		dir, err := ioutil.TempDir("", "braft")
		if err != nil {
			return nil, err
		}
		nodeConfig.DataDir = dir
	} else {
		// stable/log/snapshot store config
		if !util.IsDir(nodeConfig.DataDir) {
			if err := util.RemoveCreateDir(nodeConfig.DataDir); err != nil {
				return nil, err
			}
		}
	}
	log.Printf("node data dir: %s", nodeConfig.DataDir)

	h, _ := os.Hostname()
	_, ips := goip.MainIP()
	raftID := RaftID{
		ID:       ksuid.New().String(),
		Rport:    EnvRport,
		Dport:    EnvDport,
		Hport:    EnvHport,
		Hostname: h,
		IP:       ips,
	}

	raftIDMsg, _ := msgpack.Marshal(raftID)
	nodeID := base64.RawURLEncoding.EncodeToString(raftIDMsg)

	log.Printf("nodeID: %s", nodeID)

	raftConf := raft.DefaultConfig()
	raftConf.LocalID = raft.ServerID(nodeID)
	raftConf.LogLevel = hclog.Info.String()
	raftConf.Logger = &logger{}

	stableStoreFile := filepath.Join(nodeConfig.DataDir, "store.boltdb")
	if util.FileExists(stableStoreFile) {
		if err := os.Remove(stableStoreFile); err != nil {
			return nil, err
		}
	}
	// StableStore 稳定存储,存储Raft集群的节点信息
	stableStore, err := raftboltdb.NewBoltStore(stableStoreFile)
	if err != nil {
		return nil, err
	}

	// LogStore 存储Raft的日志
	logStore, err := raft.NewLogCache(512, stableStore)
	if err != nil {
		return nil, err
	}

	// SnapshotStore 快照存储,存储节点的快照信息
	snapshotStore := raft.NewDiscardSnapshotStore()

	// FSM 有限状态机
	sm := fsm.NewRoutingFSM(raftID.ID, nodeConfig.Services, nodeConfig.TypeRegister)

	// memberlist config
	discoveryConfig := memberlist.DefaultLocalConfig()
	discoveryConfig.BindPort = EnvDport
	discoveryConfig.Name = fmt.Sprintf("%s:%d", nodeID, EnvRport)
	discoveryConfig.Logger = log.Default()

	// default raft config
	addr := fmt.Sprintf("%s:%d", EnvIP, EnvRport)
	// grpc transport, Transpot Raft节点之间的通信通道
	t := transport.New(raft.ServerAddress(addr), []ggrpc.DialOption{ggrpc.WithInsecure()})

	// raft server
	raftServer, err := raft.NewRaft(raftConf, sm, logStore, stableStore, snapshotStore, t.Transport())
	if err != nil {
		return nil, err
	}

	return &Node{
		ID:               nodeID,
		RaftID:           raftID,
		addr:             fmt.Sprintf(":%d", EnvRport),
		Raft:             raftServer,
		TransportManager: t,
		Conf:             nodeConfig,
		discoveryConfig:  discoveryConfig,
		distributor:      fsm.NewDistributor(),
	}, nil
}

// Start starts the Node and returns a channel that indicates, that the node has been stopped properly
func (n *Node) Start() error {
	n.StartTime = time.Now()
	log.Printf("Node starting, rport: %d, dport: %d, hport: %d, discovery: %s", EnvRport, EnvDport, EnvHport, EnvDiscovery.Name())

	// 防止各个节点同时启动太快，随机休眠
	util.Think(ss.Or(util.Env("BRAFT_SLEEP", "BSL"), "100ms-3s"))

	// set stopped as false
	atomic.CompareAndSwapUint32(&n.stopped, 1, 0)

	// raft server
	configuration := raft.Configuration{
		Servers: []raft.Server{
			{ID: raft.ServerID(n.ID), Address: n.TransportManager.Transport().LocalAddr()},
		},
	}
	f := n.Raft.BootstrapCluster(configuration)
	if err := f.Error(); err != nil {
		return err
	}

	// memberlist discovery
	n.discoveryConfig.Events = n
	list, err := memberlist.Create(n.discoveryConfig)
	if err != nil {
		return err
	}
	n.mList = list

	// grpc server
	grpcListen, err := net.Listen("tcp", n.addr)
	if err != nil {
		return err
	}
	n.GrpcServer = ggrpc.NewServer()

	// register management services
	n.TransportManager.Register(n.GrpcServer)

	// register client services
	proto.RegisterRaftServer(n.GrpcServer, NewClientGrpcService(n))

	logfmt.RegisterLevelKey("[DEBUG]", logrus.DebugLevel)

	// discovery method
	discoveryChan, err := n.Conf.Discovery.Start(n.ID, EnvRport)
	if err != nil {
		return err
	}
	go n.handleDiscoveredNodes(discoveryChan)

	// serve grpc
	go func() {
		if err := n.GrpcServer.Serve(grpcListen); err != nil {
			log.Fatal(err)
		}
	}()

	log.Printf("Node started")

	return nil
}

// DiscoveryName returns the name of discovery.
func (n *Node) DiscoveryName() string {
	return n.Conf.Discovery.Name()
}

// Stop stops the node and notifies on stopped channel returned in Start.
func (n *Node) Stop() {
	if !atomic.CompareAndSwapUint32(&n.stopped, 0, 1) {
		return
	}

	log.Print("Stopping Node...")
	n.Conf.Discovery.Stop()
	if err := n.mList.Leave(10 * time.Second); err != nil {
		log.Printf("Failed to leave from discovery: %q", err.Error())
	}
	if err := n.mList.Shutdown(); err != nil {
		log.Printf("Failed to shutdown discovery: %q", err.Error())
	}
	log.Print("Discovery stopped")
	if err := n.Raft.Shutdown().Error(); err != nil {
		log.Printf("Failed to shutdown Raft: %q", err.Error())
	}
	log.Print("Raft stopped")
	n.GrpcServer.GracefulStop()
	log.Print("GrpcServer Server stopped")
	log.Print("Node Stopped!")
}

func (n *Node) findServer(serverID string) bool {
	for _, s := range n.Raft.GetConfiguration().Configuration().Servers {
		if string(s.ID) == serverID {
			return true
		}
	}
	return false
}

// handleDiscoveredNodes handles the discovered Node additions
func (n *Node) handleDiscoveredNodes(discoveryChan chan string) {
	for peer := range discoveryChan {
		peerHost, port := util.Cut(peer, ":")
		if port == "" {
			peer = fmt.Sprintf("%s:%d", peerHost, EnvRport)
		}

		if rsp, err := GetPeerDetails(peer); err == nil {
			if n.findServer(rsp.ServerId) {
				continue
			}

			peerAddr := fmt.Sprintf("%s:%d", peerHost, rsp.DiscoveryPort)
			log.Printf("join to cluster using discovery address: %s", peerAddr)
			if _, err = n.mList.Join([]string{peerAddr}); err != nil {
				log.Printf("W! failed to join to cluster using discovery address: %s", peerAddr)
			}
		}
	}
}

// NotifyJoin triggered when a new Node has been joined to the cluster (discovery only)
// and capable of joining the Node to the raft cluster
func (n *Node) NotifyJoin(node *memberlist.Node) {
	if n.IsLeader() {
		nodeID, nodePort := util.Cut(node.Name, ":")
		nodeAddr := fmt.Sprintf("%s:%s", node.Addr, nodePort)
		if r := n.Raft.AddVoter(raft.ServerID(nodeID), raft.ServerAddress(nodeAddr), 0, 0); r.Error() != nil {
			log.Println(r.Error().Error())
		}
	}
}

// NotifyLeave triggered when a Node becomes unavailable after a period of time
// it will remove the unavailable Node from the Raft cluster
func (n *Node) NotifyLeave(node *memberlist.Node) {
	if n.Conf.Discovery.IsStatic() {
		return
	}

	if n.IsLeader() {
		nodeID, _ := util.Cut(node.Name, ":")
		if r := n.Raft.RemoveServer(raft.ServerID(nodeID), 0, 0); r.Error() != nil {
			log.Println(r.Error().Error())
		}
	}
}

// NotifyUpdate responses the update of raft cluster member.
func (n *Node) NotifyUpdate(_ *memberlist.Node) {}

// IsLeader tells whether the current node is the leader.
func (n *Node) IsLeader() bool { return n.Raft.VerifyLeader().Error() == nil }

// RaftApply is used to apply any new logs to the raft cluster
// this method does automatic forwarding to Leader Node
func (n *Node) RaftApply(request interface{}, timeout time.Duration) (interface{}, error) {
	payload, err := n.Conf.TypeRegister.Marshal(request)
	if err != nil {
		return nil, err
	}

	if n.IsLeader() {
		r := n.Raft.Apply(payload, timeout)
		if r.Error() != nil {
			return nil, r.Error()
		}

		rsp := r.Response()
		if err, ok := rsp.(error); ok {
			return nil, err
		}

		return rsp,
			nil
	}

	log.Printf("transfer to leader")

	return n.ApplyOnLeader(payload)
}

// ShortNodeIds returns a list of short node IDs of the current raft cluster.
func (n *Node) ShortNodeIds() (nodeIds []string) {
	for _, server := range n.Raft.GetConfiguration().Configuration().Servers {
		var rid RaftID
		data, _ := base64.RawURLEncoding.DecodeString(string(server.ID))
		msgpack.Unmarshal(data, &rid)
		nodeIds = append(nodeIds, rid.ID)
	}
	return
}

// Distribute distribute the given bean to all the nodes in the cluster.
func (n *Node) Distribute(bean fsm.Distributable) (interface{}, error) {
	items := bean.GetDistributableItems()
	dataLen := n.distributor.Distribute(n.ShortNodeIds(), items)

	log.Printf("distribute %d items: %s", dataLen, codec.Json(bean))

	return n.RaftApply(fsm.DistributeRequest{Payload: bean}, time.Second)
}

// logger adapters logger to LevelLogger.
type logger struct{}

// Log Emit a message and key/value pairs at a provided log level
func (l *logger) Log(level hclog.Level, msg string, args ...interface{}) {
	switch {
	case level <= hclog.Debug:
		log.Print(append([]interface{}{"D!", msg}, args...)...)
	case level == hclog.Info:
		log.Print(append([]interface{}{"I!", msg}, args...)...)
	case level == hclog.Warn:
		log.Print(append([]interface{}{"W!", msg}, args...)...)
	case level >= hclog.Error:
		log.Print(append([]interface{}{"E!", msg}, args...)...)
	}
}

func (l *logger) Trace(msg string, args ...interface{}) { l.Log(hclog.Trace, msg, args...) }
func (l *logger) Debug(msg string, args ...interface{}) { l.Log(hclog.Debug, msg, args...) }
func (l *logger) Info(msg string, args ...interface{})  { l.Log(hclog.Info, msg, args...) }
func (l *logger) Warn(msg string, args ...interface{})  { l.Log(hclog.Warn, msg, args...) }
func (l *logger) Error(msg string, args ...interface{}) { l.Log(hclog.Error, msg, args...) }

func (l *logger) IsTrace() bool { return false }
func (l *logger) IsDebug() bool { return false }
func (l *logger) IsInfo() bool  { return false }
func (l *logger) IsWarn() bool  { return false }
func (l *logger) IsError() bool { return false }

func (l *logger) ImpliedArgs() []interface{}            { return nil }
func (l *logger) With(args ...interface{}) hclog.Logger { return l }
func (l *logger) Name() string                          { return "" }
func (l *logger) Named(name string) hclog.Logger        { return l }
func (l *logger) ResetNamed(name string) hclog.Logger   { return l }
func (l *logger) SetLevel(level hclog.Level)            {}

func (l *logger) StandardLogger(opts *hclog.StandardLoggerOptions) *log.Logger { return nil }
func (l *logger) StandardWriter(opts *hclog.StandardLoggerOptions) io.Writer   { return nil }
