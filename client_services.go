package braft

import (
	"context"

	rgrpc "github.com/bingoohuang/braft/grpc"
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
	rgrpc.UnimplementedRaftServer
}

// ApplyLog responses the request.
func (s *ClientGrpcServices) ApplyLog(ctx context.Context, request *rgrpc.ApplyRequest) (*rgrpc.ApplyResponse, error) {
	result := s.Node.Raft.Apply(request.GetRequest(), 0)
	if result.Error() != nil {
		return nil, result.Error()
	}
	respPayload, err := s.Node.conf.Serializer.Serialize(result.Response())
	if err != nil {
		return nil, err
	}
	return &rgrpc.ApplyResponse{Response: respPayload}, nil
}

// GetDetails returns the node details.
func (s *ClientGrpcServices) GetDetails(context.Context, *rgrpc.GetDetailsRequest) (*rgrpc.GetDetailsResponse, error) {
	return &rgrpc.GetDetailsResponse{
		ServerId:      s.Node.ID,
		RaftState:     s.Node.Raft.State().String(),
		Leader:        string(s.Node.Raft.Leader()),
		DiscoveryPort: int32(EnvDport),
		HttpPort:      int32(EnvHport),
		RaftPort:      int32(EnvRport),
	}, nil
}
