package tangle

import (
	"sync"
	"testing"

	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/kvstore/mapdb"
	"github.com/stretchr/testify/assert"
)

func TestRetrieveAllTips(t *testing.T) {
	messageTangle := NewTangle(mapdb.NewMapDB())

	messageA := newTestParentsDataMessage("A", EmptyMessageID, EmptyMessageID)
	messageB := newTestParentsDataMessage("B", messageA.ID(), EmptyMessageID)
	messageC := newTestParentsDataMessage("C", messageA.ID(), EmptyMessageID)

	var wg sync.WaitGroup

	messageTangle.Events.MessageSolid.Attach(events.NewClosure(func(cachedMsgEvent *CachedMessageEvent) {
		cachedMsgEvent.Message.Release()
		cachedMsgEvent.MessageMetadata.Release()
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
