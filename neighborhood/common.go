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

// // packetHash returns the hash of a packet
// func packetHash(data []byte) []byte {
// 	sum := sha256.Sum256(data)
// 	return sum[:]
// }
