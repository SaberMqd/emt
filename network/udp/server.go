package udp

import (
	s "emt/network"
	"emt/network/session"
	"fmt"
	"net"
	"sync"

	"github.com/google/uuid"
)

type server struct {
	opts       s.ServerOptions
	conns      map[net.Conn]struct{}
	ln         net.Listener
	wgLn       sync.WaitGroup
	wgConns    sync.WaitGroup
	mutexConns sync.Mutex
	mp         *msgParser
}

func NewServer(opts ...s.ServerOption) s.Server {
	svr := &server{}

	for _, o := range opts {
		o(&svr.opts)
	}

	if svr.opts.MaxConnNum == 0 {
		svr.opts.MaxConnNum = s.DefaultMaxConnNum
	}

	if svr.opts.MaxWriteBufLen == 0 {
		svr.opts.MaxWriteBufLen = s.DefaultMaxMsgLen
	}

	if svr.opts.MaxMsgLen == 0 {
		svr.opts.MaxMsgLen = s.DefaultMaxMsgLen
	}

	if svr.opts.ID == "" {
		svr.opts.ID = uuid.New().String()
	}

	return svr
}

func (svr *server) Options() s.ServerOptions {
	return svr.opts
}

func (svr *server) Start() error {
	if err := svr.init(); err != nil {
		return err
	}

	go svr.run()

	return nil
}

func (svr *server) String() string {
	return "udp-server"
}

func (svr *server) init() error {
	ln, err := net.Listen("udp", svr.opts.Addr)
	if err != nil {
		return fmt.Errorf("failed to listen %w", err)
	}

	svr.ln = ln
	svr.conns = make(map[net.Conn]struct{})
	svr.mp = newMsgParser()

	return nil
}

func (svr *server) run() {
	svr.wgLn.Add(1)
	defer svr.wgLn.Done()

	for {
		conn, err := svr.ln.Accept()
		if err != nil {
			return
		}

		svr.mutexConns.Lock()

		if uint32(len(svr.conns)) >= svr.opts.MaxConnNum {
			svr.mutexConns.Unlock()
			conn.Close()

			continue
		}

		svr.conns[conn] = struct{}{}
		svr.mutexConns.Unlock()

		svr.wgConns.Add(1)

		co := newConn(conn, svr.opts.MaxWriteBufLen, svr.mp)
		session := session.NewSession(
			session.OptionWithConn(co),
			session.OptionWithCodec(svr.opts.Codec),
			session.OptionWithCrypt(svr.opts.Crypt),
			session.OptionWithHandler(svr.opts.Handler))

		go func() {
			session.OnConnect()
			session.Run()

			co.Close()
			svr.mutexConns.Lock()
			delete(svr.conns, conn)
			svr.mutexConns.Unlock()
			session.OnClose()

			svr.wgConns.Done()
		}()
	}
}

func (svr *server) Stop() {
	svr.ln.Close()
	svr.wgLn.Wait()
	svr.mutexConns.Lock()

	for conn := range svr.conns {
		conn.Close()
	}

	svr.conns = nil
	svr.mutexConns.Unlock()
	svr.wgConns.Wait()
}
