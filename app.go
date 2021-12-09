package emt

import (
	"emt/broker"
	"emt/network"
	"emt/registry"
	"emt/rpc"
	"emt/util/addr"
	"emt/util/timer"
	"fmt"
	"net"
	"os"
	"os/signal"
	"sync"
	"syscall"
)

type (
	clientSet struct {
		clis     map[string]network.Client
		template network.Client
	}

	clientCompositeSet map[string]*clientSet

	app struct {
		registry        registry.Registry
		brokers         map[string]broker.Broker
		clis            map[string]network.Client
		svrs            map[string]network.Server
		rpcClis         map[string]rpc.Client
		rpcSvrs         map[string]rpc.Server
		cliCompositeSet clientCompositeSet

		regTicker timer.Ticker

		cliMtx    sync.RWMutex
		cliSetMtx sync.RWMutex
	}
)

func newClientSet(c network.Client) *clientSet {
	return &clientSet{
		template: c,
		clis:     make(map[string]network.Client),
	}
}

func (app *app) AddRegistry(r registry.Registry) {
	app.registry = r
}

func (app *app) AddServer(svrs ...network.Server) error {
	for _, v := range svrs {
		if _, ok := app.svrs[v.Options().Name]; ok {
			return ErrorNameIsExist
		}

		app.svrs[v.Options().Name] = v
	}

	return nil
}

func (app *app) AddClient(clis ...network.Client) error {
	for _, v := range clis {
		if _, ok := app.clis[v.Options().Name]; ok {
			return ErrorNameIsExist
		}

		app.clis[v.Options().Name] = v
	}

	return nil
}

func (app *app) AddClientSet(clis ...network.Client) error {
	for _, v := range clis {
		if _, ok := app.cliCompositeSet[v.Options().Name]; ok {
			return ErrorNameIsExist
		}

		app.cliCompositeSet[v.Options().Name] = newClientSet(v)
	}

	return nil
}

func (app *app) AddRPCServer(svrs ...rpc.Server) error {
	for _, v := range svrs {
		if _, ok := app.rpcSvrs[v.Options().Name]; ok {
			return ErrorNameIsExist
		}

		app.rpcSvrs[v.Options().Name] = v
	}

	return nil
}

func (app *app) AddRPCClient(clis ...rpc.Client) error {
	for _, v := range clis {
		if _, ok := app.rpcClis[v.Options().Name]; ok {
			return ErrorNameIsExist
		}

		app.rpcClis[v.Options().Name] = v
	}

	return nil
}

func (app *app) AddBroker(brokers ...broker.Broker) error {
	for _, v := range brokers {
		if _, ok := app.brokers[v.Options().Name]; ok {
			return ErrorNameIsExist
		}

		app.brokers[v.Options().Name] = v
	}

	return nil
}

func (app *app) Run() error {
	if app.registry != nil {
		if err := app.registry.Init(); err != nil {
			return fmt.Errorf("failed to init registry %w", err)
		}

		if err := app.register(); err != nil {
			return err
		}
	}

	for k := range app.svrs {
		go func(key string) {
			if err := app.svrs[key].Start(); err != nil {
				panic("failed to start server")
			}
		}(k)
	}

	for k := range app.rpcSvrs {
		go func(key string) {
			if err := app.rpcSvrs[key].Start(); err != nil {
				panic("failed to start server")
			}
		}(k)
	}

	if err := app.doClientSetStart(); err != nil {
		return err
	}

	if err := app.doClientStart(); err != nil {
		return err
	}

	if err := app.doRPCClientStart(); err != nil {
		return err
	}

	app.regTicker = timer.NewTicker(DefaultRegisterInterval, func() {
		err := app.register()
		if err != nil {
		}
	})

	return app.doClose()
}

