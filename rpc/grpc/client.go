package grpc

import (
	rpc "emt/rpc"
	"fmt"

	"google.golang.org/grpc"
)

type client struct {
	opts       rpc.ClientOptions
	conn       *grpc.ClientConn
	sub        ClientSub
	grpcClient interface{}
}

func NewClient(opts ...rpc.ClientOption) rpc.Client {
	cli := &client{}

	for _, o := range opts {
		o(&cli.opts)
	}

	if cli.opts.MaxReconnectNum == 0 {
		cli.opts.MaxReconnectNum = rpc.DefaultMaxReconnectNum
	}

	if cli.opts.ReconnectInterval == 0 {
		cli.opts.ReconnectInterval = rpc.DefualtReconnectInterval
	}

	if cli.opts.MaxMsgLen == 0 {
		cli.opts.MaxMsgLen = rpc.DefaultMaxMsgLen
	}

	if cli.opts.Context != nil {
		cfg, ok := cli.opts.Context.Value(grpcClientConfigKey{}).(*grpcClientConfig)
		if ok {
			cli.sub = cfg.Sub
		}
	}

	if cli.sub == nil {
		panic("grpc server sub is nil")
	}

	return cli
}

func (cli *client) Options() rpc.ClientOptions {
	return cli.opts
}

func (cli *client) SetOption(opts ...rpc.ClientOption) {
	for _, o := range opts {
		o(&cli.opts)
	}
}

func (cli *client) SetOptions(opts rpc.ClientOptions) {
	cli.opts = opts
}

func (cli *client) Start() error {
	var err error
	cli.conn, err = grpc.Dial(cli.opts.Addr, grpc.WithInsecure())

	if err != nil {
		cli.conn.Close()

		return fmt.Errorf("grpc connect %w", err)
	}

	cli.grpcClient = cli.sub.OnConnected(cli.conn)

	return nil
}

func (cli *client) Stop() {
	if cli.conn != nil {
		cli.conn.Close()
	}
}

func (cli *client) String() string {
	return "grpc-client"
}

func (cli *client) Client() interface{} {
	return cli.grpcClient
}
