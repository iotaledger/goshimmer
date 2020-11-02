package tangle

import (
	"bytes"
	"crypto/rand"
	"sort"
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
	pl := payload.NewGenericDataPayload([]byte("test"))

	unsigned := NewMessage([]MessageID{EmptyMessageID}, []MessageID{}, time.Time{}, keyPair.PublicKey, 0, pl, 0, ed25519.Signature{})
	assert.False(t, unsigned.VerifySignature())

	unsignedBytes := unsigned.Bytes()
	signature := keyPair.PrivateKey.Sign(unsignedBytes[:len(unsignedBytes)-ed25519.SignatureSize])

	signed := NewMessage([]MessageID{EmptyMessageID}, []MessageID{}, time.Time{}, keyPair.PublicKey, 0, pl, 0, signature)
	assert.True(t, signed.VerifySignature())
}

func TestMessage_MarshalUnmarshal(t *testing.T) {
	msgFactory := NewMessageFactory(mapdb.NewMapDB(), []byte(DBSequenceNumber), identity.GenerateLocalIdentity(), NewMessageTipSelector())
	defer msgFactory.Shutdown()

	testMessage, err := msgFactory.IssuePayload(payload.NewGenericDataPayload([]byte("test")))
	require.NoError(t, err)
	assert.Equal(t, true, testMessage.VerifySignature())

	t.Log(testMessage)

	restoredMessage, _, err := MessageFromBytes(testMessage.Bytes())
	if assert.NoError(t, err, err) {
		assert.Equal(t, testMessage.ID(), restoredMessage.ID())
		assert.ElementsMatch(t, testMessage.StrongParents(), restoredMessage.StrongParents())
		assert.ElementsMatch(t, testMessage.WeakParents(), restoredMessage.WeakParents())
		assert.Equal(t, testMessage.IssuerPublicKey(), restoredMessage.IssuerPublicKey())
		assert.Equal(t, testMessage.IssuingTime().Round(time.Second), restoredMessage.IssuingTime().Round(time.Second))
		assert.Equal(t, testMessage.SequenceNumber(), restoredMessage.SequenceNumber())
		assert.Equal(t, testMessage.Nonce(), restoredMessage.Nonce())
		assert.Equal(t, testMessage.Signature(), restoredMessage.Signature())
		assert.Equal(t, true, restoredMessage.VerifySignature())
	}
}

func randomParents(count int) []MessageID {
	parents := make([]MessageID, 0, count)
	for i := 0; i < count; i++ {
		b := make([]byte, MessageIDLength)
		_, _ = rand.Read(b)
		randID, _, _ := MessageIDFromBytes(b)
		parents = append(parents, randID)
	}
	return parents
}

func TestMessage_NewMessageTooManyParents(t *testing.T) {
	// too many strong parents
	strongParents := randomParents(MaxParentsCount + 1)

	assert.Panics(t, func() {
		NewMessage(
			strongParents,
			nil,
			time.Now(),
			ed25519.PublicKey{},
			0,
			payload.NewGenericDataPayload([]byte("")),
			0,
			ed25519.Signature{},
		)
	})

	// strong and weak parents are individually okay, but, their sum is bigger
	strongParents = randomParents(MaxParentsCount - 1)

	weakParents := randomParents(2)

	assert.Panics(t, func() {
		NewMessage(
			strongParents,
			weakParents,
			time.Now(),
			ed25519.PublicKey{},
			0,
			payload.NewGenericDataPayload([]byte("")),
			0,
			ed25519.Signature{},
		)
	})
}

func TestMessage_NewMessageNotEnoughParents(t *testing.T) {
	// no parents at all
	assert.Panics(t, func() {
		NewMessage(
			nil,
			nil,
			time.Now(),
			ed25519.PublicKey{},
			0,
			payload.NewGenericDataPayload([]byte("")),
			0,
			ed25519.Signature{},
		)
	})

	// only one weak parent
	assert.Panics(t, func() {
		NewMessage(
			nil,
			[]MessageID{EmptyMessageID},
			time.Now(),
			ed25519.PublicKey{},
			0,
			payload.NewGenericDataPayload([]byte("")),
			0,
			ed25519.Signature{},
		)
	})
}

