package discover

import (
	"time"

	"github.com/iotaledger/autopeering-sim/peer"
	"go.uber.org/zap"
)

const (
	ReverifyInterval = 10 * time.Second
	ReverifyTries    = 2

	MaxKnow         = 1000
	MaxReplacements = 10

	PingExpiration     = 12 * time.Hour // time until a peer verification expires
	MaxPeersInResponse = 6              // maximum number of peers returned in DiscoveryResponse
	MaxNumServices     = 5
)

// Config holds discovery related settings.
type Config struct {
	// These settings are required and configure the listener:
	Log *zap.SugaredLogger

	// These settings are optional:
	MasterPeers []*peer.Peer // list of master peers used for bootstrapping

	Param *Parameters
}

type Parameters struct {
	ReverifyInterval time.Duration
	ReverifyTries    int
	MaxKnow          int
	MaxReplacements  int
	PingExpiration   time.Duration
}
