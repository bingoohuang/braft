package easyraft

import (
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"os"
	"os/signal"
	"path/filepath"
	"sync/atomic"
	"syscall"
	"time"

	transport "github.com/Jille/raft-grpc-transport"
	"github.com/bingoohuang/easyraft/discovery"
	"github.com/bingoohuang/easyraft/fsm"
	"github.com/bingoohuang/easyraft/grpc"
	"github.com/bingoohuang/easyraft/serializer"
	"github.com/bingoohuang/easyraft/util"
	"github.com/hashicorp/go-hclog"
	"github.com/hashicorp/memberlist"
	"github.com/hashicorp/raft"
	raftboltdb "github.com/hashicorp/raft-boltdb"
	"github.com/segmentio/ksuid"
	ggrpc "google.golang.org/grpc"
)

type Node struct {
	ID               string
	RaftPort         int
	DiscoveryPort    int
	address          string
	Raft             *raft.Raft
	GrpcServer       *ggrpc.Server
	DiscoveryMethod  discovery.DiscoveryMethod
	TransportManager *transport.Manager
	discoveryConfig  *memberlist.Config
	mList            *memberlist.Memberlist
	stopped          *uint32
	logger           *log.Logger
	stoppedCh        chan interface{}
	nodeConfig       *NodeConfig
}

type NodeConfig struct {
	Serializer      serializer.Serializer
	SnapshotEnabled bool
	DataDir         string
}

type NodeConfigFn func(*NodeConfig)

func WithDataDir(s string) NodeConfigFn {
	return func(c *NodeConfig) {
		c.DataDir = s
	}
}

func WithSerializer(s serializer.Serializer) NodeConfigFn {
	return func(c *NodeConfig) {
		c.Serializer = s
	}
}

// NewNode returns an EasyRaft node
func NewNode(raftPort, discoveryPort int, services []fsm.FSMService,
	discoveryMethod discovery.DiscoveryMethod, fns ...NodeConfigFn) (*Node, error) {
	nodeConfig := &NodeConfig{}
	for _, f := range fns {
		f(nodeConfig)
	}
	if nodeConfig.Serializer == nil {
		nodeConfig.Serializer = serializer.NewMsgPackSerializer()
	}
	if nodeConfig.DataDir == "" {
		dir, err := ioutil.TempDir("", "easyraft")
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

	// default raft config
	addr := fmt.Sprintf("%s:%d", "0.0.0.0", raftPort)

	nodeId := ksuid.New().String()
	raftConf := raft.DefaultConfig()
	raftConf.LocalID = raft.ServerID(nodeId)
	raftConf.LogLevel = hclog.Info.String()

	stableStoreFile := filepath.Join(nodeConfig.DataDir, "store.boltdb")
	if util.FileExists(stableStoreFile) {
		if err := os.Remove(stableStoreFile); err != nil {
			return nil, err
		}
	}
	stableStore, err := raftboltdb.NewBoltStore(stableStoreFile)
	if err != nil {
		return nil, err
	}

	const raftLogCacheSize = 512
	logStore, err := raft.NewLogCache(raftLogCacheSize, stableStore)
	if err != nil {
		return nil, err
	}

	var snapshotStore raft.SnapshotStore
	if !nodeConfig.SnapshotEnabled {
		snapshotStore = raft.NewDiscardSnapshotStore()
	} else {
		// TODO: implement: snapshotStore = NewLogsOnlySnapshotStore(serializer)
		return nil, errors.New("snapshots are not supported at the moment")
	}

	// grpc transport
	grpcTransport := transport.New(raft.ServerAddress(addr), []ggrpc.DialOption{ggrpc.WithInsecure()})

	// init FSM
	sm := fsm.NewRoutingFSM(services)
	sm.Init(nodeConfig.Serializer)

	// memberlist config
	mlConfig := memberlist.DefaultWANConfig()
	mlConfig.BindPort = discoveryPort
	mlConfig.Name = fmt.Sprintf("%s:%d", nodeId, raftPort)

	// raft server
	raftServer, err := raft.NewRaft(raftConf, sm, logStore, stableStore, snapshotStore, grpcTransport.Transport())
	if err != nil {
		return nil, err
	}

	// logging
	logger := log.Default()
	logger.SetPrefix("[EasyRaft] ")

	// initial stopped flag
	var stopped uint32

	return &Node{
		ID:               nodeId,
		RaftPort:         raftPort,
		address:          addr,
		Raft:             raftServer,
		TransportManager: grpcTransport,
		nodeConfig:       nodeConfig,
		DiscoveryPort:    discoveryPort,
		DiscoveryMethod:  discoveryMethod,
		discoveryConfig:  mlConfig,
		logger:           logger,
		stopped:          &stopped,
	}, nil
}

// Start starts the Node and returns a channel that indicates, that the node has been stopped properly
func (n *Node) Start() (chan interface{}, error) {
	n.logger.Print("Starting Node...")
	// set stopped as false
	atomic.CompareAndSwapUint32(n.stopped, 1, 0)

	// raft server
	configuration := raft.Configuration{
		Servers: []raft.Server{
			{
				ID:      raft.ServerID(n.ID),
				Address: n.TransportManager.Transport().LocalAddr(),
			},
		},
	}
	f := n.Raft.BootstrapCluster(configuration)
	if err := f.Error(); err != nil {
		return nil, err
	}

	// memberlist discovery
	n.discoveryConfig.Events = n
	list, err := memberlist.Create(n.discoveryConfig)
	if err != nil {
		return nil, err
	}
	n.mList = list

	// grpc server
	grpcListen, err := net.Listen("tcp", n.address)
	if err != nil {
		log.Fatal(err)
	}
	grpcServer := ggrpc.NewServer()
	n.GrpcServer = grpcServer

	// register management services
	n.TransportManager.Register(grpcServer)

	// register client services
	grpc.RegisterRaftServer(grpcServer, NewClientGrpcService(n))

	// discovery method
	discoveryChan, err := n.DiscoveryMethod.Start(n.ID, n.RaftPort)
	if err != nil {
		return nil, err
	}
	go n.handleDiscoveredNodes(discoveryChan)

	// serve grpc
	go func() {
		if err := grpcServer.Serve(grpcListen); err != nil {
			n.logger.Fatal(err)
		}
	}()

	// handle interruption
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM, syscall.SIGABRT, syscall.SIGKILL)
	go func() {
		<-sigs
		n.Stop()
	}()

	n.logger.Printf("Node started on port %d and discovery port %d\n", n.RaftPort, n.DiscoveryPort)
	n.stoppedCh = make(chan interface{})

	return n.stoppedCh, nil
}

