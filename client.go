package easyraft

import (
	"context"
	"errors"
	"strings"

	"github.com/bingoohuang/easyraft/grpc"
	ggrpc "google.golang.org/grpc"
)

func ApplyOnLeader(node *Node, payload []byte) (interface{}, error) {
	leader := string(node.Raft.Leader())
	if leader == "" {
		return nil, errors.New("unknown leader")
	}
	conn, err := ggrpc.Dial(string(leader), ggrpc.WithInsecure(), ggrpc.WithBlock(), ggrpc.EmptyDialOption{})
	if err != nil {
		return nil, err
	}
	defer conn.Close()

	response, err := grpc.NewRaftClient(conn).ApplyLog(context.Background(), &grpc.ApplyRequest{Request: payload})
	if err != nil {
		return nil, err
	}

	result, err := node.nodeConfig.Serializer.Deserialize(response.Response)
	if err != nil {
		return nil, err
	}

	return result, nil
}

func Cut(s, sep string) (a, b string) {
	ret := strings.Split(s, sep)
	if len(ret) >= 2 {
		return ret[0], ret[1]
	} else if len(ret) >= 1 {
		return ret[0], ""
	} else {
		return "", ""
	}
}

func GetPeerDetails(address string) (*grpc.GetDetailsResponse, error) {
	addr, port := Cut(string(address), ":")
	switch addr {
	case "0.0.0.0":
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
