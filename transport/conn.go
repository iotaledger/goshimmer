package transport

import (
	"net"

	"github.com/golang/protobuf/proto"
	pb "github.com/wollac/autopeering/proto"
)

// TransportConn wraps a PacketConn my un-/marshaling the packets using protobuf.
type TransportConn struct {
	conn net.PacketConn
}

// Conn creates a new transport layer by using the underlying PacketConn.
func Conn(conn net.PacketConn) *TransportConn {
	return &TransportConn{conn}
}

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

func (t *TransportConn) WriteTo(pkt *pb.Packet, address string) error {
	b, err := proto.Marshal(pkt)
	if err != nil {
		return err
	}

	network := t.conn.LocalAddr().Network()
	_, err = t.conn.WriteTo(b, &addr{network, address})
	return err
}

func (t *TransportConn) Close() {
	t.conn.Close()
}

func (t *TransportConn) LocalAddr() string {
	return t.conn.LocalAddr().String()
}

type addr struct {
	network, address string
}

func (a *addr) Network() string { return a.network }
func (a *addr) String() string  { return a.address }
