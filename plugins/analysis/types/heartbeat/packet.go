package heartbeat

import "errors"

var (
	ErrMalformedHeartbeatPacket = errors.New("malformed heartbeat packet")
	ErrTooManyNeighborsToReport = errors.New("too many neighbors to report in packet")
)

// A heartbeat packet
type Packet struct {
	OwnID       []byte
	OutboundIDs [][]byte
	InboundIDs  [][]byte
}

func Unmarshal(data []byte) (*Packet, error) {
	// So far we are only sure about the static part
	MARSHALED_TOTAL_SIZE := MARSHALED_PACKET_HEADER_SIZE + MARSHALED_OWN_ID_SIZE
	// Check if len is smaller than the static parts we know at the moment
	if len(data) < MARSHALED_TOTAL_SIZE || data[0] != MARSHALED_PACKET_HEADER {
		return nil, ErrMalformedHeartbeatPacket
	}

	// First the static part
	unmarshaledOwnID := make([]byte, MARSHALED_OWN_ID_SIZE)
	copy(unmarshaledOwnID[:MARSHALED_OWN_ID_SIZE], data[MARSHALED_OWN_ID_START:MARSHALED_OWN_ID_END])

	// Now the dynamic parts, first outbound neighbors
	lengthOutboundIDs := int(data[MARSHALED_OUTBOUND_IDS_LENGTH_START])

	MARSHALED_TOTAL_SIZE += MARSHALED_OUTBOUND_IDS_LENGTH_SIZE + lengthOutboundIDs*MARSHALED_OUTBOUND_ID_SIZE
	// Check if len is smaller than the size we know at the moment
	if len(data) < MARSHALED_TOTAL_SIZE {
		return nil, ErrMalformedHeartbeatPacket
	}

	unmarshaledOutboundIDs := make([][]byte, lengthOutboundIDs)

	for i := range unmarshaledOutboundIDs {
		// Allocate space for each ID
		unmarshaledOutboundIDs[i] = make([]byte, MARSHALED_OUTBOUND_ID_SIZE)
		copy(unmarshaledOutboundIDs[i][:MARSHALED_OUTBOUND_ID_SIZE], data[MARSHALED_OUTBOUND_IDS_LENGTH_END+i*MARSHALED_OUTBOUND_ID_SIZE:MARSHALED_OUTBOUND_IDS_LENGTH_END+(i+1)*MARSHALED_OUTBOUND_ID_SIZE])
	}

	MARSHALED_INBOUND_IDS_LENGTH_START := MARSHALED_OUTBOUND_IDS_LENGTH_END + lengthOutboundIDs*MARSHALED_OUTBOUND_ID_SIZE
	MARSHALED_INBOUND_IDS_LENGTH_END := MARSHALED_INBOUND_IDS_LENGTH_START + MARSHALED_INBOUND_IDS_LENGTH_SIZE

	// Second dynamic part, inbound neighbors
	lengthInboundIDs := int(data[MARSHALED_INBOUND_IDS_LENGTH_START])

	MARSHALED_TOTAL_SIZE += MARSHALED_INBOUND_IDS_LENGTH_SIZE + lengthInboundIDs*MARSHALED_INBOUND_ID_SIZE
	// Check if len is smaller than the size we know at the moment
	if len(data) < MARSHALED_TOTAL_SIZE {
		return nil, ErrMalformedHeartbeatPacket
	}

	unmarshaledInboundIDs := make([][]byte, lengthInboundIDs)

	for i := range unmarshaledInboundIDs {
		// Allocate space for each ID
		unmarshaledInboundIDs[i] = make([]byte, MARSHALED_INBOUND_ID_SIZE)
		copy(unmarshaledInboundIDs[i][:MARSHALED_INBOUND_ID_SIZE], data[MARSHALED_INBOUND_IDS_LENGTH_END+i*MARSHALED_INBOUND_ID_SIZE:MARSHALED_INBOUND_IDS_LENGTH_END+(i+1)*MARSHALED_INBOUND_ID_SIZE])
	}

	unmarshaledPackage := &Packet{
		OwnID:       unmarshaledOwnID,
		OutboundIDs: unmarshaledOutboundIDs,
		InboundIDs:  unmarshaledInboundIDs,
	}

	return unmarshaledPackage, nil

}

