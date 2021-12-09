package kcp

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
	ln         *Listener
	wgLn       sync.WaitGroup
	wgConns    sync.WaitGroup
	mutexConns sync.Mutex

	dataShards   int
	parityShards int
}

func NewServer(opts ...s.ServerOption) s.Server {
	svr := &server{}

	for _, o := range opts {
		o(&svr.opts)
	}

	svr.dataShards = DefaultDataShards
	svr.parityShards = DefaultParityShards

	if svr.opts.Context != nil {
		cfg, ok := svr.opts.Context.Value(kcpServerConfigKey{}).(*kcpServerConfig)
		if ok {
			svr.dataShards = cfg.DataShards
			svr.parityShards = cfg.ParityShards
		}
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

func (svr *server) init() error {
	ln, err := ListenWithOptions(svr.opts.Addr, svr.dataShards, svr.parityShards)
	if err != nil {
		return fmt.Errorf("failed to listen %w", err)
	}

	svr.ln = ln
	svr.conns = make(map[net.Conn]struct{})

	return nil
}

func (svr *server) run() {
	for {
		conn, err := svr.ln.AcceptKCP()
		if err != nil {
			panic(err)
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

		co := newConn(conn, svr.opts.MaxWriteBufLen)
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

func (svr *server) String() string {
	return "kcp"
}

// ServeConn serves KCP protocol for a single packet connection.
func ServeConn(dataShards, parityShards int, conn net.PacketConn) (*Listener, error) {
	return serveConn(dataShards, parityShards, conn, false)
}

func serveConn(dataShards, parityShards int, conn net.PacketConn, ownConn bool) (*Listener, error) {
	l := new(Listener)
	l.conn = conn
	l.ownConn = ownConn
	l.sessions = make(map[string]*UDPSession)
	l.chAccepts = make(chan *UDPSession, acceptBacklog)
	l.chSessionClosed = make(chan net.Addr)
	l.die = make(chan struct{})
	l.dataShards = dataShards
	l.parityShards = parityShards
	l.chSocketReadError = make(chan struct{})

	go l.monitor()

	return l, nil
}
