package kcp

import (
	"encoding/binary"
	"sync/atomic"
	"time"
)

const (
	IKcpRtoNdl     = 30  // no delay min rto
	IKcpRtoMin     = 100 // normal min rto
	IKcpRtoDef     = 200
	IKcpRtoMax     = 60000
	IKcpCmdPush    = 81 // cmd: push data
	IKcpCmdAck     = 82 // cmd: ack
	IKcpCmdWask    = 83 // cmd: window probe (ask)
	IKcpCmdWins    = 84 // cmd: window size (tell)
	IKcpAskSend    = 1  // need to send IKCP_CMD_WASK
	IKcpAskTell    = 2  // need to send IKCP_CMD_WINS
	IKcpWndSnd     = 32
	IKcpWndRcv     = 32
	IKcpMtuDef     = 1400
	IKcpAckFast    = 3
	IKcpInterval   = 100
	IKcpOverhead   = 24
	IKcpDeadlink   = 20
	IKcpThreshInit = 2
	IKcpThreshMin  = 2
	IKcpProbeInit  = 7000   // 7 secs to probe window size
	IKcpProbeLimit = 120000 // up to 120 secs to probe window
	IKcpSNOffset   = 12
)

var _refTime time.Time = time.Now()

func currentMs() uint32 { return uint32(time.Since(_refTime) / time.Millisecond) }

type OutputCallback func(buf []byte, size int)

func IkcpEncode8u(p []byte, c byte) []byte {
	p[0] = c

	return p[1:]
}

func IkcpDecode8u(p []byte, c *byte) []byte {
	*c = p[0]

	return p[1:]
}

func IkcpEncode16u(p []byte, w uint16) []byte {
	binary.LittleEndian.PutUint16(p, w)

	return p[2:]
}

func IkcpDecode16u(p []byte, w *uint16) []byte {
	*w = binary.LittleEndian.Uint16(p)

	return p[2:]
}

func IkcpEncode32u(p []byte, l uint32) []byte {
	binary.LittleEndian.PutUint32(p, l)

	return p[4:]
}

func IkcpDecode32u(p []byte, l *uint32) []byte {
	*l = binary.LittleEndian.Uint32(p)

	return p[4:]
}

func _imin(a, b uint32) uint32 {
	if a <= b {
		return a
	}

	return b
}

func _imax(a, b uint32) uint32 {
	if a >= b {
		return a
	}

	return b
}

func _ibound(lower, middle, upper uint32) uint32 {
	return _imin(_imax(lower, middle), upper)
}

func _itimediff(later, earlier uint32) int32 {
	return (int32)(later - earlier)
}

type segment struct {
	Conv     uint32
	Cmd      uint8
	Frg      uint8
	Wnd      uint16
	TS       uint32
	SN       uint32
	Una      uint32
	Rto      uint32
	Xmit     uint32
	Resendts uint32
	Fastack  uint32
	Acked    uint32
	Data     []byte
}

func (seg *segment) encode(ptr []byte) []byte {
	ptr = IkcpEncode32u(ptr, seg.Conv)
	ptr = IkcpEncode8u(ptr, seg.Cmd)
	ptr = IkcpEncode8u(ptr, seg.Frg)
	ptr = IkcpEncode16u(ptr, seg.Wnd)
	ptr = IkcpEncode32u(ptr, seg.TS)
	ptr = IkcpEncode32u(ptr, seg.SN)
	ptr = IkcpEncode32u(ptr, seg.Una)
	ptr = IkcpEncode32u(ptr, uint32(len(seg.Data)))

	atomic.AddUint64(&DefaultSnmp.OutSegs, 1)

	return ptr
}

type KCP struct {
	Conv, Mtu, Mss, State               uint32
	SndUna, SndNxt, RcvNxt              uint32
	Ssthresh                            uint32
	RxRttvar, RxSrtt                    int32
	RxRto, RxMinrto                     uint32
	SndWnd, RcvWnd, RmtWnd, Cwnd, Probe uint32
	Interval, TSFlush                   uint32
	Nodelay, Updated                    uint32
	TSProbe, ProbeWait                  uint32
	DeadLink, Incr                      uint32

	Fastresend     int32
	Nocwnd, Stream int32

	SndQueue []segment
	RcvQueue []segment
	SndBuf   []segment
	RcvBuf   []segment

	Acklist []ackItem

	Buffer   []byte
	Reserved int
	Output   OutputCallback
}

