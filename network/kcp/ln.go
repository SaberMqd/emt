package kcp

import (
	"encoding/binary"
	"io"
	"net"
	"sync"
	"sync/atomic"
	"time"

	"github.com/pkg/errors"
	"golang.org/x/net/ipv4"
	"golang.org/x/net/ipv6"
)

type (
	Listener struct {
		ownConn           bool
		chAccepts         chan *UDPSession
		chSessionClosed   chan net.Addr
		die               chan struct{}
		chSocketReadError chan struct{}
		dataShards        int
		parityShards      int
		conn              net.PacketConn

		sessionLock         sync.RWMutex
		dieOnce             sync.Once
		socketReadErrorOnce sync.Once

		socketReadError atomic.Value
		rd              atomic.Value

		sessions map[string]*UDPSession
	}
)

func (l *Listener) lsess(data []byte, addr net.Addr) {
	l.sessionLock.RLock()
	s, ok := l.sessions[addr.String()]
	l.sessionLock.RUnlock()

	var conv, sn uint32

	convRecovered := false

	fecFlag := binary.LittleEndian.Uint16(data[4:])

	if (fecFlag == typeData || fecFlag == typeParity) && fecFlag == typeData && len(data) >= fecHeaderSizePlus2+IKcpOverhead {
		conv = binary.LittleEndian.Uint32(data[fecHeaderSizePlus2:])
		sn = binary.LittleEndian.Uint32(data[fecHeaderSizePlus2+IKcpSNOffset:])
		convRecovered = true
	} else {
		conv = binary.LittleEndian.Uint32(data)
		sn = binary.LittleEndian.Uint32(data[IKcpSNOffset:])
		convRecovered = true
	}

	if ok {
		if !convRecovered || conv == s.kcp.Conv {
			s.kcpInput(data)
		} else if sn == 0 {
			s.Close()
			s = nil
		}
	}

	if s == nil && convRecovered && len(l.chAccepts) < cap(l.chAccepts) {
		s := newUDPSession(conv, l.dataShards, l.parityShards, l, l.conn, false, addr)
		s.kcpInput(data)
		l.sessionLock.Lock()
		l.sessions[addr.String()] = s
		l.sessionLock.Unlock()
		l.chAccepts <- s
	}
}

func (l *Listener) packetInput(data []byte, addr net.Addr) {
	if len(data) >= IKcpOverhead {
		l.lsess(data, addr)
	}
}

func (l *Listener) notifyReadError(err error) {
	l.socketReadErrorOnce.Do(func() {
		l.socketReadError.Store(err)
		close(l.chSocketReadError)

		l.sessionLock.RLock()

		for _, s := range l.sessions {
			s.notifyReadError(err)
		}

		l.sessionLock.RUnlock()
	})
}

func (l *Listener) SetReadBuffer(bytes int) error {
	if nc, ok := l.conn.(setReadBuffer); ok {
		return nc.SetReadBuffer(bytes)
	}

	return errInvalidOperation
}

func (l *Listener) SetWriteBuffer(bytes int) error {
	if nc, ok := l.conn.(setWriteBuffer); ok {
		return nc.SetWriteBuffer(bytes)
	}

	return errInvalidOperation
}

func (l *Listener) SetDSCP(dscp int) error {
	if ts, ok := l.conn.(setDSCP); ok {
		return ts.SetDSCP(dscp)
	}

	if nc, ok := l.conn.(net.Conn); ok {
		var succeed bool

		if err := ipv4.NewConn(nc).SetTOS(dscp << const2); err == nil {
			succeed = true
		}

		if err := ipv6.NewConn(nc).SetTrafficClass(dscp); err == nil {
			succeed = true
		}

		if succeed {
			return nil
		}
	}

	err := errInvalidOperation

	return err
}

func (l *Listener) Accept() (net.Conn, error) {
	return l.AcceptKCP()
}

func (l *Listener) AcceptKCP() (*UDPSession, error) {
	var timeout <-chan time.Time

	if tdeadline, ok := l.rd.Load().(time.Time); ok && !tdeadline.IsZero() {
		timeout = time.After(time.Until(tdeadline))
	}

	select {
	case <-timeout:
		return nil, errors.WithStack(errTimeout)

	case c := <-l.chAccepts:
		return c, nil

	case <-l.chSocketReadError:
		return nil, l.socketReadError.Load().(error)

	case <-l.die:
		return nil, errors.WithStack(io.ErrClosedPipe)
	}
}

func (l *Listener) SetDeadline(t time.Time) error {
	_ = l.SetReadDeadline(t)
	_ = l.SetWriteDeadline(t)

	return nil
}

func (l *Listener) SetReadDeadline(t time.Time) error {
	l.rd.Store(t)

	return nil
}

func (l *Listener) SetWriteDeadline(t time.Time) error { return errInvalidOperation }

func (l *Listener) Close() error {
	var once bool

	l.dieOnce.Do(func() {
		close(l.die)

		once = true
	})

	var err error

	if once && l.ownConn {
		err = l.conn.Close()
	} else {
		err1 := errors.WithStack(io.ErrClosedPipe)
		err = err1
	}

	return err
}

func (l *Listener) closeSession(remote net.Addr) (ret bool) {
	l.sessionLock.Lock()
	defer l.sessionLock.Unlock()

	if _, ok := l.sessions[remote.String()]; ok {
		delete(l.sessions, remote.String())

		return true
	}

	return false
}

func (l *Listener) Addr() net.Addr { return l.conn.LocalAddr() }

func Listen(laddr string) (net.Listener, error) { return ListenWithOptions(laddr, 0, 0) }

func ListenWithOptions(laddr string, dataShards, parityShards int) (*Listener, error) {
	udpaddr, err := net.ResolveUDPAddr("udp", laddr)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	conn, err := net.ListenUDP("udp", udpaddr)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	return serveConn(dataShards, parityShards, conn, true)
}
