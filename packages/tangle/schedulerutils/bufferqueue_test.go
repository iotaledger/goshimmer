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
	maxBuffer   = numMessages
	maxQueue    = 2 * maxBuffer / numMessages
)

var (
	selfLocalIdentity = identity.GenerateLocalIdentity()
	selfNode          = identity.New(selfLocalIdentity.PublicKey())
	noManaIdentity    = identity.GenerateIdentity()
	noManaNode        = identity.New(noManaIdentity.PublicKey())
	aMana             = 1.0
)

func TestBufferQueue_Submit(t *testing.T) {
	b := schedulerutils.NewBufferQueue(maxBuffer, maxQueue)

	var size int
	for i := 0; i < numMessages; i++ {
		msg := newTestMessageWithIndex(identity.GenerateIdentity().PublicKey(), i)
		size++
		assert.Empty(t, b.Submit(msg, mockAccessManaRetriever))
		assert.EqualValues(t, i+1, b.NumActiveNodes())
		assert.EqualValues(t, i+1, ringLen(b))
	}
	assert.EqualValues(t, size, b.Size())
}

func TestBufferQueue_Unsubmit(t *testing.T) {
	b := schedulerutils.NewBufferQueue(maxBuffer, maxQueue)

	messages := make([]*testMessage, numMessages)
	for i := range messages {
		messages[i] = newTestMessageWithIndex(identity.GenerateIdentity().PublicKey(), i)
		assert.Empty(t, b.Submit(messages[i], mockAccessManaRetriever))
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

// Drop unready messages from the longest queue. Drop one message to fit new message. Drop two messages to fit one new larger message.
func TestBufferQueue_SubmitWithDrop_Unready(t *testing.T) {
	b := schedulerutils.NewBufferQueue(maxBuffer, maxQueue)
	preparedMessages := make([]*testMessage, 0, 2*numMessages)
	for i := 0; i < numMessages/2; i++ {
		preparedMessages = append(preparedMessages, newTestMessageWithIndex(noManaNode.PublicKey(), i))
	}
	for i := 0; i < numMessages/2; i++ {
		preparedMessages = append(preparedMessages, newTestMessageWithIndex(selfNode.PublicKey(), i))
	}
	for _, msg := range preparedMessages {
		droppedMessages := b.Submit(msg, mockAccessManaRetriever)
		assert.Empty(t, droppedMessages)
	}
	assert.EqualValues(t, 2, b.NumActiveNodes())
	assert.EqualValues(t, 2, ringLen(b))
	assert.EqualValues(t, maxBuffer, b.Size())

	// dropping single unready message
	droppedMessages := b.Submit(newTestMessageWithIndex(selfNode.PublicKey(), 0), mockAccessManaRetriever)
	assert.Len(t, droppedMessages, 1)
	assert.Equal(t, preparedMessages[0].IDBytes(), droppedMessages[0][:])
	assert.LessOrEqual(t, maxBuffer, b.Size())

	// dropping two unready messages to fit the new one
	droppedMessages = b.Submit(newLargeTestMessage(selfNode.PublicKey(), make([]byte, 44)), mockAccessManaRetriever)
	assert.Len(t, droppedMessages, 1)
	assert.Equal(t, preparedMessages[1].IDBytes(), droppedMessages[0][:])
	assert.EqualValues(t, maxBuffer, b.Size())
}

// Drop newly submitted message because the node doesn't have enough access mana to send such a big message when the buffer is full.
func TestBufferQueue_SubmitWithDrop_DropNewMessage(t *testing.T) {
	b := schedulerutils.NewBufferQueue(maxBuffer, maxQueue)
	preparedMessages := make([]*testMessage, 0, 2*numMessages)
	for i := 0; i < numMessages; i++ {
		preparedMessages = append(preparedMessages, newTestMessageWithIndex(selfNode.PublicKey(), i))
	}
	for _, msg := range preparedMessages {
		droppedMessages := b.Submit(msg, mockAccessManaRetriever)
		assert.Empty(t, droppedMessages)
	}
	assert.EqualValues(t, 1, b.NumActiveNodes())
	assert.EqualValues(t, 1, ringLen(b))
	assert.EqualValues(t, maxBuffer, b.Size())

	// drop newly submitted message when all messages in the buffer are not ready
	newMessage := newLargeTestMessage(noManaNode.PublicKey(), make([]byte, 40))
	droppedMessages := b.Submit(newMessage, mockAccessManaRetriever)

	assert.Len(t, droppedMessages, 1)
	assert.Equal(t, newMessage.IDBytes(), droppedMessages[0][:])
	assert.Equal(t, maxBuffer, b.Size())
}

// Drop ready messages from the longest queue. Drop one ready message to fit new message. Drop two ready messages to fit one new larger message.
func TestBufferQueue_SubmitWithDrop_Ready(t *testing.T) {
	b := schedulerutils.NewBufferQueue(maxBuffer, maxQueue)
	preparedMessages := make([]*testMessage, 0, 2*numMessages)
	for i := 0; i < numMessages/2; i++ {
		preparedMessages = append(preparedMessages, newTestMessageWithIndex(noManaNode.PublicKey(), i))
	}
	for i := 0; i < numMessages/2; i++ {
		preparedMessages = append(preparedMessages, newTestMessageWithIndex(selfNode.PublicKey(), i))
	}
	for _, msg := range preparedMessages {
		droppedMessages := b.Submit(msg, mockAccessManaRetriever)
		assert.Empty(t, droppedMessages)
		b.Ready(msg)
	}
	assert.EqualValues(t, 2, b.NumActiveNodes())
	assert.EqualValues(t, 2, ringLen(b))
	assert.EqualValues(t, maxBuffer, b.Size())

	// drop single ready message
	droppedMessages := b.Submit(newTestMessageWithIndex(selfNode.PublicKey(), 0), mockAccessManaRetriever)
	assert.Len(t, droppedMessages, 1)
	assert.Equal(t, preparedMessages[0].IDBytes(), droppedMessages[0][:])
	assert.LessOrEqual(t, maxBuffer, b.Size())

	// drop two ready messages to fit the newly submitted one
	droppedMessages = b.Submit(newLargeTestMessage(selfNode.PublicKey(), make([]byte, 44)), mockAccessManaRetriever)
	assert.Len(t, droppedMessages, 1)
	assert.Equal(t, preparedMessages[1].IDBytes(), droppedMessages[0][:])
	assert.EqualValues(t, maxBuffer, b.Size())
}

func TestBufferQueue_Ready(t *testing.T) {
	b := schedulerutils.NewBufferQueue(maxBuffer, maxQueue)

	messages := make([]*testMessage, numMessages)
	for i := range messages {
		messages[i] = newTestMessageWithIndex(identity.GenerateIdentity().PublicKey(), i)
		assert.Empty(t, b.Submit(messages[i], mockAccessManaRetriever))
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
	assert.Empty(t, b.Submit(future, mockAccessManaRetriever))
	assert.True(t, b.Ready(future))

	now := newTestMessage(selfNode.PublicKey())
	assert.Empty(t, b.Submit(now, mockAccessManaRetriever))
	assert.True(t, b.Ready(now))

	assert.Equal(t, now, b.PopFront())
	assert.Equal(t, future, b.PopFront())

	assert.EqualValues(t, 0, b.Size())
}

func TestBufferQueue_Ring(t *testing.T) {
	b := schedulerutils.NewBufferQueue(maxBuffer, maxQueue)

	messages := make([]*testMessage, numMessages)
	for i := range messages {
		messages[i] = newTestMessageWithIndex(identity.GenerateIdentity().PublicKey(), i)
		assert.Empty(t, b.Submit(messages[i], mockAccessManaRetriever))
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
		msg := newTestMessageWithIndex(identity.GenerateIdentity().PublicKey(), i)
		assert.Empty(t, b.Submit(msg, mockAccessManaRetriever))
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

	assert.Empty(t, b.Submit(newTestMessage(selfNode.PublicKey()), mockAccessManaRetriever))

	otherNode := identity.GenerateIdentity()
	assert.Empty(t, b.Submit(newTestMessage(otherNode.PublicKey()), mockAccessManaRetriever))

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
	payload     []byte
	bytes       []byte
	idx         int
}

func newTestMessageWithIndex(pubKey ed25519.PublicKey, index int) *testMessage {
	return &testMessage{
		pubKey:      pubKey,
		idx:         index,
		issuingTime: time.Now(),
	}
}

func newTestMessage(pubKey ed25519.PublicKey) *testMessage {
	return &testMessage{
		pubKey:      pubKey,
		issuingTime: time.Now(),
	}
}

func newLargeTestMessage(pubKey ed25519.PublicKey, payload []byte) *testMessage {
	return &testMessage{
		pubKey:      pubKey,
		issuingTime: time.Now(),
		payload:     payload,
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
	marshalUtil.WriteBytes(m.payload)
	marshalUtil.WriteInt32(int32(m.idx))
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

// mockAccessManaRetriever returns mocked access mana value for a node.
func mockAccessManaRetriever(id identity.ID) float64 {
	if id == selfNode.ID() {
		return aMana
	}
	return 0
}
