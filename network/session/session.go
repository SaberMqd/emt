package session

import (
	"emt/log"
	"fmt"
	"net"
)

type Session struct {
	opts Options
	data map[string]interface{}
}

func NewSession(opts ...Option) *Session {
	s := &Session{
		data: make(map[string]interface{}),
	}

	for _, o := range opts {
		o(&s.opts)
	}

	return s
}

func (s *Session) Run() {
	for {
		data, err := s.opts.Conn.ReadMessage()
		if err != nil {
			log.Warn(err.Error())

			break
		}

		data1, err := func(d []byte) ([]byte, error) {
			if s.opts.Crypt != nil {
				return s.opts.Crypt.Decrypt(d)
			}

			return d, nil
		}(data)
		if err != nil {
			log.Warn(err.Error())

			break
		}

		msg, err := s.opts.Codec.Unmarshal(data1)
		if err != nil {
			log.Warn(err.Error())

			break
		}

		s.opts.Handler.Handle(s, msg)
	}
}

func (s *Session) OnClose() {
	s.opts.Handler.OnClose(s)
}

func (s *Session) OnConnect() {
	s.opts.Handler.OnConnect(s)
}

func (s *Session) WriteMessage(msg interface{}) error {
	data, err := s.opts.Codec.Marshal(msg)
	if err != nil {
		return fmt.Errorf("faild to marshal %w", err)
	}

	data1, err := func(d []byte) ([]byte, error) {
		if s.opts.Crypt != nil {
			return s.opts.Crypt.Encrypt(d)
		}

		return d, nil
	}(data)
	if err != nil {
		return fmt.Errorf("faild to encrypt %w", err)
	}

	if err = s.opts.Conn.WriteMessage(data1); err != nil {
		return fmt.Errorf("faild to write %w", err)
	}

	return nil
}

func (s *Session) GetData(key string) interface{} {
	if v, ok := s.data[key]; ok {
		return v
	}

	return nil
}

func (s *Session) SetData(key string, v interface{}) {
	s.data[key] = v
}

func (s *Session) RemoteAddr() net.Addr {
	return s.opts.Conn.RemoteAddr()
}

func (s *Session) LocalAddr() net.Addr {
	return s.opts.Conn.LocalAddr()
}
