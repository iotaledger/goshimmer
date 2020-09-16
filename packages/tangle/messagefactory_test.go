package tangle

import (
	"context"
	"crypto"
	"crypto/ed25519"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/iotaledger/goshimmer/packages/pow"
	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/identity"
	"github.com/iotaledger/hive.go/kvstore/mapdb"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	_ "golang.org/x/crypto/blake2b"
)

const (
	sequenceKey   = "seq"
	targetPOW     = 10
	totalMessages = 2000
)

func TestMessageFactory_BuildMessage(t *testing.T) {
	msgFactory := NewMessageFactory(
		mapdb.NewMapDB(),
		[]byte(sequenceKey),
		identity.GenerateLocalIdentity(),
		TipSelectorFunc(func() (MessageID, MessageID) { return EmptyMessageID, EmptyMessageID }),
	)
	defer msgFactory.Shutdown()

	// keep track of sequence numbers
	sequenceNumbers := sync.Map{}

	// attach to event and count
	countEvents := uint64(0)
	msgFactory.Events.MessageConstructed.Attach(events.NewClosure(func(msg *Message) {
		atomic.AddUint64(&countEvents, 1)
	}))

	t.Run("CheckProperties", func(t *testing.T) {
		p := NewDataPayload([]byte("TestCheckProperties"))
		msg, err := msgFactory.IssuePayload(p)
		require.NoError(t, err)

		assert.NotNil(t, msg.Parent1ID())
		assert.NotNil(t, msg.Parent2ID())

		// time in range of 0.1 seconds
		assert.InDelta(t, time.Now().UnixNano(), msg.IssuingTime().UnixNano(), 100000000)

		// check payload
		assert.Equal(t, p, msg.Payload())

		// check total events and sequence number
		assert.EqualValues(t, 1, countEvents)
		assert.EqualValues(t, 0, msg.SequenceNumber())

		sequenceNumbers.Store(msg.SequenceNumber(), true)
	})

	// create messages in parallel
	t.Run("ParallelCreation", func(t *testing.T) {
		for i := 1; i < totalMessages; i++ {
			t.Run("test", func(t *testing.T) {
				t.Parallel()

				p := NewDataPayload([]byte("TestParallelCreation"))
				msg, err := msgFactory.IssuePayload(p)
				require.NoError(t, err)

				assert.NotNil(t, msg.Parent1ID())
				assert.NotNil(t, msg.Parent2ID())

				// time in range of 0.1 seconds
				assert.InDelta(t, time.Now().UnixNano(), msg.IssuingTime().UnixNano(), 100000000)

				// check payload
				assert.Equal(t, p, msg.Payload())

				sequenceNumbers.Store(msg.SequenceNumber(), true)
			})
		}
	})

	// check total events and sequence number
	assert.EqualValues(t, totalMessages, countEvents)

	max := uint64(0)
	countSequence := 0
	sequenceNumbers.Range(func(key, value interface{}) bool {
		seq := key.(uint64)
		val := value.(bool)
		if val != true {
			return false
		}

		// check for max sequence number
		if seq > max {
			max = seq
		}
		countSequence++
		return true
	})
	assert.EqualValues(t, totalMessages-1, max)
	assert.EqualValues(t, totalMessages, countSequence)
}

func TestMessageFactory_POW(t *testing.T) {
	msgFactory := NewMessageFactory(
		mapdb.NewMapDB(),
		[]byte(sequenceKey),
		identity.GenerateLocalIdentity(),
		TipSelectorFunc(func() (MessageID, MessageID) { return EmptyMessageID, EmptyMessageID }),
	)
	defer msgFactory.Shutdown()

	worker := pow.New(crypto.BLAKE2b_512, 1)

	msgFactory.SetWorker(WorkerFunc(func(msgBytes []byte) (uint64, error) {
		content := msgBytes[:len(msgBytes)-ed25519.SignatureSize-8]
		return worker.Mine(context.Background(), content, targetPOW)
	}))

	msg, err := msgFactory.IssuePayload(NewDataPayload([]byte("test")))
	require.NoError(t, err)

	msgBytes := msg.Bytes()
	content := msgBytes[:len(msgBytes)-ed25519.SignatureSize-8]

	zeroes, err := worker.LeadingZerosWithNonce(content, msg.Nonce())
	assert.GreaterOrEqual(t, zeroes, targetPOW)
	assert.NoError(t, err)
}

func TestWorkerFunc_PayloadSize(t *testing.T) {
	msgFactory := NewMessageFactory(
		mapdb.NewMapDB(),
		[]byte(sequenceKey),
		identity.GenerateLocalIdentity(),
		TipSelectorFunc(func() (MessageID, MessageID) { return EmptyMessageID, EmptyMessageID }),
	)
	defer msgFactory.Shutdown()

	// issue message with max allowed payload size
	// dataPayload headers: type|32bit + size|32bit
	data := make([]byte, MaxPayloadSize-4-4)
	msg, err := msgFactory.IssuePayload(NewDataPayload(data))
	require.NoError(t, err)
	assert.Truef(t, MaxMessageSize == len(msg.Bytes()), "message size should be exactly %d bytes but is %d", MaxMessageSize, len(msg.Bytes()))

	// issue message bigger than max allowed payload size
	data = make([]byte, MaxPayloadSize)
	msg, err = msgFactory.IssuePayload(NewDataPayload(data))
	require.Error(t, err)
	assert.Nil(t, msg)
}
