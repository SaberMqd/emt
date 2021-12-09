package kcp

import (
	"encoding/binary"
	"fmt"
	"sync/atomic"

	"github.com/klauspost/reedsolomon"
)

const (
	fecHeaderSize      = 6
	fecHeaderSizePlus2 = fecHeaderSize + 2
	typeData           = 0xf1
	typeParity         = 0xf2
	fecExpire          = 60000
	rxFECMulti         = 3
)

type fecPacket []byte

func (bts fecPacket) seqid() uint32 { return binary.LittleEndian.Uint32(bts) }
func (bts fecPacket) flag() uint16  { return binary.LittleEndian.Uint16(bts[4:]) }
func (bts fecPacket) data() []byte  { return bts[6:] }

type fecElement struct {
	fecPacket
	ts uint32
}

type fecDecoder struct {
	rxlimit      int
	dataShards   int
	parityShards int
	shardSize    int
	rx           []fecElement

	decodeCache [][]byte
	flagCache   []bool

	zeros []byte

	codec reedsolomon.Encoder

	autoTune autoTune
}

func newFECDecoder(dataShards, parityShards int) *fecDecoder {
	if dataShards <= 0 || parityShards <= 0 {
		return nil
	}

	dec := new(fecDecoder)
	dec.dataShards = dataShards
	dec.parityShards = parityShards
	dec.shardSize = dataShards + parityShards
	dec.rxlimit = rxFECMulti * dec.shardSize

	if codec, err := reedsolomon.New(dataShards, parityShards); err == nil {
		dec.codec = codec
		dec.decodeCache = make([][]byte, dec.shardSize)
		dec.flagCache = make([]bool, dec.shardSize)
		dec.zeros = make([]byte, mtuLimit)
	}

	return dec
}

func (dec *fecDecoder) fecNestif(searchBegin, searchEnd int, shardEnd, shardBegin uint32, recovered [][]byte) [][]byte {
	var numshard, numDataShard, first, maxlen int

	shards := dec.decodeCache
	shardsflag := dec.flagCache

	for k := range dec.decodeCache {
		shards[k] = nil
		shardsflag[k] = false
	}

	for i := searchBegin; i <= searchEnd; i++ {
		seqid := dec.rx[i].seqid()

		if _itimediff(seqid, shardEnd) > 0 {
			break
		} else if _itimediff(seqid, shardBegin) >= 0 {
			shards[seqid%uint32(dec.shardSize)] = dec.rx[i].data()
			shardsflag[seqid%uint32(dec.shardSize)] = true
			numshard++
			if dec.rx[i].flag() == typeData {
				numDataShard++
			}

			if numshard == 1 {
				first = i
			}

			if len(dec.rx[i].data()) > maxlen {
				maxlen = len(dec.rx[i].data())
			}
		}
	}

	switch {
	case numDataShard == dec.dataShards:
		dec.rx = dec.freeRange(first, numshard, dec.rx)

	case numshard >= dec.dataShards:
		for k := range shards {
			switch {
			case shards[k] != nil:
				dlen := len(shards[k])
				shards[k] = shards[k][:maxlen]
				copy(shards[k][dlen:], dec.zeros)

			case k < dec.dataShards:
				shards[k] = _xmitBuf.Get().([]byte)[:0]
			}
		}

		if err := dec.codec.ReconstructData(shards); err == nil {
			for k := range shards[:dec.dataShards] {
				if !shardsflag[k] {
					recovered = append(recovered, shards[k])
				}
			}
		}

		dec.rx = dec.freeRange(first, numshard, dec.rx)
	}

	return recovered
}

func (dec *fecDecoder) shoulTune(in fecPacket) error {
	var shouldTune bool

	if int(in.seqid())%dec.shardSize < dec.dataShards {
		if in.flag() != typeData {
			shouldTune = true
		}
	} else {
		if in.flag() != typeParity {
			shouldTune = true
		}
	}

	if shouldTune {
		autoDS := dec.autoTune.FindPeriod(true)
		autoPS := dec.autoTune.FindPeriod(false)

		codec, err := reedsolomon.New(autoDS, autoPS)
		if err != nil {
			return fmt.Errorf("reedsolomon.New: %w", err)
		}

		if autoDS > 0 && autoPS > 0 && autoDS < 256 && autoPS < 256 && (autoDS != dec.dataShards || autoPS != dec.parityShards) {
			dec.dataShards = autoDS
			dec.parityShards = autoPS
			dec.shardSize = autoDS + autoPS
			dec.rxlimit = rxFECMulti * dec.shardSize
			dec.codec = codec
			dec.decodeCache = make([][]byte, dec.shardSize)
			dec.flagCache = make([]bool, dec.shardSize)
		}
	}

	return nil
}