type ackItem struct {
	SN uint32
	TS uint32
}

func NewKCP(conv uint32, output OutputCallback) *KCP {
	kcp := new(KCP)
	kcp.Conv = conv
	kcp.SndWnd = IKcpWndSnd
	kcp.RcvWnd = IKcpWndRcv
	kcp.RmtWnd = IKcpWndRcv
	kcp.Mtu = IKcpMtuDef
	kcp.Mss = kcp.Mtu - IKcpOverhead
	kcp.Buffer = make([]byte, kcp.Mtu)
	kcp.RxRto = IKcpRtoDef
	kcp.RxMinrto = IKcpRtoMin
	kcp.Interval = IKcpInterval
	kcp.TSFlush = IKcpInterval
	kcp.Ssthresh = IKcpThreshInit
	kcp.DeadLink = IKcpDeadlink
	kcp.Output = output

	return kcp
}

func (kcp *KCP) newSegment(size int) (seg segment) {
	seg.Data = _xmitBuf.Get().([]byte)[:size]

	return
}

func (kcp *KCP) delSegment(seg *segment) {
	if seg.Data != nil {
		_xmitBuf.Put(seg.Data)
		seg.Data = nil
	}
}

func (kcp *KCP) ReserveBytes(n int) bool {
	if n >= int(kcp.Mtu-IKcpOverhead) || n < 0 {
		return false
	}

	kcp.Reserved = n
	kcp.Mss = kcp.Mtu - IKcpOverhead - uint32(n)

	return true
}

func (kcp *KCP) PeekSize() (length int) {
	if len(kcp.RcvQueue) == 0 {
		return -1
	}

	seg := &kcp.RcvQueue[0]
	if seg.Frg == 0 {
		return len(seg.Data)
	}

	if len(kcp.RcvQueue) < int(seg.Frg+1) {
		return -1
	}

	for k := range kcp.RcvQueue {
		seg := &kcp.RcvQueue[k]
		length += len(seg.Data)

		if seg.Frg == 0 {
			break
		}
	}

	return
}

func (kcp *KCP) Recv(buffer []byte) (n int) {
	peeksize := kcp.PeekSize()
	if peeksize < 0 {
		return -1
	}

	if peeksize > len(buffer) {
		return -2
	}

	var fastRecover bool

	if len(kcp.RcvQueue) >= int(kcp.RcvWnd) {
		fastRecover = true
	}

	count := 0

	for k := range kcp.RcvQueue {
		seg := &kcp.RcvQueue[k]
		copy(buffer, seg.Data)
		buffer = buffer[len(seg.Data):]
		n += len(seg.Data)

		count++

		kcp.delSegment(seg)

		if seg.Frg == 0 {
			break
		}
	}

	if count > 0 {
		kcp.RcvQueue = kcp.removeFront(kcp.RcvQueue, count)
	}

	count = 0

	for k := range kcp.RcvBuf {
		seg := &kcp.RcvBuf[k]

		if seg.SN == kcp.RcvNxt && len(kcp.RcvQueue)+count < int(kcp.RcvWnd) {
			kcp.RcvNxt++
			count++
		} else {
			break
		}
	}

	if count > 0 {
		kcp.RcvQueue = append(kcp.RcvQueue, kcp.RcvBuf[:count]...)
		kcp.RcvBuf = kcp.removeFront(kcp.RcvBuf, count)
	}

	if len(kcp.RcvQueue) < int(kcp.RcvWnd) && fastRecover {
		kcp.Probe |= IKcpAskTell
	}

	return n
}

