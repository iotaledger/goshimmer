package packet

import (
	"errors"

	"github.com/iotaledger/hive.go/protocol/message"
	"github.com/iotaledger/hive.go/protocol/tlv"
)

var (
	// ErrMalformedPacket is returned when malformed packets are tried to be parsed.
	ErrMalformedPacket = errors.New("malformed packet")
)

// AnalysisMsgRegistry holds all message definitions for analysis server related messages
var AnalysisMsgRegistry *message.Registry

func init() {
	// message definitions to be registered in registry
	definitions := []*message.Definition{
		tlv.HeaderMessageDefinition,
		HeartbeatMessageDefinition,
		FPCHeartbeatMessageDefinition,
	}
	AnalysisMsgRegistry = message.NewRegistry(definitions)
}
