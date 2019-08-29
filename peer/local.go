package peer

import (
	"sync"

	"github.com/wollac/autopeering/id"
	"github.com/wollac/autopeering/salt"
)

// Local defines the struct of a local peer
type Local struct {
	Private     *id.Private
	Service     ServiceMap
	publicSalt  *salt.Salt
	mPubSalt    sync.RWMutex
	privateSalt *salt.Salt
	mPrivSalt   sync.RWMutex
}

// NewLocal returns a new Local peer with a newly generated private identity
func NewLocal() *Local {
	return &Local{
		Private: id.GeneratePrivate(),
	}
}

// Identity returns the public identity
func (l *Local) Identity() *id.Identity {
	return &l.Private.Identity
}

// GetPublicSalt returns the public salt
func (l *Local) GetPublicSalt() *salt.Salt {
	l.mPubSalt.RLock()
	defer l.mPubSalt.RUnlock()
	return l.publicSalt
}

// SetPublicSalt sets the public salt
func (l *Local) SetPublicSalt(salt *salt.Salt) {
	l.mPubSalt.Lock()
	defer l.mPubSalt.Unlock()
	l.publicSalt = salt
}

// GetPrivateSalt returns the private salt
func (l *Local) GetPrivateSalt() *salt.Salt {
	l.mPrivSalt.RLock()
	defer l.mPrivSalt.RUnlock()
	return l.privateSalt
}

// SetPrivateSalt sets the private salt
func (l *Local) SetPrivateSalt(salt *salt.Salt) {
	l.mPrivSalt.Lock()
	defer l.mPrivSalt.Unlock()
	l.privateSalt = salt
}
