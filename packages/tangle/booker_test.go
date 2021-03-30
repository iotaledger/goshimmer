package tangle

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/iotaledger/goshimmer/packages/ledgerstate"
	"github.com/iotaledger/goshimmer/packages/markers"
)

func TestBookerBook(t *testing.T) {
	tangle := New()
	tangle.Setup()
	defer tangle.Shutdown()

	testFramework := NewMessageTestFramework(
		tangle,
		WithGenesisOutput("A", 500),
		WithGenesisOutput("B", 500),
	)

	testFramework.CreateMessage("Message1", WithStrongParents("Genesis"), WithInputs("A"), WithOutput("C", 500))
	testFramework.CreateMessage("Message4", WithStrongParents("Message1"))
	testFramework.CreateMessage("Message2", WithStrongParents("Genesis"), WithInputs("A", "B"), WithOutput("D", 1000))
	testFramework.CreateMessage("Message3", WithStrongParents("Genesis"), WithInputs("B"), WithOutput("E", 500))

	testFramework.IssueMessages("Message1", "Message2", "Message3", "Message4").WaitMessagesBooked()

	branchIDs := map[string]ledgerstate.BranchID{
		"A": ledgerstate.NewBranchID(testFramework.TransactionID("Message1")),
		"B": ledgerstate.NewBranchID(testFramework.TransactionID("Message3")),
		"C": ledgerstate.NewBranchID(testFramework.TransactionID("Message2")),
	}

	checkMarkers(t, testFramework, map[string]*markers.Markers{
		"Message1": markers.NewMarkers(markers.NewMarker(1, 1)),
		"Message2": markers.NewMarkers(markers.NewMarker(2, 1)),
		"Message3": markers.NewMarkers(markers.NewMarker(3, 1)),
		"Message4": markers.NewMarkers(markers.NewMarker(1, 2)),
	})

	checkMessageMetadataBranchIDs(t, testFramework, map[string]ledgerstate.BranchID{
		"Message1": ledgerstate.UndefinedBranchID,
		"Message2": ledgerstate.UndefinedBranchID,
		"Message3": ledgerstate.UndefinedBranchID,
		"Message4": ledgerstate.UndefinedBranchID,
	})

	checkBranchIDs(t, testFramework, map[string]ledgerstate.BranchID{
		"Message1": branchIDs["A"],
		"Message2": branchIDs["C"],
		"Message3": branchIDs["B"],
		"Message4": branchIDs["A"],
	})

	fmt.Println("DONE")
}

func checkMarkers(t *testing.T, testFramework *MessageTestFramework, expectedMarkers map[string]*markers.Markers) {
	for messageID, expectedMarkersOfMessage := range expectedMarkers {
		assert.True(t, testFramework.tangle.Storage.MessageMetadata(testFramework.Message(messageID).ID()).Consume(func(messageMetadata *MessageMetadata) {
			assert.Equal(t, expectedMarkersOfMessage, messageMetadata.StructureDetails().PastMarkers)
		}))
	}
}

func checkBranchIDs(t *testing.T, testFramework *MessageTestFramework, expectedBranchIDs map[string]ledgerstate.BranchID) {
	for messageID, expectedBranchID := range expectedBranchIDs {
		assert.Equal(t, expectedBranchID, testFramework.tangle.Booker.BranchIDOfMessage(testFramework.Message(messageID).ID()))
	}
}

func checkMessageMetadataBranchIDs(t *testing.T, testFramework *MessageTestFramework, expectedBranchIDs map[string]ledgerstate.BranchID) {
	for messageID, expectedBranchID := range expectedBranchIDs {
		assert.True(t, testFramework.tangle.Storage.MessageMetadata(testFramework.Message(messageID).ID()).Consume(func(messageMetadata *MessageMetadata) {
			assert.Equal(t, expectedBranchID, messageMetadata.BranchID())
		}))
	}
}
