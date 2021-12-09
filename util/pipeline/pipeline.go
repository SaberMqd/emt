package pipeline

import (
	"emt/log"
	"errors"
	"fmt"
	"reflect"

	"go.uber.org/zap"
)

var DefaultGoLen uint32 = 1

var ErrorChanFull = errors.New("channel full")

type CallMsg interface {
	Wait() interface{}
	RetCh() chan interface{}
}

type CallMsgIns struct {
	ch chan interface{}
}

func (c *CallMsgIns) Init() {
	c.ch = make(chan interface{})
}

func (c *CallMsgIns) Wait() interface{} {
	return <-c.ch
}

func (c *CallMsgIns) RetCh() chan interface{} {
	return c.ch
}

type GoFunc func([]interface{})

type CallFunc func(CallMsg) interface{}

type Pipeline struct {
	stopCh   chan bool
	isStopCh chan bool

	goCh   chan []interface{}
	callCh chan interface{}

	goFuncs   map[reflect.Type]GoFunc
	callFuncs map[reflect.Type]CallFunc

	dftGoFunc GoFunc

	isClosed bool
}

func NewDefault() *Pipeline {
	p := &Pipeline{}
	p.init(DefaultGoLen)

	return p
}

func NewPipeline(goLen uint32) *Pipeline {
	p := &Pipeline{}
	p.init(goLen)

	return p
}

func (p *Pipeline) init(goLen uint32) {
	p.stopCh = make(chan bool, 1)
	p.isStopCh = make(chan bool, 1)
	p.goCh = make(chan []interface{}, int(goLen))
	p.callCh = make(chan interface{}, 1)
	p.goFuncs = make(map[reflect.Type]GoFunc)
	p.callFuncs = make(map[reflect.Type]CallFunc)
}

func (p *Pipeline) Run() {
GoEndFor:
	for {
		select {
		case <-p.stopCh:
			break GoEndFor
		case msg := <-p.goCh:
			p.execGo(msg)
		case msg := <-p.callCh:
			p.execCall(msg)
		}
	}
	// 尽可能避免这里处理
CallEndFor:
	for {
		select {
		case msg := <-p.callCh:
			p.execCall(msg)
		case msg := <-p.goCh:
			p.execGo(msg)
		default:
			break CallEndFor
		}
	}

	p.isStopCh <- true

	close(p.goCh)
	close(p.callCh)
}

func (p *Pipeline) execGo(msg []interface{}) {
	t := reflect.TypeOf(msg[0])
	if f, ok := p.goFuncs[t]; ok {
		f(msg)
	} else {
		if p.dftGoFunc != nil {
			p.dftGoFunc(msg)
		} else {
			log.Warn("dispatchMessage", zap.String("msgtype", t.String()))
		}
	}
}

func (p *Pipeline) execCall(msg interface{}) {
	t := reflect.TypeOf(msg)
	if f, ok := p.callFuncs[t]; ok {
		m := msg.(CallMsg)
		m.RetCh() <- f(m)
	} else {
		if p.dftGoFunc != nil {
			p.dftGoFunc([]interface{}{msg})
		} else {
			log.Warn("dispatchMessage", zap.String("msgtype", t.String()))
		}
	}
}

func (p *Pipeline) RegisterGo(m interface{}, f GoFunc) {
	if _, ok := p.goFuncs[reflect.TypeOf(m)]; ok {
		panic(fmt.Sprintf("msg %v: already registered", m))
	}

	p.goFuncs[reflect.TypeOf(m)] = f
}

func (p *Pipeline) RegisterCall(m CallMsg, f CallFunc) {
	if _, ok := p.callFuncs[reflect.TypeOf(m)]; ok {
		panic(fmt.Sprintf("msg %v: already registered", m))
	}

	p.callFuncs[reflect.TypeOf(m)] = f
}

func (p *Pipeline) SetDefaultGoFunc(f GoFunc) {
	p.dftGoFunc = f
}

func (p *Pipeline) Stop() {
	if p.isClosed {
		return
	}

	p.isClosed = true

	select {
	case p.stopCh <- true:
	default:
	}

	<-p.isStopCh
	close(p.isStopCh)
}

func (p *Pipeline) Go(m []interface{}) error {
	select {
	case p.goCh <- m:
	default:
		return ErrorChanFull
	}

	return nil
}

func (p *Pipeline) Call(m CallMsg) (interface{}, error) {
	select {
	case p.callCh <- m:
	default:
		return nil, ErrorChanFull
	}

	return m.Wait(), nil
}
