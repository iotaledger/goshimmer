package distance

import (
	"crypto/sha256"

	"github.com/wollac/autopeering/peer"
	"github.com/wollac/autopeering/salt"
)

// type Distancer interface {
// 	Distance(a, b interface{}) []byte
// }

// type BySalt struct {
// 	salt salt.Salt
// }

// func (s BySalt) SetSalt(salt salt.Salt) {
// 	s.salt = salt
// }

func BySalt(x, y []byte, s salt.Salt, df DistanceFunction) []byte {
	return df(x, JoinBytes(y, s.Bytes))
}

func BySalt(x, y peer.Peer, s salt.Salt, df DistanceFunction) []byte {
	return df(x.Identity.ID, JoinBytes(y.Identity.ID, s.Bytes))
}

type DistanceFunction func([]byte, []byte) []byte

func MakeChosenFilter(own *peer.Own, df DistanceFunction) func(remote *peer.Peer) []byte {
	return func(remote *peer.Peer) []byte {
		return df(own.Public.Identity.ID, JoinBytes(remote.Identity.ID, own.Public.Salt.Bytes))
	}
}

func MakeAcceptedFilter(own *peer.Own, df DistanceFunction) func(remote *peer.Peer) []byte {
	return func(remote *peer.Peer) []byte {
		return df(own.Public.Identity.ID, JoinBytes(remote.Identity.ID, own.Private.Salt.Bytes))
	}
}

func JoinBytes(a, b []byte) (out []byte) {
	out = make([]byte, len(a)+len(b))
	copy(out[0:], a)
	copy(out[len(a):], b)
	return out
}

func ByXor(a, b []byte) []byte {
	return xor(sha256.Sum256(a), sha256.Sum256(b))
}

func xor(a, b [HashSize]byte) (out []byte) {
	out = make([]byte, HashSize)
	for i := 0; i < HashSize; i++ {
		out[i] = a[i] ^ b[i]
	}
	return out
}

func CompareFixedBytes(a, b [HashSize]byte) int {
	for i := HashSize - 1; i >= 0; i-- {
		switch {
		case a[i] > b[i]:
			return 1
		case a[i] < b[i]:
			return -1
		default:
			continue
		}
	}
	return 0
}
