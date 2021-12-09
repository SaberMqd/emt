package framework

type (
	Options struct {
		Module []Module
	}

	Option func(*Options)
)

func OptionWithModule(m Module) Option {
	return func(o *Options) {
		o.Module = append(o.Module, m)
	}
}
