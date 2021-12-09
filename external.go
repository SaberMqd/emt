package emt

import (
	"emt/broker"
	"emt/network"
	"emt/registry"
	"emt/rpc"
	"errors"
	"time"
)

const (
	DefaultRegisterTTL      = 30 * time.Second
	DefaultRegisterInterval = 20 * time.Second
)

var (
	ErrorNameIsExist      = errors.New("name is exist")
	ErrorServerIsNotExist = errors.New("server is not exist")
	ErrorInvaildAddr      = errors.New("invaild addr")
)

type App interface {
	AddRegistry(registry.Registry)
	AddServer(...network.Server) error
	AddClient(...network.Client) error
	AddClientSet(...network.Client) error
	AddRPCServer(...rpc.Server) error
	AddRPCClient(...rpc.Client) error
	AddBroker(...broker.Broker) error
	Run() error
	Transport
}

type Transport interface {
	WriteMessageToServer0(string, interface{}) error
	WriteMessageToServer1(string, string, interface{}) error
}

func NewApp() App {
	return &app{
		brokers:         make(map[string]broker.Broker),
		svrs:            make(map[string]network.Server),
		clis:            make(map[string]network.Client),
		cliCompositeSet: make(clientCompositeSet),
		rpcSvrs:         make(map[string]rpc.Server),
		rpcClis:         make(map[string]rpc.Client),
	}
}