func (kcp *KCP) Send(buffer []byte) int {
	var count int

	if len(buffer) == 0 {
		return -1
	}

	if kcp.Stream != 0 && len(buffer) == 0 {
		return 0
	}

	if kcp.Stream != 0 && len(kcp.SndQueue) > 0 && len(kcp.SndQueue[len(kcp.SndQueue)-1].Data) < int(kcp.Mss) {
		capacity := int(kcp.Mss) - len(kcp.SndQueue[len(kcp.SndQueue)-1].Data)
		extend := capacity

		if len(buffer) < capacity {
			extend = len(buffer)
		}

		oldlen := len(kcp.SndQueue[len(kcp.SndQueue)-1].Data)
		kcp.SndQueue[len(kcp.SndQueue)-1].Data = kcp.SndQueue[len(kcp.SndQueue)-1].Data[:oldlen+extend]
		copy(kcp.SndQueue[len(kcp.SndQueue)-1].Data[oldlen:], buffer)
		buffer = buffer[extend:]
	}

	if len(buffer) <= int(kcp.Mss) {
		count = 1
	} else {
		count = (len(buffer) + int(kcp.Mss) - 1) / int(kcp.Mss)
	}

	if count > const255 {
		return -2
	}

	if count == 0 {
		count = 1
	}

	for i := 0; i < count; i++ {
		var size int

		if len(buffer) > int(kcp.Mss) {
			size = int(kcp.Mss)
		} else {
			size = len(buffer)
		}

		seg := kcp.newSegment(size)
		copy(seg.Data, buffer[:size])

		if kcp.Stream == 0 {
			seg.Frg = uint8(count - i - 1)
		} else {
			seg.Frg = 0
		}

		kcp.SndQueue = append(kcp.SndQueue, seg)
		buffer = buffer[size:]
	}

	return 0
}

const (
	const2     = 2
	const3     = 3
	const5     = 5
	const10    = 10
	const50    = 50
	const255   = 255
	const5000  = 5000
	const10000 = 10000
)

func (kcp *KCP) updateAck(rtt int32) {
	var rto uint32

	if kcp.RxSrtt == 0 {
		kcp.RxSrtt = rtt
		kcp.RxRttvar = rtt >> 1
	} else {
		delta := rtt - kcp.RxSrtt
		kcp.RxSrtt += delta >> const3
		if delta < 0 {
			delta = -delta
		}

		if rtt < kcp.RxSrtt-kcp.RxRttvar {
			kcp.RxRttvar += (delta - kcp.RxRttvar) >> const5
		} else {
			kcp.RxRttvar += (delta - kcp.RxRttvar) >> const2
		}
	}

	rto = uint32(kcp.RxSrtt) + _imax(kcp.Interval, uint32(kcp.RxRttvar)<<const2)
	kcp.RxRto = _ibound(kcp.RxMinrto, rto, IKcpRtoMax)
}

func (kcp *KCP) shrinkBuf() {
	if len(kcp.SndBuf) > 0 {
		seg := &kcp.SndBuf[0]
		kcp.SndUna = seg.SN
	} else {
		kcp.SndUna = kcp.SndNxt
	}
}

func (kcp *KCP) parseAck(sn uint32) {
	if _itimediff(sn, kcp.SndUna) < 0 || _itimediff(sn, kcp.SndNxt) >= 0 {
		return
	}

	for k := range kcp.SndBuf {
		seg := &kcp.SndBuf[k]
		if sn == seg.SN {
			seg.Acked = 1
			kcp.delSegment(seg)

			break
		}

		if _itimediff(sn, seg.SN) < 0 {
			break
		}
	}
}

func (kcp *KCP) parseFastack(sn, ts uint32) {
	if _itimediff(sn, kcp.SndUna) < 0 || _itimediff(sn, kcp.SndNxt) >= 0 {
		return
	}

	for k := range kcp.SndBuf {
		seg := &kcp.SndBuf[k]
		if _itimediff(sn, seg.SN) < 0 {
			break
		} else if sn != seg.SN && _itimediff(seg.TS, ts) <= 0 {
			seg.Fastack++
		}
	}
}

func (kcp *KCP) parseUna(una uint32) int {
	count := 0

	for k := range kcp.SndBuf {
		seg := &kcp.SndBuf[k]
		if _itimediff(una, seg.SN) > 0 {
			kcp.delSegment(seg)
			count++
		} else {
			break
		}
	}

	if count > 0 {
		kcp.SndBuf = kcp.removeFront(kcp.SndBuf, count)
	}

	return count
}

func (kcp *KCP) ackPush(sn, ts uint32) {
	kcp.Acklist = append(kcp.Acklist, ackItem{sn, ts})
}

