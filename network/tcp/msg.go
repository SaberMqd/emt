package tcp

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"math"
)

const (
	_uint8Len  int = 1
	_uint16Len int = 2
	_uint32Len int = 4
)

var (
	ErrorMsgTooLong  = errors.New("message too long")
	ErrorMsgTooShort = errors.New("message too short")
)

// | len | data |.
type msgParser struct {
	lenMsgLen    int
	minMsgLen    uint32
	maxMsgLen    uint32
	littleEndian bool
}

func newMsgParser() *msgParser {
	p := new(msgParser)
	p.lenMsgLen = 2
	p.minMsgLen = 1
	p.maxMsgLen = 4096
	p.littleEndian = false

	return p
}

func (p *msgParser) SetMsgLen(lenMsgLen int, minMsgLen uint32, maxMsgLen uint32) {
	if lenMsgLen == _uint8Len || lenMsgLen == _uint16Len || lenMsgLen == _uint32Len {
		p.lenMsgLen = lenMsgLen
	}

	if minMsgLen != 0 {
		p.minMsgLen = minMsgLen
	}

	if maxMsgLen != 0 {
		p.maxMsgLen = maxMsgLen
	}

	var max uint32

	switch p.lenMsgLen {
	case _uint8Len:
		max = math.MaxUint8
	case _uint16Len:
		max = math.MaxUint16
	case _uint32Len:
		max = math.MaxUint32
	}

	if p.minMsgLen > max {
		p.minMsgLen = max
	}

	if p.maxMsgLen > max {
		p.maxMsgLen = max
	}
}

func (p *msgParser) SetByteOrder(littleEndian bool) {
	p.littleEndian = littleEndian
}

func (p *msgParser) Read(conn *Conn) ([]byte, error) {
	var b [4]byte
	bufMsgLen := b[:p.lenMsgLen]

	if _, err := io.ReadFull(conn, bufMsgLen); err != nil {
		return nil, fmt.Errorf("faild to io read len %v , err : %w", p.lenMsgLen, err)
	}

	var msgLen uint32

	switch p.lenMsgLen {
	case _uint8Len:
		msgLen = uint32(bufMsgLen[0])
	case _uint16Len:
		if p.littleEndian {
			msgLen = uint32(binary.LittleEndian.Uint16(bufMsgLen))
		} else {
			msgLen = uint32(binary.BigEndian.Uint16(bufMsgLen))
		}
	case _uint32Len:
		if p.littleEndian {
			msgLen = binary.LittleEndian.Uint32(bufMsgLen)
		} else {
			msgLen = binary.BigEndian.Uint32(bufMsgLen)
		}
	}

	if msgLen > p.maxMsgLen {
		return nil, ErrorMsgTooLong
	} else if msgLen < p.minMsgLen {
		return nil, ErrorMsgTooShort
	}

	msgData := make([]byte, msgLen)
	if _, err := io.ReadFull(conn, msgData); err != nil {
		return nil, fmt.Errorf("faild to io read len %v , err : %w", msgLen, err)
	}

	return msgData, nil
}

func (p *msgParser) Write(conn *Conn, args ...[]byte) error {
	var msgLen uint32
	for i := 0; i < len(args); i++ {
		msgLen += uint32(len(args[i]))
	}

	if msgLen > p.maxMsgLen {
		return ErrorMsgTooLong
	} else if msgLen < p.minMsgLen {
		return ErrorMsgTooShort
	}

	msg := make([]byte, uint32(p.lenMsgLen)+msgLen)

	switch p.lenMsgLen {
	case _uint8Len:
		msg[0] = byte(msgLen)
	case _uint16Len:
		if p.littleEndian {
			binary.LittleEndian.PutUint16(msg, uint16(msgLen))
		} else {
			binary.BigEndian.PutUint16(msg, uint16(msgLen))
		}
	case _uint32Len:
		if p.littleEndian {
			binary.LittleEndian.PutUint32(msg, msgLen)
		} else {
			binary.BigEndian.PutUint32(msg, msgLen)
		}
	}

	l := p.lenMsgLen

	for i := 0; i < len(args); i++ {
		copy(msg[l:], args[i])
		l += len(args[i])
	}

	return conn.Write(msg)
}

func (p *msgParser) String() string {
	return "ltv"
}
