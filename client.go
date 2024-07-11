// Package braft 提供了 raft 更加方便的集成 API 胶水代码。
package braft

import (
	"context"
	"errors"
	"log"
	"strings"
	"time"

	"github.com/bingoohuang/braft/discovery"
	"github.com/bingoohuang/braft/proto"
	"github.com/bingoohuang/braft/util"
	"github.com/bingoohuang/gg/pkg/goip"
	"github.com/bingoohuang/gg/pkg/ss"
	"go.uber.org/multierr"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// ApplyOnLeader apply a payload on the leader node.
func (n *Node) ApplyOnLeader(payload []byte, timeout time.Duration) (any, error) {
	addr, _ := n.Raft.LeaderWithID()
	if addr == "" {
		return nil, errors.New("unknown leader")
	}

	ctx, deferFn, c, err := GetRaftClient(string(addr), timeout)
	defer deferFn()
	if err != nil {
		return nil, err
	}

	response, err := c.ApplyLog(ctx, &proto.ApplyRequest{Request: payload})
	if err != nil {
		return nil, err
	}

	return n.Conf.TypeRegister.Unmarshal(response.Response)
}

// GetPeerDetails returns the remote peer details.
func GetPeerDetails(addr string, timeout time.Duration) (*proto.GetDetailsResponse, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	ctx, deferFn, c, err := GetRaftClient(addr, timeout)
	defer deferFn()
	if err != nil {
		return nil, err
	}

	return c.GetDetails(ctx, &proto.GetDetailsRequest{Addr: addr})
}

// GetRaftClient returns the raft client with timeout context.
func GetRaftClient(addr string, timeout time.Duration) (ctx context.Context, deferFn func(), client proto.RaftClient, err error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	deferFns := []func() error{func() error { cancel(); return nil }}

	addr = strings.Replace(addr, "0.0.0.0", "127.0.0.1", 1)
	c, err := grpc.DialContext(ctx, addr, grpc.WithTransportCredentials(insecure.NewCredentials()), grpc.WithBlock(), grpc.EmptyDialOption{})
	if err != nil {
		return ctx, wrapDefers(deferFns), nil, err
	}
	deferFns = append(deferFns, c.Close)

	return ctx, wrapDefers(deferFns), proto.NewRaftClient(c), nil
}

func wrapDefers(fns []func() error) func() {
	return func() {
		var err error
		for _, fn := range fns {
			err = multierr.Append(err, fn())
		}
		if err != nil {
			log.Printf("E! error occured: %v", err)
		}
	}
}

// CreateDiscovery creates a new discovery from the given discovery method.
func CreateDiscovery(discoveryMethod string) discovery.Discovery {
	switch s := discoveryMethod; {
	case s == "k8s" || strings.HasPrefix(s, "k8s:"):
		s := ss.If(s == "k8s", "", s)
		s = strings.TrimPrefix(s, "k8s:")
		m := ss.SplitToMap(s, "=", ",")
		findAndDelete := func(key string) string {
			for k, v := range m {
				if strings.EqualFold(key, k) {
					delete(m, k)
					return v
				}
			}
			return ""
		}

		namespace := ss.Or(findAndDelete("namespace"), DefaultK8sNamespace)
		portname := ss.Or(findAndDelete("portname"), DefaultK8sPortName)
		serviceLabels := util.OrSlice(m, DefaultK8sServiceLabels)
		return discovery.NewKubernetesDiscovery(namespace, portname, serviceLabels)
	case s == "mdns" || s == "" || strings.HasPrefix(s, "mdns:"):
		s := ss.If(s == "mdns", "", s)
		s = strings.TrimPrefix(s, "mdns:")
		serverName := ss.Or(s, DefaultMdnsService)
		return discovery.NewMdnsDiscovery(serverName)
	default:
		s = strings.TrimPrefix(s, "static:")
		peers := ss.Split(s, ss.WithSeps(","), ss.WithIgnoreEmpty(true), ss.WithTrimSpace(true))
		if len(peers) == 0 {
			peers = DefaultStaticPeers
		}

		return discovery.NewStaticDiscovery(peers)
	}
}

var (
	// DefaultMdnsService 默认 Mdns 服务名称
	DefaultMdnsService string
	// DefaultK8sNamespace 默认 k8s 命名空间名称
	DefaultK8sNamespace string
	// DefaultK8sPortName 默认 k8s 端口名称
	DefaultK8sPortName string
	// DefaultK8sServiceLabels 默认 i8s 服务标签
	DefaultK8sServiceLabels map[string]string
	// DefaultDiscovery 默认发现策略
	DefaultDiscovery string
	// EnvIP IP 值
	EnvIP string
	// EnvRport Raft 端口值
	EnvRport int
	// EnvDport Discovery 端口值
	EnvDport int
	// EnvHport HTTP 端口值
	EnvHport int
	// DefaultStaticPeers 静态端点列表
	DefaultStaticPeers []string
)

func init() {
	Setup()
}

// Setup 初始化设置客户端
func Setup() {
	// DefaultMdnsService is the default mDNS service.
	DefaultMdnsService = ss.Or(util.Env("MDNS_SERVICE", "MDS"), "_braft._tcp,_windows")
	// DefaultK8sNamespace is the default namespace for k8s.
	DefaultK8sNamespace = util.Env("K8S_NAMESPACE", "K8N")
	// DefaultK8sPortName is the default port name for k8s.
	DefaultK8sPortName = util.Env("K8S_PORTNAME", "K8P")
	// DefaultK8sServiceLabels is the default labels for k8s.
	// e.g. svc=braft,type=demo
	DefaultK8sServiceLabels = ss.SplitToMap(util.Env("K8S_LABELS", "K8L"), "=", ",")
	// DefaultStaticPeers is the default static peers of static raft cluster nodes.

	// DefaultDiscovery is the default discovery method for the raft cluster member discovery.
	// e.g.
	// static:192.168.1.1,192.168.1.2,192.168.1.3
	// k8s:svcType=braft;svcBiz=rig
	// mdns:_braft._tcp
	DefaultDiscovery = strings.ToLower(util.Env("BRAFT_DISCOVERY", "BDI"))

	// EnvIP specifies the current host IP.
	// Priority
	// 1. env $BRAFT_IP, like export BRAFT_IP=192.168.1.1
	// 2. network interface by env $BRAFT_IF (like, export BRAFT_IF=eth0,en0)
	// 3. all zero ip: 0.0.0.0
	EnvIP = func() string {
		if env := util.Env("BRAFT_IP", "BIP"); env != "" {
			return env
		}

		var ifaces []string
		if env := util.Env("BRAFT_IF", "BIF"); env != "" {
			ifaces = append(ifaces, strings.Split(env, ",")...)
		}

		if ip, _ := goip.MainIP(ifaces...); ip != "" {
			return ip
		}

		return "0.0.0.0"
	}()

	// EnvRport is the raft cluster internal port.
	EnvRport = util.Atoi(util.Env("BRAFT_RPORT", "BRP"), util.FindFreePort("", 15000))
	// EnvDport is for the raft cluster nodes discovery.
	EnvDport = util.Atoi(util.Env("BRAFT_DPORT", "BDP"), EnvRport+1)
	// EnvHport is used for the http service.
	EnvHport = util.Atoi(util.Env("BRAFT_HPORT", "BHP"), EnvDport+1)
}
