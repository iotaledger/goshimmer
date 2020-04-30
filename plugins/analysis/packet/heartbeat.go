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
	// HeartbeatMaxOutboundPeersCount is the maximum amount of outbound peer ids a heartbeat packet can contain.
	HeartbeatMaxOutboundPeersCount = 4
	// HeartbeatMaxInboundPeersCount is the maximum amount of inbound peer ids a heartbeat packet can contain.
	HeartbeatMaxInboundPeersCount = 4
	// HeartbeatPacketHeader is the byte value denoting a heartbeat packet.
	HeartbeatPacketHeader              = 0x01
	heartbeatPacketHeaderSize          = 1
	heartbeatPacketPeerIDSize          = sha256.Size
	heartbeatPacketOutboundIDCountSize = 1
	heartbeatPacketMinSize             = heartbeatPacketHeaderSize + heartbeatPacketPeerIDSize + heartbeatPacketOutboundIDCountSize
	// HeartbeatPacketMaxSize is the maximum size a heartbeat packet can have.
	HeartbeatPacketMaxSize = heartbeatPacketHeaderSize + heartbeatPacketPeerIDSize + heartbeatPacketOutboundIDCountSize +
		HeartbeatMaxOutboundPeersCount*sha256.Size + HeartbeatMaxInboundPeersCount*sha256.Size
)

// Heartbeat is a heartbeat packet.
type Heartbeat struct {
	// The id of the node who sent the heartbeat.
	OwnID []byte
	// The ids of the outbound peers.
	OutboundIDs [][]byte
	// The ids of the inbound peers.
	InboundIDs [][]byte
}

// ParseHeartbeat parses a slice of bytes into a heartbeat.
func ParseHeartbeat(data []byte) (*Heartbeat, error) {
	// check minimum size
	if len(data) < heartbeatPacketMinSize {
		return nil, fmt.Errorf("%w: packet doesn't reach minimum heartbeat packet size of %d", ErrMalformedPacket, heartbeatPacketMinSize)
	}

	if len(data) > HeartbeatPacketMaxSize {
		return nil, fmt.Errorf("%w: packet exceeds maximum heartbeat packet size of %d", ErrMalformedPacket, HeartbeatPacketMaxSize)
	}

	// check whether we're actually dealing with a heartbeat packet
	if data[0] != HeartbeatPacketHeader {
		return nil, fmt.Errorf("%w: packet isn't of type heartbeat", ErrMalformedPacket)
	}

	// copy own ID
	ownID := make([]byte, heartbeatPacketPeerIDSize)
	copy(ownID, data[heartbeatPacketHeaderSize:heartbeatPacketPeerIDSize])

	// read outbound ids count
	outboundIDCount := int(data[heartbeatPacketMinSize])
	if outboundIDCount > HeartbeatMaxOutboundPeersCount {
		return nil, fmt.Errorf("%w: heartbeat packet exceeds maximum outbound ids of %d", ErrMalformedPacket, HeartbeatMaxOutboundPeersCount)
	}

	// outbound ids can be zero
	outboundIDs := make([][]byte, outboundIDCount)
	if outboundIDCount != 0 {
		offset := heartbeatPacketMinSize
		for i := range outboundIDs {
			outboundIDs[i] = make([]byte, heartbeatPacketPeerIDSize)
			copy(outboundIDs[i], data[offset+i*heartbeatPacketPeerIDSize:offset+(i+1)*heartbeatPacketPeerIDSize])
		}
	}

	// (packet size - min packet size + read outbound ids) / id size = inbound ids count
	inboundIDCount := (len(data) - heartbeatPacketMinSize + outboundIDCount*heartbeatPacketPeerIDSize) / heartbeatPacketPeerIDSize
	if inboundIDCount > HeartbeatMaxInboundPeersCount {
		return nil, fmt.Errorf("%w: heartbeat packet exceeds maximum inbound ids of %d", ErrMalformedPacket, HeartbeatMaxInboundPeersCount)
	}

	// inbound ids can be zero
	inboundIDs := make([][]byte, inboundIDCount)
	offset := heartbeatPacketHeaderSize + heartbeatPacketPeerIDSize + heartbeatPacketOutboundIDCountSize + outboundIDCount*heartbeatPacketPeerIDSize
	for i := range inboundIDs {
		inboundIDs[i] = make([]byte, heartbeatPacketPeerIDSize)
		copy(inboundIDs[i], data[offset+i*heartbeatPacketPeerIDSize:offset+(i+1)*heartbeatPacketPeerIDSize])
	}

	return &Heartbeat{OwnID: ownID, OutboundIDs: outboundIDs, InboundIDs: inboundIDs}, nil
}

// NewHeartbeatMessage serializes the given heartbeat into a byte slice.
func NewHeartbeatMessage(hb *Heartbeat) ([]byte, error) {
	if len(hb.InboundIDs) > HeartbeatMaxInboundPeersCount {
		return nil, fmt.Errorf("%w: heartbeat exceeds maximum inbound ids of %d", ErrInvalidHeartbeat, HeartbeatMaxInboundPeersCount)
	}
	if len(hb.OutboundIDs) > HeartbeatMaxOutboundPeersCount {
		return nil, fmt.Errorf("%w: heartbeat exceeds maximum outbound ids of %d", ErrInvalidHeartbeat, HeartbeatMaxOutboundPeersCount)
	}

	// calculate total needed bytes based on packet
	packetSize := heartbeatPacketMinSize + len(hb.OutboundIDs)*heartbeatPacketPeerIDSize + len(hb.InboundIDs)*heartbeatPacketPeerIDSize
	packet := make([]byte, packetSize)

	// header byte
	packet[0] = HeartbeatPacketHeader

	// own nodeId
	copy(packet[heartbeatPacketHeaderSize:heartbeatPacketPeerIDSize], hb.OwnID[:heartbeatPacketPeerIDSize])

	// outbound id count
	packet[heartbeatPacketHeaderSize+heartbeatPacketPeerIDSize] = byte(len(hb.OutboundIDs))

	// copy contents of hb.OutboundIDs
	offset := heartbeatPacketMinSize
	for i, outboundID := range hb.OutboundIDs {
		copy(packet[offset+i*heartbeatPacketPeerIDSize:offset+(i+1)*heartbeatPacketPeerIDSize], outboundID[:heartbeatPacketPeerIDSize])
	}

	// advance offset to after outbound ids
	offset += len(hb.OutboundIDs) * heartbeatPacketPeerIDSize

	// copy contents of hb.InboundIDs
	for i, inboundID := range hb.InboundIDs {
		copy(packet[offset+i*heartbeatPacketPeerIDSize:offset+(i+1)*heartbeatPacketPeerIDSize], inboundID[:heartbeatPacketPeerIDSize])
	}

	return packet, nil
}
