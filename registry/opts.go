package registry

import (
	"context"
	"time"
)

type Options struct {
	Addr     string
	Password string
	Timeout  time.Duration
	Context  context.Context
}

type RegisterOptions struct {
	TTL     time.Duration
	Context context.Context
	Domain  string
}

type DeregisterOptions struct {
	Context context.Context
	Domain  string
}

type ListOptions struct {
	Context context.Context
	Domain  string
}

type WatchOptions struct {
	Context      context.Context
	Domain       string
	Interval     time.Duration
	EventHandler func(*Event)
}

type RegisterOption func(*RegisterOptions)

type DeregisterOption func(*DeregisterOptions)

type ListOption func(*ListOptions)

type WatchOption func(*WatchOptions)

type Option func(*Options)

func OptionWithAddr(a string) Option {
	return func(o *Options) {
		o.Addr = a
	}
}

func OptionWithPassword(p string) Option {
	return func(o *Options) {
		o.Password = p
	}
}

func RegisterOptionWithTTL(t time.Duration) RegisterOption {
	return func(o *RegisterOptions) {
		o.TTL = t
	}
}

func RegisterOptionWithDomain(d string) RegisterOption {
	return func(o *RegisterOptions) {
		o.Domain = d
	}
}

func DeregisterOptionWithDomain(d string) DeregisterOption {
	return func(o *DeregisterOptions) {
		o.Domain = d
	}
}

func ListOptionWithDomain(d string) ListOption {
	return func(o *ListOptions) {
		o.Domain = d
	}
}

func WatchOptionWithDomain(d string) WatchOption {
	return func(o *WatchOptions) {
		o.Domain = d
	}
}

func WatchOptionWithInterval(t time.Duration) WatchOption {
	return func(o *WatchOptions) {
		o.Interval = t
	}
}

func WatchOptionWithEventHandler(h func(*Event)) WatchOption {
	return func(o *WatchOptions) {
		o.EventHandler = h
	}
}
