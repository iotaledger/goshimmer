package selection

import (
	"time"

	"github.com/iotaledger/goshimmer/packages/autopeering/peer/service"
	"github.com/iotaledger/hive.go/logger"
)

// Default values for the global parameters
const (
	DefaultInboundNeighborSize        = 4
	DefaultOutboundNeighborSize       = 4
	DefaultSaltLifetime               = 30 * time.Minute
	DefaultOutboundUpdateInterval     = 1 * time.Second
	DefaultFullOutboundUpdateInterval = 1 * time.Minute
)

var (
	inboundNeighborSize        = DefaultInboundNeighborSize        // number of inbound neighbors
	outboundNeighborSize       = DefaultOutboundNeighborSize       // number of outbound neighbors
	saltLifetime               = DefaultSaltLifetime               // lifetime of the private and public local salt
	outboundUpdateInterval     = DefaultOutboundUpdateInterval     // time after which out neighbors are updated
	fullOutboundUpdateInterval = DefaultFullOutboundUpdateInterval // time after which full out neighbors are updated
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
	InboundNeighborSize        int           // number of inbound neighbors
	OutboundNeighborSize       int           // number of outbound neighbors
	SaltLifetime               time.Duration // lifetime of the private and public local salt
	OutboundUpdateInterval     time.Duration // time interval after which the outbound neighbors are checked
	FullOutboundUpdateInterval time.Duration // time after which the full outbound neighbors are updated
}

// SetParameters sets the global parameters for this package.
// This function cannot be used concurrently.
func SetParameters(param Parameters) {
	if param.InboundNeighborSize > 0 {
		inboundNeighborSize = param.InboundNeighborSize
	} else {
		inboundNeighborSize = DefaultInboundNeighborSize
	}
	if param.OutboundNeighborSize > 0 {
		outboundNeighborSize = param.OutboundNeighborSize
	} else {
		outboundNeighborSize = DefaultOutboundNeighborSize
	}
	if param.SaltLifetime > 0 {
		saltLifetime = param.SaltLifetime
	} else {
		saltLifetime = DefaultSaltLifetime
	}
	if param.OutboundUpdateInterval > 0 {
		outboundUpdateInterval = param.OutboundUpdateInterval
	} else {
		outboundUpdateInterval = DefaultOutboundUpdateInterval
	}
	if param.FullOutboundUpdateInterval > 0 {
		fullOutboundUpdateInterval = param.FullOutboundUpdateInterval
	} else {
		fullOutboundUpdateInterval = DefaultFullOutboundUpdateInterval
	}
}
