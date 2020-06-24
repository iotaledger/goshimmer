package messagefactory

import (
	"context"
	"crypto"
	"crypto/ed25519"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/iotaledger/goshimmer/packages/binary/messagelayer/message"
	"github.com/iotaledger/goshimmer/packages/binary/messagelayer/payload"
	"github.com/iotaledger/goshimmer/packages/pow"
	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/identity"
	"github.com/iotaledger/hive.go/kvstore/mapdb"
	"github.com/stretchr/testify/assert"
	_ "golang.org/x/crypto/blake2b"
)

const (
	sequenceKey   = "seq"
	targetPOW     = 10
	totalMessages = 2000
)

func TestMessageFactory_BuildMessage(t *testing.T) {
	msgFactory := New(
		mapdb.NewMapDB(),
		[]byte(sequenceKey),
		identity.GenerateLocalIdentity(),
		TipSelectorFunc(func() (message.Id, message.Id) { return message.EmptyId, message.EmptyId }),
	)
	defer msgFactory.Shutdown()

	// keep track of sequence numbers
	sequenceNumbers := sync.Map{}

	// attach to event and count
	countEvents := uint64(0)
	msgFactory.Events.MessageConstructed.Attach(events.NewClosure(func(msg *message.Message) {
		atomic.AddUint64(&countEvents, 1)
	}))

	t.Run("CheckProperties", func(t *testing.T) {
		p := payload.NewData([]byte("TestCheckProperties"))
		msg := msgFactory.IssuePayload(p)

		assert.NotNil(t, msg.TrunkId())
		assert.NotNil(t, msg.BranchId())

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

				p := payload.NewData([]byte("TestParallelCreation"))
				msg := msgFactory.IssuePayload(p)

				assert.NotNil(t, msg.TrunkId())
				assert.NotNil(t, msg.BranchId())

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
	msgFactory := New(
		mapdb.NewMapDB(),
		[]byte(sequenceKey),
		identity.GenerateLocalIdentity(),
		TipSelectorFunc(func() (message.Id, message.Id) { return message.EmptyId, message.EmptyId }),
	)
	defer msgFactory.Shutdown()

	worker := pow.New(crypto.BLAKE2b_512, 1)

	msgFactory.SetWorker(WorkerFunc(func(msgBytes []byte) (uint64, error) {
		content := msgBytes[:len(msgBytes)-ed25519.SignatureSize-8]
		return worker.Mine(context.Background(), content, targetPOW)
	}))

	msg := msgFactory.IssuePayload(payload.NewData([]byte("test")))
	msgBytes := msg.Bytes()
	content := msgBytes[:len(msgBytes)-ed25519.SignatureSize-8]

	zeroes, err := worker.LeadingZerosWithNonce(content, msg.Nonce())
	assert.GreaterOrEqual(t, zeroes, targetPOW)
	assert.NoError(t, err)
}
