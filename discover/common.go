package discover

import (
	"crypto/sha256"

	"github.com/wollac/autopeering/peer"
	"github.com/wollac/autopeering/salt"
	"go.uber.org/zap"
)

// Config holds discovery related settings.
type Config struct {
	// These settings are required and configure the listener:
	Log *zap.SugaredLogger

	// These settings are optional:
	Bootnodes []*peer.Peer // list of bootstrap nodes

	AcceptRequest func(*peer.Peer, *salt.Salt) bool
	DropReceived  chan<- peer.ID
}

// packetHash returns the hash of a packet
func packetHash(data []byte) []byte {
	sum := sha256.Sum256(data)
	return sum[:]
}
