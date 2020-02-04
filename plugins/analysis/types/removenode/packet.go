package removenode

import "errors"

var (
	ErrMalformedRemovePacket = errors.New("malformed remove node packet")
)

type Packet struct {
	NodeId []byte
}

func Unmarshal(data []byte) (*Packet, error) {
	if len(data) < MARSHALED_TOTAL_SIZE || data[0] != MARSHALED_PACKET_HEADER {
		return nil, ErrMalformedRemovePacket
	}

	unmarshaledPackage := &Packet{
		NodeId: make([]byte, MARSHALED_ID_SIZE),
	}

	copy(unmarshaledPackage.NodeId, data[MARSHALED_ID_START:MARSHALED_ID_END])

	return unmarshaledPackage, nil
}

func (packet *Packet) Marshal() []byte {
	marshaledPackage := make([]byte, MARSHALED_TOTAL_SIZE)

	marshaledPackage[MARSHALED_PACKET_HEADER_START] = MARSHALED_PACKET_HEADER
	copy(marshaledPackage[MARSHALED_ID_START:MARSHALED_ID_END], packet.NodeId[:MARSHALED_ID_SIZE])

	return marshaledPackage
}