func TestMessage_NewMessage(t *testing.T) {
	// minimum number of parents supplied
	assert.NotPanics(t, func() {
		NewMessage(
			[]MessageID{EmptyMessageID},
			nil,
			time.Now(),
			ed25519.PublicKey{},
			0,
			payload.NewGenericDataPayload([]byte("")),
			0,
			ed25519.Signature{},
		)
	})

	// max number of parents supplied (only strong)
	strongParents := randomParents(MaxParentsCount)
	assert.NotPanics(t, func() {
		NewMessage(
			strongParents,
			nil,
			time.Now(),
			ed25519.PublicKey{},
			0,
			payload.NewGenericDataPayload([]byte("")),
			0,
			ed25519.Signature{},
		)
	})

	// max number of parents supplied (one strong)
	weakParents := randomParents(MaxParentsCount - 1)
	assert.NotPanics(t, func() {
		NewMessage(
			[]MessageID{EmptyMessageID},
			weakParents,
			time.Now(),
			ed25519.PublicKey{},
			0,
			payload.NewGenericDataPayload([]byte("")),
			0,
			ed25519.Signature{},
		)
	})
}

func testAreParentsSorted(parents []MessageID) bool {
	return sort.SliceIsSorted(parents, func(i, j int) bool {
		return bytes.Compare(parents[i].Bytes(), parents[j].Bytes()) < 0
	})
}

func testSortParents(parents []MessageID) {
	sort.Slice(parents, func(i, j int) bool {
		return bytes.Compare(parents[i].Bytes(), parents[j].Bytes()) < 0
	})
}

func TestMessage_NewMessageStrongParentsSorted(t *testing.T) {
	// max number of parents supplied (only strong)
	strongParents := randomParents(MaxParentsCount)
	if !testAreParentsSorted(strongParents) {
		testSortParents(strongParents)
		assert.True(t, testAreParentsSorted(strongParents))
	}

	msg := NewMessage(
		strongParents,
		nil,
		time.Now(),
		ed25519.PublicKey{},
		0,
		payload.NewGenericDataPayload([]byte("")),
		0,
		ed25519.Signature{},
	)

	msgStrongParents := msg.StrongParents()
	assert.True(t, testAreParentsSorted(msgStrongParents))
}

func TestMessage_NewMessageWeakParentsSorted(t *testing.T) {
	// max number of weak parents supplied (MaxParentsCount-1)
	weakParents := randomParents(MaxParentsCount - 1)
	if !testAreParentsSorted(weakParents) {
		testSortParents(weakParents)
		assert.True(t, testAreParentsSorted(weakParents))
	}

	msg := NewMessage(
		[]MessageID{EmptyMessageID},
		weakParents,
		time.Now(),
		ed25519.PublicKey{},
		0,
		payload.NewGenericDataPayload([]byte("")),
		0,
		ed25519.Signature{},
	)

	msgWeakParents := msg.WeakParents()
	assert.True(t, testAreParentsSorted(msgWeakParents))
}

func TestMessage_NewMessageDuplicateStrongParents(t *testing.T) {
	// max number of parents supplied (only strong)
	strongParents := randomParents(MaxParentsCount / 2)

	strongParents = append(strongParents, strongParents...)

	msg := NewMessage(
		strongParents,
		nil,
		time.Now(),
		ed25519.PublicKey{},
		0,
		payload.NewGenericDataPayload([]byte("")),
		0,
		ed25519.Signature{},
	)

	msgStrongParents := msg.StrongParents()
	assert.True(t, testAreParentsSorted(msgStrongParents))
	assert.Equal(t, MaxParentsCount/2, len(msgStrongParents))
}

func TestMessage_NewMessageDuplicateWeakParents(t *testing.T) {
	weakParents := randomParents(3)

	weakParents = append(weakParents, weakParents...)

	msg := NewMessage(
		[]MessageID{EmptyMessageID},
		weakParents,
		time.Now(),
		ed25519.PublicKey{},
		0,
		payload.NewGenericDataPayload([]byte("")),
		0,
		ed25519.Signature{},
	)

	msgWeakParents := msg.WeakParents()
	assert.True(t, testAreParentsSorted(msgWeakParents))
	assert.Equal(t, 3, len(msgWeakParents))
}

// TODO: check if parents are sorted in Bytes()

func TestMessage_BytesParentsSorted(t *testing.T) {

}

// TODO: write unit tests
