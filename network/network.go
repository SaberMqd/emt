package network

import (
	"net"
)

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
	WriteMessage(interface{}) error
	Options() ClientOptions
	SetOption(...ClientOption)
	SetOptions(ClientOptions)
}

type Handler interface {
	Handle(Agent, interface{})
	OnConnect(Agent)
	OnClose(Agent)
}

type Agent interface {
	WriteMessage(interface{}) error
	GetData(string) interface{}
	SetData(string, interface{})
	RemoteAddr() net.Addr
	LocalAddr() net.Addr
}

type Codec interface {
	Marshal(interface{}) ([]byte, error)
	Unmarshal([]byte) (interface{}, error)
	String() string
}

type Conn interface {
	ReadMessage() ([]byte, error)
	WriteMessage([]byte) error
	RemoteAddr() net.Addr
	LocalAddr() net.Addr
	String() string
}

type Crypt interface {
	Encrypt([]byte) ([]byte, error)
	Decrypt([]byte) ([]byte, error)
}
