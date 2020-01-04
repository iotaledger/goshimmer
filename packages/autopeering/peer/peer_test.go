package peer

import (
	"crypto/ed25519"
	"testing"

	"github.com/iotaledger/goshimmer/packages/autopeering/peer/service"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	testNetwork = "udp"
	testAddress = "127.0.0.1:8000"
	testMessage = "Hello World!"
)

func newTestServiceRecord() *service.Record {
	services := service.New()
	services.Update(service.PeeringKey, testNetwork, testAddress)

	return services
}

func newTestPeer() *Peer {
	key := make([]byte, ed25519.PublicKeySize)
	return NewPeer(key, newTestServiceRecord())
}

func TestNoServicePeer(t *testing.T) {
	key := make([]byte, ed25519.PublicKeySize)
	services := service.New()

	assert.Panics(t, func() {
		_ = NewPeer(key, services)
	})
}

func TestInvalidServicePeer(t *testing.T) {
	key := make([]byte, ed25519.PublicKeySize)
	services := service.New()
	services.Update(service.FPCKey, "network", "address")

	assert.Panics(t, func() {
		_ = NewPeer(key, services)
	})
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

	sig := ed25519.Sign(priv, msg)

	d := signedData{pub: pub, msg: msg, sig: sig}
	key, err := RecoverKeyFromSignedData(d)
	require.NoError(t, err)

	assert.Equal(t, PublicKey(pub).ID(), key.ID())
}

type signedData struct {
	pub, msg, sig []byte
}

func (d signedData) GetPublicKey() []byte { return d.pub }
func (d signedData) GetData() []byte      { return d.msg }
func (d signedData) GetSignature() []byte { return d.sig }
