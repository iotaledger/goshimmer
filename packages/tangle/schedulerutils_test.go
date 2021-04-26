package tangle

import (
	"testing"
	"time"

	"github.com/iotaledger/hive.go/identity"
	"github.com/stretchr/testify/assert"
)

// region Buffered Queue test /////////////////////////////////////////////////////////////////////////////////////////////

const numMessages = 100

func TestSubmit(t *testing.T) {
	b := NewBufferQueue()
	var size int
	for i := 0; i < numMessages; i++ {
		msg := newMessage(identity.GenerateIdentity().PublicKey())
		size += len(msg.Bytes())
		assert.NoError(t, b.Submit(msg, 1))
		assert.EqualValues(t, i+1, b.NumActiveNodes())
		assert.EqualValues(t, i+1, ringLen(b))
	}
	assert.EqualValues(t, size, b.Size())
}

func TestUnsubmit(t *testing.T) {
	b := NewBufferQueue()

	messages := make([]*Message, numMessages)
	for i := range messages {
		messages[i] = newMessage(identity.GenerateIdentity().PublicKey())
		assert.NoError(t, b.Submit(messages[i], 1))
	}
	assert.EqualValues(t, numMessages, b.NumActiveNodes())
	assert.EqualValues(t, numMessages, ringLen(b))
	for i := range messages {
		b.Unsubmit(messages[i])
		assert.EqualValues(t, numMessages-1-i, b.NumActiveNodes())
		assert.EqualValues(t, numMessages-1-i, ringLen(b))
	}
	assert.EqualValues(t, 0, b.Size())
}

func TestReady(t *testing.T) {
	b := NewBufferQueue()

	messages := make([]*Message, numMessages)
	for i := range messages {
		messages[i] = newMessage(identity.GenerateIdentity().PublicKey())
		assert.NoError(t, b.Submit(messages[i], 1))
	}
	for i := range messages {
		assert.True(t, b.Ready(messages[i]))
		assert.False(t, b.Ready(messages[i]))
		assert.Equal(t, messages[i], b.PopFront())
	}
	assert.EqualValues(t, 0, b.Size())
}

func TestTime(t *testing.T) {
	b := NewBufferQueue()

	future := newMessage(selfNode.PublicKey())
	future.issuingTime = time.Now().Add(time.Second)
	assert.NoError(t, b.Submit(future, 1))
	assert.True(t, b.Ready(future))

	now := newMessage(selfNode.PublicKey())
	assert.NoError(t, b.Submit(now, 1))
	assert.True(t, b.Ready(now))

	assert.Equal(t, now, b.PopFront())
	assert.Equal(t, future, b.PopFront())

	assert.EqualValues(t, 0, b.Size())
}

func TestRing(t *testing.T) {
	b := NewBufferQueue()

	messages := make([]*Message, numMessages)
	for i := range messages {
		messages[i] = newMessage(identity.GenerateIdentity().PublicKey())
		assert.NoError(t, b.Submit(messages[i], 1))
		b.Ready(messages[i])
	}
	for i := range messages {
		assert.Equal(t, messages[i], b.Current().Front())
		b.Next()
	}
	assert.Equal(t, messages[0], b.Current().Front())
}

func ringLen(b *BufferQueue) int {
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
