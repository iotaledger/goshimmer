package removenode

import "github.com/pkg/errors"

type Packet struct {
	NodeId []byte
}

func Unmarshal(data []byte) (*Packet, error) {
	if len(data) < MARSHALLED_TOTAL_SIZE || data[0] != MARSHALLED_PACKET_HEADER {
		return nil, errors.New("malformed remove node packet")
	}

	unmarshalledPackage := &Packet{
		NodeId: make([]byte, MARSHALLED_ID_SIZE),
	}

	copy(unmarshalledPackage.NodeId, data[MARSHALLED_ID_START:MARSHALLED_ID_END])

	return unmarshalledPackage, nil
}

func (packet *Packet) Marshal() []byte {
	marshalledPackage := make([]byte, MARSHALLED_TOTAL_SIZE)

	marshalledPackage[MARSHALLED_PACKET_HEADER_START] = MARSHALLED_PACKET_HEADER
	copy(marshalledPackage[MARSHALLED_ID_START:MARSHALLED_ID_END], packet.NodeId[:MARSHALLED_ID_SIZE])

	return marshalledPackage
}
