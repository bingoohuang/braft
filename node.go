package braft

import (
	"encoding/base64"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"path/filepath"
	"sort"
	"sync/atomic"
	"time"

	"google.golang.org/grpc/reflection"

	"github.com/hashicorp/go-hclog"
	"github.com/vmihailenco/msgpack/v5"

	"google.golang.org/grpc/credentials/insecure"

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
	"github.com/hashicorp/memberlist"
	"github.com/hashicorp/raft"
	raftboltdb "github.com/hashicorp/raft-boltdb"
	"github.com/segmentio/ksuid"
	"github.com/sirupsen/logrus"
	"google.golang.org/grpc"
)

// Node is the raft cluster node.
type Node struct {
	ID               string
	RaftID           RaftID
	addr             string
	Raft             *raft.Raft
	GrpcServer       *grpc.Server
	TransportManager *transport.Manager
	memberConfig     *memberlist.Config
	mList            *memberlist.Memberlist
	stopped          uint32
	Conf             *Config

	StartTime   time.Time
	distributor *fsm.Distributor
	raftLogSum  *uint64

	addrQueue *util.UniqueQueue
}

// Config is the configuration of the node.
type Config struct {
	TypeRegister *marshal.TypeRegister
	DataDir      string
	Discovery    discovery.Discovery
	Services     []fsm.Service
	LeaderChange LeaderChanger
	BizData      func() interface{}
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
	conf, err := createConfig(fns)
	if err != nil {
		return nil, err
	}

	log.Printf("node data dir: %s", conf.DataDir)

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

	stableStoreFile := filepath.Join(conf.DataDir, "store.boltdb")
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
	sm := fsm.NewRoutingFSM(raftID.ID, conf.Services, conf.TypeRegister)

	memberConfig := memberlist.DefaultLocalConfig()
	memberConfig.BindPort = EnvDport
	memberConfig.Name = fmt.Sprintf("%s:%d", nodeID, EnvRport)
	memberConfig.Logger = log.Default()

	// default raft config
	addr := fmt.Sprintf("%s:%d", EnvIP, EnvRport)
	// grpc transport, Transport Raft节点之间的通信通道
	t := transport.New(raft.ServerAddress(addr),
		[]grpc.DialOption{grpc.WithTransportCredentials(insecure.NewCredentials())})

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
		Conf:             conf,
		memberConfig:     memberConfig,
		distributor:      fsm.NewDistributor(),
		raftLogSum:       &sm.RaftLogSum,
		addrQueue:        util.NewUniqueQueue(100),
	}, nil
}

// Start starts the Node and returns a channel that indicates, that the node has been stopped properly
func (n *Node) Start() (err error) {
	n.StartTime = time.Now()
	log.Printf("Node starting, rport: %d, dport: %d, hport: %d, discovery: %s", EnvRport, EnvDport, EnvHport, n.DiscoveryName())

	// 防止各个节点同时启动太快，随机休眠
	util.Think(ss.Or(util.Env("BRAFT_SLEEP", "BSL"), "10ms-15s"))

	// set stopped as false
	atomic.CompareAndSwapUint32(&n.stopped, 1, 0)

	f := n.Raft.BootstrapCluster(raft.Configuration{
		Servers: []raft.Server{{ID: raft.ServerID(n.ID), Address: n.TransportManager.Transport().LocalAddr()}},
	})
	if err := f.Error(); err != nil {
		return err
	}

	// memberlist discovery
	n.memberConfig.Events = n
	if n.mList, err = memberlist.Create(n.memberConfig); err != nil {
		return err
	}

	// grpc server
	grpcListen, err := net.Listen("tcp", n.addr)
	if err != nil {
		return err
	}
	n.GrpcServer = grpc.NewServer()
	// register management services
	n.TransportManager.Register(n.GrpcServer)

	// register client services
	proto.RegisterRaftServer(n.GrpcServer, NewClientGrpcService(n))

	if off := ss.ParseBool(util.Env("DISABLE_GRPC_REFLECTION", "DGR")); !off {
		reflection.Register(n.GrpcServer)
	}

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

	if n.Conf.LeaderChange != nil {
		go func() {
			for becameLeader := range n.Raft.LeaderCh() {
				log.Printf("becameLeader: %v", becameLeader)
				n.Conf.LeaderChange(becameLeader)
			}
		}()
	}

	log.Printf("Node started")

	return nil
}

