package broker

type Broker interface {
	Connect() error
	Disconnect() error
	Publish(topic string, m *Message) error
	Subscribe(topic string, h *Handler) error
	Options() Options
	String() string
}

type Message struct {
	Header map[string]string
	Body   []byte
}

type Handler func(Event) error

type Event interface {
	Topic() string
	Message() *Message
	Ack() error
	Error() error
}
