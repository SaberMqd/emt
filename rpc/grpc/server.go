package grpc

import (
	rpc "emt/rpc"
	"fmt"
	"net"

	"github.com/google/uuid"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

type server struct {
	opts rpc.ServerOptions

	sub  ServerSub
	lis  net.Listener
	gsvr *grpc.Server
}

func NewServer(opts ...rpc.ServerOption) rpc.Server {
	svr := &server{}

	for _, o := range opts {
		o(&svr.opts)
	}

	if svr.opts.MaxMsgLen == 0 {
		svr.opts.MaxMsgLen = rpc.DefaultMaxMsgLen
	}

	if svr.opts.ID == "" {
		svr.opts.ID = uuid.New().String()
	}

	if svr.opts.Context != nil {
		cfg, ok := svr.opts.Context.Value(grpcServerConfigKey{}).(*grpcServerConfig)
		if ok {
			svr.sub = cfg.Sub
		}
	}

	if svr.sub == nil {
		panic("grpc server sub is nil")
	}

	return svr
}

func (svr *server) Start() error {
	var err error

	svr.lis, err = net.Listen("tcp", svr.opts.Addr)
	if err != nil {
		return fmt.Errorf("failed to listen %w", err)
	}

	svr.gsvr = grpc.NewServer()
	svr.sub.OnListened(svr.gsvr)
	reflection.Register(svr.gsvr)

	if err := svr.gsvr.Serve(svr.lis); err != nil {
		return fmt.Errorf("grpc server %w", err)
	}

	return nil
}

func (svr *server) Options() rpc.ServerOptions {
	return svr.opts
}

func (svr *server) Stop() {
	svr.gsvr.Stop()
}

func (svr *server) String() string {
	return "grpc-server"
}
