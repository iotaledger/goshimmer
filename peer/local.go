package peer

import (
	"crypto/ed25519"
	"sync"

	"github.com/wollac/autopeering/salt"
)

// Local defines the struct of a local peer
type Local struct {
	id  ID
	key ed25519.PrivateKey
	db  DB

	// everything below is protected by a lock
	mu          sync.RWMutex
	publicSalt  *salt.Salt
	privateSalt *salt.Salt
}

// NewLocal returns a new Local peer with a newly generated private identity
func NewLocal(key ed25519.PrivateKey, db DB) *Local {
	publicKey := PublicKey(key.Public().(ed25519.PublicKey))
	return &Local{
		id:  publicKey.ID(),
		key: key,
		db:  db,
	}
}

// ID returns the local node ID.
func (l *Local) ID() ID {
	return l.id
}

// PublicKey returns the public key of the local node.
func (l *Local) PublicKey() []byte {
	return l.key.Public().(ed25519.PublicKey)
}

// Database returns the node database associated with the local node.
func (l *Local) Database() DB {
	return l.db
}

// Sign signs the message with the local node's private key and returns a signature.
func (l *Local) Sign(message []byte) []byte {
	return ed25519.Sign(l.key, message)
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

// GeneratePrivateKey generates a private key that can be used for Local.
func GeneratePrivateKey() (priv ed25519.PrivateKey, err error) {
	_, priv, err = ed25519.GenerateKey(nil)
	return
}
