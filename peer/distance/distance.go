package distance

import (
	"crypto/sha256"

	"github.com/wollac/autopeering/peer"
)

type DistanceFunction func([]byte, []byte) []byte

func MakeFromAnchor(anchor *peer.Peer, df DistanceFunction) func(p *peer.Peer) []byte {
	saltedIdentifier := make([]byte, len(anchor.Identity.ID)+len(anchor.Salt.Bytes))
	copy(saltedIdentifier[0:], anchor.Identity.ID)
	copy(saltedIdentifier[len(anchor.Identity.ID):], anchor.Salt.Bytes)
	return func(p *peer.Peer) []byte {
		return df(saltedIdentifier, p.Identity.ID)
	}
}

func ByXor(a, b []byte) []byte {
	return xor(sha256.Sum256(a), sha256.Sum256(b))
}

func xor(v1, v2 [32]byte) []byte {
	r := make([]byte, 32)
	for i := 0; i < len(v1); i++ {
		r[i] = v1[i] ^ v2[i]
	}
	return r
}
