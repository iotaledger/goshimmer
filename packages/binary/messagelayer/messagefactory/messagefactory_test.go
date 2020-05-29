package messagefactory

import (
	"encoding"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/iotaledger/goshimmer/packages/binary/messagelayer/message"
	"github.com/iotaledger/goshimmer/packages/binary/messagelayer/payload"
	"github.com/iotaledger/goshimmer/packages/binary/messagelayer/tipselector"
	"github.com/iotaledger/hive.go/kvstore/mapdb"

	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/identity"
	"github.com/stretchr/testify/assert"
)

const (
	sequenceKey   = "seq"
	totalMessages = 2000
)

func TestMessageFactory_BuildMessage(t *testing.T) {
	msgFactory := New(mapdb.NewMapDB(), identity.GenerateLocalIdentity(), tipselector.New(), []byte(sequenceKey))
	defer msgFactory.Shutdown()

	// keep track of sequence numbers
	sequenceNumbers := sync.Map{}

	// attach to event and count
	countEvents := uint64(0)
	msgFactory.Events.MessageConstructed.Attach(events.NewClosure(func(msg *message.Message) {
		atomic.AddUint64(&countEvents, 1)
	}))

	t.Run("CheckProperties", func(t *testing.T) {
		data := []byte("TestCheckProperties")
		var p payload.Payload = NewMockPayload(data)
		msg := msgFactory.IssuePayload(p)

		assert.NotNil(t, msg.TrunkId())
		assert.NotNil(t, msg.BranchId())

		// time in range of 0.1 seconds
		assert.InDelta(t, time.Now().UnixNano(), msg.IssuingTime().UnixNano(), 100000000)

		// check payload
		assert.Same(t, p, msg.Payload())
		assert.Equal(t, data, msg.Payload().Bytes())

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
				data := []byte("TestCheckProperties")
				var p payload.Payload = NewMockPayload(data)
				msg := msgFactory.IssuePayload(p)

				assert.NotNil(t, msg.TrunkId())
				assert.NotNil(t, msg.BranchId())

				// time in range of 0.1 seconds
				assert.InDelta(t, time.Now().UnixNano(), msg.IssuingTime().UnixNano(), 100000000)

				// check payload
				assert.Same(t, p, msg.Payload())
				assert.Equal(t, data, msg.Payload().Bytes())

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

type MockPayload struct {
	data []byte
	encoding.BinaryMarshaler
	encoding.BinaryUnmarshaler
}

func NewMockPayload(data []byte) *MockPayload {
	return &MockPayload{data: data}
}

func (m *MockPayload) Bytes() []byte {
	return m.data
}

func (m *MockPayload) Type() payload.Type {
	return payload.Type(0)
}

func (m *MockPayload) String() string {
	return string(m.data)
}

func (m *MockPayload) Unmarshal(bytes []byte) error {
	panic("implement me")
}
