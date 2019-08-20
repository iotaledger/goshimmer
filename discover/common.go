package discover

import (
	"net"

	pb "github.com/wollac/autopeering/proto"
)

// Abstraction of the transport layer for the protocol
type Transport interface {
	Read(*pb.Packet) (net.Addr, error)
	Write(*pb.Packet, net.Addr) error

	Close() error
	LocalAddr() net.Addr
}
