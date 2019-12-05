package transport

import (
	"net"

	"github.com/iotaledger/autopeering-sim/peer"
)

const (
	// MaxPacketSize specifies the maximum allowed size of packets.
	// Packets larger than this will be cut and thus treated as invalid.
	MaxPacketSize = 1280
)

type Connection struct {
	peer *peer.Peer
	conn net.Conn
}

func newConnection(p *peer.Peer, c net.Conn) *Connection {
	return &Connection{
		peer: p,
		conn: c,
	}
}

func (c *Connection) Close() {
	c.conn.Close()
}

func (c *Connection) Read() ([]byte, error) {
	b := make([]byte, MaxPacketSize)
	n, err := c.conn.Read(b)
	if err != nil {
		return nil, err
	}

	return b[:n], nil
}

func (c *Connection) Write(b []byte) error {
	_, err := c.conn.Write(b)
	return err
}
