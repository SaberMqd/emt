package kcp

import (
	"net"
	"sync"
)

type Conn struct {
	sync.Mutex
	conn           *UDPSession
	wCh            chan []byte
	msgBufferCount uint32
	isClose        bool
}

func (c *Conn) ReadMessage() (data []byte, err error) {
	data, _ = c.Read()

	return data, err
}

func (c *Conn) WriteMessage(bytes []byte) error {
	return c.Write(bytes)
}

func newConn(c *UDPSession, msgBufferCount uint32) *Conn {
	conn := &Conn{
		msgBufferCount: msgBufferCount,
		conn:           c,
	}
	conn.wCh = make(chan []byte, conn.msgBufferCount)

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

func (c *Conn) Read() ([]byte, error) {
	data, _ := c.ReadMsg1()

	return data, nil
}

func (c *Conn) ReadMsg1() ([]byte, error) {
	data := make([]byte, 65535)
	l, _ := c.conn.Read(data)

	return data[:l], nil
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

	c.wCh <- nil
	c.isClose = true
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

func (c *Conn) String() string {
	return "kcp_conn"
}
