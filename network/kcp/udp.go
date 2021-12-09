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

const (
	mtuLimit = 1500

	acceptBacklog = 128
)

var (
	errInvalidOperation = errors.New("invalid operation")
	errTimeout          = errors.New("timeout")

	_xmitBuf sync.Pool
)

func init() {
	_xmitBuf.New = func() interface{} {
		return make([]byte, mtuLimit)
	}
}

type batchConn interface {
	WriteBatch(ms []ipv4.Message, flags int) (int, error)
	ReadBatch(ms []ipv4.Message, flags int) (int, error)
}

type (
	UDPSession struct {
		fecDecoder *fecDecoder
		fecEncoder *fecEncoder
		kcp        *KCP
		l          *Listener
		headerSize int
		dup        int

		mu                 sync.Mutex
		die                chan struct{} // notify current session has Closed
		chReadEvent        chan struct{} // notify Read() can be called without blocking
		chWriteEvent       chan struct{} // notify Write() can be called without blocking
		chSocketReadError  chan struct{}
		chSocketWriteError chan struct{}

		socketReadErrorOnce  sync.Once // 12
		socketWriteErrorOnce sync.Once

		conn   net.PacketConn // 16
		remote net.Addr
		nonce  Entropy // 16

		socketReadError  atomic.Value // 16
		socketWriteError atomic.Value // 16
		xconn            batchConn    // 16

		recvbuf []byte
		bufptr  []byte
		rd      time.Time
		wd      time.Time

		txqueue    []ipv4.Message
		dieOnce    sync.Once
		ownConn    bool
		ackNoDelay bool
		writeDelay bool // delay kcp.flush() for Write() for bulk transfer
	}

	setReadBuffer interface {
		SetReadBuffer(bytes int) error
	}

	setWriteBuffer interface {
		SetWriteBuffer(bytes int) error
	}

	setDSCP interface {
		SetDSCP(int) error
	}
)

func newUDPSession(conv uint32, dataShards, parityShards int, l *Listener, conn net.PacketConn, ownConn bool, remote net.Addr) *UDPSession {
	sess := new(UDPSession)
	sess.die = make(chan struct{})
	sess.nonce = new(nonceAES128)
	sess.nonce.Init()
	sess.chReadEvent = make(chan struct{}, 1)
	sess.chWriteEvent = make(chan struct{}, 1)
	sess.chSocketReadError = make(chan struct{})
	sess.chSocketWriteError = make(chan struct{})
	sess.remote = remote
	sess.conn = conn
	sess.ownConn = ownConn
	sess.l = l
	sess.recvbuf = make([]byte, mtuLimit)

	if _, ok := conn.(*net.UDPConn); ok {
		addr, err := net.ResolveUDPAddr("udp", conn.LocalAddr().String())
		if err == nil {
			if addr.IP.To4() != nil {
				sess.xconn = ipv4.NewPacketConn(conn)
			} else {
				sess.xconn = ipv6.NewPacketConn(conn)
			}
		}
	}

	sess.fecDecoder = newFECDecoder(dataShards, parityShards)
	sess.fecEncoder = newFECEncoder(dataShards, parityShards, 0)

	if sess.fecEncoder != nil {
		sess.headerSize += fecHeaderSizePlus2
	}

	sess.kcp = NewKCP(conv, func(buf []byte, size int) {
		if size >= IKcpOverhead+sess.headerSize {
			sess.output(buf[:size])
		}
	})

	sess.kcp.ReserveBytes(sess.headerSize)

	if sess.l == nil {
		go sess.readLoop()
		atomic.AddUint64(&DefaultSnmp.ActiveOpens, 1)
	} else {
		atomic.AddUint64(&DefaultSnmp.PassiveOpens, 1)
	}

	_SystemTimedSched.Put(sess.update, time.Now())

	currestab := atomic.AddUint64(&DefaultSnmp.CurrEstab, 1)
	maxconn := atomic.LoadUint64(&DefaultSnmp.MaxConn)

	if currestab > maxconn {
		atomic.CompareAndSwapUint64(&DefaultSnmp.MaxConn, maxconn, currestab)
	}

	return sess
}

