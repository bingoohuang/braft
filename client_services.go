package braft

import (
	"context"
	"log"
	"os"
	"sync/atomic"
	"time"

	"github.com/bingoohuang/braft/pidusage"
	"github.com/bingoohuang/braft/proto"
	"github.com/bingoohuang/braft/util"
	"github.com/bingoohuang/gg/pkg/jsoni"
)

// NewClientGrpcService creates a new ClientGrpcService.
func NewClientGrpcService(node *Node) *ClientGrpcServices {
	return &ClientGrpcServices{Node: node}
}

// ClientGrpcServices is the client of grpc services.
type ClientGrpcServices struct {
	proto.UnimplementedRaftServer
	Node *Node
}

// ApplyLog responses the request.
func (s *ClientGrpcServices) ApplyLog(_ context.Context, r *proto.ApplyRequest) (*proto.ApplyResponse, error) {
	result := s.Node.Raft.Apply(r.GetRequest(), 0)
	if result.Error() != nil {
		return nil, result.Error()
	}
	respPayload, err := s.Node.Conf.TypeRegister.Marshal(result.Response())
	if err != nil {
		return nil, err
	}
	return &proto.ApplyResponse{Response: respPayload}, nil
}

// GetDetails returns the node details.
func (s *ClientGrpcServices) GetDetails(_ context.Context, r *proto.GetDetailsRequest) (response *proto.GetDetailsResponse, err error) {
	discoveryNodes, resultErr := s.Node.Conf.Discovery.Search()

	if r.Addr != "" {
		s.Node.addrQueue.Put(r.Addr)
	}

	leaderAddr, leaderID := s.Node.Raft.LeaderWithID()
	return &proto.GetDetailsResponse{
		ServerId:       s.Node.ID,
		RaftState:      s.Node.Raft.State().String(),
		Leader:         string(leaderAddr),
		LeaderID:       string(leaderID),
		DiscoveryPort:  int32(s.Node.Conf.Dport),
		HttpPort:       int32(s.Node.Conf.Hport),
		RaftPort:       int32(s.Node.Conf.Rport),
		Error:          util.ErrorString(resultErr),
		DiscoveryNodes: discoveryNodes,
		StartTime:      s.Node.StartTime.Format(time.RFC3339Nano),
		Duration:       time.Since(s.Node.StartTime).String(),
		Rss:            util.Rss(),
		Pcpu: func() float32 {
			stat, err := pidusage.GetStat(os.Getpid())
			if err != nil {
				log.Printf("E! failed to call pidusage.GetStat, error: %v", err)
				return 0
			}
			return float32(stat.Pcpu)
		}(),
		RaftLogSum: atomic.LoadUint64(s.Node.raftLogSum),
		Pid:        uint64(os.Getpid()),
		BizData:    createBizData(s.Node.Conf.BizData),
		Addr:       s.Node.addrQueue.Get(),
		NodeIds:    s.Node.ShortNodeIds(),
	}, nil
}

func createBizData(f func() any) string {
	if f == nil {
		return ""
	}

	s, _ := jsoni.MarshalToString(f())
	return s
}
