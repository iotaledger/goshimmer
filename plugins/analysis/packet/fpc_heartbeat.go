package packet

import (
	"bytes"
	"encoding/binary"
	"encoding/gob"
	"errors"

	"github.com/iotaledger/goshimmer/packages/vote"
	"github.com/iotaledger/goshimmer/packages/vote/opinion"
	"github.com/iotaledger/goshimmer/plugins/banner"
	"github.com/iotaledger/hive.go/protocol/message"
	"github.com/iotaledger/hive.go/protocol/tlv"
)

var (
	// ErrInvalidFPCHeartbeat is returned for invalid FPC heartbeats.
	ErrInvalidFPCHeartbeat = errors.New("invalid FPC heartbeat")
	// ErrInvalidFPCHeartbeatVersion is returned for invalid FPC heartbeat versions.
	ErrInvalidFPCHeartbeatVersion = errors.New("invalid FPC heartbeat version")
)

// FPCHeartbeat represents a heartbeat packet.
type FPCHeartbeat struct {
	// The version of GoShimmer.
	Version string
	// The ID of the node who sent the heartbeat.
	// Must be contained when a heartbeat is serialized.
	OwnID []byte
	// RoundStats contains stats about an FPC round.
	RoundStats vote.RoundStats
	// Finalized contains the finalized conflicts within the last FPC round.
	Finalized map[string]opinion.Opinion
}

// FPCHeartbeatMessageDefinition gets the fpcHeartbeatMessageDefinition.
func FPCHeartbeatMessageDefinition() *message.Definition {
	// fpcHeartbeatMessageDefinition defines a heartbeat message's format.
	var fpcHeartbeatMessageDefinition *message.Definition
	fpcHeartBeatOnce.Do(func() {
		fpcHeartbeatMessageDefinition = &message.Definition{
			ID:             MessageTypeFPCHeartbeat,
			MaxBytesLength: 65535,
			VariableLength: true,
		}
	})
	return fpcHeartbeatMessageDefinition
}

// ParseFPCHeartbeat parses a slice of bytes (serialized packet) into a FPC heartbeat.
func ParseFPCHeartbeat(data []byte) (*FPCHeartbeat, error) {
	hb := &FPCHeartbeat{}

	buf := new(bytes.Buffer)
	_, err := buf.Write(data)
	if err != nil {
		return nil, err
	}

	decoder := gob.NewDecoder(buf)
	err = decoder.Decode(hb)
	if err != nil {
		return nil, err
	}

	if hb.Version != banner.SimplifiedAppVersion {
		return nil, ErrInvalidFPCHeartbeatVersion
	}

	return hb, nil
}

// Bytes return the FPC heartbeat encoded as bytes
func (hb FPCHeartbeat) Bytes() ([]byte, error) {
	buf := new(bytes.Buffer)
	encoder := gob.NewEncoder(buf)
	err := encoder.Encode(hb)
	if err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// NewFPCHeartbeatMessage serializes the given FPC heartbeat into a byte slice and adds a tlv header to the packet.
// message = tlv header + serialized packet
func NewFPCHeartbeatMessage(hb *FPCHeartbeat) ([]byte, error) {
	packet, err := hb.Bytes()
	if err != nil {
		return nil, err
	}

	// calculate total needed bytes based on packet
	packetSize := len(packet)

	// create a buffer for tlv header plus the packet
	buf := bytes.NewBuffer(make([]byte, 0, tlv.HeaderMessageDefinition.MaxBytesLength+uint16(packetSize)))
	// write tlv header into buffer
	if err := tlv.WriteHeader(buf, MessageTypeFPCHeartbeat, uint16(packetSize)); err != nil {
		return nil, err
	}
	// write serialized packet bytes into the buffer
	if err := binary.Write(buf, binary.BigEndian, packet); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}
