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
		WithGenesisOutput("C", 500),
	)

	branchIDs := make(map[string]ledgerstate.BranchID)

	// ISSUE Message1
	{
		testFramework.CreateMessage("Message1", WithStrongParents("Genesis"), WithInputs("A"), WithOutput("G", 500))
		testFramework.IssueMessages("Message1").WaitMessagesBooked()

		checkMarkers(t, testFramework, map[string]*markers.Markers{
			"Message1": markers.NewMarkers(markers.NewMarker(1, 1)),
		})
		checkMessageMetadataBranchIDs(t, testFramework, map[string]ledgerstate.BranchID{
			"Message1": ledgerstate.UndefinedBranchID,
		})
		checkBranchIDs(t, testFramework, map[string]ledgerstate.BranchID{
			"Message1": ledgerstate.MasterBranchID,
		})

		//
	}

	// ISSUE Message2
	{
		testFramework.CreateMessage("Message2", WithStrongParents("Genesis"), WithInputs("A", "B"), WithOutput("E", 1000))
		testFramework.IssueMessages("Message2").WaitMessagesBooked()

		branchIDs["A"] = ledgerstate.NewBranchID(testFramework.TransactionID("Message1"))
		branchIDs["B"] = ledgerstate.NewBranchID(testFramework.TransactionID("Message2"))

		checkMarkers(t, testFramework, map[string]*markers.Markers{
			"Message1": markers.NewMarkers(markers.NewMarker(1, 1)),
			"Message2": markers.NewMarkers(markers.NewMarker(2, 1)),
		})
		checkMessageMetadataBranchIDs(t, testFramework, map[string]ledgerstate.BranchID{
			"Message1": ledgerstate.UndefinedBranchID,
			"Message2": ledgerstate.UndefinedBranchID,
		})
		checkBranchIDs(t, testFramework, map[string]ledgerstate.BranchID{
			"Message1": branchIDs["A"],
			"Message2": branchIDs["B"],
		})
	}

	// ISSUE Message3
	{
		testFramework.CreateMessage("Message3", WithStrongParents("Genesis"), WithInputs("B"), WithOutput("F", 500))
		testFramework.IssueMessages("Message3").WaitMessagesBooked()

		branchIDs["C"] = ledgerstate.NewBranchID(testFramework.TransactionID("Message3"))

		checkMarkers(t, testFramework, map[string]*markers.Markers{
			"Message1": markers.NewMarkers(markers.NewMarker(1, 1)),
			"Message2": markers.NewMarkers(markers.NewMarker(2, 1)),
			"Message3": markers.NewMarkers(markers.NewMarker(3, 1)),
		})
		checkMessageMetadataBranchIDs(t, testFramework, map[string]ledgerstate.BranchID{
			"Message1": ledgerstate.UndefinedBranchID,
			"Message2": ledgerstate.UndefinedBranchID,
			"Message3": ledgerstate.UndefinedBranchID,
		})
		checkBranchIDs(t, testFramework, map[string]ledgerstate.BranchID{
			"Message1": branchIDs["A"],
			"Message2": branchIDs["B"],
			"Message3": branchIDs["C"],
		})
	}

	// ISSUE Message4
	{
		testFramework.CreateMessage("Message4", WithStrongParents("Genesis"))
		testFramework.IssueMessages("Message4").WaitMessagesBooked()

		checkMarkers(t, testFramework, map[string]*markers.Markers{
			"Message1": markers.NewMarkers(markers.NewMarker(1, 1)),
			"Message2": markers.NewMarkers(markers.NewMarker(2, 1)),
			"Message3": markers.NewMarkers(markers.NewMarker(3, 1)),
			"Message4": markers.NewMarkers(markers.NewMarker(4, 1)),
		})
		checkMessageMetadataBranchIDs(t, testFramework, map[string]ledgerstate.BranchID{
			"Message1": ledgerstate.UndefinedBranchID,
			"Message2": ledgerstate.UndefinedBranchID,
			"Message3": ledgerstate.UndefinedBranchID,
			"Message4": ledgerstate.UndefinedBranchID,
		})
		checkBranchIDs(t, testFramework, map[string]ledgerstate.BranchID{
			"Message1": branchIDs["A"],
			"Message2": branchIDs["B"],
			"Message3": branchIDs["C"],
			"Message4": ledgerstate.MasterBranchID,
		})
	}

	/*
		testFramework.CreateMessage("Message4", WithStrongParents("Genesis"), WithInputs("C"), WithOutput("D", 500))
		testFramework.CreateMessage("Message5", WithStrongParents("Message1"))
		testFramework.CreateMessage("Message6", WithStrongParents("Message3"))
		testFramework.CreateMessage("Message7", WithStrongParents("Message5"))
		testFramework.CreateMessage("Message8", WithStrongParents("Message6"))
		testFramework.CreateMessage("Message9", WithStrongParents("Message2"), WithInputs("C"), WithOutput("H", 500))

		branchIDs := map[string]ledgerstate.BranchID{
			"A": ledgerstate.NewBranchID(testFramework.TransactionID("Message1")),
			"B": ledgerstate.NewBranchID(testFramework.TransactionID("Message3")),
			"C": ledgerstate.NewBranchID(testFramework.TransactionID("Message2")),
			"D": ledgerstate.NewBranchID(testFramework.TransactionID("Message9")),
		}

		testFramework.IssueMessages("Message1", "Message2", "Message3", "Message4", "Message5", "Message6", "Message7", "Message8", "Message9").WaitMessagesBooked()

		checkMarkers(t, testFramework, map[string]*markers.Markers{
			"Message1": markers.NewMarkers(markers.NewMarker(1, 1)),
			"Message2": markers.NewMarkers(markers.NewMarker(2, 1)),
			"Message3": markers.NewMarkers(markers.NewMarker(3, 1)),
			"Message5": markers.NewMarkers(markers.NewMarker(1, 2)),
			"Message6": markers.NewMarkers(markers.NewMarker(3, 2)),
			"Message7": markers.NewMarkers(markers.NewMarker(1, 3)),
			"Message8": markers.NewMarkers(markers.NewMarker(3, 3)),
			"Message9": markers.NewMarkers(markers.NewMarker(4, 2)),
		})

		checkMessageMetadataBranchIDs(t, testFramework, map[string]ledgerstate.BranchID{
			"Message1": ledgerstate.UndefinedBranchID,
			"Message2": ledgerstate.UndefinedBranchID,
			"Message3": ledgerstate.UndefinedBranchID,
			"Message5": ledgerstate.UndefinedBranchID,
			"Message6": ledgerstate.UndefinedBranchID,
			"Message7": ledgerstate.UndefinedBranchID,
			"Message8": ledgerstate.UndefinedBranchID,
			"Message9": ledgerstate.UndefinedBranchID,
		})

		checkBranchIDs(t, testFramework, map[string]ledgerstate.BranchID{
			"Message1": branchIDs["A"],
			"Message2": branchIDs["C"],
			"Message3": branchIDs["B"],
			"Message5": branchIDs["A"],
			"Message6": branchIDs["B"],
			"Message7": branchIDs["A"],
			"Message8": branchIDs["B"],
			"Message9": aggregatedBranchID(branchIDs["C"], branchIDs["D"]),
		})
	*/
	fmt.Println("DONE")
}

func aggregatedBranchID(branchIDs ...ledgerstate.BranchID) ledgerstate.BranchID {
	return ledgerstate.NewAggregatedBranch(ledgerstate.NewBranchIDs(branchIDs...)).ID()
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
		assert.Equal(t, expectedBranchID, testFramework.tangle.Booker.BranchIDOfMessage(testFramework.Message(messageID).ID()), "BranchID of %s is wrong", messageID)
	}
}

func checkMessageMetadataBranchIDs(t *testing.T, testFramework *MessageTestFramework, expectedBranchIDs map[string]ledgerstate.BranchID) {
	for messageID, expectedBranchID := range expectedBranchIDs {
		assert.True(t, testFramework.tangle.Storage.MessageMetadata(testFramework.Message(messageID).ID()).Consume(func(messageMetadata *MessageMetadata) {
			assert.Equal(t, expectedBranchID, messageMetadata.BranchID())
		}))
	}
}
