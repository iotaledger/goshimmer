package proto

import (
	"github.com/golang/protobuf/proto"
)

// PacketType is the type of packet type enum.
type PacketType uint

// An enum for the different packet types.
const (
	PacketMessage PacketType = 20 + iota
	PacketMessageRequest
)

// Packet extends the proto.Message interface with additional util functions.
type Packet interface {
	proto.Message

	// Name returns the name of the corresponding packet type for debugging.
	Name() string
	// Type returns the type of the corresponding packet as an enum.
	Type() PacketType
}

// Name returns the name of the message packet.
func (m *Message) Name() string { return "message" }

// Type returns the packet type id of the message packet.
func (m *Message) Type() PacketType { return PacketMessage }

// Name returns the name of the message request packet.
func (m *MessageRequest) Name() string { return "message_request" }

// Type returns the packet type id of the message request packet.
func (m *MessageRequest) Type() PacketType { return PacketMessageRequest }
