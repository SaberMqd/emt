package game

import (
	pb "emt/example/proto"
	"emt/network"
)

func (g *Game) OnFrame(args []interface{}) {
	a := args[1].(network.Agent)
	m := args[0].(*pb.Frame)
	u := getAgentUser(a)
	if u == nil {
		return
	}
	m.ID = int32(u.id)
	g.broadcast(a, m)
}

func (g *Game) OnLeft(args []interface{}) {
	a := args[1].(network.Agent)
	m := args[0].(*pb.MoveLeft)
	u := getAgentUser(a)
	if u == nil {
		return
	}
	m.ID = int32(u.id)
	g.broadcast(a, m)
}

func (g *Game) OnLeftOver(args []interface{}) {
	a := args[1].(network.Agent)
	m := args[0].(*pb.MoveLeftOver)
	u := getAgentUser(a)
	if u == nil {
		return
	}
	m.ID = int32(u.id)
	g.broadcast(a, m)
}

func (g *Game) OnRight(args []interface{}) {
	a := args[1].(network.Agent)
	m := args[0].(*pb.MoveRight)
	u := getAgentUser(a)
	if u == nil {
		return
	}
	m.ID = int32(u.id)
	g.broadcast(a, m)
}

func (g *Game) OnRightOver(args []interface{}) {
	a := args[1].(network.Agent)
	m := args[0].(*pb.MoveRightOver)
	u := getAgentUser(a)
	if u == nil {
		return
	}
	m.ID = int32(u.id)
	g.broadcast(a, m)
}

func (g *Game) OnAction(args []interface{}) {
	a := args[1].(network.Agent)
	m := args[0].(*pb.Action)
	u := getAgentUser(a)
	if u == nil {
		return
	}
	m.ID = int32(u.id)
	g.broadcast(a, m)
}

func (g *Game) OnPlayerInfo(args []interface{}) {
	a := args[1].(network.Agent)
	m := args[0].(*pb.PlayerInfo)
	u := getAgentUser(a)
	if u == nil {
		return
	}
	m.ID = int32(u.id)
	g.broadcast(a, m)
}

func (g *Game) OnUp(args []interface{}) {
	a := args[1].(network.Agent)
	m := args[0].(*pb.MoveUp)
	u := getAgentUser(a)
	if u == nil {
		return
	}
	m.ID = int32(u.id)
	g.broadcast(a, m)
}

func (g *Game) OnDown(args []interface{}) {
	a := args[1].(network.Agent)
	m := args[0].(*pb.MoveDown)
	u := getAgentUser(a)
	if u == nil {
		return
	}
	m.ID = int32(u.id)
	g.broadcast(a, m)
}

func (g *Game) OnDownOver(args []interface{}) {
	a := args[1].(network.Agent)
	m := args[0].(*pb.MoveDownOver)
	u := getAgentUser(a)
	if u == nil {
		return
	}
	m.ID = int32(u.id)
	g.broadcast(a, m)
}

func (g *Game) OnFall(args []interface{}) {
	a := args[1].(network.Agent)
	m := args[0].(*pb.Fall)
	u := getAgentUser(a)
	if u == nil {
		return
	}
	m.ID = int32(u.id)
	g.broadcast(a, m)
}

func (g *Game) OnIdel(args []interface{}) {
	a := args[1].(network.Agent)
	m := args[0].(*pb.Idle)
	u := getAgentUser(a)
	if u == nil {
		return
	}
	m.ID = int32(u.id)
	g.broadcast(a, m)
}

func (g *Game) OnDie(args []interface{}) {
	a := args[1].(network.Agent)
	m := args[0].(*pb.Die)
	u := getAgentUser(a)
	if u == nil {
		return
	}
	m.ID = int32(u.id)
	g.broadcast(a, m)
}

func (g *Game) OnBounce(args []interface{}) {
	a := args[1].(network.Agent)
	m := args[0].(*pb.Bounce)
	u := getAgentUser(a)
	if u == nil {
		return
	}
	m.ID = int32(u.id)
	g.broadcast(a, m)
}

func (g *Game) OnProps(args []interface{}) {
	a := args[1].(network.Agent)
	m := args[0].(*pb.Props)
	u := getAgentUser(a)
	if u == nil {
		return
	}
	m.ID = int32(u.id)
	g.broadcast(a, m)
}

func (g *Game) OnFootBoard(args []interface{}) {
	a := args[1].(network.Agent)
	m := args[0].(*pb.FootBoard)
	u := getAgentUser(a)
	if u == nil {
		return
	}
	m.ID = int32(u.id)
	g.broadcast(a, m)
}

func (g *Game) OnHit(args []interface{}) {
	a := args[1].(network.Agent)
	m := args[0].(*pb.Hit)
	u := getAgentUser(a)
	if u == nil {
		return
	}
	m.ID = int32(u.id)
	g.broadcast(a, m)
}

func (g *Game) OnSyncGear(args []interface{}) {
	a := args[1].(network.Agent)
	m := args[0].(*pb.SyncGear)
	u := getAgentUser(a)
	if u == nil {
		return
	}
	m.ID = int32(u.id)
	g.broadcast(a, m)
}
