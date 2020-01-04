package selection

import (
	"time"

	"github.com/iotaledger/goshimmer/packages/autopeering/peer/service"
	"go.uber.org/zap"
)

// Config holds settings for the peer selection.
type Config struct {
	// Logger
	Log *zap.SugaredLogger

	// These settings are optional:
	Param *Parameters // parameters
}

// default parameter values
const (
	// DefaultInboundNeighborSize is the default number of inbound neighbors.
	DefaultInboundNeighborSize = 4
	// DefaultOutboundNeighborSize is the default number of outbound neighbors.
	DefaultOutboundNeighborSize = 4

	// DefaultSaltLifetime is the default lifetime of the private and public local salt.
	DefaultSaltLifetime = 30 * time.Minute

	// DefaultUpdateOutboundInterval is the default time interval after which the outbound neighbors are checked.
	DefaultUpdateOutboundInterval = 200 * time.Millisecond
)

// Parameters holds the parameters that can be configured.
type Parameters struct {
	InboundNeighborSize    int           // number of inbound neighbors
	OutboundNeighborSize   int           // number of outbound neighbors
	SaltLifetime           time.Duration // lifetime of the private and public local salt
	UpdateOutboundInterval time.Duration // time interval after which the outbound neighbors are checked
	DropNeighborsOnUpdate  bool          // set true to drop all neighbors when the distance is updated
	RequiredService        []service.Key // services required in order to select/be selected during autopeering
}
