package redis

import (
	"emt/registry"
	"emt/util/timer"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	redigo "github.com/gomodule/redigo/redis"
)

var (
	ErrorNoConn     = errors.New("no redigo conn")
	ErrorInvalidKey = errors.New("invalid key")
)

const (
	DefaultMaxIdle      uint32        = 10
	DefaultMaxActive    uint32        = 10
	DefaultIdleTimeout  time.Duration = 1000 * time.Millisecond
	DefaultSeparator    string        = "_"
	DefaultSeparatorNum int           = 3
	DefaultDomainIndex  int           = 0
	DefaultAddrIndex    int           = 1
	DefaultNameIndex    int           = 2
)

type redisreg struct {
	opts        registry.Options
	pool        *redigo.Pool
	maxIdle     uint32
	maxActive   uint32
	idleTimeout time.Duration

	watchers []*watcher
	sync.Mutex
}

type watcher struct {
	ticker  timer.Ticker
	service []*registry.Service
	opts    *registry.WatchOptions
}

type serviceList []*registry.Service

func (sl serviceList) Contain(s *registry.Service) bool {
	for _, v := range sl {
		if v.ID == s.ID && v.Addr == s.Addr {
			return true
		}
	}

	return false
}

func service2StringKey(domain string, s *registry.Service) (string, error) {
	if find := strings.Contains(domain, "_"); find {
		return "", ErrorInvalidKey
	}

	if find := strings.Contains(s.ID, "_"); find {
		return "", ErrorInvalidKey
	}

	if find := strings.Contains(s.Addr, "_"); find {
		return "", ErrorInvalidKey
	}

	key := domain + "_" + s.Addr + "_" + s.ID

	return key, nil
}

func stringKey2Service(key string) (*registry.Service, error) {
	strs := strings.Split(key, "_")
	if len(strs) != DefaultSeparatorNum {
		return nil, ErrorInvalidKey
	}

	s := &registry.Service{
		Addr: strs[DefaultAddrIndex],
		ID:   strs[DefaultNameIndex],
	}

	return s, nil
}

func NewRegistry(opts ...registry.Option) registry.Registry {
	reg := &redisreg{}

	for _, o := range opts {
		o(&reg.opts)
	}

	reg.maxIdle = DefaultMaxIdle
	reg.maxActive = DefaultMaxActive
	reg.idleTimeout = DefaultIdleTimeout

	if reg.opts.Context != nil {
		cfg, ok := reg.opts.Context.Value(redisRegistryConfigKey{}).(*redisRegistryConfig)
		if ok {
			reg.maxIdle = cfg.MaxIdle
			reg.maxActive = cfg.MaxActive
			reg.idleTimeout = cfg.IdleTimeout
		}
	}

	reg.pool = &redigo.Pool{
		MaxIdle:     int(reg.maxIdle),
		MaxActive:   int(reg.maxActive),
		IdleTimeout: reg.idleTimeout,
		Dial: func() (redigo.Conn, error) {
			c, err := redigo.Dial("tcp", reg.opts.Addr)
			if err != nil {
				return nil, fmt.Errorf("failed to dial addr %w", err)
			}

			if reg.opts.Password == "" {
				return c, nil
			}

			if _, err := c.Do("AUTH", reg.opts.Password); err != nil {
				return nil, fmt.Errorf("failed to auth %w", err)
			}

			return c, nil
		},
	}

	return reg
}

func (r *redisreg) Init() error {
	c := r.pool.Get()
	if c == nil {
		return ErrorNoConn
	}

	defer c.Close()

	if _, err := c.Do("PING"); err != nil {
		return fmt.Errorf("failed to ping %w", err)
	}

	return nil
}

func (r *redisreg) Register(s *registry.Service, opt ...registry.RegisterOption) error {
	var opts registry.RegisterOptions

	for _, o := range opt {
		o(&opts)
	}

	c := r.pool.Get()
	if c == nil {
		return ErrorNoConn
	}

	defer c.Close()

	key, err := service2StringKey(opts.Domain, s)
	if err != nil {
		return err
	}

	if _, err := c.Do("SET", key, "1", "EX", opts.TTL.Seconds()); err != nil {
		return fmt.Errorf("faild to setex %w", err)
	}

	return nil
}

func (r *redisreg) DeRegister(s *registry.Service, opt ...registry.DeregisterOption) error {
	var opts registry.DeregisterOptions

	for _, o := range opt {
		o(&opts)
	}

	c := r.pool.Get()
	if c == nil {
		return ErrorNoConn
	}

	defer c.Close()

	key, err := service2StringKey(opts.Domain, s)
	if err != nil {
		return err
	}

	if _, err := c.Do("DEL", key); err != nil {
		return fmt.Errorf("faild to del %w", err)
	}

	return nil
}

func (r *redisreg) ListServices(opt ...registry.ListOption) (service []*registry.Service, err error) {
	var opts registry.ListOptions

	for _, o := range opt {
		o(&opts)
	}

	c := r.pool.Get()
	if c == nil {
		return nil, ErrorNoConn
	}

	defer c.Close()

	res, err := redigo.Strings(c.Do("KEYS", opts.Domain+DefaultSeparator+"*"))
	if err != nil {
		return service, fmt.Errorf("faild to get %w", err)
	}

	for _, v := range res {
		s, err := stringKey2Service(v)
		if err != nil {
			continue
		}

		service = append(service, s)
	}

	return service, nil
}

func (r *redisreg) Watch(opt ...registry.WatchOption) error {
	var opts registry.WatchOptions

	for _, o := range opt {
		o(&opts)
	}

	svrs, err := r.ListServices(registry.ListOptionWithDomain(opts.Domain))
	if err != nil {
		return err
	}

	w := &watcher{
		opts:    &opts,
		service: svrs,
	}

	w.ticker = timer.NewTicker(opts.Interval, func() {
		currentSvrs, err := r.ListServices(registry.ListOptionWithDomain(opts.Domain))
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

	r.Lock()
	defer r.Unlock()

	r.watchers = append(r.watchers, w)

	return nil
}

func (r *redisreg) Options() registry.Options {
	return r.opts
}

func (r *redisreg) Release() error {
	r.Lock()
	defer r.Unlock()

	for _, v := range r.watchers {
		v.ticker.Stop()
	}

	return nil
}

func (r *redisreg) String() string {
	return "redis"
}
