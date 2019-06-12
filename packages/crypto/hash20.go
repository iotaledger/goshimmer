package crypto

import (
	"crypto/sha256"

	"golang.org/x/crypto/ripemd160"
)

func Hash20(input []byte) []byte {
	sha256Hasher := sha256.New()
	sha256Hasher.Write(input)

	ripemd160Hasher := ripemd160.New()
	ripemd160Hasher.Write(sha256Hasher.Sum(nil))

	return ripemd160Hasher.Sum(nil)
}
