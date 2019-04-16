package disconnectnodes

import "github.com/pkg/errors"

type Packet struct {
    SourceId []byte
    TargetId []byte
}

func Unmarshal(data []byte) (*Packet, error) {
    if len(data) < MARSHALLED_TOTAL_SIZE || data[0] != MARSHALLED_PACKET_HEADER {
        return nil, errors.New("malformed disconnect nodes packet")
    }

    unmarshalledPackage := &Packet{
        SourceId: make([]byte, MARSHALLED_SOURCE_ID_SIZE),
        TargetId: make([]byte, MARSHALLED_TARGET_ID_SIZE),
    }

    copy(unmarshalledPackage.SourceId, data[MARSHALLED_SOURCE_ID_START:MARSHALLED_SOURCE_ID_END])
    copy(unmarshalledPackage.TargetId, data[MARSHALLED_TARGET_ID_START:MARSHALLED_TARGET_ID_END])

    return unmarshalledPackage, nil
}

func (packet *Packet) Marshal() []byte {
    marshalledPackage := make([]byte, MARSHALLED_TOTAL_SIZE)

    marshalledPackage[MARSHALLED_PACKET_HEADER_START] = MARSHALLED_PACKET_HEADER
    copy(marshalledPackage[MARSHALLED_SOURCE_ID_START:MARSHALLED_SOURCE_ID_END], packet.SourceId[:MARSHALLED_SOURCE_ID_SIZE])
    copy(marshalledPackage[MARSHALLED_TARGET_ID_START:MARSHALLED_TARGET_ID_END], packet.TargetId[:MARSHALLED_TARGET_ID_SIZE])

    return marshalledPackage
}
