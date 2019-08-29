package discover

import (
	"crypto/sha256"

	"github.com/wollac/autopeering/id"
	"go.uber.org/zap"
)

type Peer struct {
	Identity *id.Identity // identity of the peer
	Address  string       // address of the peer
}

func NewPeer(id *id.Identity, addr string) *Peer {
	return &Peer{Identity: id, Address: addr}
}

// Config holds discovery related settings.
type Config struct {
	// These settings are required and configure the listener:
	ID  *id.Private
	Log *zap.SugaredLogger

	// These settings are optional:
	Bootnodes []*Peer // list of bootstrap nodes
}

// packetHash returns the hash of a packet
func packetHash(data []byte) []byte {
	sum := sha256.Sum256(data)
	return sum[:]
}
