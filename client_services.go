package braft

import (
	"context"
	"errors"
	"time"

	"github.com/bingoohuang/braft/discovery"
	"github.com/bingoohuang/braft/proto"
	"github.com/hashicorp/go-multierror"
)

// NewClientGrpcService creates a new ClientGrpcService.
func NewClientGrpcService(node *Node) *ClientGrpcServices {
	return &ClientGrpcServices{
		Node: node,
	}
}

// ClientGrpcServices is the client of grpc services.
type ClientGrpcServices struct {
	Node *Node
	proto.UnimplementedRaftServer
}

// ApplyLog responses the request.
func (s *ClientGrpcServices) ApplyLog(ctx context.Context, r *proto.ApplyRequest) (*proto.ApplyResponse, error) {
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

// ErrNone is the a special error which means no error occurred.
var ErrNone = errors.New("")

// GetDetails returns the node details.
func (s *ClientGrpcServices) GetDetails(context.Context, *proto.GetDetailsRequest) (*proto.GetDetailsResponse, error) {
	var resultErr error
	var discoveryNodes []string
	if search, ok := s.Node.Conf.Discovery.(discovery.Searchable); ok {
		nodes, err := search.Search()
		if err != nil {
			resultErr = multierror.Append(resultErr, err)
		}
		discoveryNodes = nodes
	}
	if resultErr == nil {
		resultErr = ErrNone
	}
	return &proto.GetDetailsResponse{
		ServerId:       s.Node.ID,
		RaftState:      s.Node.Raft.State().String(),
		Leader:         string(s.Node.Raft.Leader()),
		DiscoveryPort:  int32(EnvDport),
		HttpPort:       int32(EnvHport),
		RaftPort:       int32(EnvRport),
		Error:          resultErr.Error(),
		DiscoveryNodes: discoveryNodes,
		StartTime:      s.Node.StartTime.Format(time.RFC3339Nano),
		Duration:       time.Since(s.Node.StartTime).String(),
	}, nil
}