func (s *UDPSession) Read(b []byte) (n int, err error) {
	for {
		s.mu.Lock()

		if len(s.bufptr) > 0 {
			n = copy(b, s.bufptr)
			s.bufptr = s.bufptr[n:]
			s.mu.Unlock()
			atomic.AddUint64(&DefaultSnmp.BytesReceived, uint64(n))

			return n, nil
		}

		if size := s.kcp.PeekSize(); size > 0 { // peek data size from kcp
			if len(b) >= size { // receive data into 'b' directly
				s.kcp.Recv(b)
				s.mu.Unlock()
				atomic.AddUint64(&DefaultSnmp.BytesReceived, uint64(size))

				return size, nil
			}

			if cap(s.recvbuf) < size {
				s.recvbuf = make([]byte, size)
			}

			s.recvbuf = s.recvbuf[:size]
			s.kcp.Recv(s.recvbuf)
			n = copy(b, s.recvbuf)   // copy to 'b'
			s.bufptr = s.recvbuf[n:] // pointer update
			s.mu.Unlock()
			atomic.AddUint64(&DefaultSnmp.BytesReceived, uint64(n))

			return n, nil
		}

		var timeout *time.Timer

		var c <-chan time.Time

		if !s.rd.IsZero() {
			if time.Now().After(s.rd) {
				s.mu.Unlock()

				return 0, errors.WithStack(errTimeout)
			}

			delay := time.Until(s.rd)
			timeout = time.NewTimer(delay)
			c = timeout.C
		}
		s.mu.Unlock()

		select {
		case <-s.chReadEvent:
			if timeout != nil {
				timeout.Stop()
			}

		case <-c:
			return 0, errors.WithStack(errTimeout)

		case <-s.chSocketReadError:
			return 0, s.socketReadError.Load().(error)

		case <-s.die:
			return 0, errors.WithStack(io.ErrClosedPipe)
		}
	}
}

func (s *UDPSession) Write(b []byte) (n int, err error) { return s.WriteBuffers([][]byte{b}) }

func (s *UDPSession) WaitSnd(waitsnd int, v [][]byte) (n int) {
	for _, b := range v {
		n += len(b)

		for {
			if len(b) <= int(s.kcp.Mss) {
				s.kcp.Send(b)

				break
			} else {
				s.kcp.Send(b[:s.kcp.Mss])
				b = b[s.kcp.Mss:]
			}
		}
	}

	waitsnd = s.kcp.WaitSnd()

	if waitsnd >= int(s.kcp.SndWnd) || waitsnd >= int(s.kcp.RmtWnd) || !s.writeDelay {
		s.kcp.flush(false)
		s.uncork()
	}
	s.mu.Unlock()
	atomic.AddUint64(&DefaultSnmp.BytesSent, uint64(n))

	return n
}

func (s *UDPSession) WriteBuffers(v [][]byte) (n int, err error) {
	for {
		select {
		case <-s.chSocketWriteError:
			return 0, s.socketWriteError.Load().(error)

		case <-s.die:
			return 0, errors.WithStack(io.ErrClosedPipe)

		default:
		}

		s.mu.Lock()

		waitsnd := s.kcp.WaitSnd()
		if waitsnd < int(s.kcp.SndWnd) && waitsnd < int(s.kcp.RmtWnd) {
			return s.WaitSnd(waitsnd, v), nil
		}

		var timeout *time.Timer

		var c <-chan time.Time

		if !s.wd.IsZero() {
			if time.Now().After(s.wd) {
				s.mu.Unlock()

				return 0, errors.WithStack(errTimeout)
			}

			delay := time.Until(s.wd)
			timeout = time.NewTimer(delay)
			c = timeout.C
		}
		s.mu.Unlock()

		select {
		case <-s.chWriteEvent:
			if timeout != nil {
				timeout.Stop()
			}

		case <-c:
			return 0, errors.WithStack(errTimeout)

		case <-s.chSocketWriteError:
			return 0, s.socketWriteError.Load().(error)

		case <-s.die:
			return 0, errors.WithStack(io.ErrClosedPipe)
		}
	}
}

func (s *UDPSession) uncork() {
	if len(s.txqueue) > 0 {
		s.tx(s.txqueue)

		for k := range s.txqueue {
			_xmitBuf.Put(s.txqueue[k].Buffers[0])
			s.txqueue[k].Buffers = nil
		}

		s.txqueue = s.txqueue[:0]
	}
}

func (s *UDPSession) Close() error {
	var once bool

	s.dieOnce.Do(func() {
		close(s.die)
		once = true
	})

	if once {
		atomic.AddUint64(&DefaultSnmp.CurrEstab, ^uint64(0))

		s.mu.Lock()
		s.kcp.flush(false)
		s.uncork()
		s.kcp.ReleaseTX()

		if s.fecDecoder != nil {
			s.fecDecoder.release()
		}

		s.mu.Unlock()

		if s.l != nil {
			s.l.closeSession(s.remote)

			return nil
		}

		if s.l != nil && s.ownConn {
			return s.conn.Close()
		}

		return nil
	}

	return errors.WithStack(io.ErrClosedPipe)
}

