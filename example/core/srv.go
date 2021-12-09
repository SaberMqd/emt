package core

import (
	game "emt/example/core/game"
	pb "emt/example/proto"
	"emt/framework"
	"emt/network"
	"emt/network/tcp"
	"emt/util/pipeline"
)

var DefaultSrv network.Server

func init() {
	router := framework.NewRouter()

	g := &game.Game{
		Pipeline: pipeline.NewDefault(),
	}

	router.RegisterPipeline(g.Pipeline, (*framework.OnClose)(nil), g.Offline)
	router.RegisterPipeline(g.Pipeline, (*framework.OnConnect)(nil), g.Login)
	router.RegisterPipeline(g.Pipeline, (*pb.Frame)(nil), g.OnFrame)
	router.RegisterPipeline(g.Pipeline, (*pb.MoveDown)(nil), g.OnDown)
	router.RegisterPipeline(g.Pipeline, (*pb.MoveUp)(nil), g.OnUp)
	router.RegisterPipeline(g.Pipeline, (*pb.MoveLeft)(nil), g.OnLeft)
	router.RegisterPipeline(g.Pipeline, (*pb.MoveRight)(nil), g.OnRight)
	router.RegisterPipeline(g.Pipeline, (*pb.Action)(nil), g.OnAction)
	router.RegisterPipeline(g.Pipeline, (*pb.PlayerInfo)(nil), g.OnPlayerInfo)
	router.RegisterPipeline(g.Pipeline, (*pb.JoinRoom)(nil), g.OnJoinRoom)
	router.RegisterPipeline(g.Pipeline, (*pb.MoveDownOver)(nil), g.OnDownOver)
	router.RegisterPipeline(g.Pipeline, (*pb.MoveLeftOver)(nil), g.OnLeftOver)
	router.RegisterPipeline(g.Pipeline, (*pb.MoveRightOver)(nil), g.OnRightOver)
	router.RegisterPipeline(g.Pipeline, (*pb.Fall)(nil), g.OnFall)
	router.RegisterPipeline(g.Pipeline, (*pb.Die)(nil), g.OnDie)
	router.RegisterPipeline(g.Pipeline, (*pb.Idle)(nil), g.OnIdel)
	router.RegisterPipeline(g.Pipeline, (*pb.Bounce)(nil), g.OnBounce)
	router.RegisterPipeline(g.Pipeline, (*pb.Props)(nil), g.OnProps)
	router.RegisterPipeline(g.Pipeline, (*pb.FootBoard)(nil), g.OnFootBoard)
	router.RegisterPipeline(g.Pipeline, (*pb.Hit)(nil), g.OnHit)
	router.RegisterPipeline(g.Pipeline, (*pb.SyncGear)(nil), g.OnSyncGear)
	g.Init()

	DefaultSrv = tcp.NewServer(
		network.ServerOptionWithAddr(ListenAddr),
		network.ServerOptionWithName(ServerName),
		network.ServerOptionWithCodec(DefaultCodec),
		network.ServerOptionWithHandler(router))
}
