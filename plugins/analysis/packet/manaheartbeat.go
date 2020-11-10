package packet

import (
	"bytes"
	"encoding/binary"
	"encoding/gob"
	"errors"

	"github.com/iotaledger/hive.go/identity"

	"github.com/iotaledger/goshimmer/packages/mana"
	"github.com/iotaledger/goshimmer/plugins/banner"
	"github.com/iotaledger/hive.go/protocol/message"
	"github.com/iotaledger/hive.go/protocol/tlv"
)

var (
	// ErrInvalidManaHeartbeat is returned for invalid Mana heartbeats.
	ErrInvalidManaHeartbeat = errors.New("invalid Mana heartbeat")
	// ErrInvalidManaHeartbeatVersion is returned for invalid mana heartbeat versions.
	ErrInvalidManaHeartbeatVersion = errors.New("invalid mana heartbeat version")
)

// ManaHeartbeat represents a mana heartbeat.
type ManaHeartbeat struct {
	Version          string
	NetworkMap       map[mana.Type]mana.NodeMap
	OnlineNetworkMap map[mana.Type][]mana.Node
	PledgeEvents     []mana.PledgedEvent
	RevokeEvents     []mana.RevokedEvent
	NodeID           identity.ID
}

// ManaHeartbeatMessageDefinition gets the manaHeartbeatMessageDefinition.
func ManaHeartbeatMessageDefinition() *message.Definition {
	var def *message.Definition
	manaHeartBeatOnce.Do(func() {
		def = &message.Definition{
			ID:             MessageTypeManaHeartbeat,
			MaxBytesLength: 65535,
			VariableLength: true,
		}
	})
	return def
}

// Bytes returns the mana heartbeat encoded as bytes.
func (hb ManaHeartbeat) Bytes() ([]byte, error) {
	buf := new(bytes.Buffer)
	encoder := gob.NewEncoder(buf)
	err := encoder.Encode(hb)
	if err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// ParseManaHeartbeat parses a slice of bytes (serialized packet) into a FPC heartbeat.
func ParseManaHeartbeat(data []byte) (*ManaHeartbeat, error) {
	hb := &ManaHeartbeat{}

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

	if hb.Version != banner.AppVersion {
		return nil, ErrInvalidFPCHeartbeatVersion
	}

	return hb, nil
}

// NewManaHeartbeatMessage serializes the given mana heartbeat into a byte slice and adds a tlv header to the packet.
// message = tlv header + serialized packet
func NewManaHeartbeatMessage(hb *ManaHeartbeat) ([]byte, error) {
	packet, err := hb.Bytes()
	if err != nil {
		return nil, err
	}

	// calculate total needed bytes based on packet
	packetSize := len(packet)

	// create a buffer for tlv header plus the packet
	buf := bytes.NewBuffer(make([]byte, 0, tlv.HeaderMessageDefinition.MaxBytesLength+uint16(packetSize)))
	// write tlv header into buffer
	if err := tlv.WriteHeader(buf, MessageTypeManaHeartbeat, uint16(packetSize)); err != nil {
		return nil, err
	}
	// write serialized packet bytes into the buffer
	if err := binary.Write(buf, binary.BigEndian, packet); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}
