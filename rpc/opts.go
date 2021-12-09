package rpc

import (
	"context"
	"time"
)

const (
	DefaultMaxConnNum uint32 = 3000
	DefaultMaxMsgLen  uint32 = 1400

	DefaultMaxReconnectNum   uint32        = 3
	DefualtReconnectInterval time.Duration = 5 * time.Second
)

type Options struct {
	Addr string
	Name string

	MaxMsgLen uint32

	Context context.Context
}

type ClientOptions struct {
	Options
	MaxReconnectNum   uint32
	ReconnectInterval time.Duration
}

type ServerOptions struct {
	Options
	MaxConnNum uint32
	ID         string
}

type ClientOption func(*ClientOptions)

type ServerOption func(*ServerOptions)

func ServerOptionWithAddr(a string) ServerOption {
	return func(o *ServerOptions) {
		o.Addr = a
	}
}

func ServerOptionWithName(n string) ServerOption {
	return func(o *ServerOptions) {
		o.Name = n
	}
}

func ServerOptionWithMaxMsgLen(n uint32) ServerOption {
	return func(o *ServerOptions) {
		o.MaxMsgLen = n
	}
}

func ClientOptionWithAddr(a string) ClientOption {
	return func(o *ClientOptions) {
		o.Addr = a
	}
}

func ClientOptionWithName(n string) ClientOption {
	return func(o *ClientOptions) {
		o.Name = n
	}
}

func ClientOptionWithMaxReconnectNum(n uint32) ClientOption {
	return func(o *ClientOptions) {
		o.MaxReconnectNum = n
	}
}

func ClientOptionWithMaxMsgLen(n uint32) ClientOption {
	return func(o *ClientOptions) {
		o.MaxMsgLen = n
	}
}

func ClientOptionWithReconnectInterval(t time.Duration) ClientOption {
	return func(o *ClientOptions) {
		o.ReconnectInterval = t
	}
}
