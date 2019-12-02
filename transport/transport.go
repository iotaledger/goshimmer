// Package transport provides implementations for simple address-based packet
// transfers.
package transport

import (
	"net"
)

const (
	// MaxPacketSize specifies the maximum allowed size of packets.
	// Packets larger than this will be cut and thus treated as invalid.
	MaxPacketSize = 1280
)

// Transport is generic network connection to transfer protobuf packages.
// Multiple goroutines may invoke methods on a Conn simultaneously.
type Transport interface {
	// ReadFrom reads a packet from the connection. It returns the package and
	// the return address for that package in string form.
	ReadFrom() (pkt []byte, address string, err error)

	// WriteTo writes a packet to the string encoded target address.
	WriteTo(pkt []byte, address string) error

	// Close closes the transport layer.
	// Any blocked ReadFrom or WriteTo operations will return errors.
	Close()

	// LocalAddr returns the local network address.
	LocalAddr() net.Addr
}

// transfer represents a send and contains the package and the return address.
type transfer struct {
	pkt  []byte
	addr string
}