// Stop stops the node and notifies on stopped channel returned in Start
func (n *Node) Stop() {
	if !atomic.CompareAndSwapUint32(n.stopped, 0, 1) {
		return
	}

	if n.nodeConfig.SnapshotEnabled {
		n.logger.Print("Creating snapshot...")
		if err := n.Raft.Snapshot().Error(); err != nil {
			n.logger.Print("Failed to create snapshot!")
		}
	}
	n.logger.Print("Stopping Node...")
	n.DiscoveryMethod.Stop()
	if err := n.mList.Leave(10 * time.Second); err != nil {
		n.logger.Printf("Failed to leave from discovery: %q", err.Error())
	}
	if err := n.mList.Shutdown(); err != nil {
		n.logger.Printf("Failed to shutdown discovery: %q", err.Error())
	}
	n.logger.Print("Discovery stopped")
	if err := n.Raft.Shutdown().Error(); err != nil {
		n.logger.Printf("Failed to shutdown Raft: %q", err.Error())
	}
	n.logger.Print("Raft stopped")
	n.GrpcServer.GracefulStop()
	n.logger.Print("Raft Server stopped")
	n.logger.Print("Node Stopped!")
	n.stoppedCh <- true
}

func (n *Node) findPeerServer(peer, serverId string) bool {
	for _, s := range n.Raft.GetConfiguration().Configuration().Servers {
		if s.ID == raft.ServerID(serverId) || string(s.Address) == peer {
			return true
		}
	}
	return false
}

// handleDiscoveredNodes handles the discovered Node additions
func (n *Node) handleDiscoveredNodes(discoveryChan chan string) {
	for peer := range discoveryChan {
		if rsp, err := GetPeerDetails(peer); err == nil {
			if !n.findPeerServer(peer, rsp.ServerId) {
				peerHost, _ := Cut(peer, ":")
				peerAddr := fmt.Sprintf("%s:%d", peerHost, rsp.DiscoveryPort)
				if _, err = n.mList.Join([]string{peerAddr}); err != nil {
					log.Printf("failed to join to cluster using discovery address: %s", peerAddr)
				}
			}
		}
	}
}

// NotifyJoin triggered when a new Node has been joined to the cluster (discovery only)
// and capable of joining the Node to the raft cluster
func (n *Node) NotifyJoin(node *memberlist.Node) {
	nodeId, nodePort := Cut(node.Name, ":")
	nodeAddr := fmt.Sprintf("%s:%s", node.Addr, nodePort)
	if err := n.Raft.VerifyLeader().Error(); err == nil {
		serverID := raft.ServerID(nodeId)
		addr := raft.ServerAddress(nodeAddr)
		if r := n.Raft.AddVoter(serverID, addr, 0, 0); r.Error() != nil {
			log.Println(r.Error().Error())
		}
	}
}

// NotifyLeave triggered when a Node becomes unavailable after a period of time
// it will remove the unavailable Node from the Raft cluster
func (n *Node) NotifyLeave(node *memberlist.Node) {
	if !n.DiscoveryMethod.SupportsNodeAutoRemoval() {
		return
	}

	if err := n.Raft.VerifyLeader().Error(); err == nil {
		nodeId, _ := Cut(node.Name, ":")
		if r := n.Raft.RemoveServer(raft.ServerID(nodeId), 0, 0); r.Error() != nil {
			log.Println(r.Error().Error())
		}
	}
}

func (n *Node) NotifyUpdate(_ *memberlist.Node) {}

// RaftApply is used to apply any new logs to the raft cluster
// this method does automatic forwarding to Leader Node
func (n *Node) RaftApply(request interface{}, timeout time.Duration) (interface{}, error) {
	payload, err := n.nodeConfig.Serializer.Serialize(request)
	if err != nil {
		return nil, err
	}

	if err := n.Raft.VerifyLeader().Error(); err == nil {
		r := n.Raft.Apply(payload, timeout)
		if r.Error() != nil {
			return nil, r.Error()
		}
		switch r.Response().(type) {
		case error:
			return nil, r.Response().(error)
		default:
			return r.Response(), nil
		}
	}

	return ApplyOnLeader(n, payload)
}
