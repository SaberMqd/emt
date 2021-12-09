package core

import (
	"context"
	rpcpb "emt/example/proto/db"
	"emt/rpc"
	emtgrpc "emt/rpc/grpc"

	"google.golang.org/grpc"
)

var DefaultRPCSrv rpc.Server

func init() {
	DefaultRPCSrv = emtgrpc.NewServer(
		rpc.ServerOptionWithAddr(RPCListenAddr),
		rpc.ServerOptionWithName(RPCServerName),
		emtgrpc.ServerOptionWithConfig(&RPCServer{}))
}

type RPCServer struct {
}

func (s *RPCServer) BattleDailyTask(ctx context.Context, in *rpcpb.BattleDailyTaskReq) (*rpcpb.BattleDailyTaskRes, error) {
	return nil, nil
}

func (s *RPCServer) GetCompUserInfo(ctx context.Context, in *rpcpb.CompUserInfoReq) (*rpcpb.CompUserInfoRes, error) {
	return nil, nil
}

func (s *RPCServer) GameOver(ctx context.Context, in *rpcpb.GameOverReq) (*rpcpb.GameOverRes, error) {
	return nil, nil
}

func (s *RPCServer) GetUserInfo(ctx context.Context, in *rpcpb.UserInfoReq) (*rpcpb.UserInfoRes, error) {
	return &rpcpb.UserInfoRes{Code: TestID}, nil
}

func (s *RPCServer) OnListened(se interface{}) {
	rpcpb.RegisterDBRPCServer(se.(*grpc.Server), s)
}
