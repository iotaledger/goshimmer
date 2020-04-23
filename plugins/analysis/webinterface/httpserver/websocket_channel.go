package httpserver

import (
	"fmt"

	"golang.org/x/net/websocket"
)

// WebSocketChannel holds the websocket connection and its channel.
type WebSocketChannel struct {
	ws   *websocket.Conn
	send chan string
}

// NewWebSocketChannel returns a new *WebSocketChannel for the given websocket connection.
func NewWebSocketChannel(ws *websocket.Conn) *WebSocketChannel {
	wsChan := &WebSocketChannel{
		ws:   ws,
		send: make(chan string, 1024),
	}

	go wsChan.writer()

	return wsChan
}

// Write writes into the given WebSocketChannel.
func (c *WebSocketChannel) Write(update string) {
	c.send <- update
}

// KeepAlive keeps the websocket connection alive.
func (c *WebSocketChannel) KeepAlive() {
	buf := make([]byte, 1)
	for {
		if _, err := c.ws.Read(buf); err != nil {
			break
		}

		_, _ = fmt.Fprint(c.ws, "_")
	}
}

// Close closes the WebSocketChannel.
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
