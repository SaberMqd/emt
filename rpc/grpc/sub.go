package grpc

type ClientSub interface {
	OnConnected(interface{}) interface{}
}

type ServerSub interface {
	OnListened(interface{})
}
