package packet

import (
	"errors"
	"sync"

	"github.com/iotaledger/hive.go/protocol/message"
	"github.com/iotaledger/hive.go/protocol/tlv"
)

var (
	// ErrMalformedPacket is returned when malformed packets are tried to be parsed.
	ErrMalformedPacket = errors.New("malformed packet")
)

var (
	// analysisMsgRegistry holds all message definitions for analysis server related messages
	analysisMsgRegistry *message.Registry
	fpcHeartBeatOnce    sync.Once
	heartBeatOnce       sync.Once
)

func init() {
	// message definitions to be registered in registry
	definitions := []*message.Definition{
		tlv.HeaderMessageDefinition,
		HeartBeatMessageDefinition(),
		FPCHeartbeatMessageDefinition(),
		MetricHeartbeatMessageDefinition,
	}
	analysisMsgRegistry = message.NewRegistry(definitions)
}

// AnalysisMsgRegistry gets the analysisMsgRegistry.
func AnalysisMsgRegistry() *message.Registry {
	return analysisMsgRegistry
}
