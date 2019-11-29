package transport

import (
	"net"

	"github.com/golang/protobuf/proto"
	pb "github.com/iotaledger/autopeering-sim/server/proto"
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
func (t *TransportConn) ReadFrom() (*pb.Packet, string, error) {
	b := make([]byte, MaxPacketSize)
	n, addr, err := t.conn.ReadFrom(b)
	if err != nil {
		return nil, "", err
	}

	pkt := new(pb.Packet)
	if err := proto.Unmarshal(b[:n], pkt); err != nil {
		return nil, "", err
	}
	return pkt, addr.String(), nil
}

// WriteTo implements the Transport WriteTo method.
func (t *TransportConn) WriteTo(pkt *pb.Packet, address string) error {
	b, err := proto.Marshal(pkt)
	if err != nil {
		return err
	}
	network := t.conn.LocalAddr().Network()
	addr, err := t.res(network, address)
	if err != nil {
		return err
	}

	_, err = t.conn.WriteTo(b, addr)
	return err
}

// Close closes the transport layer.
func (t *TransportConn) Close() {
	t.conn.Close()
}

// LocalAddr returns the local network address.
func (t *TransportConn) LocalAddr() net.Addr {
	return t.conn.LocalAddr()
}