func (kcp *KCP) parseData(newseg segment) bool {
	sn := newseg.SN
	if _itimediff(sn, kcp.RcvNxt+kcp.RcvWnd) >= 0 ||
		_itimediff(sn, kcp.RcvNxt) < 0 {
		return true
	}

	n := len(kcp.RcvBuf) - 1
	insertIdx := 0
	repeat := false

	for i := n; i >= 0; i-- {
		seg := &kcp.RcvBuf[i]
		if seg.SN == sn {
			repeat = true

			break
		}

		if _itimediff(sn, seg.SN) > 0 {
			insertIdx = i + 1

			break
		}
	}

	if !repeat {
		dataCopy := _xmitBuf.Get().([]byte)[:len(newseg.Data)]
		copy(dataCopy, newseg.Data)
		newseg.Data = dataCopy

		if insertIdx == n+1 {
			kcp.RcvBuf = append(kcp.RcvBuf, newseg)
		} else {
			kcp.RcvBuf = append(kcp.RcvBuf, segment{})
			copy(kcp.RcvBuf[insertIdx+1:], kcp.RcvBuf[insertIdx:])
			kcp.RcvBuf[insertIdx] = newseg
		}
	}

	count := 0

	for k := range kcp.RcvBuf {
		seg := &kcp.RcvBuf[k]

		if seg.SN == kcp.RcvNxt && len(kcp.RcvQueue)+count < int(kcp.RcvWnd) {
			kcp.RcvNxt++
			count++
		} else {
			break
		}
	}

	if count > 0 {
		kcp.RcvQueue = append(kcp.RcvQueue, kcp.RcvBuf[:count]...)
		kcp.RcvBuf = kcp.removeFront(kcp.RcvBuf, count)
	}

	return repeat
}

func (kcp *KCP) settings(regular bool, una uint32, wnd uint16) bool {
	var windowSlides bool

	if regular {
		kcp.RmtWnd = uint32(wnd)
	}

	if kcp.parseUna(una) > 0 {
		windowSlides = true
	}

	return windowSlides
}

func (kcp *KCP) Input(data []byte, regular, ackNoDelay bool) int {
	SndUna := kcp.SndUna

	if len(data) < IKcpOverhead {
		return -1
	}

	var latest uint32

	var flag int

	var inSegs uint64

	var windowSlides bool

	for {
		var ts, sn, length, una, conv uint32

		var wnd uint16

		var cmd, frg uint8

		if len(data) < int(IKcpOverhead) {
			break
		}

		data = IkcpDecode32u(data, &conv)
		data = IkcpDecode8u(data, &cmd)
		data = IkcpDecode8u(data, &frg)
		data = IkcpDecode16u(data, &wnd)
		data = IkcpDecode32u(data, &ts)
		data = IkcpDecode32u(data, &sn)
		data = IkcpDecode32u(data, &una)
		data = IkcpDecode32u(data, &length)

		switch {
		case conv != kcp.Conv:
			return -1

		case len(data) < int(length):
			return -2
		}

		windowSlides = kcp.settings(regular, una, wnd)
		kcp.shrinkBuf()

		switch cmd {
		case IKcpCmdAck:
			kcp.parseAck(sn)
			kcp.parseFastack(sn, ts)

			flag |= 1
			latest = ts

		case IKcpCmdPush:
			repeat := true

			if _itimediff(sn, kcp.RcvNxt+kcp.RcvWnd) < 0 {
				kcp.ackPush(sn, ts)

				if _itimediff(sn, kcp.RcvNxt) >= 0 {
					var seg segment

					seg.Conv = conv
					seg.Cmd = cmd
					seg.Frg = frg
					seg.Wnd = wnd
					seg.TS = ts
					seg.SN = sn
					seg.Una = una
					seg.Data = data[:length]
					repeat = kcp.parseData(seg)
				}
			}

			if regular && repeat {
				atomic.AddUint64(&DefaultSnmp.RepeatSegs, 1)
			}

		case IKcpCmdWask:
			kcp.Probe |= IKcpAskTell

		case IKcpCmdWins:

		default:
			return -3
		}

		inSegs++

		data = data[length:]
	}
	atomic.AddUint64(&DefaultSnmp.InSegs, inSegs)

	if flag != 0 && regular {
		kcp.current(latest)
	}

	if kcp.Nocwnd == 0 && _itimediff(kcp.SndUna, SndUna) > 0 {
		kcp.nocwnd()
	}

	switch {
	case windowSlides:
		kcp.flush(false)

	case ackNoDelay && len(kcp.Acklist) > 0:
		kcp.flush(true)
	}

	return 0
}

func (kcp *KCP) current(latest uint32) {
	current := currentMs()

	if _itimediff(current, latest) >= 0 {
		kcp.updateAck(_itimediff(current, latest))
	}
}

