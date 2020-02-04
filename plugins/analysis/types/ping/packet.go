package ping

import "errors"

var (
	ErrMalformedPingPacket = errors.New("malformed ping packet")
)

type Packet struct{}

func Unmarshal(data []byte) (*Packet, error) {
	if len(data) < MARSHALED_TOTAL_SIZE || data[MARSHALED_PACKET_HEADER_START] != MARSHALED_PACKET_HEADER {
		return nil, ErrMalformedPingPacket
	}

	unmarshaledPacket := &Packet{}

	return unmarshaledPacket, nil
}

func (packet *Packet) Marshal() []byte {
	marshaledPackage := make([]byte, MARSHALED_TOTAL_SIZE)

	marshaledPackage[MARSHALED_PACKET_HEADER_START] = MARSHALED_PACKET_HEADER

	return marshaledPackage
}