func (app *app) doClientSetStart() error {
	for name, set := range app.cliCompositeSet {
		svrs, err := app.registry.ListServices(registry.ListOptionWithDomain(name))
		if err != nil {
			return fmt.Errorf("failed to list services %w", err)
		}

		if len(svrs) == 0 {
			return ErrorServerIsNotExist
		}

		for _, v := range svrs {
			if _, ok := set.clis[v.ID]; !ok {
				set.clis[v.ID].SetOptions(set.template.Options())
				set.clis[v.ID].SetOption(network.ClientOptionWithAddr(v.Addr))
			}

			go func(name, id string) {
				if err := app.cliCompositeSet[name].clis[id].Start(); err != nil {
					panic("failed to start client")
				}
			}(name, v.ID)
		}

		if err := app.registry.Watch(
			registry.WatchOptionWithDomain(name),
			registry.WatchOptionWithInterval(DefaultRegisterInterval),
			registry.WatchOptionWithEventHandler(func(e *registry.Event) {
				switch e.Type {
				case registry.Create:
				case registry.Delete:
				case registry.Update:
				}
			})); err != nil {
			return fmt.Errorf("failed to watch server %w", err)
		}
	}

	return nil
}

func (app *app) doClientStart() error {
	for k, v := range app.clis {
		if v.Options().Addr != "" {
			go func(name string) {
				if err := app.clis[name].Start(); err != nil {
					panic("failed to start client")
				}
			}(k)

			continue
		}

		svrs, err := app.registry.ListServices(registry.ListOptionWithDomain(k))
		if err != nil {
			return fmt.Errorf("failed to list services %w", err)
		}

		if len(svrs) == 0 {
			return ErrorServerIsNotExist
		}

		go func(name, addr string) {
			app.clis[name].SetOption(network.ClientOptionWithAddr(addr))

			if err := app.clis[name].Start(); err != nil {
				panic("failed to start client")
			}
		}(k, svrs[0].Addr)

		if err := app.registry.Watch(
			registry.WatchOptionWithDomain(k),
			registry.WatchOptionWithInterval(DefaultRegisterInterval),
			registry.WatchOptionWithEventHandler(func(e *registry.Event) {
				switch e.Type {
				case registry.Create:
				case registry.Delete:
				case registry.Update:
				}
			})); err != nil {
			return fmt.Errorf("failed to watch server %w", err)
		}
	}

	return nil
}

func (app *app) doRPCClientStart() error {
	for k, v := range app.rpcClis {
		if v.Options().Addr != "" {
			go func(name string) {
				if err := app.clis[name].Start(); err != nil {
					panic("failed to start client")
				}
			}(k)

			continue
		}

		svrs, err := app.registry.ListServices(registry.ListOptionWithDomain(k))
		if err != nil {
			return fmt.Errorf("failed to list services %w", err)
		}

		if len(svrs) == 0 {
			return ErrorServerIsNotExist
		}

		go func(name, addr string) {
			app.rpcClis[name].SetOption(rpc.ClientOptionWithAddr(addr))

			if err := app.rpcClis[name].Start(); err != nil {
				panic("failed to start client")
			}
		}(k, svrs[0].Addr)

		if err := app.registry.Watch(
			registry.WatchOptionWithDomain(k),
			registry.WatchOptionWithInterval(DefaultRegisterInterval),
			registry.WatchOptionWithEventHandler(func(e *registry.Event) {
				switch e.Type {
				case registry.Create:
				case registry.Delete:
				case registry.Update:
				}
			})); err != nil {
			return fmt.Errorf("failed to watch server %w", err)
		}
	}

	return nil
}

func (app *app) doDeregisterRPCServer() error {
	for _, v := range app.rpcSvrs {
		host, port, err := net.SplitHostPort(v.Options().Addr)
		if err != nil {
			return fmt.Errorf("failed to split hostport %w", err)
		}

		addr, err := addr.Extract(host)
		if err != nil {
			return fmt.Errorf("failed to get addr %w", err)
		}

		addr += ":" + port

		if err := app.registry.DeRegister(
			&registry.Service{
				ID:   v.Options().ID,
				Addr: addr,
			},
			registry.DeregisterOptionWithDomain(v.Options().Name)); err != nil {
		}
	}

	return nil
}

