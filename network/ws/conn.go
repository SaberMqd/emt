package ws

import (
	"fmt"
	"net"
	"sync"

	"github.com/gorilla/websocket"
)

type Conn struct {
	sync.Mutex
	conn           *websocket.Conn
	wCh            chan []byte
	msgBufferCount uint32
	isClose        bool
	maxMsgLen      uint32
}

func newConn(c *websocket.Conn, msgBufferCount uint32, maxMsgLen uint32) *Conn {
	conn := &Conn{
		msgBufferCount: msgBufferCount,
		conn:           c,
		maxMsgLen:      maxMsgLen,
	}
	conn.wCh = make(chan []byte, conn.msgBufferCount)

	go func() {
		for data := range conn.wCh {
			if data == nil {
				break
			}

			e := c.WriteMessage(websocket.BinaryMessage, data)
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

func (c *Conn) ReadMessage() ([]byte, error) {
	_, b, err := c.conn.ReadMessage()
	if err != nil {
		return nil, fmt.Errorf("faild to read message  %w", err)
	}

	return b, nil
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

	_ = c.conn.UnderlyingConn().(*net.TCPConn).SetLinger(0)
	c.wCh <- nil
}

func (c *Conn) WriteMessage(data []byte) error {
	if c.isClose {
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
	return "websocket_conn"
}
