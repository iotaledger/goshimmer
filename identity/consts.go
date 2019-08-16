package identity

import "golang.org/x/crypto/ed25519"

const (
	PublicKeySize = ed25519.PublicKeySize
	SignatureSize = ed25519.SignatureSize
)
