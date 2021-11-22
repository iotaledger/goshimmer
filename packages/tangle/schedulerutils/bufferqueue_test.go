package schedulerutils_test

import (
	"testing"
	"time"

	"github.com/iotaledger/goshimmer/packages/tangle/schedulerutils"

	"github.com/iotaledger/hive.go/crypto/ed25519"
	"github.com/iotaledger/hive.go/identity"
	"github.com/iotaledger/hive.go/marshalutil"
	"github.com/stretchr/testify/assert"
	"golang.org/x/crypto/blake2b"
)

// region Buffered Queue test /////////////////////////////////////////////////////////////////////////////////////////////

const (
	numMessages = 100
	maxBuffer   = 40 * numMessages
	maxQueue    = 2 * maxBuffer / numMessages
)

var (
	selfLocalIdentity = identity.GenerateLocalIdentity()
	selfNode          = identity.New(selfLocalIdentity.PublicKey())
)

func TestBufferQueue_Submit(t *testing.T) {
	b := schedulerutils.NewBufferQueue(maxBuffer, maxQueue)

	var size int
	for i := 0; i < numMessages; i++ {
		msg := newTestMessage(identity.GenerateIdentity().PublicKey())
		size += len(msg.Bytes())
		assert.NoError(t, b.Submit(msg, 1))
		assert.EqualValues(t, i+1, b.NumActiveNodes())
		assert.EqualValues(t, i+1, ringLen(b))
	}
	assert.EqualValues(t, size, b.Size())
}

func TestBufferQueue_Unsubmit(t *testing.T) {
	b := schedulerutils.NewBufferQueue(maxBuffer, maxQueue)

	messages := make([]*testMessage, numMessages)
	for i := range messages {
		messages[i] = newTestMessage(identity.GenerateIdentity().PublicKey())
		assert.NoError(t, b.Submit(messages[i], 1))
	}
	assert.EqualValues(t, numMessages, b.NumActiveNodes())
	assert.EqualValues(t, numMessages, ringLen(b))
	for i := range messages {
		b.Unsubmit(messages[i])
		assert.EqualValues(t, numMessages, b.NumActiveNodes())
		assert.EqualValues(t, numMessages, ringLen(b))
	}
	assert.EqualValues(t, 0, b.Size())
}

func TestBufferQueue_Ready(t *testing.T) {
	b := schedulerutils.NewBufferQueue(maxBuffer, maxQueue)

	messages := make([]*testMessage, numMessages)
	for i := range messages {
		messages[i] = newTestMessage(identity.GenerateIdentity().PublicKey())
		assert.NoError(t, b.Submit(messages[i], 1))
	}
	for i := range messages {
		assert.True(t, b.Ready(messages[i]))
		assert.False(t, b.Ready(messages[i]))
		for ; b.Current().Size() == 0; b.Next() {
		}

		assert.Equal(t, messages[i], b.PopFront())
	}
	assert.EqualValues(t, 0, b.Size())
}

func TestBufferQueue_Time(t *testing.T) {
	b := schedulerutils.NewBufferQueue(maxBuffer, maxQueue)

	future := newTestMessage(selfNode.PublicKey())
	future.issuingTime = time.Now().Add(time.Second)
	assert.NoError(t, b.Submit(future, 1))
	assert.True(t, b.Ready(future))

	now := newTestMessage(selfNode.PublicKey())
	assert.NoError(t, b.Submit(now, 1))
	assert.True(t, b.Ready(now))

	assert.Equal(t, now, b.PopFront())
	assert.Equal(t, future, b.PopFront())

	assert.EqualValues(t, 0, b.Size())
}

func TestBufferQueue_Ring(t *testing.T) {
	b := schedulerutils.NewBufferQueue(maxBuffer, maxQueue)

	messages := make([]*testMessage, numMessages)
	for i := range messages {
		messages[i] = newTestMessage(identity.GenerateIdentity().PublicKey())
		assert.NoError(t, b.Submit(messages[i], 1))
		assert.True(t, b.Ready(messages[i]))
	}
	for i := range messages {
		assert.Equal(t, messages[i], b.Current().Front())
		b.Next()
	}
	assert.Equal(t, messages[0], b.Current().Front())
}

func TestBufferQueue_IDs(t *testing.T) {
	b := schedulerutils.NewBufferQueue(maxBuffer, maxQueue)

	assert.Empty(t, b.IDs())

	ids := make([]schedulerutils.ElementID, numMessages)
	for i := range ids {
		msg := newTestMessage(identity.GenerateIdentity().PublicKey())
		assert.NoError(t, b.Submit(msg, 1))
		if i%2 == 0 {
			assert.True(t, b.Ready(msg))
		}
		ids[i] = schedulerutils.ElementIDFromBytes(msg.IDBytes())
	}
	assert.ElementsMatch(t, ids, b.IDs())
}

func TestBufferQueue_InsertNode(t *testing.T) {
	b := schedulerutils.NewBufferQueue(maxBuffer, maxQueue)

	otherNode := identity.GenerateIdentity()
	b.InsertNode(selfNode.ID())
	assert.Equal(t, selfNode.ID(), b.Current().NodeID())
	assert.Equal(t, 0, b.Current().Size())

	b.InsertNode(otherNode.ID())
	b.Next()
	assert.Equal(t, otherNode.ID(), b.Current().NodeID())
	assert.Equal(t, 0, b.Current().Size())
}

func TestBufferQueue_RemoveNode(t *testing.T) {
	b := schedulerutils.NewBufferQueue(maxBuffer, maxQueue)

	assert.NoError(t, b.Submit(newTestMessage(selfNode.PublicKey()), 1))

	otherNode := identity.GenerateIdentity()
	assert.NoError(t, b.Submit(newTestMessage(otherNode.PublicKey()), 1))

	assert.Equal(t, selfNode.ID(), b.Current().NodeID())
	b.RemoveNode(selfNode.ID())
	assert.Equal(t, otherNode.ID(), b.Current().NodeID())
	b.RemoveNode(otherNode.ID())
	assert.Nil(t, b.Current())
}

func ringLen(b *schedulerutils.BufferQueue) int {
	n := 0
	if q := b.Current(); q != nil {
		n = 1
		for b.Next() != q {
			n++
		}
	}
	return n
}

// endregion ///////////////////////////////////////////////////////////////////////////////////////////////////////////

type testMessage struct {
	pubKey      ed25519.PublicKey
	issuingTime time.Time
	bytes       []byte
}

func newTestMessage(pubKey ed25519.PublicKey) *testMessage {
	return &testMessage{
		pubKey:      pubKey,
		issuingTime: time.Now(),
	}
}

func (m *testMessage) IDBytes() []byte {
	tmp := blake2b.Sum256(m.Bytes())
	return tmp[:]
}

func (m *testMessage) Bytes() []byte {
	if m.bytes != nil {
		return m.bytes
	}
	// marshal result
	marshalUtil := marshalutil.New()
	marshalUtil.Write(m.pubKey)
	marshalUtil.WriteTime(m.issuingTime)

	m.bytes = marshalUtil.Bytes()

	return m.bytes
}

func (m *testMessage) Size() int {
	return len(m.Bytes())
}

func (m *testMessage) IssuerPublicKey() ed25519.PublicKey {
	return m.pubKey
}

func (m *testMessage) IssuingTime() time.Time {
	return m.issuingTime
}