func (packet *Packet) Marshal() ([]byte, error) {
	// Calculate total needed bytes based on packet
	MARSHALED_TOTAL_SIZE := MARSHALED_PACKET_HEADER_SIZE + MARSHALED_OWN_ID_SIZE +
		// Dynamic part 1, outbound IDs
		MARSHALED_OUTBOUND_IDS_LENGTH_SIZE + len(packet.OutboundIDs)*MARSHALED_OUTBOUND_ID_SIZE +
		// Dynamic part 2, Inbound IDs
		MARSHALED_INBOUND_IDS_LENGTH_SIZE + len(packet.InboundIDs)*MARSHALED_INBOUND_ID_SIZE

	marshaledPackage := make([]byte, MARSHALED_TOTAL_SIZE)

	// Header byte
	marshaledPackage[MARSHALED_PACKET_HEADER_START] = MARSHALED_PACKET_HEADER

	// Own nodeId
	copy(marshaledPackage[MARSHALED_OWN_ID_START:MARSHALED_OWN_ID_END], packet.OwnID[:MARSHALED_OWN_ID_SIZE])

	// Outbound nodeIds, need to tell first how many we have to be able to unmarshal it later
	lengthOutboundIDs := len(packet.OutboundIDs)
	if lengthOutboundIDs > MAX_OUTBOUND_NEIGHBOR_COUNT {
		return nil, ErrTooManyNeighborsToReport
	} else {
		marshaledPackage[MARSHALED_OUTBOUND_IDS_LENGTH_START] = byte(lengthOutboundIDs)
	}

	// Copy contents of packet.OutboundIDs
	for i, outboundID := range packet.OutboundIDs {
		copy(marshaledPackage[MARSHALED_OUTBOUND_IDS_LENGTH_END+i*MARSHALED_OUTBOUND_ID_SIZE:MARSHALED_OUTBOUND_IDS_LENGTH_END+(i+1)*MARSHALED_OUTBOUND_ID_SIZE], outboundID[:MARSHALED_OUTBOUND_ID_SIZE])
	}

	// Calculate where inbound nodeId-s start
	MARSHALED_INBOUND_IDS_LENGTH_START := MARSHALED_OUTBOUND_IDS_LENGTH_END + lengthOutboundIDs*MARSHALED_OUTBOUND_ID_SIZE

	// Tell how many inbound nodeId-s we have
	lengthInboundIDs := len(packet.InboundIDs)
	if lengthInboundIDs > MAX_INBOUND_NEIGHBOR_COUNT {
		return nil, ErrTooManyNeighborsToReport
	} else {
		marshaledPackage[MARSHALED_INBOUND_IDS_LENGTH_START] = byte(lengthInboundIDs)
	}

	// End of length is the start of inbound nodeId-s
	MARSHALED_INBOUND_IDS_LENGTH_END := MARSHALED_INBOUND_IDS_LENGTH_START + MARSHALED_INBOUND_IDS_LENGTH_SIZE

	// Copy contents of packet.InboundIDs
	for i, inboundID := range packet.InboundIDs {
		copy(marshaledPackage[MARSHALED_INBOUND_IDS_LENGTH_END+i*MARSHALED_INBOUND_ID_SIZE:MARSHALED_INBOUND_IDS_LENGTH_END+(i+1)*MARSHALED_INBOUND_ID_SIZE], inboundID[:MARSHALED_INBOUND_ID_SIZE])
	}

	return marshaledPackage, nil
}
