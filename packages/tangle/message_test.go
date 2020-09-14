package tangle

import (
	"testing"
	"time"

	"github.com/iotaledger/hive.go/crypto/ed25519"
	"github.com/iotaledger/hive.go/identity"
	"github.com/iotaledger/hive.go/kvstore/mapdb"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMessage_FromStorageKey(t *testing.T) {
	key, err := NewMessageID("2DYebCqnZ8PS5PyXBEvAvLB1fCF77Rn9RtofNHjEb2pSTujKi889d31FmguAs5DgL7YURw4GP2Y28JdJ7K4bjudG")
	if err != nil {
		panic(err)
	}

	messageFromKey, consumedBytes, err := MessageFromStorageKey(key.Bytes())
	if err != nil {
		panic(err)
	}

	assert.Equal(t, MessageIDLength, consumedBytes)
	assert.Equal(t, key, messageFromKey.(*Message).ID())
}

func TestMessage_VerifySignature(t *testing.T) {
	keyPair := ed25519.GenerateKeyPair()
	pl := NewDataPayload([]byte("test"))

	unsigned := NewMessage(EmptyMessageID, EmptyMessageID, time.Time{}, keyPair.PublicKey, 0, pl, 0, ed25519.Signature{})
	assert.False(t, unsigned.VerifySignature())

	unsignedBytes := unsigned.Bytes()
	signature := keyPair.PrivateKey.Sign(unsignedBytes[:len(unsignedBytes)-ed25519.SignatureSize])

	signed := NewMessage(EmptyMessageID, EmptyMessageID, time.Time{}, keyPair.PublicKey, 0, pl, 0, signature)
	assert.True(t, signed.VerifySignature())
}

func TestMessage_MarshalUnmarshal(t *testing.T) {
	msgFactory := NewFactory(mapdb.NewMapDB(), []byte(DBSequenceNumber), identity.GenerateLocalIdentity(), NewMessageTipSelector())
	defer msgFactory.Shutdown()

	testMessage, err := msgFactory.IssuePayload(NewDataPayload([]byte("test")))
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
