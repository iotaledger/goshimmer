package selection

import (
	"time"

	"github.com/iotaledger/goshimmer/packages/autopeering/peer/service"
	"github.com/iotaledger/hive.go/logger"
)

// Default values for the global parameters
const (
	DefaultInboundNeighborSize    = 4
	DefaultOutboundNeighborSize   = 4
	DefaultSaltLifetime           = 30 * time.Minute
	DefaultUpdateOutboundInterval = 10 * time.Second
)

var (
	inboundNeighborSize    = DefaultInboundNeighborSize    // number of inbound neighbors
	outboundNeighborSize   = DefaultOutboundNeighborSize   // number of outbound neighbors
	saltLifetime           = DefaultSaltLifetime           // lifetime of the private and public local salt
	updateOutboundInterval = DefaultUpdateOutboundInterval // time after which the outbound neighbors are updated
)

// Config holds settings for the peer selection.
type Config struct {
	// Logger
	Log *logger.Logger

	// These settings are optional:
	DropOnUpdate     bool          // set true to drop all neighbors when the salt is updated
	RequiredServices []service.Key // services required in order to select/be selected during autopeering
}

// Parameters holds the parameters that can be configured.
type Parameters struct {
	InboundNeighborSize    int           // number of inbound neighbors
	OutboundNeighborSize   int           // number of outbound neighbors
	SaltLifetime           time.Duration // lifetime of the private and public local salt
	UpdateOutboundInterval time.Duration // time interval after which the outbound neighbors are checked
}

// SetParameters sets the global parameters for this package.
// This function cannot be used concurrently.
func SetParameters(param Parameters) {
	if param.InboundNeighborSize > 0 {
		inboundNeighborSize = param.InboundNeighborSize
	}
	if param.OutboundNeighborSize > 0 {
		outboundNeighborSize = param.OutboundNeighborSize
	}
	if param.SaltLifetime > 0 {
		saltLifetime = param.SaltLifetime
	}
	if param.UpdateOutboundInterval > 0 {
		updateOutboundInterval = param.UpdateOutboundInterval
	}
}
