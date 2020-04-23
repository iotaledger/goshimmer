package ping

import "errors"

var (
	ErrMalformedPingPacket = errors.New("malformed ping packet")
)

type Packet struct{}

func Unmarshal(data []byte) (*Packet, error) {
	if len(data) < MarshaledTotalSize || data[MarshaledPacketHeaderStart] != MarshaledPacketHeader {
		return nil, ErrMalformedPingPacket
	}

	unmarshaledPacket := &Packet{}

	return unmarshaledPacket, nil
}

func (packet *Packet) Marshal() []byte {
	marshaledPackage := make([]byte, MarshaledTotalSize)

	marshaledPackage[MarshaledPacketHeaderStart] = MarshaledPacketHeader

	return marshaledPackage
}
