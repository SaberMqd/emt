package broker

type Options struct {
	Name     string
	Addr     string
	Password string
}

type Option func(*Options)
