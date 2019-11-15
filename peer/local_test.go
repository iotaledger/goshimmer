package peer

import (
	"crypto/ed25519"
	"testing"
	"time"

	"github.com/iotaledger/autopeering-sim/salt"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestID(t *testing.T) {
	pub, priv, err := ed25519.GenerateKey(nil)
	require.NoError(t, err)

	local := NewLocal(priv, nil)
	id := PublicKey(pub).ID()
	assert.Equal(t, id, local.ID())
}

func TestPublicKey(t *testing.T) {
	pub, priv, err := ed25519.GenerateKey(nil)
	require.NoError(t, err)

	local := NewLocal(priv, nil)
	assert.EqualValues(t, pub, local.PublicKey())
}

func newTestLocal(t require.TestingT) *Local {
	priv, err := GeneratePrivateKey()
	require.NoError(t, err)
	return NewLocal(priv, nil)
}

func TestPrivateSalt(t *testing.T) {
	p := newTestLocal(t)

	salt, _ := salt.NewSalt(time.Second * 10)
	p.SetPrivateSalt(salt)

	got := p.GetPrivateSalt()
	assert.Equal(t, salt, got, "Private salt")
}

func TestPublicSalt(t *testing.T) {
	p := newTestLocal(t)

	salt, _ := salt.NewSalt(time.Second * 10)
	p.SetPublicSalt(salt)

	got := p.GetPublicSalt()

	assert.Equal(t, salt, got, "Public salt")
}
