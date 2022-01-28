package braft

import (
	"context"

	"github.com/bingoohuang/braft/proto"
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
	respPayload, err := s.Node.conf.Serializer.Serialize(result.Response())
	if err != nil {
		return nil, err
	}
	return &proto.ApplyResponse{Response: respPayload}, nil
}

// GetDetails returns the node details.
func (s *ClientGrpcServices) GetDetails(context.Context, *proto.GetDetailsRequest) (*proto.GetDetailsResponse, error) {
	return &proto.GetDetailsResponse{
		ServerId:      s.Node.ID,
		RaftState:     s.Node.Raft.State().String(),
		Leader:        string(s.Node.Raft.Leader()),
		DiscoveryPort: int32(EnvDport),
		HttpPort:      int32(EnvHport),
		RaftPort:      int32(EnvRport),
	}, nil
}
