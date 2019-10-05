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

// sliceUniqMap returns the unique elements from the slice
func sliceUniqMap(s []*peer.Peer) []*peer.Peer {
	seen := make(map[*peer.Peer]struct{}, len(s))
	j := 0
	for _, v := range s {
		if _, ok := seen[v]; ok {
			continue
		}
		seen[v] = struct{}{}
		s[j] = v
		j++
	}
	return s[:j]
}
