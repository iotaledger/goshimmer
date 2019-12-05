package transport

import (
	"io"
	"net"
	"strings"
)

// ResolveFunc resolves a string address to the corresponding net.Addr.
type ResolveFunc func(network, address string) (net.Addr, error)

// TransportConn wraps a PacketConn my un-/marshaling the packets using protobuf.
type TransportConn struct {
	conn net.PacketConn
	res  ResolveFunc
}

// Conn creates a new transport layer by using the underlying PacketConn.
func Conn(conn net.PacketConn, res ResolveFunc) *TransportConn {
	return &TransportConn{conn: conn, res: res}
}

// ReadFrom implements the Transport ReadFrom method.
func (t *TransportConn) ReadFrom() ([]byte, string, error) {
	b := make([]byte, MaxPacketSize)
	n, addr, err := t.conn.ReadFrom(b)
	if err != nil {
		// make ErrNetClosing handled consistently
		if strings.Contains(err.Error(), "use of closed network connection") {
			err = io.EOF
		}
		return nil, "", err
	}

	return b[:n], addr.String(), nil
}

// WriteTo implements the Transport WriteTo method.
func (t *TransportConn) WriteTo(pkt []byte, address string) error {
	network := t.conn.LocalAddr().Network()
	addr, err := t.res(network, address)
	if err != nil {
		return err
	}

	_, err = t.conn.WriteTo(pkt, addr)
	return err
}

// Close closes the transport layer.
func (t *TransportConn) Close() {
	_ = t.conn.Close()
}

// LocalAddr returns the local network address.
func (t *TransportConn) LocalAddr() net.Addr {
	return t.conn.LocalAddr()
}
