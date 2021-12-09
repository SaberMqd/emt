package redis

import (
	"emt/broker"
	"errors"
	"fmt"
	"time"

	"github.com/gomodule/redigo/redis"
)

var ErrorNoConn = errors.New("no redigo conn")

const (
	DefaultMaxIdle     uint32        = 10
	DefaultMaxActive   uint32        = 10
	DefaultIdleTimeout time.Duration = 1000 * time.Millisecond
)

type redisBroker struct {
	opts        broker.Options
	pool        *redis.Pool
	maxIdle     uint32
	maxActive   uint32
	idleTimeout time.Duration
}

func (b *redisBroker) String() string {
	return "redis-broker"
}

func (b *redisBroker) Connect() error {
	c := b.pool.Get()
	if c == nil {
		return ErrorNoConn
	}

	defer c.Close()

	if _, err := c.Do("PING"); err != nil {
		return fmt.Errorf("failed to ping %w", err)
	}

	return nil
}

func (b *redisBroker) Disconnect() error {
	err := b.pool.Close()
	b.pool = nil

	return fmt.Errorf("failed to disconnect %w", err)
}

func (b *redisBroker) Publish(topic string, msg *broker.Message) error {
	// v, err := b.opts.Codec.Marshal(msg)
	// if err != nil {
	// 	return err
	// }
	conn := b.pool.Get()
	defer conn.Close()

	if _, err := redis.Int(conn.Do("PUBLISH", topic, msg.Body)); err != nil {
		return fmt.Errorf("failed to publish %w", err)
	}

	return nil
}

func (b *redisBroker) Subscribe(topic string, h *broker.Handler) error {
	return nil
}

func (b *redisBroker) Options() broker.Options {
	return b.opts
}

func NewBroker(opts ...broker.Option) broker.Broker {
	b := &redisBroker{}

	for _, o := range opts {
		o(&b.opts)
	}

	b.maxIdle = DefaultMaxIdle
	b.maxActive = DefaultMaxActive
	b.idleTimeout = DefaultIdleTimeout

	/*
		if b.opts.Context != nil {
			cfg, ok := b.opts.Context.Value(redisRegistryConfigKey{}).(*redisRegistryConfig)
			if ok {
				b.maxIdle = cfg.MaxIdle
				b.maxActive = cfg.MaxActive
				b.idleTimeout = cfg.IdleTimeout
			}
		}
	*/

	b.pool = &redis.Pool{
		MaxIdle:     int(b.maxIdle),
		MaxActive:   int(b.maxActive),
		IdleTimeout: b.idleTimeout,
		Dial: func() (redis.Conn, error) {
			c, err := redis.Dial("tcp", b.opts.Addr)
			if err != nil {
				return nil, fmt.Errorf("failed to dial addr %w", err)
			}

			if b.opts.Password == "" {
				return c, nil
			}

			if _, err := c.Do("AUTH", b.opts.Password); err != nil {
				return nil, fmt.Errorf("failed to auth %w", err)
			}

			return c, nil
		},
	}

	return b
}
