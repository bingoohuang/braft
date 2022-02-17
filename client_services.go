package braft

import (
	"context"
	"log"
	"os"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/bingoohuang/braft/util"

	"github.com/bingoohuang/gg/pkg/jsoni"

	"github.com/bingoohuang/braft/pidusage"

	"github.com/bingoohuang/braft/proto"
)

// NewClientGrpcService creates a new ClientGrpcService.
func NewClientGrpcService(node *Node) *ClientGrpcServices {
	return &ClientGrpcServices{Node: node}
}

// ClientGrpcServices is the client of grpc services.
type ClientGrpcServices struct {
	Node *Node
	proto.UnimplementedRaftServer
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
func (s *ClientGrpcServices) GetDetails(c context.Context, r *proto.GetDetailsRequest) (response *proto.GetDetailsResponse, err error) {
	discoveryNodes, resultErr := s.Node.Conf.Discovery.Search()

	if r.Addr != "" {
		s.Node.addrQueue.Put(r.Addr)
	}

	return &proto.GetDetailsResponse{
		ServerId:       s.Node.ID,
		RaftState:      s.Node.Raft.State().String(),
		Leader:         string(s.Node.Raft.Leader()),
		DiscoveryPort:  int32(EnvDport),
		HttpPort:       int32(EnvHport),
		RaftPort:       int32(EnvRport),
		Error:          util.ErrorString(resultErr),
		DiscoveryNodes: discoveryNodes,
		StartTime:      s.Node.StartTime.Format(time.RFC3339Nano),
		Duration:       time.Since(s.Node.StartTime).String(),
		Rss: func() uint64 {
			var mem syscall.Rusage
			if err := syscall.Getrusage(syscall.RUSAGE_SELF, &mem); err != nil {
				log.Printf("E! failed to call syscall.Getrusage, error: %v", err)
			}
			return uint64(mem.Maxrss)
		}(),
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
	}, nil
}

func createBizData(f func() interface{}) string {
	if f == nil {
		return ""
	}

	s, _ := jsoni.MarshalToString(f())
	return s
}
