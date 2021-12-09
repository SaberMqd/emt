package core

import (
	pb "emt/example/proto"
	"emt/framework"
	"emt/log"
	"emt/network"
	"emt/network/tcp"
	"os"
	"os/signal"
	"syscall"
	"time"
)

var DefaultCli network.Client

func init() {
	router := framework.NewRouter()

	router.Register((*framework.OnClose)(nil), func(args []interface{}) {
		a := args[0].(network.Agent)
		log.Info(a.RemoteAddr().String())
	})

	router.Register((*framework.OnConnect)(nil), func(args []interface{}) {
		a := args[0].(network.Agent)
		log.Info(a.RemoteAddr().String())
	})

	router.Register((*pb.MoveUp)(nil), func(args []interface{}) {
		a := args[0].(network.Agent)
		_ = a
	})

	DefaultCli = tcp.NewClient(
		network.ClientOptionWithName(ServerName),
		network.ClientOptionWithCodec(DefaultCodec),
		network.ClientOptionWithHandler(router))
}

func TestCliWriteMsg() {
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, os.Interrupt, syscall.SIGTERM)

EndFor:
	for {
		select {
		case <-ch:
			log.Info("recv close sign")

			break EndFor
		default:
			time.Sleep(time.Second)
			if err := DefaultCli.WriteMessage(&pb.MoveUp{
				ID: TestID,
			}); err != nil {
				log.Error(err.Error())

				break
			}
		}
	}
}
