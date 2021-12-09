package framesync

import (
	"emt/log"
	"emt/util/pipeline"
	"emt/util/timer"
	"sync"
	"time"

	"go.uber.org/zap"
)

type frameSync struct {
	frame               uint32
	frameInterval       uint32
	giveUpFrameInterval uint32
	isSaveHistory       bool
	running             bool
	t                   timer.Ticker
	history             []*Frame
	h                   BroadcastHandler
	f                   *Frame
	pip                 *pipeline.Pipeline
	sync.Mutex
}

type updateMsg struct{}

func (f *frameSync) Init() {
	if f.pip == nil {
		f.pip = pipeline.NewDefault()
		f.pip.RegisterGo(&updateMsg{}, f.update)
		f.pip.RegisterGo(&Cmd{}, f.input)
	}
}

func (f *frameSync) Run() {
	f.pip.Run()
}

func (f *frameSync) Pipline() *pipeline.Pipeline {
	return f.pip
}

func (f *frameSync) Start() {
	if f.running {
		return
	}

	f.running = true
	f.frame = 1
	f.f = &Frame{}
	f.history = nil

	f.t = timer.NewTicker(time.Duration(f.frameInterval)*time.Millisecond, func() {
		if err := f.pip.Go([]interface{}{&updateMsg{}}); err != nil {
			log.Warn("FrameUpdate", zap.String("err", err.Error()))
		}
	})
}

func (f *frameSync) update([]interface{}) {
	if !f.running {
		return
	}

	f.f.Frame = f.frame
	f.f.TimeStamp = time.Now().UnixNano() / int64((time.Millisecond / time.Nanosecond))

	if f.h != nil {
		f.h(f.f)
	}

	if f.isSaveHistory {
		f.Lock()
		f.history = append(f.history, &Frame{
			Cmds:      f.f.Cmds,
			Frame:     f.f.Frame,
			TimeStamp: f.f.TimeStamp,
		})
	}

	f.Unlock()
	f.frame++
	f.f = &Frame{}
}

func (f *frameSync) input(param []interface{}) {
	in := param[0].(*Cmd)
	if in == nil {
		return
	}

	if in.Frame > f.frame {
		return
	}

	if f.frame-in.Frame > f.giveUpFrameInterval {
		return
	}

	f.f.Cmds = append(f.f.Cmds, &Cmd{
		Frame:     in.Frame,
		Cmds:      in.Cmds,
		TimeStamp: in.TimeStamp,
	})
}

func (f *frameSync) Input(in *Cmd) {
	if err := f.pip.Go([]interface{}{in}); err != nil {
		log.Warn("FrameInput", zap.String("err", err.Error()))
	}
}

func (f *frameSync) HistoryFrames(frame uint32) []*Frame {
	f.Lock()
	defer f.Unlock()

	if f.history == nil || frame == 0 {
		return nil
	}

	l := len(f.history)
	if l == 0 || l < int(frame) {
		return nil
	}

	res := f.history[frame-1 : l-1]

	return res
}

func (f *frameSync) Release() {
	f.Stop()

	if f.pip == nil {
		return
	}

	f.pip.Stop()
}

func (f *frameSync) Stop() {
	if !f.running {
		return
	}

	f.running = false
	f.t.Stop()
	f.t = nil
}
