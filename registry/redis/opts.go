package redis

import (
	"context"
	"emt/registry"
	"time"
)

type redisRegistryConfigKey struct{}

type redisRegistryConfig struct {
	MaxIdle     uint32
	MaxActive   uint32
	IdleTimeout time.Duration
}

func OptionWithConfig(maxIdle, maxActive uint32, idleTimeout time.Duration) registry.Option {
	return func(o *registry.Options) {
		if o.Context == nil {
			o.Context = context.Background()
		}

		o.Context = context.WithValue(o.Context, redisRegistryConfigKey{}, &redisRegistryConfig{
			MaxIdle:     maxIdle,
			MaxActive:   maxActive,
			IdleTimeout: idleTimeout,
		})
	}
}
