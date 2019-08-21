package discover

import (
	"crypto/sha256"
)

// packetHash returns the hash of a packet
func packetHash(data []byte) []byte {
	sum := sha256.Sum256(data)
	return sum[:]
}
