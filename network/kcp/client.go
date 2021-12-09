package kcp

import (
	"crypto/rand"
	"emt/network"
	"emt/network/session"
	"encoding/binary"
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/pkg/errors"
)

const (
	DefaultDataShards   = 10
	DefaultParityShards = 3
)

type client struct {
	sync.Mutex
	opts      network.ClientOptions
	conn      *UDPSession
	closeFlag bool
	sess      *session.Session

	dataShards   int
	parityShards int
}

func NewClient(opts ...network.ClientOption) network.Client {
	cli := &client{}

	for _, o := range opts {
		o(&cli.opts)
	}

	cli.dataShards = DefaultDataShards
	cli.parityShards = DefaultParityShards

	if cli.opts.Context != nil {
		cfg, ok := cli.opts.Context.Value(kcpClientConfigKey{}).(*kcpClientConfig)
		if ok {
			cli.dataShards = cfg.DataShards
			cli.parityShards = cfg.ParityShards
		}
	}

	if cli.opts.MaxReconnectNum == 0 {
		cli.opts.MaxReconnectNum = network.DefaultMaxReconnectNum
	}

	if cli.opts.ReconnectInterval == 0 {
		cli.opts.ReconnectInterval = network.DefualtReconnectInterval
	}

	if cli.opts.MaxWriteBufLen == 0 {
		cli.opts.MaxWriteBufLen = network.DefaultMaxMsgLen
	}

	if cli.opts.MaxMsgLen == 0 {
		cli.opts.MaxMsgLen = network.DefaultMaxMsgLen
	}

	return cli
}

func (cli *client) Options() network.ClientOptions {
	return cli.opts
}

func (cli *client) SetOption(opts ...network.ClientOption) {
	for _, o := range opts {
		o(&cli.opts)
	}
}

func (cli *client) SetOptions(opts network.ClientOptions) {
	cli.opts = opts
}

func (cli *client) Start() error {
	var reCount uint32

	for {
		if cli.closeFlag {
			break
		}

		cli.Lock()

		time.Sleep(time.Second)

		conn, err := DialWithOptions(cli.opts.Addr, cli.dataShards, cli.parityShards)
		if err != nil {
			if reCount >= cli.opts.MaxReconnectNum {
				cli.Unlock()

				return fmt.Errorf("reconnect count > max reconnect count, %w", err)
			}

			cli.Unlock()
			time.Sleep(cli.opts.ReconnectInterval)
			reCount++

			continue
		}

		if err == nil {
			reCount = 0
		}

		cli.conn = conn
		co := newConn(cli.conn, cli.opts.MaxWriteBufLen)
		session := session.NewSession(
			session.OptionWithConn(co),
			session.OptionWithCodec(cli.opts.Codec),
			session.OptionWithCrypt(cli.opts.Crypt),
			session.OptionWithHandler(cli.opts.Handler))
		cli.sess = session

		cli.Unlock()

		session.OnConnect()
		session.Run()
		co.Close()
		session.OnClose()
	}

	return nil
}

func (cli *client) Stop() {
	cli.Lock()
	cli.closeFlag = true
	cli.conn.Close()
	cli.conn = nil
	cli.Unlock()
}

func (cli *client) String() string {
	return "kcp-client"
}

func DialWithOptions(raddr string, dataShards, parityShards int) (*UDPSession, error) {
	udpaddr, err := net.ResolveUDPAddr("udp", raddr)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	network := "udp4"

	if udpaddr.IP.To4() == nil {
		network = "udp"
	}

	conn, err := net.ListenUDP(network, nil)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	var convid uint32

	_ = binary.Read(rand.Reader, binary.LittleEndian, &convid)

	return newUDPSession(convid, dataShards, parityShards, nil, conn, true, udpaddr), nil
}

func (cli *client) WriteMessage(data interface{}) error {
	if cli.sess == nil {
		return nil
	}

	return cli.sess.WriteMessage(data)
}
