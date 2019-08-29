package peer

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/wollac/autopeering/id"
	"github.com/wollac/autopeering/salt"
)

func newTestLocal() *Local {
	prv := id.GeneratePrivate()
	p := &Local{}
	p.Identity, _ = id.NewIdentity(prv.PublicKey)
	p.Address = "127.0.0.1:8000"
	return p
}

func TestLocalPrivateSalt(t *testing.T) {
	p := newTestLocal()

	salt, _ := salt.NewSalt(time.Second * 10)
	p.SetPrivateSalt(salt)

	got := p.GetPrivateSalt()

	assert.Equal(t, salt, got, "Private salt")
}

func TestLocalPublicSalt(t *testing.T) {
	p := newTestLocal()

	salt, _ := salt.NewSalt(time.Second * 10)
	p.SetPublicSalt(salt)

	got := p.GetPublicSalt()

	assert.Equal(t, salt, got, "Public salt")
}
