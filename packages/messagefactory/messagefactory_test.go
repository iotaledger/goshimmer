package messagefactory

import (
	"encoding"
	"io/ioutil"
	"os"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/iotaledger/goshimmer/packages/binary/tangle/model/message"
	"github.com/iotaledger/goshimmer/packages/binary/tangle/model/message/payload"
	"github.com/iotaledger/goshimmer/packages/binary/tangle/tipselector"
	"github.com/iotaledger/goshimmer/packages/database"
	"github.com/iotaledger/goshimmer/plugins/config"

	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/identity"
	"github.com/iotaledger/hive.go/logger"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	sequenceKey   = "seq"
	totalMessages = 2000
)

func TestMessageFactory_BuildMessage(t *testing.T) {
	// Set up DB for testing
	dir, err := ioutil.TempDir("", t.Name())
	require.NoError(t, err)
	defer os.Remove(dir)
	// use the tempdir for the database
	config.Node.Set(database.CFG_DIRECTORY, dir)
	db := database.GetBadgerInstance()

	log := logger.NewExampleLogger("MessageFactory")
	localIdentity := identity.GenerateLocalIdentity()
	tipSelector := tipselector.New()

	// attach to event and count
	countEvents := uint64(0)
	Events.MessageConstructed.Attach(events.NewClosure(func(msg *message.Transaction) {
		atomic.AddUint64(&countEvents, 1)
	}))

	// keep track of sequence numbers
	sequenceNumbers := sync.Map{}

	msgFactory := Setup(log, db, localIdentity, tipSelector, []byte(sequenceKey))

	t.Run("CheckProperties", func(t *testing.T) {
		data := []byte("TestCheckProperties")
		var p payload.Payload = NewMockPayload(data)
		msg := msgFactory.BuildMessage(p)

		assert.NotNil(t, msg.GetTrunkTransactionId())
		assert.NotNil(t, msg.GetBranchTransactionId())

		// time in range of 0.1 seconds
		assert.InDelta(t, time.Now().UnixNano(), msg.IssuingTime().UnixNano(), 100000000)

		// check payload
		assert.Same(t, p, msg.GetPayload())
		assert.Equal(t, data, msg.GetPayload().Bytes())

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
				msg := msgFactory.BuildMessage(p)

				assert.NotNil(t, msg.GetTrunkTransactionId())
				assert.NotNil(t, msg.GetBranchTransactionId())

				// time in range of 0.1 seconds
				assert.InDelta(t, time.Now().UnixNano(), msg.IssuingTime().UnixNano(), 100000000)

				// check payload
				assert.Same(t, p, msg.GetPayload())
				assert.Equal(t, data, msg.GetPayload().Bytes())

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

	_ = db.Close()
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

func (m *MockPayload) GetType() payload.Type {
	return payload.Type(0)
}

func (m *MockPayload) String() string {
	return string(m.data)
}
