package braft

import (
	"context"
	"errors"
	"strings"

	"github.com/bingoohuang/gg/pkg/ss"

	"google.golang.org/grpc/credentials/insecure"

	"github.com/bingoohuang/braft/discovery"
	"github.com/bingoohuang/braft/proto"
	"github.com/bingoohuang/braft/util"
	"github.com/bingoohuang/gg/pkg/goip"
	"google.golang.org/grpc"
)

// ApplyOnLeader apply a payload on the leader node.
func (n *Node) ApplyOnLeader(payload []byte) (interface{}, error) {
	addr := string(n.Raft.Leader())
	if addr == "" {
		return nil, errors.New("unknown leader")
	}

	addr = strings.Replace(addr, "0.0.0.0", "127.0.0.1", 1)
	conn, err := grpc.Dial(addr, grpc.WithTransportCredentials(insecure.NewCredentials()), grpc.WithBlock(), grpc.EmptyDialOption{})
	if err != nil {
		return nil, err
	}
	defer conn.Close()

	response, err := proto.NewRaftClient(conn).ApplyLog(context.Background(), &proto.ApplyRequest{Request: payload})
	if err != nil {
		return nil, err
	}

	return n.Conf.TypeRegister.Unmarshal(response.Response)
}

// GetPeerDetails returns the remote peer details.
func GetPeerDetails(addr string) (*proto.GetDetailsResponse, error) {
	addr = strings.Replace(addr, "0.0.0.0", "127.0.0.1", 1)
	c, err := grpc.Dial(addr, grpc.WithTransportCredentials(insecure.NewCredentials()), grpc.WithBlock(), grpc.EmptyDialOption{})
	if err != nil {
		return nil, err
	}
	defer c.Close()

	response, err := proto.NewRaftClient(c).GetDetails(context.Background(), &proto.GetDetailsRequest{})
	if err != nil {
		return nil, err
	}

	return response, nil
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
		if len(peers) <= 0 {
			peers = DefaultStaticPeers
		}

		return discovery.NewStaticDiscovery(peers)
	}
}

var (
	// DefaultMdnsService is the default mDNS service.
	DefaultMdnsService = "_braft._tcp,_windows"
	// DefaultK8sNamespace is the default namespace for k8s.
	DefaultK8sNamespace = util.Env("K8S_NAMESPACE", "K8N")
	// DefaultK8sPortName is the default port name for k8s.
	DefaultK8sPortName = util.Env("K8S_PORTNAME", "K8P")
	// DefaultK8sServiceLabels is the default labels for k8s.
	// e.g. svc=braft,type=demo
	DefaultK8sServiceLabels = ss.SplitToMap(util.Env("K8S_LABELS", "K8L"), "=", ",")
	// DefaultStaticPeers is the default static peers of static raft cluster nodes.
	DefaultStaticPeers []string
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
)
