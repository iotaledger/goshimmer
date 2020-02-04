package connectnodes

import "errors"

var (
	ErrMalformedConnectNodesPacket = errors.New("malformed connect nodes packet")
)

type Packet struct {
	SourceId []byte
	TargetId []byte
}

func Unmarshal(data []byte) (*Packet, error) {
	if len(data) < MARSHALED_TOTAL_SIZE || data[0] != MARSHALED_PACKET_HEADER {
		return nil, ErrMalformedConnectNodesPacket
	}

	unmarshaledPackage := &Packet{
		SourceId: make([]byte, MARSHALED_SOURCE_ID_SIZE),
		TargetId: make([]byte, MARSHALED_TARGET_ID_SIZE),
	}

	copy(unmarshaledPackage.SourceId, data[MARSHALED_SOURCE_ID_START:MARSHALED_SOURCE_ID_END])
	copy(unmarshaledPackage.TargetId, data[MARSHALED_TARGET_ID_START:MARSHALED_TARGET_ID_END])

	return unmarshaledPackage, nil
}

func (packet *Packet) Marshal() []byte {
	marshaledPackage := make([]byte, MARSHALED_TOTAL_SIZE)

	marshaledPackage[MARSHALED_PACKET_HEADER_START] = MARSHALED_PACKET_HEADER
	copy(marshaledPackage[MARSHALED_SOURCE_ID_START:MARSHALED_SOURCE_ID_END], packet.SourceId[:MARSHALED_SOURCE_ID_SIZE])
	copy(marshaledPackage[MARSHALED_TARGET_ID_START:MARSHALED_TARGET_ID_END], packet.TargetId[:MARSHALED_TARGET_ID_SIZE])

	return marshaledPackage
}
