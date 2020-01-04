package peer

import (
	"crypto/ed25519"
	"testing"
	"time"

	"github.com/iotaledger/goshimmer/packages/autopeering/peer/service"
	"github.com/iotaledger/goshimmer/packages/autopeering/salt"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestID(t *testing.T) {
	pub, priv, err := ed25519.GenerateKey(nil)
	require.NoError(t, err)

	local := newLocal(PrivateKey(priv), newTestServiceRecord(), nil)
	id := PublicKey(pub).ID()
	assert.Equal(t, id, local.ID())
}

func TestPublicKey(t *testing.T) {
	pub, priv, err := ed25519.GenerateKey(nil)
	require.NoError(t, err)

	local := newLocal(PrivateKey(priv), newTestServiceRecord(), nil)
	assert.EqualValues(t, pub, local.PublicKey())
}

func newTestLocal(t require.TestingT) *Local {
	priv, err := generatePrivateKey()
	require.NoError(t, err)
	return newLocal(priv, newTestServiceRecord(), nil)
}

func TestAddress(t *testing.T) {
	local := newTestLocal(t)

	address := local.Services().Get(service.PeeringKey).String()
	assert.EqualValues(t, address, local.Address())
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
