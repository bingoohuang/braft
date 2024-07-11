package braft

import (
	"context"
	"encoding/base64"
	"fmt"
	"github.com/samber/lo"
	"os/signal"
	"syscall"

	"io"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"sync"
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
	"github.com/hashicorp/go-sockaddr"
	"github.com/hashicorp/memberlist"
	"github.com/hashicorp/raft"
	raftboltdb "github.com/hashicorp/raft-boltdb"
	"github.com/segmentio/ksuid"
	"github.com/sirupsen/logrus"
	"github.com/sqids/sqids-go"
	"github.com/vmihailenco/msgpack/v5"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/reflection"
)

// Node is the raft cluster node.
type Node struct {
	StartTime time.Time
	ctx       context.Context

	wg         *sync.WaitGroup
	Raft       *raft.Raft
	GrpcServer *grpc.Server

	httpServer   *http.Server
	memberConfig *memberlist.Config
	mList        *memberlist.Memberlist
	cancelFunc   context.CancelFunc

	Conf *Config

	TransportManager *transport.Manager
	distributor      *fsm.Distributor
	raftLogSum       *uint64

	addrQueue *util.UniqueQueue
	notifyCh  chan NotifyEvent
	addr      string
	ID        string
	fns       []ConfigFn
	RaftID    RaftID
	stopped   uint32
}

// Config is the configuration of the node.
type Config struct {
	TypeRegister  *marshal.TypeRegister
	DataDir       string
	Discovery     discovery.Discovery
	Services      []fsm.Service
	LeaderChange  NodeStateChanger
	BizData       func() any
	HTTPConfigFns []HTTPConfigFn
	EnableHTTP    bool
}

// RaftID is the structure of node ID.
type RaftID struct {
	ID       string `json:"id"`
	Hostname string `json:"hostname"`
	IP       string `json:"ip"`
	Sqid     string `json:"sqid"` // {RaftPort, Dport, Hport}
}

// NewNode returns an BRaft node.
func NewNode(fns ...ConfigFn) (*Node, error) {
	node := &Node{fns: fns}
	if err := node.createNode(); err != nil {
		return nil, err
	}

	return node, nil
}

func (n *Node) createNode() error {
	conf, err := createConfig(n.fns)
	if err != nil {
		return err
	}

	log.Printf("node data dir: %s", conf.DataDir)

	raftID := RaftID{
		ID: ksuid.New().String(),
		Sqid: util.Pick1(util.Pick1(sqids.New()).Encode(
			[]uint64{uint64(EnvRport), uint64(EnvDport), uint64(EnvHport)})),
		Hostname: util.Pick1(os.Hostname()),
		IP:       util.Pick1(goip.MainIP()),
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
			return err
		}
	}
	// StableStore 稳定存储,存储Raft集群的节点信息
	stableStore, err := raftboltdb.NewBoltStore(stableStoreFile)
	if err != nil {
		return err
	}

	// LogStore 存储Raft的日志
	logStore, err := raft.NewLogCache(512, stableStore)
	if err != nil {
		return err
	}

	// SnapshotStore 快照存储,存储节点的快照信息
	snapshotStore := raft.NewDiscardSnapshotStore()

	// FSM 有限状态机
	sm := fsm.NewRoutingFSM(raftID.ID, conf.Services, conf.TypeRegister)

	// default raft config
	addr := fmt.Sprintf("%s:%d", EnvIP, EnvRport)
	// grpc transport, Transport Raft节点之间的通信通道
	t := transport.New(raft.ServerAddress(addr),
		[]grpc.DialOption{grpc.WithTransportCredentials(insecure.NewCredentials())})

	// raft server
	raftServer, err := raft.NewRaft(raftConf, sm, logStore, stableStore, snapshotStore, t.Transport())
	if err != nil {
		return err
	}

	n.ID = nodeID
	n.RaftID = raftID
	n.addr = fmt.Sprintf(":%d", EnvRport)
	n.Raft = raftServer
	n.TransportManager = t
	n.Conf = conf
	n.memberConfig = func(nodeID string, dport, rport int) *memberlist.Config {
		c := memberlist.DefaultLocalConfig()

		// fix "get final advertise address: No private IP address found, and explicit IP not provided"
		if privateIP, _ := sockaddr.GetPrivateIP(); privateIP == "" {
			if allIPv4, _ := goip.ListAllIPv4(); len(allIPv4) > 0 {
				c.AdvertiseAddr = allIPv4[0]
				c.AdvertisePort = dport
			}
		}

		c.BindPort = dport
		c.Name = fmt.Sprintf("%s:%d", nodeID, rport)
		c.Logger = log.Default()
		return c
	}(nodeID, EnvDport, EnvRport)

	n.distributor = fsm.NewDistributor()
	n.raftLogSum = &sm.RaftLogSum
	n.addrQueue = util.NewUniqueQueue(100)
	n.notifyCh = make(chan NotifyEvent, 100)

	return nil
}