func (kcp *KCP) nocwnd() {
	mss := kcp.Mss

	if kcp.Cwnd < kcp.Ssthresh {
		kcp.Cwnd++
		kcp.Incr += mss
	} else {
		kcp.incr(mss)
	}

	if kcp.Cwnd > kcp.RmtWnd {
		kcp.Cwnd = kcp.RmtWnd
		kcp.Incr = kcp.RmtWnd * mss
	}
}

func (kcp *KCP) incr(mss uint32) {
	if kcp.Incr < mss {
		kcp.Incr = mss
	}

	kcp.Incr += (mss*mss)/kcp.Incr + (mss / 16)
	if (kcp.Cwnd+1)*mss <= kcp.Incr {
		if mss > 0 {
			kcp.Cwnd = (kcp.Incr + mss - 1) / mss
		} else {
			kcp.Cwnd = kcp.Incr + mss - 1
		}
	}
}

func (kcp *KCP) wndUnused() uint16 {
	if len(kcp.RcvQueue) < int(kcp.RcvWnd) {
		return uint16(int(kcp.RcvWnd) - len(kcp.RcvQueue))
	}

	return 0
}

func (kcp *KCP) makeSpace(space int, ptr []byte) []byte {
	buffer := kcp.Buffer
	size := len(buffer) - len(ptr)

	if size+space > int(kcp.Mtu) {
		kcp.Output(buffer, size)
		ptr = buffer[kcp.Reserved:]
	}

	return ptr
}

func (kcp *KCP) flushBuffer(ptr []byte) {
	buffer := kcp.Buffer
	size := len(buffer) - len(ptr)

	if size > kcp.Reserved {
		kcp.Output(buffer, size)
	}
}

func (kcp *KCP) flush(ackOnly bool) uint32 {
	var seg segment

	seg.Conv = kcp.Conv
	seg.Cmd = IKcpCmdAck
	seg.Wnd = kcp.wndUnused()
	seg.Una = kcp.RcvNxt

	buffer := kcp.Buffer
	ptr := buffer[kcp.Reserved:]

	for i, ack := range kcp.Acklist {
		ptr = kcp.makeSpace(IKcpOverhead, ptr)

		if _itimediff(ack.SN, kcp.RcvNxt) >= 0 || len(kcp.Acklist)-1 == i {
			seg.SN, seg.TS = ack.SN, ack.TS
			ptr = seg.encode(ptr)
		}
	}

	kcp.Acklist = kcp.Acklist[0:0]

	if ackOnly {
		kcp.flushBuffer(ptr)

		return kcp.Interval
	}

	if kcp.RmtWnd != 0 {
		kcp.TSProbe = 0
		kcp.ProbeWait = 0
	}

	if kcp.RmtWnd == 0 && kcp.ProbeWait == 0 {
		current := currentMs()
		kcp.ProbeWait = IKcpProbeInit
		kcp.TSProbe = current + kcp.ProbeWait
	} else if kcp.RmtWnd == 0 && _itimediff(currentMs(), kcp.TSProbe) >= 0 {
		if kcp.ProbeWait < IKcpProbeInit {
			kcp.ProbeWait = IKcpProbeInit
		}

		kcp.ProbeWait += kcp.ProbeWait / const2

		if kcp.ProbeWait > IKcpProbeLimit {
			kcp.ProbeWait = IKcpProbeLimit
		}

		kcp.TSProbe = currentMs() + kcp.ProbeWait
		kcp.Probe |= IKcpAskSend
	}

	if (kcp.Probe & IKcpAskSend) != 0 {
		seg.Cmd = IKcpCmdWask

		ptr = kcp.makeSpace(IKcpOverhead, ptr)

		ptr = seg.encode(ptr)
	}

	if (kcp.Probe & IKcpAskTell) != 0 {
		seg.Cmd = IKcpCmdWins

		ptr = kcp.makeSpace(IKcpOverhead, ptr)

		ptr = seg.encode(ptr)
	}

	kcp.Probe = 0

	minrto := kcp.segs(seg, ptr)

	return minrto
}

