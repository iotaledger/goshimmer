package packet

import "errors"

var (
	// ErrMalformedPacket is returned when malformed packets are tried to be parsed.
	ErrMalformedPacket = errors.New("malformed packet")
)
