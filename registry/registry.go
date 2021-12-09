package registry

const (
	WildcardDomain = "*"
	DefaultDomain  = "emt"
)

type Registry interface {
	Init() error
	Register(*Service, ...RegisterOption) error
	DeRegister(*Service, ...DeregisterOption) error
	ListServices(...ListOption) ([]*Service, error)
	Watch(...WatchOption) error
	Options() Options
	Release() error
	String() string
}

type Service struct {
	ID        string
	Addr      string
	Version   string
	Endpoints []*Endpoint `json:"endpoints"`
}

type Result struct {
	Action  string
	Service *Service
}

type Endpoint struct {
	Name     string            `json:"name"`
	Request  *Value            `json:"request"`
	Response *Value            `json:"response"`
	Metadata map[string]string `json:"metadata"`
}

type Value struct {
	Name   string   `json:"name"`
	Type   string   `json:"type"`
	Values []*Value `json:"values"`
}

type EventType int

const (
	Create EventType = iota
	Delete
	Update
)

type Event struct {
	Type    EventType
	Service *Service
}
