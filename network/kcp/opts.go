package kcp

import (
	"context"
	"emt/network"
)

type kcpServerConfigKey struct{}

type kcpServerConfig struct {
	DataShards   int
	ParityShards int
}

type kcpClientConfigKey struct{}

type kcpClientConfig struct {
	DataShards   int
	ParityShards int
}

func ServerOptionWithConfig(dataShards, parityShards int) network.ServerOption {
	return func(o *network.ServerOptions) {
		if o.Context == nil {
			o.Context = context.Background()
		}

		o.Context = context.WithValue(o.Context, kcpServerConfigKey{}, &kcpServerConfig{
			DataShards:   dataShards,
			ParityShards: parityShards,
		})
	}
}

func ClientOptionWithConfig(dataShards, parityShards int) network.ClientOption {
	return func(o *network.ClientOptions) {
		if o.Context == nil {
			o.Context = context.Background()
		}

		o.Context = context.WithValue(o.Context, kcpClientConfigKey{}, &kcpClientConfig{
			DataShards:   dataShards,
			ParityShards: parityShards,
		})
	}
}
