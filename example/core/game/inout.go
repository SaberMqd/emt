package game

import (
	"emt/log"
	"emt/network"

	"go.uber.org/zap"
)

func (g *Game) Login(args []interface{}) {
	a := args[1].(network.Agent)
	u := &user{
		a:   a,
		id:  0,
		rid: -1,
	}

	setAgentUser(a, u)

	log.Info("login", zap.String("addr", a.RemoteAddr().String()))
}

func (g *Game) Offline(args []interface{}) {
	a := args[1].(network.Agent)
	u := getAgentUser(a)

	if u == nil {
		return
	}

	if !invaildRoom(u.rid) {
		g.rooms[u.rid].exit(u.rid)
	}

	g.users[u.id] = nil

	log.Info("offline", zap.Int("id", u.id), zap.String("addr", a.RemoteAddr().String()))
}
