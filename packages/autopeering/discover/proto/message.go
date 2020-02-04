package proto

import (
	"github.com/golang/protobuf/proto"
	"github.com/iotaledger/goshimmer/packages/autopeering/server"
)

// MType is the type of message type enum.
type MType = server.MType

// An enum for the different message types.
const (
	MPing MType = 10 + iota
	MPong
	MDiscoveryRequest
	MDiscoveryResponse
)

// Message extends the proto.Message interface with additional util functions.
type Message interface {
	proto.Message

	// Name returns the name of the corresponding message type for debugging.
	Name() string
	// Type returns the type of the corresponding message as an enum.
	Type() MType
}

func (m *Ping) Name() string { return "PING" }
func (m *Ping) Type() MType  { return MPing }

func (m *Pong) Name() string { return "PONG" }
func (m *Pong) Type() MType  { return MPong }

func (m *DiscoveryRequest) Name() string { return "DISCOVERY_REQUEST" }
func (m *DiscoveryRequest) Type() MType  { return MDiscoveryRequest }

func (m *DiscoveryResponse) Name() string { return "DISCOVERY_RESPONSE" }
func (m *DiscoveryResponse) Type() MType  { return MDiscoveryResponse }
