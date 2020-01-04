package proto

import (
	"github.com/golang/protobuf/proto"
	"github.com/iotaledger/goshimmer/packages/autopeering/server"
)

// MType is the type of message type enum.
type MType = server.MType

// An enum for the different message types.
const (
	MPeeringRequest MType = 20 + iota
	MPeeringResponse
	MPeeringDrop
)

// Message extends the proto.Message interface with additional util functions.
type Message interface {
	proto.Message

	// Name returns the name of the corresponding message type for debugging.
	Name() string
	// Type returns the type of the corresponding message as an enum.
	Type() MType
}

func (m *PeeringRequest) Name() string { return "PEERING_REQUEST" }
func (m *PeeringRequest) Type() MType  { return MPeeringRequest }

func (m *PeeringResponse) Name() string { return "PEERING_RESPONSE" }
func (m *PeeringResponse) Type() MType  { return MPeeringResponse }

func (m *PeeringDrop) Name() string { return "PEERING_DROP" }
func (m *PeeringDrop) Type() MType  { return MPeeringDrop }
