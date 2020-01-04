package discover

import (
	"time"

	"github.com/iotaledger/goshimmer/packages/autopeering/peer"
	"go.uber.org/zap"
)

const (
	// PingExpiration is the time until a peer verification expires.
	PingExpiration = 12 * time.Hour
	// MaxPeersInResponse is the maximum number of peers returned in DiscoveryResponse.
	MaxPeersInResponse = 6
	// MaxServices is the maximum number of services a peer can support.
	MaxServices = 5

	// VersionNum specifies the expected version number for this Protocol.
	VersionNum = 0
)

// Config holds discovery related settings.
type Config struct {
	// These settings are required and configure the listener:
	Log *zap.SugaredLogger

	// These settings are optional:
	MasterPeers []*peer.Peer // list of master peers used for bootstrapping
	Param       *Parameters  // parameters
}

// default parameter values
const (
	// DefaultReverifyInterval is the default time interval after which a new peer is reverified.
	DefaultReverifyInterval = 10 * time.Second
	// DefaultReverifyTries is the default number of times a peer is pinged before it is removed.
	DefaultReverifyTries = 2

	// DefaultQueryInterval is the default time interval after which known peers are queried for new peers.
	DefaultQueryInterval = 60 * time.Second

	// DefaultMaxManaged is the default maximum number of peers that can be managed.
	DefaultMaxManaged = 1000
	// DefaultMaxReplacements is the default maximum number of peers kept in the replacement list.
	DefaultMaxReplacements = 10
)

// Parameters holds the parameters that can be configured.
type Parameters struct {
	ReverifyInterval time.Duration // time interval after which the next peer is reverified
	ReverifyTries    int           // number of times a peer is pinged before it is removed
	QueryInterval    time.Duration // time interval after which peers are queried for new peers
	MaxManaged       int           // maximum number of peers that can be managed
	MaxReplacements  int           // maximum number of peers kept in the replacement list
}
