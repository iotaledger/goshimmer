package peer

import (
	"crypto/ed25519"
	"fmt"
	"sync"

	"github.com/wollac/autopeering/salt"
)

// Local defines the struct of a local peer
type Local struct {
	id       ID
	key      PrivateKey
	services ServiceMap
	db       DB

	// everything below is protected by a lock
	mu          sync.RWMutex
	publicSalt  *salt.Salt
	privateSalt *salt.Salt
}

// PrivateKey is the type of Ed25519 private keys used for the local peer.
type PrivateKey ed25519.PrivateKey

// Public returns the PublicKey corresponding to priv.
func (priv PrivateKey) Public() PublicKey {
	publicKey := ed25519.PrivateKey(priv).Public()
	return PublicKey(publicKey.(ed25519.PublicKey))
}

// newLocal creates a new local peer.
func newLocal(key PrivateKey, db DB) *Local {
	publicKey := key.Public()
	return &Local{
		id:       publicKey.ID(),
		key:      key,
		services: NewServiceMap(),
		db:       db,
	}
}

// NewLocal creates a new local peer linked to the provided db.
func NewLocal(db DB) (*Local, error) {
	key, err := db.LocalPrivateKey()
	if err != nil {
		return nil, err
	}
	if l := len(key); l != ed25519.PrivateKeySize {
		return nil, fmt.Errorf("invalid key length: %d, need %d", l, ed25519.PublicKeySize)
	}
	return newLocal(key, db), nil
}

// ID returns the local node ID.
func (l *Local) ID() ID {
	return l.id
}

// PublicKey returns the public key of the local node.
func (l *Local) PublicKey() PublicKey {
	return l.key.Public()
}

// Database returns the node database associated with the local node.
func (l *Local) Database() DB {
	return l.db
}

// Services returns the services supported by the local node.
func (l *Local) Services() ServiceMap {
	return l.services
}

// Sign signs the message with the local node's private key and returns a signature.
func (l *Local) Sign(message []byte) []byte {
	return ed25519.Sign(ed25519.PrivateKey(l.key), message)
}

// GetPublicSalt returns the public salt
func (l *Local) GetPublicSalt() *salt.Salt {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return l.publicSalt
}

// SetPublicSalt sets the public salt
func (l *Local) SetPublicSalt(salt *salt.Salt) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.publicSalt = salt
}

// GetPrivateSalt returns the private salt
func (l *Local) GetPrivateSalt() *salt.Salt {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return l.privateSalt
}

// SetPrivateSalt sets the private salt
func (l *Local) SetPrivateSalt(salt *salt.Salt) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.privateSalt = salt
}

// generatePrivateKey generates a private key that can be used for Local.
func generatePrivateKey() (PrivateKey, error) {
	_, priv, err := ed25519.GenerateKey(nil)
	return PrivateKey(priv), err
}
