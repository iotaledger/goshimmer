package discover

import (
	"crypto/sha256"

	"github.com/wollac/autopeering/identity"
	log "go.uber.org/zap"
)

// Config holds discovery related settings.
type Config struct {
	// These settings are required and configure the listener:
	ID *identity.PrivateIdentity

	// These settings are optional:
	Log *log.Logger
}

// packetHash returns the hash of a packet
func packetHash(data []byte) []byte {
	sum := sha256.Sum256(data)
	return sum[:]
}
