package test

import (
	"testing"
	"time"

	"github.com/iotaledger/goshimmer/packages/binary/messagelayer/messagefactory"
	"github.com/iotaledger/goshimmer/packages/binary/messagelayer/tipselector"
	"github.com/iotaledger/goshimmer/plugins/messagelayer"
	"github.com/iotaledger/hive.go/crypto/ed25519"
	"github.com/iotaledger/hive.go/identity"
	"github.com/iotaledger/hive.go/kvstore/mapdb"
	"github.com/stretchr/testify/assert"

	"github.com/iotaledger/goshimmer/packages/binary/messagelayer/message"
	"github.com/iotaledger/goshimmer/packages/binary/messagelayer/payload"
)

func TestMessage_StorableObjectFromKey(t *testing.T) {
	key, err := message.NewId("2DYebCqnZ8PS5PyXBEvAvLB1fCF77Rn9RtofNHjEb2pSTujKi889d31FmguAs5DgL7YURw4GP2Y28JdJ7K4bjudG")
	if err != nil {
		panic(err)
	}

	messageFromKey, consumedBytes, err := message.StorableObjectFromKey(key.Bytes())
	if err != nil {
		panic(err)
	}

	assert.Equal(t, message.IdLength, consumedBytes)
	assert.Equal(t, key, messageFromKey.(*message.Message).Id())
}

func TestMessage_VerifySignature(t *testing.T) {
	keyPair := ed25519.GenerateKeyPair()
	pl := payload.NewData([]byte("test"))

	unsigned := message.New(message.EmptyId, message.EmptyId, time.Time{}, keyPair.PublicKey, 0, pl, 0, ed25519.Signature{})
	assert.False(t, unsigned.VerifySignature())

	unsignedBytes := unsigned.Bytes()
	signature := keyPair.PrivateKey.Sign(unsignedBytes[:len(unsignedBytes)-ed25519.SignatureSize])

	signed := message.New(message.EmptyId, message.EmptyId, time.Time{}, keyPair.PublicKey, 0, pl, 0, signature)
	assert.True(t, signed.VerifySignature())
}

func TestMessage_MarshalUnmarshal(t *testing.T) {
	msgFactory := messagefactory.New(mapdb.NewMapDB(), []byte(messagelayer.DBSequenceNumber), identity.GenerateLocalIdentity(), tipselector.New())
	defer msgFactory.Shutdown()

	testMessage := msgFactory.IssuePayload(payload.NewData([]byte("test")))
	assert.Equal(t, true, testMessage.VerifySignature())

	t.Log(testMessage)

	restoredMessage, err, _ := message.FromBytes(testMessage.Bytes())
	if assert.NoError(t, err, err) {
		assert.Equal(t, testMessage.Id(), restoredMessage.Id())
		assert.Equal(t, testMessage.TrunkId(), restoredMessage.TrunkId())
		assert.Equal(t, testMessage.BranchId(), restoredMessage.BranchId())
		assert.Equal(t, testMessage.IssuerPublicKey(), restoredMessage.IssuerPublicKey())
		assert.Equal(t, testMessage.IssuingTime().Round(time.Second), restoredMessage.IssuingTime().Round(time.Second))
		assert.Equal(t, testMessage.SequenceNumber(), restoredMessage.SequenceNumber())
		assert.Equal(t, testMessage.Nonce(), restoredMessage.Nonce())
		assert.Equal(t, testMessage.Signature(), restoredMessage.Signature())
		assert.Equal(t, true, restoredMessage.VerifySignature())
	}
}
