package identity

import "golang.org/x/crypto/ed25519"

const (
	// PublicKeySize is the length of the public key in bytes.
	PublicKeySize = ed25519.PublicKeySize
	// SignatureSize is the length of the public key in bytes.
	SignatureSize = ed25519.SignatureSize
)
