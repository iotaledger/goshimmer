package tangle

import (
	"testing"

	"github.com/iotaledger/goshimmer/packages/markers"
	"github.com/iotaledger/hive.go/events"
	"github.com/stretchr/testify/assert"
)

func TestUtils_AllTransactionsApprovedByMessages(t *testing.T) {
  // imgages/util-AllTransactionsApprovedByMessages-parallel-markers.png
	tangle := New()
	defer tangle.Shutdown()

	tangle.Setup()
	tangle.Events.Error.Attach(events.NewClosure(func(err error) {
		panic(err)
	}))

	messageTestFramework := NewMessageTestFramework(tangle, GenesisOutput("Genesis1", 5), GenesisOutput("Genesis2", 8))

	messageTestFramework.CreateMessage("Message1", StrongParents("Genesis"))
	messageTestFramework.CreateMessage("Message2", Inputs("Genesis1"), Output("A", 5), StrongParents("Message1"))
	messageTestFramework.CreateMessage("Message3", StrongParents("Message2"))
	messageTestFramework.CreateMessage("Message4", StrongParents("Genesis"))
	messageTestFramework.CreateMessage("Message5", Inputs("Genesis2"), Output("B", 4), Output("C", 4), StrongParents("Message4"))
	messageTestFramework.CreateMessage("Message6", StrongParents("Message5"))
	messageTestFramework.CreateMessage("Message7", Inputs("A", "B", "C"), Output("D", 13), StrongParents("Message3", "Message6"))

	messageTestFramework.IssueMessages("Message1", "Message2", "Message3", "Message4", "Message5", "Message6", "Message7").WaitMessagesBooked()

	for messageAlias, expectedMarkers := range map[string]*markers.Markers{
		tangle.Storage.MessageMetadata(messageTestFramework.Message(messageAlias).ID()).Consume(func(messageMetadata *MessageMetadata) {
			assert.True(t, messageMetadata.StructureDetails().PastMarkers.Equals(expectedMarkers))
		})
	}
}
