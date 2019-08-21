package transport

import (
	"net"

	pb "github.com/wollac/autopeering/proto"
)

type Addr struct {
	IP   net.IP
	Port uint16
}

func (e *Addr) Equal(o *Addr) bool {
	return e.Port == o.Port && e.IP.Equal(o.IP)
}

// Abstraction of the transport layer for the protocol
type Transport interface {
	Read() (*pb.Packet, *Addr, error)
	Write(*pb.Packet, *Addr) error

	Close()
	LocalEndpoint() *Addr
}
