package framework

import (
	"emt/log"
	"emt/network"
	"emt/util/pipeline"
	"reflect"

	"go.uber.org/zap"
)

type (
	Module interface {
		Init(Router)
	}

	Router interface {
		Register(interface{}, func([]interface{}))
		RegisterPipeline(*pipeline.Pipeline, interface{}, func([]interface{}))
		network.Handler
	}

	router struct {
		r    map[reflect.Type]interface{}
		opts Options
	}

	OnClose struct{}

	OnConnect struct{}
)

func NewRouter(opts ...Option) Router {
	r := &router{
		r: make(map[reflect.Type]interface{}),
	}

	for _, o := range opts {
		o(&r.opts)
	}

	for _, v := range r.opts.Module {
		v.Init(r)
	}

	return r
}

func (r *router) Register(m interface{}, f func([]interface{})) {
	r.r[reflect.TypeOf(m)] = f
}

func (r *router) RegisterPipeline(p *pipeline.Pipeline, m interface{}, f func([]interface{})) {
	r.r[reflect.TypeOf(m)] = func(args []interface{}) {
		if err := p.Go(args); err != nil {
			log.Warn("RouterGo", zap.String("err", err.Error()))
		}
	}

	p.RegisterGo(m, f)
}

func (r *router) Handle(a network.Agent, m interface{}) {
	if f, ok := r.r[reflect.TypeOf(m)]; ok {
		f.(func([]interface{}))([]interface{}{m, a})
	}
}

func (r *router) OnConnect(a network.Agent) {
	if f, ok := r.r[reflect.TypeOf((*OnConnect)(nil))]; ok {
		f.(func([]interface{}))([]interface{}{(*OnConnect)(nil), a})
	}
}

func (r *router) OnClose(a network.Agent) {
	if f, ok := r.r[reflect.TypeOf((*OnClose)(nil))]; ok {
		f.(func([]interface{}))([]interface{}{(*OnClose)(nil), a})
	}
}