func (s *UDPSession) LocalAddr() net.Addr { return s.conn.LocalAddr() }

func (s *UDPSession) RemoteAddr() net.Addr { return s.remote }

func (s *UDPSession) SetDeadline(t time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.rd = t
	s.wd = t
	s.notifyReadEvent()
	s.notifyWriteEvent()

	return nil
}

func (s *UDPSession) SetReadDeadline(t time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.rd = t
	s.notifyReadEvent()

	return nil
}

func (s *UDPSession) SetWriteDeadline(t time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.wd = t
	s.notifyWriteEvent()

	return nil
}

func (s *UDPSession) SetWriteDelay(delay bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.writeDelay = delay
}

func (s *UDPSession) SetWindowSize(sndwnd, rcvwnd int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.kcp.WndSize(sndwnd, rcvwnd)
}

func (s *UDPSession) SetMtu(mtu int) bool {
	if mtu > mtuLimit {
		return false
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	s.kcp.SetMtu(mtu)

	return true
}

func (s *UDPSession) SetStreamMode(enable bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if enable {
		s.kcp.Stream = 1
	} else {
		s.kcp.Stream = 0
	}
}

func (s *UDPSession) SetACKNoDelay(nodelay bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ackNoDelay = nodelay
}

func (s *UDPSession) SetDUP(dup int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.dup = dup
}

func (s *UDPSession) SetNoDelay(nodelay, interval, resend, nc int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.kcp.NoDelay(nodelay, interval, resend, nc)
}

func (s *UDPSession) SetDSCP(dscp int) error {
	var err error

	s.mu.Lock()
	defer s.mu.Unlock()

	if s.l != nil {
		err := errInvalidOperation

		return err
	}

	if ts, ok := s.conn.(setDSCP); ok {
		return ts.SetDSCP(dscp)
	}

	if nc, ok := s.conn.(net.Conn); ok {
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

	err = errInvalidOperation

	return err
}

func (s *UDPSession) SetReadBuffer(bytes int) error {
	var err error

	s.mu.Lock()
	defer s.mu.Unlock()

	if s.l == nil {
		if nc, ok := s.conn.(setReadBuffer); ok {
			return nc.SetReadBuffer(bytes)
		}
	}

	err = errInvalidOperation

	return err
}

func (s *UDPSession) SetWriteBuffer(bytes int) error {
	var err error

	s.mu.Lock()
	defer s.mu.Unlock()

	if s.l == nil {
		if nc, ok := s.conn.(setWriteBuffer); ok {
			return nc.SetWriteBuffer(bytes)
		}
	}

	err = errInvalidOperation

	return err
}

func (s *UDPSession) output(buf []byte) {
	var ecc [][]byte

	if s.fecEncoder != nil {
		ecc = s.fecEncoder.encode(buf)
	}

	var msg ipv4.Message

	for i := 0; i < s.dup+1; i++ {
		bts := _xmitBuf.Get().([]byte)[:len(buf)]
		copy(bts, buf)
		msg.Buffers = [][]byte{bts}
		msg.Addr = s.remote
		s.txqueue = append(s.txqueue, msg)
	}

	for k := range ecc {
		bts := _xmitBuf.Get().([]byte)[:len(ecc[k])]
		copy(bts, ecc[k])
		msg.Buffers = [][]byte{bts}
		msg.Addr = s.remote
		s.txqueue = append(s.txqueue, msg)
	}
}

func (s *UDPSession) update() {
	select {
	case <-s.die:
	default:
		s.mu.Lock()
		interval := s.kcp.flush(false)
		waitsnd := s.kcp.WaitSnd()

		if waitsnd < int(s.kcp.SndWnd) && waitsnd < int(s.kcp.RmtWnd) {
			s.notifyWriteEvent()
		}

		s.uncork()
		s.mu.Unlock()
		_SystemTimedSched.Put(s.update, time.Now().Add(time.Duration(interval)*time.Millisecond))
	}
}

func (s *UDPSession) GetConv() uint32 { return s.kcp.Conv }

func (s *UDPSession) GetRTO() uint32 {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.kcp.RxRto
}

func (s *UDPSession) GetSRTT() int32 {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.kcp.RxSrtt
}

func (s *UDPSession) GetSRTTVar() int32 {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.kcp.RxRttvar
}

func (s *UDPSession) notifyReadEvent() {
	select {
	case s.chReadEvent <- struct{}{}:
	default:
	}
}

func (s *UDPSession) notifyWriteEvent() {
	select {
	case s.chWriteEvent <- struct{}{}:
	default:
	}
}

func (s *UDPSession) notifyReadError(err error) {
	s.socketReadErrorOnce.Do(func() {
		s.socketReadError.Store(err)
		close(s.chSocketReadError)
	})
}

func (s *UDPSession) notifyWriteError(err error) {
	s.socketWriteErrorOnce.Do(func() {
		s.socketWriteError.Store(err)
		close(s.chSocketWriteError)
	})
}

func (s *UDPSession) packetInput(data []byte) {
	if len(data) >= IKcpOverhead {
		s.kcpInput(data)
	}
}

func (s *UDPSession) ranger(r []byte, kcpInErrors, fecErrs, fecRecovered uint64) (uint64, uint64, uint64) {
	sz := binary.LittleEndian.Uint16(r)

	if int(sz) <= len(r) && sz >= 2 {
		if ret := s.kcp.Input(r[2:sz], false, s.ackNoDelay); ret == 0 {
			fecRecovered++
		} else {
			kcpInErrors++
		}
	} else {
		fecErrs++
	}

	return kcpInErrors, fecErrs, fecRecovered
}

func (s *UDPSession) biggerThan(data []byte) (kcpInErrors, fecErrs, fecRecovered, fecParityShards uint64) {
	f := fecPacket(data)
	if f.flag() == typeParity {
		fecParityShards++
	}

	s.mu.Lock()

	if s.fecDecoder == nil {
		s.fecDecoder = newFECDecoder(1, 1)
	}

	recovers := s.fecDecoder.decode(f)

	if f.flag() == typeData {
		if ret := s.kcp.Input(data[fecHeaderSizePlus2:], true, s.ackNoDelay); ret != 0 {
			kcpInErrors++
		}
	}

	for _, r := range recovers {
		if len(r) >= const2 {
			kcpInErrors, fecErrs, fecRecovered = s.ranger(r, kcpInErrors, fecErrs, fecRecovered)
		} else {
			fecErrs++
		}

		_xmitBuf.Put(r)
	}

	if n := s.kcp.PeekSize(); n > 0 {
		s.notifyReadEvent()
	}

	waitsnd := s.kcp.WaitSnd()

	if waitsnd < int(s.kcp.SndWnd) && waitsnd < int(s.kcp.RmtWnd) {
		s.notifyWriteEvent()
	}

	s.uncork()
	s.mu.Unlock()

	return kcpInErrors, fecErrs, fecRecovered, fecParityShards
}

func (s *UDPSession) lendata(data []byte) (kcpInErrors, fecErrs, fecRecovered, fecParityShards uint64) {
	if len(data) >= fecHeaderSizePlus2 {
		kcpInErrors, fecErrs, fecRecovered, fecParityShards = s.biggerThan(data)
	} else {
		atomic.AddUint64(&DefaultSnmp.InErrs, 1)
	}

	return
}

func (s *UDPSession) kcpInput(data []byte) {
	var kcpInErrors, fecErrs, fecRecovered, fecParityShards uint64

	fecFlag := binary.LittleEndian.Uint16(data[4:])
	if fecFlag == typeData || fecFlag == typeParity {
		kcpInErrors, fecErrs, fecRecovered, fecParityShards = s.lendata(data)
	} else {
		s.mu.Lock()
		if ret := s.kcp.Input(data, true, s.ackNoDelay); ret != 0 {
			kcpInErrors++
		}

		if n := s.kcp.PeekSize(); n > 0 {
			s.notifyReadEvent()
		}

		waitsnd := s.kcp.WaitSnd()

		if waitsnd < int(s.kcp.SndWnd) && waitsnd < int(s.kcp.RmtWnd) {
			s.notifyWriteEvent()
		}

		s.uncork()
		s.mu.Unlock()
	}

	atomic.AddUint64(&DefaultSnmp.InPkts, 1)
	atomic.AddUint64(&DefaultSnmp.InBytes, uint64(len(data)))

	if fecParityShards > 0 {
		atomic.AddUint64(&DefaultSnmp.FECParityShards, fecParityShards)
	}

	if kcpInErrors > 0 {
		atomic.AddUint64(&DefaultSnmp.KCPInErrors, kcpInErrors)
	}

	if fecErrs > 0 {
		atomic.AddUint64(&DefaultSnmp.FECErrs, fecErrs)
	}

	if fecRecovered > 0 {
		atomic.AddUint64(&DefaultSnmp.FECRecovered, fecRecovered)
	}
}
