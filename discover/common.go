package discover

import (
	"crypto/sha256"

	"github.com/wollac/autopeering/id"
	log "go.uber.org/zap"
)

// Peer is just a dummy until we have a compatible struct
// TODO: remove
type Peer struct {
	Identity *id.Identity // identity of the peer (ID, StringID, PublicKey)
	Address  string       // IP address of the peer (IPv4 or IPv6)
}

func NewPeer(id *id.Identity, addr string) *Peer {
	return &Peer{Identity: id, Address: addr}
}

// Config holds discovery related settings.
type Config struct {
	// These settings are required and configure the listener:
	ID *id.Private

	// These settings are optional:
	Log *log.Logger
}

// packetHash returns the hash of a packet
func packetHash(data []byte) []byte {
	sum := sha256.Sum256(data)
	return sum[:]
}
