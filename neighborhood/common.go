package neighborhood

import (
	"time"

	"github.com/wollac/autopeering/peer"
	"go.uber.org/zap"
)

// Config holds settings for the peer selection.
type Config struct {
	// Logger
	Log *zap.SugaredLogger

	// Function to retrieve all the known peers
	PeersFunc func() []*peer.Peer

	// Salt lifetime
	SaltLifetime time.Duration
}