func (dec *fecDecoder) decode(in fecPacket) (recovered [][]byte) {
	if in.flag() == typeData {
		dec.autoTune.Sample(true, in.seqid())
	} else {
		dec.autoTune.Sample(false, in.seqid())
	}

	if err := dec.shoulTune(in); err != nil {
		return nil
	}

	n := len(dec.rx) - 1
	insertIdx := 0

	for i := n; i >= 0; i-- {
		if in.seqid() == dec.rx[i].seqid() {
			return nil
		} else if _itimediff(in.seqid(), dec.rx[i].seqid()) > 0 {
			insertIdx = i + 1

			break
		}
	}

	pkt := fecPacket(_xmitBuf.Get().([]byte)[:len(in)])

	copy(pkt, in)

	elem := fecElement{pkt, currentMs()}

	if insertIdx == n+1 {
		dec.rx = append(dec.rx, elem)
	} else {
		dec.rx = append(dec.rx, fecElement{})
		copy(dec.rx[insertIdx+1:], dec.rx[insertIdx:])
		dec.rx[insertIdx] = elem
	}

	shardBegin := pkt.seqid() - pkt.seqid()%uint32(dec.shardSize)
	shardEnd := shardBegin + uint32(dec.shardSize) - 1

	searchBegin := insertIdx - int(pkt.seqid()%uint32(dec.shardSize))

	if searchBegin < 0 {
		searchBegin = 0
	}

	searchEnd := searchBegin + dec.shardSize - 1

	if searchEnd >= len(dec.rx) {
		searchEnd = len(dec.rx) - 1
	}

	if searchEnd-searchBegin+1 >= dec.dataShards {
		recovered = dec.fecNestif(searchBegin, searchEnd, shardEnd, shardBegin, recovered)
	}

	if len(dec.rx) > dec.rxlimit {
		if dec.rx[0].flag() == typeData {
			atomic.AddUint64(&DefaultSnmp.FECShortShards, 1)
		}

		dec.rx = dec.freeRange(0, 1, dec.rx)
	}

	current := currentMs()
	numExpired := 0

	for k := range dec.rx {
		if _itimediff(current, dec.rx[k].ts) > fecExpire {
			numExpired++

			continue
		}

		break
	}

	if numExpired > 0 {
		dec.rx = dec.freeRange(0, numExpired, dec.rx)
	}

	return recovered
}

func (dec *fecDecoder) freeRange(first, n int, q []fecElement) []fecElement {
	for i := first; i < first+n; i++ {
		_xmitBuf.Put([]byte(q[i].fecPacket))
	}

	if first == 0 && n < cap(q)/2 {
		return q[n:]
	}

	copy(q[first:], q[first+n:])

	return q[:len(q)-n]
}

func (dec *fecDecoder) release() {
	if n := len(dec.rx); n > 0 {
		dec.rx = dec.freeRange(0, n, dec.rx)
	}
}

type (
	fecEncoder struct {
		dataShards   int
		parityShards int
		shardSize    int
		paws         uint32
		next         uint32

		shardCount int
		maxSize    int

		headerOffset  int
		payloadOffset int

		shardCache  [][]byte
		encodeCache [][]byte

		zeros []byte

		codec reedsolomon.Encoder
	}
)

const (
	const0  = 0xffffffff
	const0F = 0xFFFFFFFF
	const0f = 0x7fffffff
)

func newFECEncoder(dataShards, parityShards, offset int) *fecEncoder {
	if dataShards <= 0 || parityShards <= 0 {
		return nil
	}

	enc := new(fecEncoder)
	enc.dataShards = dataShards
	enc.parityShards = parityShards
	enc.shardSize = dataShards + parityShards
	enc.paws = const0 / uint32(enc.shardSize) * uint32(enc.shardSize)
	enc.headerOffset = offset
	enc.payloadOffset = enc.headerOffset + fecHeaderSize

	codec, err := reedsolomon.New(dataShards, parityShards)
	if err != nil {
		return nil
	}

	enc.codec = codec

	// caches
	enc.encodeCache = make([][]byte, enc.shardSize)
	enc.shardCache = make([][]byte, enc.shardSize)

	for k := range enc.shardCache {
		enc.shardCache[k] = make([]byte, mtuLimit)
	}

	enc.zeros = make([]byte, mtuLimit)

	return enc
}

func (enc *fecEncoder) encode(b []byte) (ps [][]byte) {
	enc.markData(b[enc.headerOffset:])
	binary.LittleEndian.PutUint16(b[enc.payloadOffset:], uint16(len(b[enc.payloadOffset:])))

	sz := len(b)
	enc.shardCache[enc.shardCount] = enc.shardCache[enc.shardCount][:sz]

	copy(enc.shardCache[enc.shardCount][enc.payloadOffset:], b[enc.payloadOffset:])

	enc.shardCount++

	if sz > enc.maxSize {
		enc.maxSize = sz
	}

	if enc.shardCount == enc.dataShards {
		for i := 0; i < enc.dataShards; i++ {
			shard := enc.shardCache[i]
			slen := len(shard)
			copy(shard[slen:enc.maxSize], enc.zeros)
		}

		cache := enc.encodeCache
		for k := range cache {
			cache[k] = enc.shardCache[k][enc.payloadOffset:enc.maxSize]
		}

		if err := enc.codec.Encode(cache); err == nil {
			ps = enc.shardCache[enc.dataShards:]
			for k := range ps {
				enc.markParity(ps[k][enc.headerOffset:])
				ps[k] = ps[k][:enc.maxSize]
			}
		}

		enc.shardCount = 0
		enc.maxSize = 0
	}

	return ps
}

func (enc *fecEncoder) markData(data []byte) {
	binary.LittleEndian.PutUint32(data, enc.next)
	binary.LittleEndian.PutUint16(data[4:], typeData)

	enc.next++
}

func (enc *fecEncoder) markParity(data []byte) {
	binary.LittleEndian.PutUint32(data, enc.next)
	binary.LittleEndian.PutUint16(data[4:], typeParity)

	enc.next = (enc.next + 1) % enc.paws
}
