package packet

import (
	"crypto/sha256"
	"errors"
	"fmt"
)

var (
	// ErrInvalidHeartbeat is returned for invalid heartbeats.
	ErrInvalidHeartbeat = errors.New("invalid heartbeat")
)

const (
	// HeartbeatMaxOutboundPeersCount is the maximum amount of outbound peer IDs a heartbeat packet can contain.
	HeartbeatMaxOutboundPeersCount = 4
	// HeartbeatMaxInboundPeersCount is the maximum amount of inbound peer IDs a heartbeat packet can contain.
	HeartbeatMaxInboundPeersCount = 4
	// HeartbeatPacketHeader is the byte value denoting a heartbeat packet.
	HeartbeatPacketHeader = 0x01
	// HeartbeatPacketHeaderSize is the byte size of the heartbeat header.
	HeartbeatPacketHeaderSize = 1
	// HeartbeatPacketPeerIDSize is the byte size of peer IDs within the heartbeat packet.
	HeartbeatPacketPeerIDSize = sha256.Size
	// HeartbeatPacketOutboundIDCountSize is the byte size of the counter indicating the amount of outbound IDs.
	HeartbeatPacketOutboundIDCountSize = 1
	// HeartbeatPacketMinSize is the minimum byte size of a heartbeat packet.
	HeartbeatPacketMinSize = HeartbeatPacketHeaderSize + HeartbeatPacketPeerIDSize + HeartbeatPacketOutboundIDCountSize
	// HeartbeatPacketMaxSize is the maximum size a heartbeat packet can have.
	HeartbeatPacketMaxSize = HeartbeatPacketHeaderSize + HeartbeatPacketPeerIDSize + HeartbeatPacketOutboundIDCountSize +
		HeartbeatMaxOutboundPeersCount*sha256.Size + HeartbeatMaxInboundPeersCount*sha256.Size
)

// Heartbeat represents a heartbeat packet.
type Heartbeat struct {
	// The ID of the node who sent the heartbeat.
	// Must be contained when a heartbeat is serialized.
	OwnID []byte
	// The IDs of the outbound peers. Can be empty or nil.
	// It must not exceed HeartbeatMaxOutboundPeersCount.
	OutboundIDs [][]byte
	// The IDs of the inbound peers. Can be empty or nil.
	// It must not exceed HeartbeatMaxInboundPeersCount.
	InboundIDs [][]byte
}

// ParseHeartbeat parses a slice of bytes into a heartbeat.
func ParseHeartbeat(data []byte) (*Heartbeat, error) {
	// check minimum size
	if len(data) < HeartbeatPacketMinSize {
		return nil, fmt.Errorf("%w: packet doesn't reach minimum heartbeat packet size of %d", ErrMalformedPacket, HeartbeatPacketMinSize)
	}

	if len(data) > HeartbeatPacketMaxSize {
		return nil, fmt.Errorf("%w: packet exceeds maximum heartbeat packet size of %d", ErrMalformedPacket, HeartbeatPacketMaxSize)
	}

	// check whether we're actually dealing with a heartbeat packet
	if data[0] != HeartbeatPacketHeader {
		return nil, fmt.Errorf("%w: packet isn't of type heartbeat", ErrMalformedPacket)
	}

	// sanity check: packet len - min packet % id size = 0,
	// since we're only dealing with IDs from that offset
	if (len(data)-HeartbeatPacketMinSize)%HeartbeatPacketPeerIDSize != 0 {
		return nil, fmt.Errorf("%w: heartbeat packet is malformed since the data length after the min. packet size offset isn't conforming with peer ID sizes", ErrMalformedPacket)
	}

	// copy own ID
	ownID := make([]byte, HeartbeatPacketPeerIDSize)
	copy(ownID, data[HeartbeatPacketHeaderSize:HeartbeatPacketHeaderSize+HeartbeatPacketPeerIDSize])

	// read outbound IDs count
	outboundIDCount := int(data[HeartbeatPacketMinSize-1])
	if outboundIDCount > HeartbeatMaxOutboundPeersCount {
		return nil, fmt.Errorf("%w: heartbeat packet exceeds maximum outbound IDs of %d", ErrMalformedPacket, HeartbeatMaxOutboundPeersCount)
	}

	// check whether we'd have the amount of data needed for the advertised outbound id count
	if (len(data)-HeartbeatPacketMinSize)/HeartbeatPacketPeerIDSize < outboundIDCount {
		return nil, fmt.Errorf("%w: heartbeat packet is malformed since remaining data length wouldn't fit advertsized outbound IDs count", ErrMalformedPacket)
	}

	// outbound IDs can be zero
	outboundIDs := make([][]byte, outboundIDCount)

	if outboundIDCount != 0 {
		offset := HeartbeatPacketMinSize
		for i := range outboundIDs {
			outboundIDs[i] = make([]byte, HeartbeatPacketPeerIDSize)
			copy(outboundIDs[i], data[offset+i*HeartbeatPacketPeerIDSize:offset+(i+1)*HeartbeatPacketPeerIDSize])
		}
	}

	// (packet size - (min packet size + read outbound IDs)) / ID size = inbound IDs count
	inboundIDCount := (len(data) - (HeartbeatPacketMinSize + outboundIDCount*HeartbeatPacketPeerIDSize)) / HeartbeatPacketPeerIDSize
	if inboundIDCount > HeartbeatMaxInboundPeersCount {
		return nil, fmt.Errorf("%w: heartbeat packet exceeds maximum inbound IDs of %d", ErrMalformedPacket, HeartbeatMaxInboundPeersCount)
	}

	// inbound IDs can be zero
	inboundIDs := make([][]byte, inboundIDCount)
	offset := HeartbeatPacketHeaderSize + HeartbeatPacketPeerIDSize + HeartbeatPacketOutboundIDCountSize + outboundIDCount*HeartbeatPacketPeerIDSize
	for i := range inboundIDs {
		inboundIDs[i] = make([]byte, HeartbeatPacketPeerIDSize)
		copy(inboundIDs[i], data[offset+i*HeartbeatPacketPeerIDSize:offset+(i+1)*HeartbeatPacketPeerIDSize])
	}

	return &Heartbeat{OwnID: ownID, OutboundIDs: outboundIDs, InboundIDs: inboundIDs}, nil
}