func (app *app) doDeregisterServer() error {
	for _, v := range app.svrs {
		host, port, err := net.SplitHostPort(v.Options().Addr)
		if err != nil {
			return fmt.Errorf("failed to split hostport %w", err)
		}

		addr, err := addr.Extract(host)
		if err != nil {
			return fmt.Errorf("failed to get addr %w", err)
		}

		addr += ":" + port

		if err := app.registry.DeRegister(
			&registry.Service{
				ID:   v.Options().ID,
				Addr: addr,
			},
			registry.DeregisterOptionWithDomain(v.Options().Name)); err != nil {
		}
	}

	return nil
}

func (app *app) doClose() error {
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, os.Interrupt, syscall.SIGTERM)
	<-ch

	app.regTicker.Stop()

	if err := app.doDeregisterServer(); err != nil {
		return err
	}

	if err := app.doDeregisterRPCServer(); err != nil {
		return err
	}

	if err := app.registry.Release(); err != nil {
	}

	for _, v := range app.clis {
		v.Stop()
	}

	for _, v := range app.cliCompositeSet {
		for _, v1 := range v.clis {
			v1.Stop()
		}
	}

	for _, v := range app.svrs {
		v.Stop()
	}

	for _, v := range app.rpcSvrs {
		v.Stop()
	}

	return nil
}

func (app *app) doRegisterServer() error {
	for _, v := range app.svrs {
		host, port, err := net.SplitHostPort(v.Options().Addr)
		if err != nil {
			return fmt.Errorf("failed to split hostport %w", err)
		}

		addr, err := addr.Extract(host)
		if err != nil {
			return fmt.Errorf("failed to get addr %w", err)
		}

		addr += ":" + port

		if err := app.registry.Register(
			&registry.Service{
				ID:   v.Options().ID,
				Addr: addr,
			},
			registry.RegisterOptionWithTTL(DefaultRegisterTTL),
			registry.RegisterOptionWithDomain(v.Options().Name)); err != nil {
			return fmt.Errorf("failed to register %w", err)
		}
	}

	return nil
}

func (app *app) doRegisterRPCServer() error {
	for _, v := range app.rpcSvrs {
		host, port, err := net.SplitHostPort(v.Options().Addr)
		if err != nil {
			return fmt.Errorf("failed to split hostport %w", err)
		}

		addr, err := addr.Extract(host)
		if err != nil {
			return fmt.Errorf("failed to get addr %w", err)
		}

		addr += ":" + port

		if err := app.registry.Register(
			&registry.Service{
				ID:   v.Options().ID,
				Addr: addr,
			},
			registry.RegisterOptionWithTTL(DefaultRegisterTTL),
			registry.RegisterOptionWithDomain(v.Options().Name)); err != nil {
			return fmt.Errorf("failed to register %w", err)
		}
	}

	return nil
}

func (app *app) register() error {
	if err := app.doRegisterServer(); err != nil {
		return err
	}

	if err := app.doRegisterRPCServer(); err != nil {
		return err
	}

	return nil
}

func (app *app) WriteMessageToServer0(name string, msg interface{}) error {
	app.cliMtx.RLock()
	defer app.cliMtx.RUnlock()

	if cli, ok := app.clis[name]; ok {
		return cli.WriteMessage(msg)
	}

	return ErrorServerIsNotExist
}

func (app *app) WriteMessageToServer1(name, id string, msg interface{}) error {
	app.cliSetMtx.RLock()
	defer app.cliSetMtx.RUnlock()

	if set, ok := app.cliCompositeSet[name]; ok {
		if cli, ok := set.clis[id]; ok {
			return cli.WriteMessage(msg)
		}
	}

	return ErrorServerIsNotExist
}
