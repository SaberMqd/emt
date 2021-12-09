package ws

import (
	"context"
	"emt/network"
	"time"
)

type wsServerConfigKey struct{}

type wsServerConfig struct {
	HTTPTimeout        time.Duration
	HTTPMaxHeaderBytes uint32
}

type wsClientConfigKey struct{}

type wsClientConfig struct {
	HTTPTimeout time.Duration
}

func ServerOptionWithConfig(httpTimeout time.Duration, httpMaxHeaderBytes uint32) network.ServerOption {
	return func(o *network.ServerOptions) {
		if o.Context == nil {
			o.Context = context.Background()
		}

		o.Context = context.WithValue(o.Context, wsServerConfigKey{}, &wsServerConfig{
			HTTPTimeout:        httpTimeout,
			HTTPMaxHeaderBytes: httpMaxHeaderBytes,
		})
	}
}

func ClientOptionWithConfig(httpTimeout time.Duration) network.ClientOption {
	return func(o *network.ClientOptions) {
		if o.Context == nil {
			o.Context = context.Background()
		}

		o.Context = context.WithValue(o.Context, wsClientConfigKey{}, &wsClientConfig{
			HTTPTimeout: httpTimeout,
		})
	}
}
