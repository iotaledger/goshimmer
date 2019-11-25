package selection

import (
	"time"

	"go.uber.org/zap"
)

const (
	UpdateOutboundInterval = 1000 * time.Millisecond

	InboundRequestQueue = 1000
	DropQueue           = 1000

	InboundNeighborSize  = 4
	OutboundNeighborSize = 4

	InboundSelectionDisabled  = false
	OutboundSelectionDisabled = false

	DropNeighborsOnUpdate = false

	SaltLifetime = 30 * time.Minute
)

// Config holds settings for the peer selection.
type Config struct {
	// Logger
	Log *zap.SugaredLogger

	Param *Parameters
}

type Parameters struct {
	SaltLifetime              time.Duration // Lifetime of the local public and private salts
	UpdateOutboundInterval    time.Duration
	InboundRequestQueue       int
	DropQueue                 int
	InboundNeighborSize       int
	OutboundNeighborSize      int
	InboundSelectionDisabled  bool
	OutboundSelectionDisabled bool
	DropNeighborsOnUpdate     bool // Whether all the neighbors are dropped when the distance is updated
}
