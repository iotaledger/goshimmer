package peer

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/ed25519"
)

const (
	testAddress = "127.0.0.1:8000"
	testMessage = "Hello World!"
)

func newTestPeer() *Peer {
	key := make([]byte, ed25519.PublicKeySize)
	return NewPeer(key, testAddress)
}

func TestMarshalUnmarshal(t *testing.T) {
	p := newTestPeer()
	data, err := p.Marshal()
	require.NoError(t, err)

	got, err := Unmarshal(data)
	require.NoError(t, err)

	assert.Equal(t, p, got)
}

func TestRecoverKeyFromSignedData(t *testing.T) {
	msg := []byte(testMessage)

	pub, priv, err := ed25519.GenerateKey(nil)
	require.NoError(t, err)

	local := NewLocal(priv, nil)
	sig := local.Sign(msg)

	d := signedData{pub: pub, msg: msg, sig: sig}
	key, err := RecoverKeyFromSignedData(d)
	require.NoError(t, err)

	assert.Equal(t, local.ID(), key.ID())
}

type signedData struct {
	pub, msg, sig []byte
}

func (d signedData) GetPublicKey() []byte { return d.pub }
func (d signedData) GetData() []byte      { return d.msg }
func (d signedData) GetSignature() []byte { return d.sig }
