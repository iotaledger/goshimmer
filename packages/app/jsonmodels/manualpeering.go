package jsonmodels

import "github.com/iotaledger/hive.go/crypto/ed25519"

// PeerToRemove holds the data that uniquely identifies the peer to be removed, e.g. public key or ID.
// Only public key is supported for now.
type PeerToRemove struct {
	PublicKey ed25519.PublicKey `json:"publicKey"`
}
