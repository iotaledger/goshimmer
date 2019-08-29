package peer

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/wollac/autopeering/id"
)

func newTestPeer() *Peer {
	prv := id.GeneratePrivate()
	p := &Peer{}
	p.Identity, _ = id.NewIdentity(prv.PublicKey)
	p.Address = "127.0.0.1:8000"
	return p
}

func TestMarshalUnmarshal(t *testing.T) {
	p := newTestPeer()
	data, err := Marshal(p)
	require.Equal(t, nil, err, p)

	got := &Peer{}
	err = Unmarshal(data, got)
	require.Equal(t, nil, err, p)

	assert.Equal(t, p.Identity, got.Identity, "Identity")

	assert.Equal(t, p.Address, got.Address, "Address")
}
