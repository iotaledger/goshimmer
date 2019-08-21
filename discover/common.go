package discover

import (
	"crypto/sha256"
)

func packetHash(data []byte) []byte {
	sum := sha256.Sum256(data)
	return sum[:]
}
