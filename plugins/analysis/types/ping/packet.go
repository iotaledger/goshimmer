package ping

import "github.com/pkg/errors"

type Packet struct{}

func Unmarshal(data []byte) (*Packet, error) {
	if len(data) < MARSHALLED_TOTAL_SIZE || data[MARSHALLED_PACKET_HEADER_START] != MARSHALLED_PACKET_HEADER {
		return nil, errors.New("malformed ping packet")
	}

	unmarshalledPacket := &Packet{}

	return unmarshalledPacket, nil
}

func (packet *Packet) Marshal() []byte {
	marshalledPackage := make([]byte, MARSHALLED_TOTAL_SIZE)

	marshalledPackage[MARSHALLED_PACKET_HEADER_START] = MARSHALLED_PACKET_HEADER

	return marshalledPackage
}
