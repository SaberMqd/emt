package tcp

import (
	"emt/network"
	"emt/network/session"
	"fmt"
	"net"
	"sync"
	"time"
)

type client struct {
	sync.Mutex
	opts      network.ClientOptions
	conn      net.Conn
	closeFlag bool
	mp        *msgParser
	sess      *session.Session
}

func NewClient(opts ...network.ClientOption) network.Client {
	cli := &client{
		mp: newMsgParser(),
	}

	for _, o := range opts {
		o(&cli.opts)
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

		conn, err := net.Dial("tcp", cli.opts.Addr)
		if err != nil {
			if reCount >= cli.opts.MaxReconnectNum {
				cli.Unlock()

				return fmt.Errorf("reconnect count > max reconnect count, %w", err)
			}

			cli.Unlock()
			fmt.Println("reconnect time ", reCount)
			time.Sleep(cli.opts.ReconnectInterval)
			reCount++

			continue
		}

		if err == nil {
			reCount = 0
		}

		cli.conn = conn
		co := newConn(cli.conn, cli.opts.MaxWriteBufLen, cli.mp)
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
	return "tcp-client"
}

func (cli *client) WriteMessage(data interface{}) error {
	if cli.sess == nil {
		return nil
	}

	return cli.sess.WriteMessage(data)
}