// Start starts the Node and returns a channel that indicates, that the node has been stopped properly
func (n *Node) Start() (err error) {
	var exitBool atomic.Bool
	c := make(chan os.Signal)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-c
		n.Stop()
		exitBool.Store(true)
	}()

	for {
		if err := n.start(); err != nil {
			return err
		}

		n.wait()

		if exitBool.Load() {
			// 等待10秒，等待 memberlist leave node
			time.Sleep(10 * time.Second)
			return fmt.Errorf("cancelled")
		}

		if err = n.createNode(); err != nil {
			log.Printf("restart failed: %v", err)
			return err
		}

		log.Printf("restart sucessfully")
	}
}

func (n *Node) start() (err error) {
	n.StartTime = time.Now()
	log.Printf("Node starting, rport: %d, dport: %d, hport: %d, discovery: %s",
		EnvRport, EnvDport, EnvHport, n.DiscoveryName())

	// 防止各个节点同时启动太快，随机休眠
	util.Think(ss.Or(util.Env("BRAFT_SLEEP", "BSL"), "10ms-15s"), "")

	// set stopped as false
	atomic.CompareAndSwapUint32(&n.stopped, 1, 0)

	f := n.Raft.BootstrapCluster(raft.Configuration{
		Servers: []raft.Server{{
			ID:      raft.ServerID(n.ID),
			Address: n.TransportManager.Transport().LocalAddr(),
		}},
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

	n.wg = &sync.WaitGroup{}

	// discovery method
	discoveryChan, err := n.Conf.Discovery.Start(n.ID, EnvDport)
	if err != nil {
		return err
	}

	n.ctx, n.cancelFunc = context.WithCancel(context.Background())

	n.goHandleDiscoveredNodes(discoveryChan)

	// serve grpc
	util.Go(n.wg, func() {
		if err := n.GrpcServer.Serve(grpcListen); err != nil {
			log.Printf("E! GrpcServer failed: %v", err)
		}
	})

	if n.Conf.LeaderChange != nil {
		d := util.EnvDuration("BRAFT_LEADER_STEADY", 60*time.Second)
		delayLeaderChanger := util.NewDelayWorker(n.wg, n.ctx, d, func(state NodeState, t time.Time) {
			n.Conf.LeaderChange(n, state)
		})
		util.GoChan(n.ctx, n.wg, n.Raft.LeaderCh(), func(becameLeader bool) error {
			log.Printf("becameLeader: %v", becameLeader)
			delayLeaderChanger.Notify(lo.Ternary(becameLeader, NodeLeader, NodeFollower))
			return nil
		})
	}

	n.goDealNotifyEvent()
	if n.Conf.EnableHTTP {
		n.runHTTP(n.Conf.HTTPConfigFns...)
	}

	log.Printf("Node started")

	return nil
}

// DiscoveryName returns the name of discovery.
func (n *Node) DiscoveryName() string { return n.Conf.Discovery.Name() }

// Stop stops the node and notifies on a stopped channel returned in Start.
func (n *Node) Stop() {
	if !atomic.CompareAndSwapUint32(&n.stopped, 0, 1) {
		return
	}

	log.Print("Stopping Node...")

	if n.Conf.LeaderChange != nil {
		n.Conf.LeaderChange(n, NodeShuttingDown)
	}

	n.Conf.Discovery.Stop()
	if err := n.mList.Leave(10 * time.Second); err != nil {
		log.Printf("E! leave from discovery: %v", err)
	}
	if err := n.mList.Shutdown(); err != nil {
		log.Printf("E! shutdown discovery failed: %v", err)
	}
	log.Print("Discovery stopped")
	if err := n.Raft.Shutdown().Error(); err != nil {
		log.Printf("E! shutdown Raft failed: %v", err)
	}
	log.Print("Raft stopped")
	n.GrpcServer.Stop()
	log.Print("GrpcServer Server stopped")

	n.cancelFunc()

	if n.httpServer != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := n.httpServer.Shutdown(ctx); err != nil {
			log.Printf("E! server Shutdown failed: %v", err)
		}
	}
}

func (n *Node) findServer(serverID string) bool {
	for _, s := range n.GetRaftServers() {
		if string(s.ID) == serverID {
			return true
		}
	}
	return false
}

// goHandleDiscoveredNodes handles the discovered Node additions
func (n *Node) goHandleDiscoveredNodes(discoveryChan chan string) {
	util.GoChan(n.ctx, n.wg, discoveryChan, func(peer string) error {
		peerHost, port := util.Cut(peer, ":")
		if port == "" {
			peer = fmt.Sprintf("%s:%d", peerHost, EnvDport)
		}

		// format of peer should ip:port (the port is for discovery)
		if _, err := n.mList.Join([]string{peer}); err != nil {
			log.Printf("E! %s joined memberlist error: %v", peer, err)
		} else {
			log.Printf("%s joined memberlist successfully", peer)
		}

		return nil
	})
}

// NotifyType 定义通知类型
type NotifyType int

const (
	_ NotifyType = iota
	// NotifyJoin 通知加入 Raft 集群
	NotifyJoin
	// NotifyLeave 通知离开 Raft 集群
	NotifyLeave
	// NotifyUpdate 通知更新 Raft 集群
	NotifyUpdate
)

func (t NotifyType) String() string {
	switch t {
	case NotifyJoin:
		return "NotifyJoin"
	case NotifyLeave:
		return "NotifyLeave"
	case NotifyUpdate:
		return "NotifyUpdate"
	default:
		return "Unknown"
	}
}

// NotifyEvent 通知事件
type NotifyEvent struct {
	*memberlist.Node
	NotifyType
}

var waitLeaderTime = util.EnvDuration("BRAFT_RESTART_MIN", 90*time.Second)

func (n *Node) goDealNotifyEvent() {
	waitLeader := make(chan NotifyEvent, 100)

	util.GoChan(n.ctx, n.wg, n.notifyCh, func(e NotifyEvent) error {
		n.processNotify(e, waitLeader)
		return nil
	})

	util.GoChan(n.ctx, n.wg, waitLeader, func(e NotifyEvent) error {
		leaderAddr, leaderID, err := n.waitLeader(waitLeaderTime)
		if err != nil {
			return err
		}

		isLeader := n.IsLeader()
		log.Printf("leader waited, type: %s, leader: %s, leaderID: %s, isLeader: %t, node: %s",
			e.NotifyType, leaderAddr, leaderID, isLeader, codec.Json(e.Node))
		n.processNotifyAtLeader(isLeader, e)
		return nil
	})
}

func (n *Node) waitLeader(minWait time.Duration) (leaderAddr, leaderID string, err error) {
	start := time.Now()
	for {
		if addr, id := n.Raft.LeaderWithID(); addr != "" {
			log.Printf("waited leader: %s cost: %s", id, time.Since(start))
			return string(addr), string(id), nil
		}
		if time.Since(start) >= minWait {
			log.Printf("stop to wait leader, expired %s >= %s", time.Since(start), minWait)
			n.Stop()
			return "", "", io.EOF
		}

		util.Think("10-20s", "wait for leader")
	}
}

func (n *Node) processNotify(e NotifyEvent, waitLeader chan NotifyEvent) {
	leader, _ := n.Raft.LeaderWithID() // return empty string if there is no current leader
	isLeader := n.IsLeader()
	log.Printf("received type: %s, leader: %s, isLeader: %t, node: %s",
		e.NotifyType, leader, isLeader, codec.Json(e.Node))
	if leader != "" {
		n.processNotifyAtLeader(isLeader, e)
		return
	}

	select {
	case waitLeader <- e:
		log.Printf("current no leader, to wait list, type: %s, leader: %s, isLeader: %t, node: %s",
			e.NotifyType, leader, isLeader, codec.Json(e.Node))
	default:
		log.Printf("too many waitLeaders")
	}
}

func (n *Node) processNotifyAtLeader(isLeader bool, e NotifyEvent) {
	leader, _ := n.Raft.LeaderWithID()
	log.Printf("processing type: %s, leader: %s, isLeader: %t, node: %s",
		e.NotifyType, leader, isLeader, codec.Json(e.Node))

	switch e.NotifyType {
	case NotifyJoin:
		n.join(e.Node)
	case NotifyLeave:
		n.leave(e.Node)
	}
}

func (n *Node) leave(node *memberlist.Node) {
	nodeID, _ := util.Cut(node.Name, ":")
	if r := n.Raft.RemoveServer(raft.ServerID(nodeID), 0, 0); r.Error() != nil {
		log.Printf("E! raft node left: %s, addr: %s, error: %v", node.Name, node.Addr, r.Error())
	} else {
		log.Printf("raft node left: %s, addr: %s sucessfully", node.Name, node.Addr)
	}
}

func (n *Node) join(node *memberlist.Node) {
	nodeID, _ := util.Cut(node.Name, ":")
	nodeAddr := fmt.Sprintf("%s:%d", node.Addr, node.Port-1)
	if r := n.Raft.AddVoter(raft.ServerID(nodeID), raft.ServerAddress(nodeAddr), 0, 0); r.Error() != nil {
		log.Printf("E! raft node joined: %s, addr: %s, error: %v", node.Name, nodeAddr, r.Error())
	} else {
		log.Printf("raft node joined: %s, addr: %s sucessfully", node.Name, nodeAddr)
	}
}

// NotifyJoin triggered when a new Node has been joined to the cluster (discovery only)
// and capable of joining the Node to the raft cluster
func (n *Node) NotifyJoin(node *memberlist.Node) {
	n.notifyCh <- NotifyEvent{NotifyType: NotifyJoin, Node: node}
}

// NotifyLeave triggered when a Node becomes unavailable after a period of time
// it will remove the unavailable Node from the Raft cluster
func (n *Node) NotifyLeave(node *memberlist.Node) {
	n.notifyCh <- NotifyEvent{NotifyType: NotifyLeave, Node: node}
}

// NotifyUpdate responses the update of raft cluster member.
func (n *Node) NotifyUpdate(node *memberlist.Node) {
	n.notifyCh <- NotifyEvent{NotifyType: NotifyUpdate, Node: node}
}

// IsLeader tells whether the current node is the leader.
func (n *Node) IsLeader() bool { return n.Raft.VerifyLeader().Error() == nil }

// RaftApply is used to apply any new logs to the raft cluster
// this method will do automatic forwarding to the Leader Node
func (n *Node) RaftApply(request any, timeout time.Duration) (any, error) {
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

// ShortNodeIds returns a sorted list of short node IDs in the current raft cluster.
func (n *Node) ShortNodeIds() (nodeIds []string) {
	for _, server := range n.GetRaftServers() {
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
func (n *Node) Distribute(bean fsm.Distributable) (any, error) {
	items := bean.GetDistributableItems()
	dataLen := n.distributor.Distribute(n.ShortNodeIds(), items)

	log.Printf("distribute %d items: %s", dataLen, codec.Json(bean))
	return n.RaftApply(fsm.DistributeRequest{Data: bean}, time.Second)
}

func (n *Node) wait() {
	n.wg.Wait()
	log.Print("Node Stopped!")
}

// logger adapters logger to LevelLogger.
type logger struct{}

func (l *logger) GetLevel() hclog.Level { return hclog.Debug }

// Log Emit a message and key/value pairs at a provided log level
func (l *logger) Log(level hclog.Level, msg string, args ...any) {
	for i, arg := range args {
		// Convert the field value to a string.
		switch st := arg.(type) {
		case hclog.Hex:
			args[i] = "0x" + strconv.FormatUint(uint64(st), 16)
		case hclog.Octal:
			args[i] = "0" + strconv.FormatUint(uint64(st), 8)
		case hclog.Binary:
			args[i] = "0b" + strconv.FormatUint(uint64(st), 2)
		case hclog.Format:
			args[i] = fmt.Sprintf(st[0].(string), st[1:]...)
		case hclog.Quote:
			args[i] = strconv.Quote(string(st))
		}
	}

	v := append([]any{"D!", msg}, args...)

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

	log.Print(logPrint(v))
}

func logPrint(a []any) string {
	var buf []byte
	for i, arg := range a {
		if i > 0 { // Add a space
			buf = append(buf, ' ')
		}
		buf = append(buf, []byte(fmt.Sprint(arg))...)
	}

	return string(buf)
}

func (l *logger) Trace(msg string, args ...any) { l.Log(hclog.Trace, msg, args...) }
func (l *logger) Debug(msg string, args ...any) { l.Log(hclog.Debug, msg, args...) }
func (l *logger) Info(msg string, args ...any)  { l.Log(hclog.Info, msg, args...) }
func (l *logger) Warn(msg string, args ...any)  { l.Log(hclog.Warn, msg, args...) }
func (l *logger) Error(msg string, args ...any) { l.Log(hclog.Error, msg, args...) }

func (l *logger) IsTrace() bool { return false }
func (l *logger) IsDebug() bool { return false }
func (l *logger) IsInfo() bool  { return false }
func (l *logger) IsWarn() bool  { return false }
func (l *logger) IsError() bool { return false }

func (l *logger) ImpliedArgs() []any             { return nil }
func (l *logger) With(...any) hclog.Logger       { return l }
func (l *logger) Name() string                   { return "" }
func (l *logger) Named(string) hclog.Logger      { return l }
func (l *logger) ResetNamed(string) hclog.Logger { return l }
func (l *logger) SetLevel(hclog.Level)           {}

func (l *logger) StandardLogger(*hclog.StandardLoggerOptions) *log.Logger { return nil }
func (l *logger) StandardWriter(*hclog.StandardLoggerOptions) io.Writer   { return nil }
