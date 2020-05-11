package packet

import (
	"fmt"
)

const (
	// PingPacketHeader is the byte value denoting a ping packet.
	PingPacketHeader = 0x00
	// PingPacketSize is the size of a ping packet.
	PingPacketSize = 1
)

// Ping defines a ping as an empty struct.
type Ping struct{}

// ParsePing parses a slice of bytes into a ping.
func ParsePing(data []byte) (*Ping, error) {
	if len(data) != PingPacketSize {
		return nil, fmt.Errorf("%w: packet doesn't match ping packet size of %d", ErrMalformedPacket, PingPacketSize)
	}

	if data[0] != PingPacketHeader {
		return nil, fmt.Errorf("%w: packet isn't of type ping", ErrMalformedPacket)
	}

	return &Ping{}, nil
}

// NewPingMessage creates a new ping message.
func NewPingMessage() []byte {
	return []byte{PingPacketHeader}
}
