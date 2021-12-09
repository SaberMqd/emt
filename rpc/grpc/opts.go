package grpc

import (
	"context"
	"emt/rpc"
)

type grpcServerConfigKey struct{}

type grpcServerConfig struct {
	Sub ServerSub
}

type grpcClientConfigKey struct{}

type grpcClientConfig struct {
	Sub ClientSub
}

func ServerOptionWithConfig(sub ServerSub) rpc.ServerOption {
	return func(o *rpc.ServerOptions) {
		if o.Context == nil {
			o.Context = context.Background()
		}

		o.Context = context.WithValue(o.Context, grpcServerConfigKey{}, &grpcServerConfig{
			Sub: sub,
		})
	}
}

func ClientOptionWithConfig(sub ClientSub) rpc.ClientOption {
	return func(o *rpc.ClientOptions) {
		if o.Context == nil {
			o.Context = context.Background()
		}

		o.Context = context.WithValue(o.Context, grpcClientConfigKey{}, &grpcClientConfig{
			Sub: sub,
		})
	}
}
