package heartbeat

import "errors"

var (
	// ErrMalformedHeartbeatPacket defines the malformed heartbeat packet error.
	ErrMalformedHeartbeatPacket = errors.New("malformed heartbeat packet")
	// ErrTooManyNeighborsToReport defines the too many neighbors to report in packet error.
	ErrTooManyNeighborsToReport = errors.New("too many neighbors to report in packet")
)

// Packet is a heartbeat packet
type Packet struct {
	OwnID       []byte
	OutboundIDs [][]byte
	InboundIDs  [][]byte
}

// Unmarshal unmarshals a slice of byte and returns a *Packet and an error (nil if successful).
func Unmarshal(data []byte) (*Packet, error) {
	// So far we are only sure about the static part
	MarshaledTotalSize := MarshaledPacketHeaderSize + MarshaledOwnIDSize
	// Check if len is smaller than the static parts we know at the moment
	if len(data) < MarshaledTotalSize || data[0] != MarshaledPacketHeader {
		return nil, ErrMalformedHeartbeatPacket
	}

	// First the static part
	unmarshalledOwnID := make([]byte, MarshaledOwnIDSize)
	copy(unmarshalledOwnID[:MarshaledOwnIDSize], data[MarshaledOwnIDStart:MarshaledOwnIDEnd])

	// Now the dynamic parts, first outbound neighbors
	lengthOutboundIDs := int(data[MarshaledOutboundIDsLengthStart])

	MarshaledTotalSize += MarshaledOutboundIDsLengthSize + lengthOutboundIDs*MarshaledOutboundIDSize
	// Check if len is smaller than the size we know at the moment
	if len(data) < MarshaledTotalSize {
		return nil, ErrMalformedHeartbeatPacket
	}

	unmarshalledOutboundIDs := make([][]byte, lengthOutboundIDs)

	for i := range unmarshalledOutboundIDs {
		// Allocate space for each ID
		unmarshalledOutboundIDs[i] = make([]byte, MarshaledOutboundIDSize)
		copy(unmarshalledOutboundIDs[i][:MarshaledOutboundIDSize], data[MarshaledOutboundIDsLengthEnd+i*MarshaledOutboundIDSize:MarshaledOutboundIDsLengthEnd+(i+1)*MarshaledOutboundIDSize])
	}

	MarshaledInboundIdsLengthStart := MarshaledOutboundIDsLengthEnd + lengthOutboundIDs*MarshaledOutboundIDSize
	MarshaledInboundIdsLengthEnd := MarshaledInboundIdsLengthStart + MarshaledInboundIDsLengthSize

	// Second dynamic part, inbound neighbors
	lengthInboundIDs := int(data[MarshaledInboundIdsLengthStart])

	MarshaledTotalSize += MarshaledInboundIDsLengthSize + lengthInboundIDs*MarshaledInboundIDSize
	// Check if len is smaller than the size we know at the moment
	if len(data) < MarshaledTotalSize {
		return nil, ErrMalformedHeartbeatPacket
	}

	unmarshalledInboundIDs := make([][]byte, lengthInboundIDs)

	for i := range unmarshalledInboundIDs {
		// Allocate space for each ID
		unmarshalledInboundIDs[i] = make([]byte, MarshaledInboundIDSize)
		copy(unmarshalledInboundIDs[i][:MarshaledInboundIDSize], data[MarshaledInboundIdsLengthEnd+i*MarshaledInboundIDSize:MarshaledInboundIdsLengthEnd+(i+1)*MarshaledInboundIDSize])
	}

	unmarshalledPackage := &Packet{
		OwnID:       unmarshalledOwnID,
		OutboundIDs: unmarshalledOutboundIDs,
		InboundIDs:  unmarshalledInboundIDs,
	}

	return unmarshalledPackage, nil

}

// Marshal marshals a given *Packet and returns the marshaled slice of byte and an error (nil if successful).
func (packet *Packet) Marshal() ([]byte, error) {
	// Calculate total needed bytes based on packet
	MarshaledTotalSize := MarshaledPacketHeaderSize + MarshaledOwnIDSize +
		// Dynamic part 1, outbound IDs
		MarshaledOutboundIDsLengthSize + len(packet.OutboundIDs)*MarshaledOutboundIDSize +
		// Dynamic part 2, Inbound IDs
		MarshaledInboundIDsLengthSize + len(packet.InboundIDs)*MarshaledInboundIDSize

	marshaledPackage := make([]byte, MarshaledTotalSize)

	// Header byte
	marshaledPackage[MarshaledPacketHeaderStart] = MarshaledPacketHeader

	// Own nodeId
	copy(marshaledPackage[MarshaledOwnIDStart:MarshaledOwnIDEnd], packet.OwnID[:MarshaledOwnIDSize])

	// Outbound nodeIds, need to tell first how many we have to be able to unmarshal it later
	lengthOutboundIDs := len(packet.OutboundIDs)
	if lengthOutboundIDs > MaxOutboundNeighborCount {
		return nil, ErrTooManyNeighborsToReport
	}
	marshaledPackage[MarshaledOutboundIDsLengthStart] = byte(lengthOutboundIDs)

	// Copy contents of packet.OutboundIDs
	for i, outboundID := range packet.OutboundIDs {
		copy(marshaledPackage[MarshaledOutboundIDsLengthEnd+i*MarshaledOutboundIDSize:MarshaledOutboundIDsLengthEnd+(i+1)*MarshaledOutboundIDSize], outboundID[:MarshaledOutboundIDSize])
	}

	// Calculate where inbound nodeId-s start
	MarshaledInboundIdsLengthStart := MarshaledOutboundIDsLengthEnd + lengthOutboundIDs*MarshaledOutboundIDSize

	// Tell how many inbound nodeId-s we have
	lengthInboundIDs := len(packet.InboundIDs)
	if lengthInboundIDs > MaxInboundNeighborCount {
		return nil, ErrTooManyNeighborsToReport
	}
	marshaledPackage[MarshaledInboundIdsLengthStart] = byte(lengthInboundIDs)

	// End of length is the start of inbound nodeId-s
	MarshaledInboundIdsLengthEnd := MarshaledInboundIdsLengthStart + MarshaledInboundIDsLengthSize

	// Copy contents of packet.InboundIDs
	for i, inboundID := range packet.InboundIDs {
		copy(marshaledPackage[MarshaledInboundIdsLengthEnd+i*MarshaledInboundIDSize:MarshaledInboundIdsLengthEnd+(i+1)*MarshaledInboundIDSize], inboundID[:MarshaledInboundIDSize])
	}

	return marshaledPackage, nil
}
