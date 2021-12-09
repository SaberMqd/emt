package core

import (
	"context"
	pb "emt/example/proto/db"
	"emt/log"
	"emt/rpc"
	emtgrpc "emt/rpc/grpc"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"google.golang.org/grpc"
)

var DefaultRPCCli rpc.Client

func init() {
	DefaultRPCCli = emtgrpc.NewClient(
		rpc.ClientOptionWithName(RPCServerName),
		emtgrpc.ClientOptionWithConfig(&RPCClient{}))
}

type RPCClient struct {
}

func (c *RPCClient) OnConnected(conn interface{}) interface{} {
	return pb.NewDBRPCClient(conn.(*grpc.ClientConn))
}

func TestRPCCliWriteMsg() {
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, os.Interrupt, syscall.SIGTERM)
EndFor:
	for {
		select {
		case <-ch:
			break EndFor
		default:
			time.Sleep(time.Second)
			res, err := DefaultRPCCli.Client().(pb.DBRPCClient).GetUserInfo(context.Background(), &pb.UserInfoReq{})
			if err != nil {
				log.Info(fmt.Sprintf("rpc send error %v", err))

				return
			}

			log.Info(fmt.Sprintf("rpc client recv code %v", res.Code))
		}
	}
}
