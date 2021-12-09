package game

import (
	pb "emt/example/proto"
	"emt/log"
	"emt/network"
	"time"

	"go.uber.org/zap"
)

type room struct {
	users map[int]*user
	seed  int64
}

func (r *room) init() {
	r.users = make(map[int]*user)
	r.seed = time.Now().UnixNano()
}

func (g *Game) broadcast(a network.Agent, m interface{}) {
	u := getAgentUser(a)
	if u == nil {
		log.Error("bugs")

		return
	}

	if invaildRoom(u.rid) {
		log.Error("invaild roomid", zap.Int("rid", u.rid))

		return
	}

	r := g.rooms[u.rid]

	for k, v := range r.users {
		if k == u.id {
			//continue
		}

		if v == nil {
			continue
		}

		_ = v.a.WriteMessage(m)
	}
}

func (g *Game) OnJoinRoom(args []interface{}) {
	a := args[1].(network.Agent)
	m := args[0].(*pb.JoinRoom)

	u := getAgentUser(a)
	if u == nil {
		return
	}

	if invaildRoom(int(m.RoomID)) {
		log.Error("invaild roomid", zap.Int32("rid", m.RoomID))

		return
	}

	u.rid = int(m.RoomID)
	u.id = int(m.UserID)
	g.users[u.id] = u
	g.rooms[u.rid].users[u.id] = g.users[u.id]

	_ = a.WriteMessage(&pb.JoinRoomRes{
		Seed: g.rooms[u.rid].seed,
	})

	log.Info("join room success", zap.Int32("uid", m.UserID))
}

func (r *room) exit(id int) {
	r.users[id] = nil
}

func invaildRoom(rid int) bool {
	return rid == -1 || rid >= 100
}
