package test

import (
	"sync"
	"testing"
	"time"

	"github.com/iotaledger/goshimmer/packages/binary/messagelayer/message"
	"github.com/iotaledger/goshimmer/packages/binary/messagelayer/payload"
	"github.com/iotaledger/goshimmer/packages/binary/messagelayer/tangle"
	"github.com/iotaledger/hive.go/crypto/ed25519"
	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/kvstore/mapdb"
	"github.com/stretchr/testify/assert"
)

func TestRetrieveAllTips(t *testing.T) {
	messageTangle := tangle.New(mapdb.NewMapDB())

	messageA := newTestMessage("A", message.EmptyID, message.EmptyID)
	messageB := newTestMessage("B", messageA.ID(), message.EmptyID)
	messageC := newTestMessage("C", messageA.ID(), message.EmptyID)

	var wg sync.WaitGroup

	messageTangle.Events.MessageSolid.Attach(events.NewClosure(func(cachedMessage *message.CachedMessage, cachedMessageMetadata *tangle.CachedMessageMetadata) {
		cachedMessage.Release()
		cachedMessageMetadata.Release()
		wg.Done()
	}))

	wg.Add(3)
	messageTangle.AttachMessage(messageA)
	messageTangle.AttachMessage(messageB)
	messageTangle.AttachMessage(messageC)

	wg.Wait()

	allTips := messageTangle.RetrieveAllTips()

	assert.Equal(t, 2, len(allTips))

	messageTangle.Shutdown()
}

func newTestMessage(payloadString string, parent1, parent2 message.ID) *message.Message {
	return message.New(parent1, parent2, time.Now(), ed25519.PublicKey{}, 0, payload.NewData([]byte(payloadString)), 0, ed25519.Signature{})
}
