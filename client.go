package braft

import (
	"context"
	"errors"
	"os"
	"strings"

	"github.com/bingoohuang/braft/discovery"
	"github.com/bingoohuang/braft/grpc"
	"github.com/bingoohuang/braft/util"
	ggrpc "google.golang.org/grpc"
)

// ApplyOnLeader apply a payload on the leader node.
func (node *Node) ApplyOnLeader(payload []byte) (interface{}, error) {
	addr := string(node.Raft.Leader())
	if addr == "" {
		return nil, errors.New("unknown leader")
	}

	addr = strings.Replace(addr, HostZero, "127.0.0.1", 1)
	conn, err := ggrpc.Dial(addr, ggrpc.WithInsecure(), ggrpc.WithBlock(), ggrpc.EmptyDialOption{})
	if err != nil {
		return nil, err
	}
	defer conn.Close()

	response, err := grpc.NewRaftClient(conn).ApplyLog(context.Background(), &grpc.ApplyRequest{Request: payload})
	if err != nil {
		return nil, err
	}

	result, err := node.conf.Serializer.Deserialize(response.Response)
	if err != nil {
		return nil, err
	}

	return result, nil
}

// GetPeerDetails returns the remote peer details.
func GetPeerDetails(address string) (*grpc.GetDetailsResponse, error) {
	addr, port := util.Cut(string(address), ":")
	switch addr {
	case HostZero:
		address = "127.0.0.1:" + port
	}
	conn, err := ggrpc.Dial(address, ggrpc.WithInsecure(), ggrpc.WithBlock(), ggrpc.EmptyDialOption{})
	if err != nil {
		return nil, err
	}
	defer conn.Close()

	response, err := grpc.NewRaftClient(conn).GetDetails(context.Background(), &grpc.GetDetailsRequest{})
	if err != nil {
		return nil, err
	}

	return response, nil
}

// HostZero is for the all zeros host.
const HostZero = "0.0.0.0"

var (
	// EnvRport is the raft cluster internal port.
	EnvRport = util.GetEnvInt("BRAFT_RPORT", util.RandPort(15000))
	// EnvDport is for the raft cluster nodes discovery.
	EnvDport = util.FindFreePort(util.GetEnvInt("BRAFT_DPORT", EnvRport+1))
	// EnvHport is used for the http service.
	EnvHport = util.FindFreePort(util.GetEnvInt("BRAFT_HPORT", EnvDport+1))
	// EnvDiscovery is the discovery method for the raft cluster.
	// e.g.
	// static:192.168.1.1:1500,192.168.1.2:1500,192.168.1.3:1500
	// k8s:svcType=braft;svcBiz=rig
	// mdns:_braft._tcp
	EnvDiscovery = os.Getenv("BRAFT_DISCOVERY")

	// EnvDiscoveryMethod is the environment defined discovery method.
	EnvDiscoveryMethod = func() discovery.Method {
		s := strings.ToLower(EnvDiscovery)
		switch {
		case s == "k8s" || strings.HasPrefix(s, "k8s:"):
			var serviceLabels map[string]string
			if strings.HasPrefix(s, "k8s:") {
				s1 := strings.TrimPrefix(EnvDiscovery, "k8s:")
				serviceLabels = util.ParseStringToMap(s1, ";", ":")
			}
			return discovery.NewKubernetesDiscovery("", serviceLabels, "")
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
)
