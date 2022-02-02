package tangle

import (
	"testing"

	"github.com/iotaledger/hive.go/events"
	"github.com/stretchr/testify/assert"

	"github.com/iotaledger/goshimmer/packages/markers"
)

func TestUtils_AllTransactionsApprovedByMessages(t *testing.T) {
	// imgages/util-AllTransactionsApprovedByMessages-parallel-markers.png
	tangle := NewTestTangle()
	defer tangle.Shutdown()

	tangle.Setup()
	tangle.Events.Error.Attach(events.NewClosure(func(err error) {
		panic(err)
	}))

	mtf := NewMessageTestFramework(tangle, WithGenesisOutput("Genesis1", 5), WithGenesisOutput("Genesis2", 8))

	mtf.CreateMessage("Message1", WithStrongParents("Genesis"))
	mtf.CreateMessage("Message2", WithInputs("Genesis1"), WithOutput("A", 5), WithStrongParents("Message1"))
	mtf.CreateMessage("Message3", WithStrongParents("Message2"))
	mtf.CreateMessage("Message4", WithStrongParents("Genesis"))
	mtf.CreateMessage("Message5", WithInputs("Genesis2"), WithOutput("B", 4), WithOutput("C", 4), WithStrongParents("Message4"))
	mtf.CreateMessage("Message6", WithStrongParents("Message5"))
	mtf.CreateMessage("Message7", WithInputs("A", "B", "C"), WithOutput("D", 13), WithStrongParents("Message3", "Message6"))

	mtf.IssueMessages("Message1", "Message2", "Message3", "Message4", "Message5", "Message6", "Message7").WaitMessagesBooked()

	for messageAlias, expectedMarkers := range map[string]*markers.Markers{
		"Message1": markers.NewMarkers(markers.NewMarker(0, 1)),
		"Message2": markers.NewMarkers(markers.NewMarker(0, 2)),
		"Message3": markers.NewMarkers(markers.NewMarker(0, 3)),
		"Message4": markers.NewMarkers(markers.NewMarker(0, 0)),
		"Message5": markers.NewMarkers(markers.NewMarker(0, 0)),
		"Message6": markers.NewMarkers(markers.NewMarker(0, 0)),
		"Message7": markers.NewMarkers(markers.NewMarker(0, 4)),
	} {
		tangle.Storage.MessageMetadata(mtf.Message(messageAlias).ID()).Consume(func(messageMetadata *MessageMetadata) {
			assert.True(t, messageMetadata.StructureDetails().PastMarkers.Equals(expectedMarkers))
		})
	}
}
