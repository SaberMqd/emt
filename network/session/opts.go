package session

import (
	"emt/network"
)

type Options struct {
	Conn    network.Conn
	Handler network.Handler
	Codec   network.Codec
	Crypt   network.Crypt
}

type Option func(*Options)

func OptionWithConn(c network.Conn) Option {
	return func(o *Options) {
		o.Conn = c
	}
}

func OptionWithHandler(h network.Handler) Option {
	return func(o *Options) {
		o.Handler = h
	}
}

func OptionWithCodec(c network.Codec) Option {
	return func(o *Options) {
		o.Codec = c
	}
}

func OptionWithCrypt(c network.Crypt) Option {
	return func(o *Options) {
		o.Crypt = c
	}
}
