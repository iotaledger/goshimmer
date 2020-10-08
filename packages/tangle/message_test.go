package tangle

import (
	"testing"
	"time"

	"github.com/iotaledger/goshimmer/packages/tangle/payload"
	"github.com/iotaledger/hive.go/crypto/ed25519"
	"github.com/iotaledger/hive.go/identity"
	"github.com/iotaledger/hive.go/kvstore/mapdb"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMessage_VerifySignature(t *testing.T) {
	keyPair := ed25519.GenerateKeyPair()
	pl := payload.NewData([]byte("test"))

	unsigned := NewMessage(EmptyMessageID, EmptyMessageID, time.Time{}, keyPair.PublicKey, 0, pl, 0, ed25519.Signature{})
	assert.False(t, unsigned.VerifySignature())

	unsignedBytes := unsigned.Bytes()
	signature := keyPair.PrivateKey.Sign(unsignedBytes[:len(unsignedBytes)-ed25519.SignatureSize])

	signed := NewMessage(EmptyMessageID, EmptyMessageID, time.Time{}, keyPair.PublicKey, 0, pl, 0, signature)
	assert.True(t, signed.VerifySignature())
}

func TestMessage_MarshalUnmarshal(t *testing.T) {
	msgFactory := NewMessageFactory(mapdb.NewMapDB(), []byte(DBSequenceNumber), identity.GenerateLocalIdentity(), NewMessageTipSelector())
	defer msgFactory.Shutdown()

	testMessage, err := msgFactory.IssuePayload(payload.NewData([]byte("test")))
	require.NoError(t, err)
	assert.Equal(t, true, testMessage.VerifySignature())

	t.Log(testMessage)

	restoredMessage, _, err := MessageFromBytes(testMessage.Bytes())
	if assert.NoError(t, err, err) {
		assert.Equal(t, testMessage.ID(), restoredMessage.ID())
		assert.Equal(t, testMessage.Parent1ID(), restoredMessage.Parent1ID())
		assert.Equal(t, testMessage.Parent2ID(), restoredMessage.Parent2ID())
		assert.Equal(t, testMessage.IssuerPublicKey(), restoredMessage.IssuerPublicKey())
		assert.Equal(t, testMessage.IssuingTime().Round(time.Second), restoredMessage.IssuingTime().Round(time.Second))
		assert.Equal(t, testMessage.SequenceNumber(), restoredMessage.SequenceNumber())
		assert.Equal(t, testMessage.Nonce(), restoredMessage.Nonce())
		assert.Equal(t, testMessage.Signature(), restoredMessage.Signature())
		assert.Equal(t, true, restoredMessage.VerifySignature())
	}
}
