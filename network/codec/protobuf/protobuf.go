package protobuf

import (
	"emt/log"
	"encoding/binary"
	"fmt"
	"reflect"

	"github.com/golang/protobuf/proto"
)

var (
	ErrorNotRegister     = fmt.Errorf("message not registered")
	ErrorMessageTooShort = fmt.Errorf("protobuf message too short")
)

const (
	_minMessageLen int = 2
)

type Processor struct {
	littleEndian bool
	msgInfo      []*MsgInfo
	msgID        map[reflect.Type]uint16
}

type MsgInfo struct {
	msgType reflect.Type
}

func NewCodec() *Processor {
	p := new(Processor)
	p.littleEndian = false
	p.msgID = make(map[reflect.Type]uint16)

	return p
}

func (p *Processor) SetByteOrder(littleEndian bool) {
	p.littleEndian = littleEndian
}

func (p *Processor) String() string {
	return "protobuf"
}

func (p *Processor) Registe(msg proto.Message) uint16 {
	msgType := reflect.TypeOf(msg)
	if msgType == nil || msgType.Kind() != reflect.Ptr {
		log.Error("protobuf message pointer required")

		return 0
	}

	if _, ok := p.msgID[msgType]; ok {
		log.Error("message is already registered")

		return 0
	}

	i := new(MsgInfo)
	i.msgType = msgType
	p.msgInfo = append(p.msgInfo, i)
	id := uint16(len(p.msgInfo) - 1)
	p.msgID[msgType] = id

	return id
}

func (p *Processor) Unmarshal(data []byte) (interface{}, error) {
	if len(data) < _minMessageLen {
		return nil, ErrorMessageTooShort
	}

	var id uint16
	if p.littleEndian {
		id = binary.LittleEndian.Uint16(data)
	} else {
		id = binary.BigEndian.Uint16(data)
	}

	if id >= uint16(len(p.msgInfo)) {
		return nil, fmt.Errorf("msg id %v  %w", id, ErrorNotRegister)
	}

	i := p.msgInfo[id]
	msg := reflect.New(i.msgType.Elem()).Interface()

	return msg, proto.UnmarshalMerge(data[2:], msg.(proto.Message))
}

func (p *Processor) Marshal(msg interface{}) ([]byte, error) {
	msgType := reflect.TypeOf(msg)

	_id, ok := p.msgID[msgType]
	if !ok {
		return nil, fmt.Errorf("msg type %s  %w", msgType, ErrorNotRegister)
	}

	id := make([]byte, 2)

	if p.littleEndian {
		binary.LittleEndian.PutUint16(id, _id)
	} else {
		binary.BigEndian.PutUint16(id, _id)
	}

	data, err := proto.Marshal(msg.(proto.Message))
	if err != nil {
		return id, fmt.Errorf("faild to protobuf marsh %w", err)
	}

	id = append(id, data...)

	return id, nil
}
