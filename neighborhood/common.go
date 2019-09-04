package neighborhood

import (
	"github.com/wollac/autopeering/peer"
	"go.uber.org/zap"
)

// Config holds discovery related settings.
type Config struct {
	// These settings are required and configure the listener:
	Log *zap.SugaredLogger

	GetKnownPeers func() []*peer.Peer
}

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
