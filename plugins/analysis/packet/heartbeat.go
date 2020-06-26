package packet

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"fmt"

	"github.com/iotaledger/hive.go/protocol/message"
	"github.com/iotaledger/hive.go/protocol/tlv"
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
	// HeartbeatPacketPeerIDSize is the byte size of peer IDs within the heartbeat packet.
	HeartbeatPacketPeerIDSize = sha256.Size
	// HeartbeatPacketOutboundIDCountSize is the byte size of the counter indicating the amount of outbound IDs.
	HeartbeatPacketOutboundIDCountSize = 1
	// HeartbeatPacketMinSize is the minimum byte size of a heartbeat packet.
	HeartbeatPacketMinSize = HeartbeatPacketPeerIDSize + HeartbeatPacketOutboundIDCountSize
	// HeartbeatPacketMaxSize is the maximum size a heartbeat packet can have.
	HeartbeatPacketMaxSize = HeartbeatPacketPeerIDSize + HeartbeatPacketOutboundIDCountSize +
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

// HeartBeatMessageDefinition gets the heartbeatMessageDefinition.
func HeartBeatMessageDefinition() *message.Definition {
	// heartbeatMessageDefinition defines a heartbeat message's format.
	var heartbeatMessageDefinition *message.Definition
	heartBeatOnce.Do(func() {
		heartbeatMessageDefinition = &message.Definition{
			ID:             MessageTypeHeartbeat,
			MaxBytesLength: uint16(HeartbeatPacketMaxSize),
			VariableLength: true,
		}
	})
	return heartbeatMessageDefinition
}

// ParseHeartbeat parses a slice of bytes (serialized packet) into a heartbeat.
func ParseHeartbeat(data []byte) (*Heartbeat, error) {
	// check minimum size
	if len(data) < HeartbeatPacketMinSize {
		return nil, fmt.Errorf("%w: packet doesn't reach minimum heartbeat packet size of %d", ErrMalformedPacket, HeartbeatPacketMinSize)
	}

	if len(data) > HeartbeatPacketMaxSize {
		return nil, fmt.Errorf("%w: packet exceeds maximum heartbeat packet size of %d", ErrMalformedPacket, HeartbeatPacketMaxSize)
	}

	// sanity check: packet len - min packet % id size = 0,
	// since we're only dealing with IDs from that offset
	if (len(data)-HeartbeatPacketMinSize)%HeartbeatPacketPeerIDSize != 0 {
		return nil, fmt.Errorf("%w: heartbeat packet is malformed since the data length after the min. packet size offset isn't conforming with peer ID sizes", ErrMalformedPacket)
	}

	// copy own ID
	ownID := make([]byte, HeartbeatPacketPeerIDSize)
	copy(ownID, data[:HeartbeatPacketPeerIDSize])

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
	offset := HeartbeatPacketPeerIDSize + HeartbeatPacketOutboundIDCountSize + outboundIDCount*HeartbeatPacketPeerIDSize
	for i := range inboundIDs {
		inboundIDs[i] = make([]byte, HeartbeatPacketPeerIDSize)
		copy(inboundIDs[i], data[offset+i*HeartbeatPacketPeerIDSize:offset+(i+1)*HeartbeatPacketPeerIDSize])
	}

	return &Heartbeat{OwnID: ownID, OutboundIDs: outboundIDs, InboundIDs: inboundIDs}, nil
}

// NewHeartbeatMessage serializes the given heartbeat into a byte slice and adds a tlv header to the packet.
// message = tlv header + serialized packet
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

	// own nodeId
	copy(packet[:HeartbeatPacketPeerIDSize], hb.OwnID[:])

	// outbound id count
	packet[HeartbeatPacketPeerIDSize] = byte(len(hb.OutboundIDs))

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

	// create a buffer for tlv header plus the packet
	buf := bytes.NewBuffer(make([]byte, 0, tlv.HeaderMessageDefinition.MaxBytesLength+uint16(packetSize)))
	// write tlv header into buffer
	if err := tlv.WriteHeader(buf, MessageTypeHeartbeat, uint16(packetSize)); err != nil {
		return nil, err
	}
	// write serialized packet bytes into the buffer
	if err := binary.Write(buf, binary.BigEndian, packet); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}
