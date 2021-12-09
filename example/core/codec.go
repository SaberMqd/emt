package core

import (
	pb "emt/example/proto"
	"emt/network"
	pbcodec "emt/network/codec/protobuf"
)

var DefaultCodec network.Codec

func init() {
	processor := pbcodec.NewCodec()
	processor.Registe((*pb.Frame)(nil))
	processor.Registe((*pb.MoveUp)(nil))
	processor.Registe((*pb.MoveDown)(nil))
	processor.Registe((*pb.MoveLeft)(nil))
	processor.Registe((*pb.MoveRight)(nil))
	processor.Registe((*pb.Action)(nil))
	processor.Registe((*pb.PlayerInfo)(nil))
	processor.Registe((*pb.JoinRoom)(nil))
	processor.Registe((*pb.JoinRoomRes)(nil))
	processor.Registe((*pb.MoveDownOver)(nil))
	processor.Registe((*pb.MoveLeftOver)(nil))
	processor.Registe((*pb.MoveRightOver)(nil))
	processor.Registe((*pb.Fall)(nil))
	processor.Registe((*pb.Idle)(nil))
	processor.Registe((*pb.Die)(nil))
	processor.Registe((*pb.Bounce)(nil))
	processor.Registe((*pb.Props)(nil))
	processor.Registe((*pb.FootBoard)(nil))
	processor.Registe((*pb.Hit)(nil))
	processor.Registe((*pb.SyncGear)(nil))
	DefaultCodec = processor
}
