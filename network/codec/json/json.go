package json

import (
	"emt/log"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
)

var (
	ErrorBug             = errors.New("bug")
	ErrorInvaildJSONData = errors.New("invalid json data")
	ErrorNoPointer       = errors.New("json message pointer required")
	ErrorNotRegister     = fmt.Errorf("message not registered")
)

type Processor struct {
	msgInfo map[string]*MsgInfo
}

type MsgInfo struct {
	msgType reflect.Type
}

func NewCodec() *Processor {
	p := new(Processor)
	p.msgInfo = make(map[string]*MsgInfo)

	return p
}

func (p *Processor) Registe(msg interface{}) string {
	msgType := reflect.TypeOf(msg)

	if msgType == nil || msgType.Kind() != reflect.Ptr {
		log.Error("protobuf message pointer required")

		return ""
	}

	msgID := msgType.Elem().Name()
	if msgID == "" {
		log.Error("unnamed json message")

		return ""
	}

	if _, ok := p.msgInfo[msgID]; ok {
		log.Error("message is already registered")

		return ""
	}

	i := new(MsgInfo)
	i.msgType = msgType
	p.msgInfo[msgID] = i

	return msgID
}

func (p *Processor) String() string {
	return "json"
}

func (p *Processor) Unmarshal(data []byte) (interface{}, error) {
	var m map[string]json.RawMessage

	err := json.Unmarshal(data, &m)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal json %w", err)
	}

	if len(m) != 1 {
		return nil, ErrorInvaildJSONData
	}

	for msgID, data := range m {
		i, ok := p.msgInfo[msgID]
		if !ok {
			return nil, fmt.Errorf("message %v not registered %w", msgID, ErrorNotRegister)
		}

		msg := reflect.New(i.msgType.Elem()).Interface()

		return msg, json.Unmarshal(data, msg)
	}

	return nil, ErrorBug
}

func (p *Processor) Marshal(msg interface{}) ([]byte, error) {
	msgType := reflect.TypeOf(msg)
	if msgType == nil || msgType.Kind() != reflect.Ptr {
		return nil, ErrorNoPointer
	}

	msgID := msgType.Elem().Name()
	if _, ok := p.msgInfo[msgID]; !ok {
		return nil, fmt.Errorf("message %v not registered %w", msgID, ErrorNotRegister)
	}

	m := map[string]interface{}{msgID: msg}

	data, err := json.Marshal(m)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal json %w", err)
	}

	return data, nil
}