// NewHeartbeatMessage serializes the given heartbeat into a byte slice.
func NewHeartbeatMessage(hb *Heartbeat) ([]byte, error) {
	if len(hb.InboundIDs) > HeartbeatMaxInboundPeersCount {
		return nil, fmt.Errorf("%w: heartbeat exceeds maximum inbound IDs of %d", ErrInvalidHeartbeat, HeartbeatMaxInboundPeersCount)
	}
	if len(hb.OutboundIDs) > HeartbeatMaxOutboundPeersCount {
		return nil, fmt.Errorf("%w: heartbeat exceeds maximum outbound IDs of %d", ErrInvalidHeartbeat, HeartbeatMaxOutboundPeersCount)
	}

	if len(hb.OwnID) != HeartbeatPacketPeerIDSize {
		return nil, fmt.Errorf("%w: heartbeat must contain the own peer ID", ErrInvalidHeartbeat)
	}

	// calculate total needed bytes based on packet
	packetSize := HeartbeatPacketMinSize + len(hb.OutboundIDs)*HeartbeatPacketPeerIDSize + len(hb.InboundIDs)*HeartbeatPacketPeerIDSize
	packet := make([]byte, packetSize)

	// header byte
	packet[0] = HeartbeatPacketHeader

	// own nodeId
	copy(packet[HeartbeatPacketHeaderSize:HeartbeatPacketHeaderSize+HeartbeatPacketPeerIDSize], hb.OwnID[:])

	// outbound id count
	packet[HeartbeatPacketHeaderSize+HeartbeatPacketPeerIDSize] = byte(len(hb.OutboundIDs))

	// copy contents of hb.OutboundIDs
	offset := HeartbeatPacketMinSize
	for i, outboundID := range hb.OutboundIDs {
		copy(packet[offset+i*HeartbeatPacketPeerIDSize:offset+(i+1)*HeartbeatPacketPeerIDSize], outboundID[:HeartbeatPacketPeerIDSize])
	}

	// advance offset to after outbound IDs
	offset += len(hb.OutboundIDs) * HeartbeatPacketPeerIDSize

	// copy contents of hb.InboundIDs
	for i, inboundID := range hb.InboundIDs {
		copy(packet[offset+i*HeartbeatPacketPeerIDSize:offset+(i+1)*HeartbeatPacketPeerIDSize], inboundID[:HeartbeatPacketPeerIDSize])
	}

	return packet, nil
}