func (kcp *KCP) segs(seg segment, ptr []byte) uint32 {
	cwnd := _imin(kcp.SndWnd, kcp.RmtWnd)

	if kcp.Nocwnd == 0 {
		cwnd = _imin(kcp.Cwnd, cwnd)
	}

	newSegsCount := 0

	for k := range kcp.SndQueue {
		if _itimediff(kcp.SndNxt, kcp.SndUna+cwnd) >= 0 {
			break
		}

		newseg := kcp.SndQueue[k]
		newseg.Conv = kcp.Conv
		newseg.Cmd = IKcpCmdPush
		newseg.SN = kcp.SndNxt
		kcp.SndBuf = append(kcp.SndBuf, newseg)
		kcp.SndNxt++
		newSegsCount++
	}

	if newSegsCount > 0 {
		kcp.SndQueue = kcp.removeFront(kcp.SndQueue, newSegsCount)
	}

	resent := uint32(kcp.Fastresend)

	if kcp.Fastresend <= 0 {
		resent = const0
	}

	minrto, change, lostSegs, fastRetransSegs, earlyRetransSegs, ptr1 := kcp.rangeRef(resent, newSegsCount, seg, ptr)

	kcp.flushBuffer(ptr1)

	sum := lostSegs

	if lostSegs > 0 {
		atomic.AddUint64(&DefaultSnmp.LostSegs, lostSegs)
	}

	if fastRetransSegs > 0 {
		atomic.AddUint64(&DefaultSnmp.FastRetransSegs, fastRetransSegs)
		sum += fastRetransSegs
	}

	if earlyRetransSegs > 0 {
		atomic.AddUint64(&DefaultSnmp.EarlyRetransSegs, earlyRetransSegs)
		sum += earlyRetransSegs
	}

	if sum > 0 {
		atomic.AddUint64(&DefaultSnmp.RetransSegs, sum)
	}

	kcp.noc(lostSegs, change, cwnd, resent)

	return minrto
}

func (kcp *KCP) noc(lostSegs, change uint64, cwnd, resent uint32) {
	if kcp.Nocwnd == 0 && lostSegs > 0 && kcp.Ssthresh < IKcpThreshMin {
		kcp.Ssthresh = IKcpThreshMin
	}

	if kcp.Nocwnd == 0 && change > 0 && kcp.Ssthresh < IKcpThreshMin {
		kcp.Ssthresh = IKcpThreshMin
	}

	if kcp.Nocwnd == 0 && lostSegs > 0 {
		kcp.Ssthresh = cwnd / const2
		kcp.Cwnd = 1
		kcp.Incr = kcp.Mss
	}

	if kcp.Nocwnd == 0 && change > 0 {
		inflight := kcp.SndNxt - kcp.SndUna
		kcp.Ssthresh = inflight / const2
		kcp.Cwnd = kcp.Ssthresh + resent
		kcp.Incr = kcp.Cwnd * kcp.Mss
	}

	if kcp.Nocwnd == 0 && kcp.Cwnd < 1 {
		kcp.Cwnd = 1
		kcp.Incr = kcp.Mss
	}
}

func (kcp *KCP) rangeRef(resent uint32, newSegsCount int, seg segment, ptr []byte) (uint32, uint64, uint64, uint64, uint64, []byte) {
	current := currentMs()

	var change, lostSegs, fastRetransSegs, earlyRetransSegs uint64

	minrto := kcp.Interval

	ref := kcp.SndBuf[:len(kcp.SndBuf)]

	for k := range ref {
		segment := &ref[k]
		needsend := false

		if segment.Acked == 1 {
			continue
		}

		switch {
		case segment.Xmit == 0:
			needsend = true
			segment.Rto = kcp.RxRto
			segment.Resendts = current + segment.Rto

		case segment.Fastack >= resent:
			needsend = true
			segment.Fastack = 0
			segment.Rto = kcp.RxRto
			segment.Resendts = current + segment.Rto
			change++
			fastRetransSegs++

		case segment.Fastack > 0 && newSegsCount == 0:
			needsend = true
			segment.Fastack = 0
			segment.Rto = kcp.RxRto
			segment.Resendts = current + segment.Rto
			change++
			earlyRetransSegs++

		case _itimediff(current, segment.Resendts) >= 0:
			needsend = true

			if kcp.Nodelay == 0 {
				segment.Rto += kcp.RxRto
			} else {
				segment.Rto += kcp.RxRto / const2
			}

			segment.Fastack = 0
			segment.Resendts = current + segment.Rto
			lostSegs++
		}

		if needsend {
			current = currentMs()
			segment.Xmit++
			segment.TS = current
			segment.Wnd = seg.Wnd
			segment.Una = seg.Una

			need := IKcpOverhead + len(segment.Data)

			ptr = kcp.makeSpace(need, ptr)

			ptr = segment.encode(ptr)

			copy(ptr, segment.Data)

			ptr = ptr[len(segment.Data):]

			if segment.Xmit >= kcp.DeadLink {
				kcp.State = const0F
			}
		}

		if rto := _itimediff(segment.Resendts, current); rto > 0 && uint32(rto) < minrto {
			minrto = uint32(rto)
		}
	}

	return minrto, change, lostSegs, fastRetransSegs, earlyRetransSegs, ptr
}

