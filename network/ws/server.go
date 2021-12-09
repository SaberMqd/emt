package ws

import (
	s "emt/network"
	"emt/network/session"
	"fmt"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
)

const (
	ErrorMethodNotAllow   int           = 405
	DefaultMaxHeaderBytes uint32        = 1024
	DefaultHTTPTimeOut    time.Duration = 1000 * time.Millisecond
)

type server struct {
	opts  s.ServerOptions
	conns map[*websocket.Conn]struct{}

	ln net.Listener

	upgrader   websocket.Upgrader
	wgLn       sync.WaitGroup
	wgConns    sync.WaitGroup
	wg         sync.WaitGroup
	mutexConns sync.Mutex

	httpMaxHeaderBytes uint32
	httpTimeout        time.Duration
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

	svr.httpTimeout = DefaultHTTPTimeOut
	svr.httpMaxHeaderBytes = DefaultMaxHeaderBytes

	if svr.opts.Context != nil {
		cfg, ok := svr.opts.Context.Value(wsServerConfigKey{}).(*wsServerConfig)
		if ok {
			svr.httpTimeout = cfg.HTTPTimeout
			svr.httpMaxHeaderBytes = cfg.HTTPMaxHeaderBytes
		}
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

	return nil
}

func (svr *server) String() string {
	return "tcp-server"
}

func (svr *server) init() error {
	ln, err := net.Listen("tcp", svr.opts.Addr)
	if err != nil {
		return fmt.Errorf("failed to listen %w", err)
	}

	svr.ln = ln
	svr.conns = make(map[*websocket.Conn]struct{})
	svr.upgrader = websocket.Upgrader{
		HandshakeTimeout: svr.httpTimeout,
		CheckOrigin:      func(_ *http.Request) bool { return true },
	}

	httpServer := &http.Server{
		Addr:           svr.opts.Addr,
		Handler:        svr,
		ReadTimeout:    svr.httpTimeout,
		WriteTimeout:   svr.httpTimeout,
		MaxHeaderBytes: int(svr.httpMaxHeaderBytes),
	}

	return httpServer.Serve(ln)
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

func (svr *server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		http.Error(w, "Method not allowed", ErrorMethodNotAllow)

		return
	}

	conn, err := svr.upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}

	conn.SetReadLimit(int64(svr.opts.MaxMsgLen))

	svr.wg.Add(1)
	defer svr.wg.Done()

	svr.mutexConns.Lock()
	if svr.conns == nil {
		svr.mutexConns.Unlock()
		conn.Close()

		return
	}

	if uint32(len(svr.conns)) >= svr.opts.MaxConnNum {
		svr.mutexConns.Unlock()
		conn.Close()

		return
	}

	svr.conns[conn] = struct{}{}
	svr.mutexConns.Unlock()

	co := newConn(conn, svr.opts.MaxWriteBufLen, svr.opts.MaxMsgLen)
	session := session.NewSession(
		session.OptionWithConn(co),
		session.OptionWithCodec(svr.opts.Codec),
		session.OptionWithCrypt(svr.opts.Crypt),
		session.OptionWithHandler(svr.opts.Handler))

	session.OnConnect()
	session.Run()
	co.Close()

	svr.mutexConns.Lock()
	delete(svr.conns, conn)
	svr.mutexConns.Unlock()
	session.OnClose()
}
