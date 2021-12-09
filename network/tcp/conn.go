package tcp

import (
	"net"
	"sync"
)

const (
	_defaultWriteChanLen = 100
)

type Conn struct {
	sync.Mutex
	conn           net.Conn
	wCh            chan []byte
	msgBufferCount uint32
	isClose        bool
	msgParser      *msgParser
}

func newConn(c net.Conn, msgBufferCount uint32, mp *msgParser) *Conn {
	conn := &Conn{
		msgBufferCount: msgBufferCount,
		conn:           c,
		msgParser:      mp,
	}
	conn.wCh = make(chan []byte, _defaultWriteChanLen)

	go func() {
		for data := range conn.wCh {
			if data == nil {
				break
			}

			_, e := c.Write(data)
			if e != nil {
				break
			}
		}

		c.Close()
		conn.Lock()
		conn.isClose = true
		conn.Unlock()
	}()

	return conn
}

func (c *Conn) LocalAddr() net.Addr {
	return c.conn.LocalAddr()
}

func (c *Conn) RemoteAddr() net.Addr {
	return c.conn.RemoteAddr()
}

func (c *Conn) Close() {
	c.Lock()
	defer c.Unlock()

	if c.isClose {
		return
	}

	_ = c.conn.(*net.TCPConn).SetLinger(0)
	c.wCh <- nil
	c.isClose = true
}

func (c *Conn) Read(data []byte) (int, error) {
	return c.conn.Read(data)
}

func (c *Conn) ReadMessage() ([]byte, error) {
	return c.msgParser.Read(c)
}

func (c *Conn) Write(data []byte) error {
	if c.isClose || data == nil {
		return nil
	}

	if len(c.wCh) == cap(c.wCh) {
		c.Close()

		return nil
	}

	c.wCh <- data

	return nil
}

func (c *Conn) WriteMessage(data []byte) error {
	return c.msgParser.Write(c, data)
}

func (c *Conn) String() string {
	return "tcp_conn"
}
