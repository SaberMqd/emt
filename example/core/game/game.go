package game

import (
	"emt/network"
	"emt/util/pipeline"
)

const (
	_userMetaKey = "user"
)

type Game struct {
	*pipeline.Pipeline
	users map[int]*user
	rooms [100]room
}

type user struct {
	a   network.Agent
	id  int
	rid int
}

func (g *Game) Init() {
	for k := range g.rooms {
		g.rooms[k].init()
	}

	g.users = make(map[int]*user)

	go g.Run()
}

func setAgentUser(a network.Agent, u *user) {
	a.SetData(_userMetaKey, u)
}

func getAgentUser(a network.Agent) *user {
	u := a.GetData(_userMetaKey)
	if u == nil {
		return nil
	}

	if us, ok := u.(*user); ok {
		return us
	}

	return nil
}
