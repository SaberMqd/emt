package framesync

import (
	"emt/util/pipeline"
)

var (
	DefaultGiveUpFrameInterval uint32 = 20
	DefaultStartFrame          uint32 = 1
)

type Cmd struct {
	Frame     uint32
	Cmds      []byte
	TimeStamp int64
}

type Frame struct {
	Cmds      []*Cmd
	Frame     uint32
	TimeStamp int64
}

type BroadcastHandler func(*Frame)

type FrameSync interface {
	Input(*Cmd)
	HistoryFrames(uint32) []*Frame
	Start()
	Stop()
	Release()
	Init()
	Run()
	Pipline() *pipeline.Pipeline
}

func NewFrameSync(frameInterval uint32, isSaveHistory bool, h BroadcastHandler) FrameSync {
	return &frameSync{
		frame:               DefaultStartFrame,
		frameInterval:       frameInterval,
		isSaveHistory:       isSaveHistory,
		giveUpFrameInterval: DefaultGiveUpFrameInterval,
		h:                   h,
	}
}
