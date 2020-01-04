package server

import (
	"net"
	"time"

	"github.com/iotaledger/goshimmer/packages/autopeering/peer"
)

// Connection represents a network connection to a neighbor peer.
type Connection struct {
	net.Conn
	peer *peer.Peer
}

func newConnection(c net.Conn, p *peer.Peer) *Connection {
	// make sure the connection has no timeouts
	_ = c.SetDeadline(time.Time{})

	return &Connection{
		Conn: c,
		peer: p,
	}
}

// Peer returns the peer associated with that connection.
func (c *Connection) Peer() *peer.Peer {
	return c.peer
}
