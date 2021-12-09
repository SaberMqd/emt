package rabbit

import (
	"emt/broker"
	"emt/log"
	"errors"
	"fmt"

	"github.com/streadway/amqp"
	"go.uber.org/zap"
)

var (
	ErrNotParamNull  = errors.New("ExchangeName or routeingKey not be nil")
	ErrConnectIsNull = errors.New("connection is nil")
)

type rabbitBroker struct {
	opts         broker.Options
	conn         *amqp.Connection
	channel      *amqp.Channel
	queueName    string
	routeingKey  string
	exchangeName string
	exchangeType string

	ChannelPrefetchCount  int
	ChannelPrefetchGlobal bool
	nackMultiple          bool
	nackRequeue           bool
}

type publication struct {
	d   amqp.Delivery
	m   *broker.Message
	t   string
	err error
}

func (r *rabbitBroker) Connect() error {
	conn, err := amqp.Dial(r.opts.Addr)
	if err != nil {
		return fmt.Errorf("fail to connect amqp %w", err)
	}

	r.conn = conn

	channel, err := conn.Channel()
	if err != nil {
		return fmt.Errorf("fail to connect channel %w", err)
	}

	r.channel = channel

	return nil
}

func (r *rabbitBroker) Disconnect() error {
	if r.channel != nil {
		r.channel.Close()
	}

	if r.conn != nil {
		r.conn.Close()
	}

	return nil
}

func (r *rabbitBroker) Channel() error {
	if r.exchangeName == "" || r.routeingKey == "" {
		return ErrNotParamNull
	}

	if r.exchangeName != "" {
		if r.exchangeType == "" {
			r.exchangeType = "direct"
		}

		err := r.channel.ExchangeDeclare(r.exchangeName, r.exchangeType, true, false, false, true, nil)
		if err != nil {
			return fmt.Errorf("fail to  register exchangeDeclare %w", err)
		}
	}

	_, err := r.channel.QueueDeclare(r.queueName, true, false, false, true, nil)
	if err != nil {
		return fmt.Errorf("fail to new queue  %w", err)
	}

	err = r.channel.QueueBind(r.queueName, r.routeingKey, r.exchangeName, true, nil)
	if err != nil {
		return fmt.Errorf("fail to publish bind queue %w", err)
	}

	return nil
}

func (r *rabbitBroker) Publish(topic string, msg *broker.Message) error {
	r.routeingKey = topic

	if r.conn == nil {
		return ErrConnectIsNull
	}

	if r.channel == nil {
		if err := r.Connect(); err != nil {
			return fmt.Errorf("publish reconnect %w", err)
		}
	}

	if err := r.Channel(); err != nil {
		return err
	}

	m := amqp.Publishing{
		Body:    msg.Body,
		Headers: amqp.Table{},
	}

	for k, v := range msg.Header {
		m.Headers[k] = v
	}

	return r.channel.Publish(r.exchangeName, r.routeingKey, false, false, m)
}

func (r *rabbitBroker) Subscribe(topic string, handler *broker.Handler) error {
	// p := &publication{}
	r.routeingKey = topic

	if r.channel == nil {
		if err := r.Connect(); err != nil {
			return fmt.Errorf("publish reconnect %w", err)
		}
	}

	if err := r.Channel(); err != nil {
		return err
	}

	if r.ChannelPrefetchCount == 0 {
		r.ChannelPrefetchCount = 1
		r.ChannelPrefetchGlobal = true
	}

	if err := r.channel.Qos(r.ChannelPrefetchCount, 0, r.ChannelPrefetchGlobal); err != nil {
		return fmt.Errorf("fail to channl Qos %w", err)
	}

	msgList, err := r.channel.Consume(r.queueName, "", false, false, false, false, nil)
	if err != nil {
		return fmt.Errorf("fail to channl consume  %w", err)
	}

	for msg := range msgList {
		header := make(map[string]string)
		for k, v := range msg.Headers {
			header[k], _ = v.(string)
		}

		m := &broker.Message{
			Header: header,
			Body:   msg.Body,
		}

		// p.d = msg
		// p.m = m
		// p.t = msg.RoutingKey

		h := *handler
		// p.err = h(p)

		p := &publication{
			d: msg,
			m: m,
			t: topic,
		}

		p.err = h(p)
		if p.err == nil {
			if err = msg.Ack(false); err != nil {
				log.Error("fail to msg.ack ", zap.String("data", err.Error()))
			}
		} else {
			if err = msg.Nack(r.nackMultiple, r.nackRequeue); err != nil {
				log.Error("fail to msg.nack ", zap.String("data", err.Error()))
			}
		}
	}

	return nil
}

func (r *rabbitBroker) Options() broker.Options {
	return r.opts
}

func (r *rabbitBroker) String() string {
	return "rabbit-broker"
}

func (p *publication) Topic() string {
	return p.t
}

func (p *publication) Message() *broker.Message {
	return p.m
}

func (p *publication) Ack() error {
	return p.d.Ack(false)
}

func (p *publication) Error() error {
	return p.err
}

func NewBroker(opts ...broker.Option) broker.Broker {
	b := &rabbitBroker{}

	for _, o := range opts {
		o(&b.opts)
	}

	return b
}
