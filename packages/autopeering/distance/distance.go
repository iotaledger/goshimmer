package distance

import (
	"crypto/sha256"
	"encoding/binary"
)

// BySalt returns the distance (uint32) between x and y
// by xoring the hash of x and y + salt
// xor(hash(x), hash(y+salt))[:4]
func BySalt(x, y, salt []byte) uint32 {
	return xorSHA32(x, joinBytes(y, salt))
}

func joinBytes(a, b []byte) (out []byte) {
	out = make([]byte, len(a)+len(b))
	copy(out[0:], a)
	copy(out[len(a):], b)
	return out
}

func xorSHA32(a, b []byte) uint32 {
	return binary.LittleEndian.Uint32(
		xorSHA(sha256.Sum256(a), sha256.Sum256(b))[:4])
}

func xorSHA(a, b [sha256.Size]byte) (out []byte) {
	out = make([]byte, sha256.Size)
	for i := 0; i < sha256.Size; i++ {
		out[i] = a[i] ^ b[i]
	}
	return out
}
