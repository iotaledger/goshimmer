package packet

import (
	"sync"

	"github.com/cockroachdb/errors"
	"github.com/iotaledger/hive.go/core/protocol/message"
	"github.com/iotaledger/hive.go/core/protocol/tlv"
)

// ErrMalformedPacket is returned when malformed packets are tried to be parsed.
var ErrMalformedPacket = errors.New("malformed packet")

var (
	// analysisBlkRegistry holds all block definitions for analysis server related blocks
	analysisBlkRegistry *message.Registry
	heartBeatOnce       sync.Once
)

func init() {
	// block definitions to be registered in registry
	definitions := []*message.Definition{
		tlv.HeaderMessageDefinition,
		HeartBeatBlockDefinition(),
		MetricHeartbeatBlockDefinition,
	}
	analysisBlkRegistry = message.NewRegistry(definitions)
}

// AnalysisBlkRegistry gets the analysisBlkRegistry.
func AnalysisBlkRegistry() *message.Registry {
	return analysisBlkRegistry
}
