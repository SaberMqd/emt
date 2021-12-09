package zookeeper

import (
	"emt/log"
	"emt/registry"
	"emt/util/timer"
	"errors"
	"fmt"
	"sync"
	"time"

	hash "github.com/mitchellh/hashstructure/v2"
	"github.com/samuel/go-zookeeper/zk"
)

var ErrorNoNode = errors.New("require at least one node")

const (
	DefaultProjectName string        = "/emt-registry"
	DefaultTimeout     time.Duration = 100
	DefaultFormat                    = hash.FormatV2
)

type zookeeperRegistry struct {
	client  *zk.Conn
	options registry.Options
	sync.Mutex

	watchers []*watcher
	register map[string]uint64
}

type watcher struct {
	ticker  timer.Ticker
	service []*registry.Service
	opts    *registry.WatchOptions
}

type serviceList []*registry.Service

func NewRegistry(opts ...registry.Option) registry.Registry {
	var options registry.Options

	for _, o := range opts {
		o(&options)
	}

	if options.Timeout == 0 {
		options.Timeout = DefaultTimeout
	}

	c, _, err := zk.Connect([]string{options.Addr}, time.Second*options.Timeout)
	if err != nil {
		log.Error(err.Error())

		return nil
	}

	if err := createPath(DefaultProjectName, []byte{}, c); err != nil {
		log.Error(err.Error())

		return nil
	}

	return &zookeeperRegistry{
		client:   c,
		options:  options,
		register: make(map[string]uint64),
	}
}

func (sl serviceList) Contain(s *registry.Service) bool {
	for _, v := range sl {
		if v.ID == s.ID && v.Addr == s.Addr {
			return true
		}
	}

	return false
}

func (z *zookeeperRegistry) Init() error {
	return nil
}

func (z *zookeeperRegistry) Options() registry.Options {
	return z.options
}

func (z *zookeeperRegistry) String() string {
	return "zookeeper"
}

func (z *zookeeperRegistry) Register(s *registry.Service, opt ...registry.RegisterOption) error {
	var opts registry.RegisterOptions

	for _, o := range opt {
		o(&opts)
	}

	h, err := hash.Hash(s, DefaultFormat, nil)
	if err != nil {
		return fmt.Errorf("failed to hash %w", err)
	}

	z.Lock()
	v, ok := z.register[s.ID]
	z.Unlock()

	service := &registry.Service{
		ID:        s.ID,
		Addr:      s.Addr,
		Endpoints: s.Endpoints,
	}

	if ok && v == h {
		return nil
	}

	exists, _, err := z.client.Exists(nodePath(opts.Domain, s.ID))
	if err != nil {
		return fmt.Errorf("failed to find client exist %w", err)
	}

	srv, err := encode(service)
	if err != nil {
		return err
	}

	if exists {
		_, err := z.client.Set(nodePath(opts.Domain, s.ID), srv, -1)
		if err != nil {
			return fmt.Errorf("failed to set client %w", err)
		}
	} else {
		err := createPath(nodePath(opts.Domain, s.ID), srv, z.client)
		if err != nil {
			return fmt.Errorf("failed to createpath %w", err)
		}
	}

	z.Lock()
	z.register[s.ID] = h
	z.Unlock()

	return nil
}

func (z *zookeeperRegistry) DeRegister(s *registry.Service, opt ...registry.DeregisterOption) error {
	var opts registry.DeregisterOptions

	for _, o := range opt {
		o(&opts)
	}

	if s.Addr == "" {
		return ErrorNoNode
	}

	z.Lock()
	delete(z.register, s.ID)
	z.Unlock()

	err := z.client.Delete(nodePath(opts.Domain, s.ID), -1)
	if err != nil {
		return fmt.Errorf("failed to delete client %w", err)
	}

	return nil
}

func (z *zookeeperRegistry) ListServices(opt ...registry.ListOption) ([]*registry.Service, error) {
	var opts registry.ListOptions

	for _, o := range opt {
		o(&opts)
	}

	// opt 加参数 name
	srv, _, err := z.client.Children(DefaultProjectName)
	if err != nil {
		return nil, fmt.Errorf("failed to find client children %w", err)
	}

	serviceMap := make(map[string]*registry.Service)

	for _, key := range srv {
		vagueKey := vaguePath(key)

		if vagueKey != opts.Domain {
			continue
		}

		_, stat, err := z.client.Children(nodePath(key, ""))
		if err != nil {
			return nil, fmt.Errorf("failed to delete client %w", err)
		}

		if stat.NumChildren == 0 {
			b, _, err := z.client.Get(nodePath(key, ""))
			if err != nil {
				return nil, fmt.Errorf("failed to get client %w", err)
			}

			i, err := decode(b)
			if err != nil {
				return nil, fmt.Errorf("failed to decode %w", err)
			}

			serviceMap[key] = &registry.Service{ID: i.ID, Addr: i.Addr}
		}
	}

	res := []*registry.Service{}

	for _, service := range serviceMap {
		res = append(res, service)
	}

	return res, nil
}

func (z *zookeeperRegistry) Watch(opt ...registry.WatchOption) error {
	var opts registry.WatchOptions

	for _, o := range opt {
		o(&opts)
	}

	svrs, err := z.ListServices(registry.ListOptionWithDomain(opts.Domain))
	if err != nil {
		return err
	}

	w := &watcher{
		opts:    &opts,
		service: svrs,
	}

	w.ticker = timer.NewTicker(opts.Interval, func() {
		currentSvrs, err := z.ListServices(registry.ListOptionWithDomain(opts.Domain))
		if err != nil {
			return
		}

		for _, v := range currentSvrs {
			if !serviceList(w.service).Contain(v) {
				w.opts.EventHandler(&registry.Event{
					Type:    registry.Create,
					Service: v,
				})
			}
		}

		for _, v := range w.service {
			if !serviceList(currentSvrs).Contain(v) {
				w.opts.EventHandler(&registry.Event{
					Type:    registry.Delete,
					Service: v,
				})
			}
		}

		w.service = currentSvrs
	})

	z.Lock()
	defer z.Unlock()

	z.watchers = append(z.watchers, w)

	return nil
}

func (z *zookeeperRegistry) Release() error {
	z.Lock()
	defer z.Unlock()

	for _, v := range z.watchers {
		v.ticker.Stop()
	}

	return nil
}