func (kcp *KCP) Update() {
	var slap int32

	current := currentMs()

	if kcp.Updated == 0 {
		kcp.Updated = 1
		kcp.TSFlush = current
	}

	slap = _itimediff(current, kcp.TSFlush)

	if slap >= const10000 || slap < -const10000 {
		kcp.TSFlush = current
		slap = 0
	}

	if slap >= 0 {
		kcp.TSFlush += kcp.Interval

		if _itimediff(current, kcp.TSFlush) >= 0 {
			kcp.TSFlush = current + kcp.Interval
		}

		kcp.flush(false)
	}
}

func (kcp *KCP) Check() uint32 {
	current := currentMs()
	tsFlush := kcp.TSFlush
	tmPacket := int32(const0f)
	minimal := uint32(0)

	if kcp.Updated == 0 {
		return current
	}

	if _itimediff(current, tsFlush) >= const10000 ||
		_itimediff(current, tsFlush) < -const10000 {
		tsFlush = current
	}

	if _itimediff(current, tsFlush) >= 0 {
		return current
	}

	tmFlush := _itimediff(tsFlush, current)

	for k := range kcp.SndBuf {
		seg := &kcp.SndBuf[k]
		diff := _itimediff(seg.Resendts, current)

		if diff <= 0 {
			return current
		}

		if diff < tmPacket {
			tmPacket = diff
		}
	}

	minimal = uint32(tmPacket)

	if tmPacket >= tmFlush {
		minimal = uint32(tmFlush)
	}

	if minimal >= kcp.Interval {
		minimal = kcp.Interval
	}

	return current + minimal
}

func (kcp *KCP) SetMtu(mtu int) int {
	if mtu < const50 || mtu < IKcpOverhead {
		return -1
	}

	if kcp.Reserved >= int(kcp.Mtu-IKcpOverhead) || kcp.Reserved < 0 {
		return -1
	}

	buffer := make([]byte, mtu)

	if buffer == nil {
		return -2
	}

	kcp.Mtu = uint32(mtu)
	kcp.Mss = kcp.Mtu - IKcpOverhead - uint32(kcp.Reserved)
	kcp.Buffer = buffer

	return 0
}

func (kcp *KCP) NoDelay(nodelay, interval, resend, nc int) int {
	if nodelay >= 0 {
		kcp.Nodelay = uint32(nodelay)
		if nodelay != 0 {
			kcp.RxMinrto = IKcpRtoNdl
		} else {
			kcp.RxMinrto = IKcpRtoMin
		}
	}

	if interval >= 0 {
		if interval > const5000 {
			interval = const5000
		} else if interval < const10 {
			interval = const10
		}

		kcp.Interval = uint32(interval)
	}

	if resend >= 0 {
		kcp.Fastresend = int32(resend)
	}

	if nc >= 0 {
		kcp.Nocwnd = int32(nc)
	}

	return 0
}

func (kcp *KCP) WndSize(sndwnd, rcvwnd int) int {
	if sndwnd > 0 {
		kcp.SndWnd = uint32(sndwnd)
	}

	if rcvwnd > 0 {
		kcp.RcvWnd = uint32(rcvwnd)
	}

	return 0
}

func (kcp *KCP) WaitSnd() int {
	return len(kcp.SndBuf) + len(kcp.SndQueue)
}

func (kcp *KCP) removeFront(q []segment, n int) []segment {
	if n > cap(q)/2 {
		newn := copy(q, q[n:])

		return q[:newn]
	}

	return q[n:]
}

func (kcp *KCP) ReleaseTX() {
	for k := range kcp.SndQueue {
		if kcp.SndQueue[k].Data != nil {
			_xmitBuf.Put(kcp.SndQueue[k].Data)
		}
	}

	for k := range kcp.SndBuf {
		if kcp.SndBuf[k].Data != nil {
			_xmitBuf.Put(kcp.SndBuf[k].Data)
		}
	}

	kcp.SndQueue = nil
	kcp.SndBuf = nil
}
