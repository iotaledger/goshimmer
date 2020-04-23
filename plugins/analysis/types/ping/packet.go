package ping

import "errors"

var (
	// ErrMalformedPingPacket defines the malformed ping packet error.
	ErrMalformedPingPacket = errors.New("malformed ping packet")
)

// Packet defines the Packet type as an empty struct.
type Packet struct{}

// Unmarshal unmarshals a given slice of byte and returns a *Packet and an error.
func Unmarshal(data []byte) (*Packet, error) {
	if len(data) < MarshaledTotalSize || data[MarshaledPacketHeaderStart] != MarshaledPacketHeader {
		return nil, ErrMalformedPingPacket
	}

	unmarshaledPacket := &Packet{}

	return unmarshaledPacket, nil
}

// Marshal marshals a given *Packet and returns the marshaled slice of byte.
func (packet *Packet) Marshal() []byte {
	marshaledPackage := make([]byte, MarshaledTotalSize)

	marshaledPackage[MarshaledPacketHeaderStart] = MarshaledPacketHeader

	return marshaledPackage
}
