package braft

import (
	"context"
	"errors"
	"os"
	"strings"

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
	conn, err := grpc.Dial(addr, grpc.WithInsecure(), grpc.WithBlock(), grpc.EmptyDialOption{})
	if err != nil {
		return nil, err
	}
	defer conn.Close()

	response, err := proto.NewRaftClient(conn).ApplyLog(context.Background(), &proto.ApplyRequest{Request: payload})
	if err != nil {
		return nil, err
	}

	result, err := n.conf.Serializer.Deserialize(response.Response)
	if err != nil {
		return nil, err
	}

	return result, nil
}

// GetPeerDetails returns the remote peer details.
func GetPeerDetails(addr string) (*proto.GetDetailsResponse, error) {
	addr = strings.Replace(addr, "0.0.0.0", "127.0.0.1", 1)
	c, err := grpc.Dial(addr, grpc.WithInsecure(), grpc.WithBlock(), grpc.EmptyDialOption{})
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

var (

	// EnvDiscovery is the discovery method for the raft cluster.
	// e.g.
	// static:192.168.1.1,192.168.1.2,192.168.1.3
	// k8s:svcType=braft;svcBiz=rig
	// mdns:_braft._tcp
	EnvDiscovery = func() discovery.Discovery {
		s := strings.ToLower(os.Getenv("BRAFT_DISCOVERY"))
		switch {
		case s == "k8s":
			return discovery.NewKubernetesDiscovery()
		case s == "mdns" || s == "" || strings.HasPrefix(s, "mdns:"):
			if strings.HasPrefix(s, "mdns:") {
				s = strings.TrimPrefix(s, "mdns:")
			} else {
				s = ""
			}
			return discovery.NewMdnsDiscovery(s)
		default:
			s = strings.TrimPrefix(s, "static:")
			return discovery.NewStaticDiscovery(strings.Split(s, ","))
		}
	}()

	// EnvIP specifies the current host IP.
	// Priority
	// 1. env $BRAFT_IP, like export BRAFT_IP=192.168.1.1
	// 2. network interface by env $BRAFT_IF (like, export BRAFT_IF=eth0,en0)
	// 3. all zero ip: 0.0.0.0
	EnvIP = func() string {
		if env := os.Getenv("BRAFT_IP"); env != "" {
			return env
		}

		var ifaces []string
		if env := os.Getenv("BRAFT_IF"); env != "" {
			ifaces = append(ifaces, strings.Split(env, ",")...)
		}

		if ip, _ := goip.MainIP(ifaces...); ip != "" {
			return ip
		}

		return "0.0.0.0"
	}()

	// EnvRport is the raft cluster internal port.
	EnvRport = util.GetEnvInt("BRAFT_RPORT", util.FindFreePort("", 15000))
	// EnvDport is for the raft cluster nodes discovery.
	EnvDport = util.FindFreePort("", util.GetEnvInt("BRAFT_DPORT", EnvRport+1))
	// EnvHport is used for the http service.
	EnvHport = util.FindFreePort("", util.GetEnvInt("BRAFT_HPORT", EnvDport+1))
)
