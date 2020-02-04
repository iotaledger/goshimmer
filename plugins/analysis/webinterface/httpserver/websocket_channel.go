package httpserver

import (
	"fmt"

	"golang.org/x/net/websocket"
)

type WebSocketChannel struct {
	ws   *websocket.Conn
	send chan string
}

func NewWebSocketChannel(ws *websocket.Conn) *WebSocketChannel {
	wsChan := &WebSocketChannel{
		ws:   ws,
		send: make(chan string, 1024),
	}

	go wsChan.writer()

	return wsChan
}

func (c *WebSocketChannel) Write(update string) {
	c.send <- update
}

func (c *WebSocketChannel) TryWrite(update string) {
	select {
	case c.send <- update:
	default:
	}
}

func (c *WebSocketChannel) KeepAlive() {
	buf := make([]byte, 1)
	for {
		if _, err := c.ws.Read(buf); err != nil {
			break
		}

		_, _ = fmt.Fprint(c.ws, "_")
	}
}

func (c *WebSocketChannel) Close() {
	close(c.send)
	_ = c.ws.Close()
}

func (c *WebSocketChannel) writer() {
	for pkt := range c.send {
		if _, err := fmt.Fprint(c.ws, pkt); err != nil {
			break
		}
	}
}
