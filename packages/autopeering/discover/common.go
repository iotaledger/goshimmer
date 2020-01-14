package discover

import (
	"time"

	"github.com/iotaledger/goshimmer/packages/autopeering/peer"
	"github.com/iotaledger/hive.go/logger"
)

// Default values for the global parameters
const (
	DefaultReverifyInterval = 10 * time.Second
	DefaultReverifyTries    = 2
	DefaultQueryInterval    = 60 * time.Second
	DefaultMaxManaged       = 1000
	DefaultMaxReplacements  = 10
)

var (
	reverifyInterval = DefaultReverifyInterval // time interval after which the next peer is reverified
	reverifyTries    = DefaultReverifyTries    // number of times a peer is pinged before it is removed
	queryInterval    = DefaultQueryInterval    // time interval after which peers are queried for new peers
	maxManaged       = DefaultMaxManaged       // maximum number of peers that can be managed
	maxReplacements  = DefaultMaxReplacements  // maximum number of peers kept in the replacement list
)

// Config holds discovery related settings.
type Config struct {
	// These settings are required and configure the listener:
	Log *logger.Logger

	// These settings are optional:
	MasterPeers []*peer.Peer // list of master peers used for bootstrapping
}

// Parameters holds the parameters that can be configured.
type Parameters struct {
	ReverifyInterval time.Duration // time interval after which the next peer is reverified
	ReverifyTries    int           // number of times a peer is pinged before it is removed
	QueryInterval    time.Duration // time interval after which peers are queried for new peers
	MaxManaged       int           // maximum number of peers that can be managed
	MaxReplacements  int           // maximum number of peers kept in the replacement list
}

// SetParameters sets the global parameters for this package.
// This function cannot be used concurrently.
func SetParameter(param Parameters) {
	if param.ReverifyInterval > 0 {
		reverifyInterval = param.ReverifyInterval
	} else {
		reverifyInterval = DefaultReverifyInterval
	}
	if param.ReverifyTries > 0 {
		reverifyTries = param.ReverifyTries
	} else {
		reverifyTries = DefaultReverifyTries
	}
	if param.QueryInterval > 0 {
		queryInterval = param.QueryInterval
	} else {
		queryInterval = DefaultQueryInterval
	}
	if param.MaxManaged > 0 {
		maxManaged = param.MaxManaged
	} else {
		maxManaged = DefaultMaxManaged
	}
	if param.MaxReplacements > 0 {
		maxReplacements = param.MaxReplacements
	} else {
		maxReplacements = DefaultMaxReplacements
	}
}
