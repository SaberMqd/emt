package kafka

import (
	"emt/broker"
	"emt/log"
	"fmt"
	"strings"
	"time"

	"github.com/Shopify/sarama"
	cluster "github.com/bsm/sarama-cluster"
	"go.uber.org/zap"
)

type publication struct {
	m   *broker.Message
	t   string
	err error
}

type kafkaBroker struct {
	opts broker.Options
	p    sarama.SyncProducer

	// 改成options
	groupID string
	timeout time.Duration
}

func (s *kafkaBroker) Connect() error {
	config := sarama.NewConfig()
	config.Producer.Return.Successes = true
	config.Producer.Timeout = s.timeout

	p, err := sarama.NewSyncProducer(strings.Split(s.opts.Addr, ","), config)
	if err != nil {
		return fmt.Errorf("fail to connect amqp %w", err)
	}

	s.p = p

	return nil
}

func (s *kafkaBroker) Disconnect() error {
	if s.p != nil {
		s.p.Close()
	}

	return nil
}

func (s *kafkaBroker) Publish(topic string, m *broker.Message) error {
	msg := &sarama.ProducerMessage{
		Topic: topic,
		Value: sarama.ByteEncoder(m.Body),
	}

	_, _, err := s.p.SendMessage(msg)
	if err != nil {
		return fmt.Errorf("fail to publish %w", err)
	}

	return nil
}

// 异步方法
/*
		go func(p sarama.AsyncProducer) {
			errors := p.Errors()
			success := p.Successes()
			for {
				select {
				case err := <-errors:
					if err != nil {
						fmt.Println(" 98 err = ",err)
					}
				case <-success:
					fmt.Println(" 发送成功")
				}
			}
		}(p)

		for {
			v := "async: " + strconv.Itoa(rand.New(rand.NewSource(time.Now().UnixNano())).Intn(10000))
			//	fmt.Fprintln(os.Stdout, v)
			msg := &sarama.ProducerMessage{
				Topic: topics,
				Value: sarama.ByteEncoder(v),
			}
			p.Input() <- msg
			time.Sleep(time.Second * 1)
		}

}

*/

func (s *kafkaBroker) Subscribe(topic string, h *broker.Handler) error {
	config := cluster.NewConfig()
	config.Group.Return.Notifications = true
	config.Consumer.Offsets.CommitInterval = 1 * time.Second
	config.Consumer.Offsets.Initial = sarama.OffsetNewest

	addr := make([]string, 1)
	addr[0] = s.opts.Addr

	c, err := cluster.NewConsumer(addr, s.groupID, strings.Split(topic, ","), config)
	if err != nil {
		return fmt.Errorf("fail to subscribe %w", err)
	}

	defer c.Close()

	go func(c *cluster.Consumer) {
		errors := c.Errors()
		noti := c.Notifications()

		for {
			select {
			case err := <-errors:
				log.Error("kafka.subscribe",
					zap.String("data", err.Error()))

			case <-noti:
			}
		}
	}(c)

	for msg := range c.Messages() {
		m := &broker.Message{

			Body: msg.Value,
		}

		push := &publication{
			m: m,
			t: topic,
		}

		hand := *h
		push.err = hand(push)

		c.MarkOffset(msg, "") // MarkOffset 并不是实时写入kafka，有可能在程序crash时丢掉未提交的offset
	}

	return nil
}

func (s *kafkaBroker) Options() broker.Options {
	return s.opts
}

func (s *kafkaBroker) String() string {
	return "kafka-broker"
}

func (p *publication) Topic() string {
	return p.t
}

func (p *publication) Message() *broker.Message {
	return p.m
}

func (p *publication) Ack() error {
	return nil
}

func (p *publication) Error() error {
	return p.err
}

func NewBroker(opts ...broker.Option) broker.Broker {
	b := &kafkaBroker{}

	for _, o := range opts {
		o(&b.opts)
	}

	return b
}