// DiscoveryName returns the name of discovery.
func (n *Node) DiscoveryName() string { return n.Conf.Discovery.Name() }

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

		if rsp, err := GetPeerDetails(peer, 3*time.Second); err == nil {
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
			log.Printf("raft node joined: %s, addr: %s error: %v", node.Name, nodeAddr, r.Error())
		} else {
			log.Printf("raft node joined: %s, addr: %s sucessfully", node.Name, nodeAddr)
		}
	}
}

// NotifyLeave triggered when a Node becomes unavailable after a period of time
// it will remove the unavailable Node from the Raft cluster
func (n *Node) NotifyLeave(node *memberlist.Node) {
	if n.IsLeader() {
		nodeID, _ := util.Cut(node.Name, ":")
		if r := n.Raft.RemoveServer(raft.ServerID(nodeID), 0, 0); r.Error() != nil {
			log.Printf("raft node left: %s, addr: %s error: %v", node.Name, node.Addr, r.Error())
		} else {
			log.Printf("raft node left: %s, addr: %s sucessfully", node.Name, node.Addr)
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

		return rsp, nil
	}

	log.Printf("transfer to leader")
	return n.ApplyOnLeader(payload, 10*time.Second)
}

// ShortNodeIds returns a sorted list of short node IDs of the current raft cluster.
func (n *Node) ShortNodeIds() (nodeIds []string) {
	for _, server := range n.Raft.GetConfiguration().Configuration().Servers {
		rid := ParseRaftID(string(server.ID))
		nodeIds = append(nodeIds, rid.ID)
	}

	sort.Strings(nodeIds)
	return
}

// ParseRaftID parses the coded raft ID string a RaftID structure.
func ParseRaftID(s string) (rid RaftID) {
	data, _ := base64.RawURLEncoding.DecodeString(s)
	if err := msgpack.Unmarshal(data, &rid); err != nil {
		log.Printf("E! msgpack.Unmarshal raft id %s error:%v", s, err)
	}
	return rid
}

// Distribute distributes the given bean to all the nodes in the cluster.
func (n *Node) Distribute(bean fsm.Distributable) (interface{}, error) {
	items := bean.GetDistributableItems()
	dataLen := n.distributor.Distribute(n.ShortNodeIds(), items)

	log.Printf("distribute %d items: %s", dataLen, codec.Json(bean))
	return n.RaftApply(fsm.DistributeRequest{Data: bean}, time.Second)
}

// logger adapters logger to LevelLogger.
type logger struct{}

// Log Emit a message and key/value pairs at a provided log level
func (l *logger) Log(level hclog.Level, msg string, args ...interface{}) {
	v := append([]interface{}{"D!", msg}, args...)

	switch {
	case level <= hclog.Debug:
		v[0] = "D!"
	case level == hclog.Info:
		v[0] = "I!"
	case level == hclog.Warn:
		v[0] = "W!"
	case level >= hclog.Error:
		v[0] = "E!"
	}

	log.Print(v...)
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

func (l *logger) ImpliedArgs() []interface{}       { return nil }
func (l *logger) With(...interface{}) hclog.Logger { return l }
func (l *logger) Name() string                     { return "" }
func (l *logger) Named(string) hclog.Logger        { return l }
func (l *logger) ResetNamed(string) hclog.Logger   { return l }
func (l *logger) SetLevel(hclog.Level)             {}

func (l *logger) StandardLogger(*hclog.StandardLoggerOptions) *log.Logger { return nil }
func (l *logger) StandardWriter(*hclog.StandardLoggerOptions) io.Writer   { return nil }
