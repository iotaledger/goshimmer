package jsonmodels

// PeerToRemove holds the data that uniquely identifies the peer to be removed, e.g. public key or ID.
// Only public key is supported for now.
type PeerToRemove struct {
	PublicKey string `json:"publicKey"`
}
