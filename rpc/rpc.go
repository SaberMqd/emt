package rpc

type Server interface {
	Start() error
	Stop()
	String() string
	Options() ServerOptions
}

type Client interface {
	Start() error
	Stop()
	String() string
	Options() ClientOptions
	SetOption(...ClientOption)
	SetOptions(ClientOptions)
	Client() interface{}
}
