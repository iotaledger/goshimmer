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
	defer tangle.Shutdown()

	testFramework := NewMessageTestFramework(
		tangle,
		WithGenesisOutput("A", 500),
		WithGenesisOutput("B", 500),
		WithGenesisOutput("C", 500),
	)

	tangle.Setup()

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

	// ISSUE Message5
	{
		testFramework.CreateMessage("Message5", WithStrongParents("Message4"), WithInputs("C"), WithOutput("D", 500))
		testFramework.IssueMessages("Message5").WaitMessagesBooked()

		checkMarkers(t, testFramework, map[string]*markers.Markers{
			"Message1": markers.NewMarkers(markers.NewMarker(1, 1)),
			"Message2": markers.NewMarkers(markers.NewMarker(2, 1)),
			"Message3": markers.NewMarkers(markers.NewMarker(3, 1)),
			"Message4": markers.NewMarkers(markers.NewMarker(4, 1)),
			"Message5": markers.NewMarkers(markers.NewMarker(4, 2)),
		})
		checkMessageMetadataBranchIDs(t, testFramework, map[string]ledgerstate.BranchID{
			"Message1": ledgerstate.UndefinedBranchID,
			"Message2": ledgerstate.UndefinedBranchID,
			"Message3": ledgerstate.UndefinedBranchID,
			"Message4": ledgerstate.UndefinedBranchID,
			"Message5": ledgerstate.UndefinedBranchID,
		})
		checkBranchIDs(t, testFramework, map[string]ledgerstate.BranchID{
			"Message1": branchIDs["A"],
			"Message2": branchIDs["B"],
			"Message3": branchIDs["C"],
			"Message4": ledgerstate.MasterBranchID,
			"Message5": ledgerstate.MasterBranchID,
		})
	}

	// ISSUE Message6
	{
		testFramework.CreateMessage("Message6", WithStrongParents("Message1"), WithInputs("G"), WithOutput("I", 500))
		testFramework.IssueMessages("Message6").WaitMessagesBooked()

		checkMarkers(t, testFramework, map[string]*markers.Markers{
			"Message1": markers.NewMarkers(markers.NewMarker(1, 1)),
			"Message2": markers.NewMarkers(markers.NewMarker(2, 1)),
			"Message3": markers.NewMarkers(markers.NewMarker(3, 1)),
			"Message4": markers.NewMarkers(markers.NewMarker(4, 1)),
			"Message5": markers.NewMarkers(markers.NewMarker(4, 2)),
			"Message6": markers.NewMarkers(markers.NewMarker(1, 2)),
		})
		checkMessageMetadataBranchIDs(t, testFramework, map[string]ledgerstate.BranchID{
			"Message1": ledgerstate.UndefinedBranchID,
			"Message2": ledgerstate.UndefinedBranchID,
			"Message3": ledgerstate.UndefinedBranchID,
			"Message4": ledgerstate.UndefinedBranchID,
			"Message5": ledgerstate.UndefinedBranchID,
			"Message6": ledgerstate.UndefinedBranchID,
		})
		checkBranchIDs(t, testFramework, map[string]ledgerstate.BranchID{
			"Message1": branchIDs["A"],
			"Message2": branchIDs["B"],
			"Message3": branchIDs["C"],
			"Message4": ledgerstate.MasterBranchID,
			"Message5": ledgerstate.MasterBranchID,
			"Message6": branchIDs["A"],
		})
	}

	// ISSUE Message7
	{
		testFramework.CreateMessage("Message7", WithStrongParents("Message3"))
		testFramework.IssueMessages("Message7").WaitMessagesBooked()

		checkMarkers(t, testFramework, map[string]*markers.Markers{
			"Message1": markers.NewMarkers(markers.NewMarker(1, 1)),
			"Message2": markers.NewMarkers(markers.NewMarker(2, 1)),
			"Message3": markers.NewMarkers(markers.NewMarker(3, 1)),
			"Message4": markers.NewMarkers(markers.NewMarker(4, 1)),
			"Message5": markers.NewMarkers(markers.NewMarker(4, 2)),
			"Message6": markers.NewMarkers(markers.NewMarker(1, 2)),
			"Message7": markers.NewMarkers(markers.NewMarker(3, 2)),
		})
		checkMessageMetadataBranchIDs(t, testFramework, map[string]ledgerstate.BranchID{
			"Message1": ledgerstate.UndefinedBranchID,
			"Message2": ledgerstate.UndefinedBranchID,
			"Message3": ledgerstate.UndefinedBranchID,
			"Message4": ledgerstate.UndefinedBranchID,
			"Message5": ledgerstate.UndefinedBranchID,
			"Message6": ledgerstate.UndefinedBranchID,
			"Message7": ledgerstate.UndefinedBranchID,
		})
		checkBranchIDs(t, testFramework, map[string]ledgerstate.BranchID{
			"Message1": branchIDs["A"],
			"Message2": branchIDs["B"],
			"Message3": branchIDs["C"],
			"Message4": ledgerstate.MasterBranchID,
			"Message5": ledgerstate.MasterBranchID,
			"Message6": branchIDs["A"],
			"Message7": branchIDs["C"],
		})
	}

	// ISSUE Message8
	{
		testFramework.CreateMessage("Message8", WithStrongParents("Message6"))
		testFramework.IssueMessages("Message8").WaitMessagesBooked()

		checkMarkers(t, testFramework, map[string]*markers.Markers{
			"Message1": markers.NewMarkers(markers.NewMarker(1, 1)),
			"Message2": markers.NewMarkers(markers.NewMarker(2, 1)),
			"Message3": markers.NewMarkers(markers.NewMarker(3, 1)),
			"Message4": markers.NewMarkers(markers.NewMarker(4, 1)),
			"Message5": markers.NewMarkers(markers.NewMarker(4, 2)),
			"Message6": markers.NewMarkers(markers.NewMarker(1, 2)),
			"Message7": markers.NewMarkers(markers.NewMarker(3, 2)),
			"Message8": markers.NewMarkers(markers.NewMarker(1, 3)),
		})
		checkMessageMetadataBranchIDs(t, testFramework, map[string]ledgerstate.BranchID{
			"Message1": ledgerstate.UndefinedBranchID,
			"Message2": ledgerstate.UndefinedBranchID,
			"Message3": ledgerstate.UndefinedBranchID,
			"Message4": ledgerstate.UndefinedBranchID,
			"Message5": ledgerstate.UndefinedBranchID,
			"Message6": ledgerstate.UndefinedBranchID,
			"Message7": ledgerstate.UndefinedBranchID,
			"Message8": ledgerstate.UndefinedBranchID,
		})
		checkBranchIDs(t, testFramework, map[string]ledgerstate.BranchID{
			"Message1": branchIDs["A"],
			"Message2": branchIDs["B"],
			"Message3": branchIDs["C"],
			"Message4": ledgerstate.MasterBranchID,
			"Message5": ledgerstate.MasterBranchID,
			"Message6": branchIDs["A"],
			"Message7": branchIDs["C"],
			"Message8": branchIDs["A"],
		})
	}

	// ISSUE Message9
	{
		testFramework.CreateMessage("Message9", WithStrongParents("Message7"))
		testFramework.IssueMessages("Message9").WaitMessagesBooked()

		checkMarkers(t, testFramework, map[string]*markers.Markers{
			"Message1": markers.NewMarkers(markers.NewMarker(1, 1)),
			"Message2": markers.NewMarkers(markers.NewMarker(2, 1)),
			"Message3": markers.NewMarkers(markers.NewMarker(3, 1)),
			"Message4": markers.NewMarkers(markers.NewMarker(4, 1)),
			"Message5": markers.NewMarkers(markers.NewMarker(4, 2)),
			"Message6": markers.NewMarkers(markers.NewMarker(1, 2)),
			"Message7": markers.NewMarkers(markers.NewMarker(3, 2)),
			"Message8": markers.NewMarkers(markers.NewMarker(1, 3)),
			"Message9": markers.NewMarkers(markers.NewMarker(3, 3)),
		})
		checkMessageMetadataBranchIDs(t, testFramework, map[string]ledgerstate.BranchID{
			"Message1": ledgerstate.UndefinedBranchID,
			"Message2": ledgerstate.UndefinedBranchID,
			"Message3": ledgerstate.UndefinedBranchID,
			"Message4": ledgerstate.UndefinedBranchID,
			"Message5": ledgerstate.UndefinedBranchID,
			"Message6": ledgerstate.UndefinedBranchID,
			"Message7": ledgerstate.UndefinedBranchID,
			"Message8": ledgerstate.UndefinedBranchID,
			"Message9": ledgerstate.UndefinedBranchID,
		})
		checkBranchIDs(t, testFramework, map[string]ledgerstate.BranchID{
			"Message1": branchIDs["A"],
			"Message2": branchIDs["B"],
			"Message3": branchIDs["C"],
			"Message4": ledgerstate.MasterBranchID,
			"Message5": ledgerstate.MasterBranchID,
			"Message6": branchIDs["A"],
			"Message7": branchIDs["C"],
			"Message8": branchIDs["A"],
			"Message9": branchIDs["C"],
		})
	}

	// ISSUE Message10
	{
		testFramework.CreateMessage("Message10", WithStrongParents("Message2"), WithInputs("C"), WithOutput("H", 500))
		testFramework.IssueMessages("Message10").WaitMessagesBooked()

		branchIDs["D"] = ledgerstate.NewBranchID(testFramework.TransactionID("Message5"))
		branchIDs["E"] = ledgerstate.NewBranchID(testFramework.TransactionID("Message10"))
		branchIDs["B+E"] = aggregatedBranchID(branchIDs["B"], branchIDs["E"])

		checkMarkers(t, testFramework, map[string]*markers.Markers{
			"Message1":  markers.NewMarkers(markers.NewMarker(1, 1)),
			"Message2":  markers.NewMarkers(markers.NewMarker(2, 1)),
			"Message3":  markers.NewMarkers(markers.NewMarker(3, 1)),
			"Message4":  markers.NewMarkers(markers.NewMarker(4, 1)),
			"Message5":  markers.NewMarkers(markers.NewMarker(4, 2)),
			"Message6":  markers.NewMarkers(markers.NewMarker(1, 2)),
			"Message7":  markers.NewMarkers(markers.NewMarker(3, 2)),
			"Message8":  markers.NewMarkers(markers.NewMarker(1, 3)),
			"Message9":  markers.NewMarkers(markers.NewMarker(3, 3)),
			"Message10": markers.NewMarkers(markers.NewMarker(5, 2)),
		})
		checkMessageMetadataBranchIDs(t, testFramework, map[string]ledgerstate.BranchID{
			"Message1":  ledgerstate.UndefinedBranchID,
			"Message2":  ledgerstate.UndefinedBranchID,
			"Message3":  ledgerstate.UndefinedBranchID,
			"Message4":  ledgerstate.UndefinedBranchID,
			"Message5":  ledgerstate.UndefinedBranchID,
			"Message6":  ledgerstate.UndefinedBranchID,
			"Message7":  ledgerstate.UndefinedBranchID,
			"Message8":  ledgerstate.UndefinedBranchID,
			"Message9":  ledgerstate.UndefinedBranchID,
			"Message10": ledgerstate.UndefinedBranchID,
		})
		checkBranchIDs(t, testFramework, map[string]ledgerstate.BranchID{
			"Message1":  branchIDs["A"],
			"Message2":  branchIDs["B"],
			"Message3":  branchIDs["C"],
			"Message4":  ledgerstate.MasterBranchID,
			"Message5":  branchIDs["D"],
			"Message6":  branchIDs["A"],
			"Message7":  branchIDs["C"],
			"Message8":  branchIDs["A"],
			"Message9":  branchIDs["C"],
			"Message10": branchIDs["B+E"],
		})
	}

	// ISSUE Message11
	{
		testFramework.CreateMessage("Message11", WithStrongParents("Message8", "Message9"))
		testFramework.IssueMessages("Message11").WaitMessagesBooked()

		branchIDs["A+C"] = aggregatedBranchID(branchIDs["A"], branchIDs["C"])

		checkMarkers(t, testFramework, map[string]*markers.Markers{
			"Message1":  markers.NewMarkers(markers.NewMarker(1, 1)),
			"Message2":  markers.NewMarkers(markers.NewMarker(2, 1)),
			"Message3":  markers.NewMarkers(markers.NewMarker(3, 1)),
			"Message4":  markers.NewMarkers(markers.NewMarker(4, 1)),
			"Message5":  markers.NewMarkers(markers.NewMarker(4, 2)),
			"Message6":  markers.NewMarkers(markers.NewMarker(1, 2)),
			"Message7":  markers.NewMarkers(markers.NewMarker(3, 2)),
			"Message8":  markers.NewMarkers(markers.NewMarker(1, 3)),
			"Message9":  markers.NewMarkers(markers.NewMarker(3, 3)),
			"Message10": markers.NewMarkers(markers.NewMarker(5, 2)),
			"Message11": markers.NewMarkers(markers.NewMarker(6, 4)),
		})
		checkMessageMetadataBranchIDs(t, testFramework, map[string]ledgerstate.BranchID{
			"Message1":  ledgerstate.UndefinedBranchID,
			"Message2":  ledgerstate.UndefinedBranchID,
			"Message3":  ledgerstate.UndefinedBranchID,
			"Message4":  ledgerstate.UndefinedBranchID,
			"Message5":  ledgerstate.UndefinedBranchID,
			"Message6":  ledgerstate.UndefinedBranchID,
			"Message7":  ledgerstate.UndefinedBranchID,
			"Message8":  ledgerstate.UndefinedBranchID,
			"Message9":  ledgerstate.UndefinedBranchID,
			"Message10": ledgerstate.UndefinedBranchID,
			"Message11": ledgerstate.UndefinedBranchID,
		})
		checkBranchIDs(t, testFramework, map[string]ledgerstate.BranchID{
			"Message1":  branchIDs["A"],
			"Message2":  branchIDs["B"],
			"Message3":  branchIDs["C"],
			"Message4":  ledgerstate.MasterBranchID,
			"Message5":  branchIDs["D"],
			"Message6":  branchIDs["A"],
			"Message7":  branchIDs["C"],
			"Message8":  branchIDs["A"],
			"Message9":  branchIDs["C"],
			"Message10": branchIDs["B+E"],
			"Message11": branchIDs["A+C"],
		})
	}

	// ISSUE Message12
	{
		testFramework.CreateMessage("Message12", WithStrongParents("Message8", "Message9"))
		testFramework.IssueMessages("Message12").WaitMessagesBooked()

		checkMarkers(t, testFramework, map[string]*markers.Markers{
			"Message1":  markers.NewMarkers(markers.NewMarker(1, 1)),
			"Message2":  markers.NewMarkers(markers.NewMarker(2, 1)),
			"Message3":  markers.NewMarkers(markers.NewMarker(3, 1)),
			"Message4":  markers.NewMarkers(markers.NewMarker(4, 1)),
			"Message5":  markers.NewMarkers(markers.NewMarker(4, 2)),
			"Message6":  markers.NewMarkers(markers.NewMarker(1, 2)),
			"Message7":  markers.NewMarkers(markers.NewMarker(3, 2)),
			"Message8":  markers.NewMarkers(markers.NewMarker(1, 3)),
			"Message9":  markers.NewMarkers(markers.NewMarker(3, 3)),
			"Message10": markers.NewMarkers(markers.NewMarker(5, 2)),
			"Message11": markers.NewMarkers(markers.NewMarker(6, 4)),
			"Message12": markers.NewMarkers(markers.NewMarker(1, 3), markers.NewMarker(3, 3)),
		})
		checkMessageMetadataBranchIDs(t, testFramework, map[string]ledgerstate.BranchID{
			"Message1":  ledgerstate.UndefinedBranchID,
			"Message2":  ledgerstate.UndefinedBranchID,
			"Message3":  ledgerstate.UndefinedBranchID,
			"Message4":  ledgerstate.UndefinedBranchID,
			"Message5":  ledgerstate.UndefinedBranchID,
			"Message6":  ledgerstate.UndefinedBranchID,
			"Message7":  ledgerstate.UndefinedBranchID,
			"Message8":  ledgerstate.UndefinedBranchID,
			"Message9":  ledgerstate.UndefinedBranchID,
			"Message10": ledgerstate.UndefinedBranchID,
			"Message11": ledgerstate.UndefinedBranchID,
			"Message12": branchIDs["A+C"],
		})
		checkBranchIDs(t, testFramework, map[string]ledgerstate.BranchID{
			"Message1":  branchIDs["A"],
			"Message2":  branchIDs["B"],
			"Message3":  branchIDs["C"],
			"Message4":  ledgerstate.MasterBranchID,
			"Message5":  branchIDs["D"],
			"Message6":  branchIDs["A"],
			"Message7":  branchIDs["C"],
			"Message8":  branchIDs["A"],
			"Message9":  branchIDs["C"],
			"Message10": branchIDs["B+E"],
			"Message11": branchIDs["A+C"],
			"Message12": branchIDs["A+C"],
		})
	}

	// ISSUE Message13
	{
		testFramework.CreateMessage("Message13", WithStrongParents("Message6", "Message7"), WithWeakParents("Message10"))
		testFramework.IssueMessages("Message13").WaitMessagesBooked()

		branchIDs["A+C+E"] = aggregatedBranchID(branchIDs["A"], branchIDs["C"], branchIDs["E"])

		checkMarkers(t, testFramework, map[string]*markers.Markers{
			"Message1":  markers.NewMarkers(markers.NewMarker(1, 1)),
			"Message2":  markers.NewMarkers(markers.NewMarker(2, 1)),
			"Message3":  markers.NewMarkers(markers.NewMarker(3, 1)),
			"Message4":  markers.NewMarkers(markers.NewMarker(4, 1)),
			"Message5":  markers.NewMarkers(markers.NewMarker(4, 2)),
			"Message6":  markers.NewMarkers(markers.NewMarker(1, 2)),
			"Message7":  markers.NewMarkers(markers.NewMarker(3, 2)),
			"Message8":  markers.NewMarkers(markers.NewMarker(1, 3)),
			"Message9":  markers.NewMarkers(markers.NewMarker(3, 3)),
			"Message10": markers.NewMarkers(markers.NewMarker(5, 2)),
			"Message11": markers.NewMarkers(markers.NewMarker(6, 4)),
			"Message12": markers.NewMarkers(markers.NewMarker(1, 3), markers.NewMarker(3, 3)),
			"Message13": markers.NewMarkers(markers.NewMarker(7, 3)),
		})
		checkMessageMetadataBranchIDs(t, testFramework, map[string]ledgerstate.BranchID{
			"Message1":  ledgerstate.UndefinedBranchID,
			"Message2":  ledgerstate.UndefinedBranchID,
			"Message3":  ledgerstate.UndefinedBranchID,
			"Message4":  ledgerstate.UndefinedBranchID,
			"Message5":  ledgerstate.UndefinedBranchID,
			"Message6":  ledgerstate.UndefinedBranchID,
			"Message7":  ledgerstate.UndefinedBranchID,
			"Message8":  ledgerstate.UndefinedBranchID,
			"Message9":  ledgerstate.UndefinedBranchID,
			"Message10": ledgerstate.UndefinedBranchID,
			"Message11": ledgerstate.UndefinedBranchID,
			"Message12": branchIDs["A+C"],
			"Message13": ledgerstate.UndefinedBranchID,
		})
		checkBranchIDs(t, testFramework, map[string]ledgerstate.BranchID{
			"Message1":  branchIDs["A"],
			"Message2":  branchIDs["B"],
			"Message3":  branchIDs["C"],
			"Message4":  ledgerstate.MasterBranchID,
			"Message5":  branchIDs["D"],
			"Message6":  branchIDs["A"],
			"Message7":  branchIDs["C"],
			"Message8":  branchIDs["A"],
			"Message9":  branchIDs["C"],
			"Message10": branchIDs["B+E"],
			"Message11": branchIDs["A+C"],
			"Message12": branchIDs["A+C"],
			"Message13": branchIDs["A+C+E"],
		})
	}

	// ISSUE Message14
	{
		testFramework.CreateMessage("Message14", WithStrongParents("Message12"))
		testFramework.IssueMessages("Message14").WaitMessagesBooked()

		checkMarkers(t, testFramework, map[string]*markers.Markers{
			"Message1":  markers.NewMarkers(markers.NewMarker(1, 1)),
			"Message2":  markers.NewMarkers(markers.NewMarker(2, 1)),
			"Message3":  markers.NewMarkers(markers.NewMarker(3, 1)),
			"Message4":  markers.NewMarkers(markers.NewMarker(4, 1)),
			"Message5":  markers.NewMarkers(markers.NewMarker(4, 2)),
			"Message6":  markers.NewMarkers(markers.NewMarker(1, 2)),
			"Message7":  markers.NewMarkers(markers.NewMarker(3, 2)),
			"Message8":  markers.NewMarkers(markers.NewMarker(1, 3)),
			"Message9":  markers.NewMarkers(markers.NewMarker(3, 3)),
			"Message10": markers.NewMarkers(markers.NewMarker(5, 2)),
			"Message11": markers.NewMarkers(markers.NewMarker(6, 4)),
			"Message12": markers.NewMarkers(markers.NewMarker(1, 3), markers.NewMarker(3, 3)),
			"Message13": markers.NewMarkers(markers.NewMarker(7, 3)),
			"Message14": markers.NewMarkers(markers.NewMarker(1, 3), markers.NewMarker(3, 3)),
		})
		checkMessageMetadataBranchIDs(t, testFramework, map[string]ledgerstate.BranchID{
			"Message1":  ledgerstate.UndefinedBranchID,
			"Message2":  ledgerstate.UndefinedBranchID,
			"Message3":  ledgerstate.UndefinedBranchID,
			"Message4":  ledgerstate.UndefinedBranchID,
			"Message5":  ledgerstate.UndefinedBranchID,
			"Message6":  ledgerstate.UndefinedBranchID,
			"Message7":  ledgerstate.UndefinedBranchID,
			"Message8":  ledgerstate.UndefinedBranchID,
			"Message9":  ledgerstate.UndefinedBranchID,
			"Message10": ledgerstate.UndefinedBranchID,
			"Message11": ledgerstate.UndefinedBranchID,
			"Message12": branchIDs["A+C"],
			"Message13": ledgerstate.UndefinedBranchID,
			"Message14": branchIDs["A+C"],
		})
		checkBranchIDs(t, testFramework, map[string]ledgerstate.BranchID{
			"Message1":  branchIDs["A"],
			"Message2":  branchIDs["B"],
			"Message3":  branchIDs["C"],
			"Message4":  ledgerstate.MasterBranchID,
			"Message5":  branchIDs["D"],
			"Message6":  branchIDs["A"],
			"Message7":  branchIDs["C"],
			"Message8":  branchIDs["A"],
			"Message9":  branchIDs["C"],
			"Message10": branchIDs["B+E"],
			"Message11": branchIDs["A+C"],
			"Message12": branchIDs["A+C"],
			"Message13": branchIDs["A+C+E"],
			"Message14": branchIDs["A+C"],
		})
	}

	// ISSUE Message15
	{
		testFramework.CreateMessage("Message15", WithStrongParents("Message11", "Message14"))
		testFramework.PreventNewMarkers(true).IssueMessages("Message15").WaitMessagesBooked().PreventNewMarkers(false)

		checkMarkers(t, testFramework, map[string]*markers.Markers{
			"Message1":  markers.NewMarkers(markers.NewMarker(1, 1)),
			"Message2":  markers.NewMarkers(markers.NewMarker(2, 1)),
			"Message3":  markers.NewMarkers(markers.NewMarker(3, 1)),
			"Message4":  markers.NewMarkers(markers.NewMarker(4, 1)),
			"Message5":  markers.NewMarkers(markers.NewMarker(4, 2)),
			"Message6":  markers.NewMarkers(markers.NewMarker(1, 2)),
			"Message7":  markers.NewMarkers(markers.NewMarker(3, 2)),
			"Message8":  markers.NewMarkers(markers.NewMarker(1, 3)),
			"Message9":  markers.NewMarkers(markers.NewMarker(3, 3)),
			"Message10": markers.NewMarkers(markers.NewMarker(5, 2)),
			"Message11": markers.NewMarkers(markers.NewMarker(6, 4)),
			"Message12": markers.NewMarkers(markers.NewMarker(1, 3), markers.NewMarker(3, 3)),
			"Message13": markers.NewMarkers(markers.NewMarker(7, 3)),
			"Message14": markers.NewMarkers(markers.NewMarker(1, 3), markers.NewMarker(3, 3)),
			"Message15": markers.NewMarkers(markers.NewMarker(6, 4)),
		})
		checkMessageMetadataBranchIDs(t, testFramework, map[string]ledgerstate.BranchID{
			"Message1":  ledgerstate.UndefinedBranchID,
			"Message2":  ledgerstate.UndefinedBranchID,
			"Message3":  ledgerstate.UndefinedBranchID,
			"Message4":  ledgerstate.UndefinedBranchID,
			"Message5":  ledgerstate.UndefinedBranchID,
			"Message6":  ledgerstate.UndefinedBranchID,
			"Message7":  ledgerstate.UndefinedBranchID,
			"Message8":  ledgerstate.UndefinedBranchID,
			"Message9":  ledgerstate.UndefinedBranchID,
			"Message10": ledgerstate.UndefinedBranchID,
			"Message11": ledgerstate.UndefinedBranchID,
			"Message12": branchIDs["A+C"],
			"Message13": ledgerstate.UndefinedBranchID,
			"Message14": branchIDs["A+C"],
			"Message15": ledgerstate.UndefinedBranchID,
		})
		checkBranchIDs(t, testFramework, map[string]ledgerstate.BranchID{
			"Message1":  branchIDs["A"],
			"Message2":  branchIDs["B"],
			"Message3":  branchIDs["C"],
			"Message4":  ledgerstate.MasterBranchID,
			"Message5":  branchIDs["D"],
			"Message6":  branchIDs["A"],
			"Message7":  branchIDs["C"],
			"Message8":  branchIDs["A"],
			"Message9":  branchIDs["C"],
			"Message10": branchIDs["B+E"],
			"Message11": branchIDs["A+C"],
			"Message12": branchIDs["A+C"],
			"Message13": branchIDs["A+C+E"],
			"Message14": branchIDs["A+C"],
			"Message15": branchIDs["A+C"],
		})
	}

	fmt.Println("DONE")
}

func aggregatedBranchID(branchIDs ...ledgerstate.BranchID) ledgerstate.BranchID {
	return ledgerstate.NewAggregatedBranch(ledgerstate.NewBranchIDs(branchIDs...)).ID()
}

func checkMarkers(t *testing.T, testFramework *MessageTestFramework, expectedMarkers map[string]*markers.Markers) {
	for messageID, expectedMarkersOfMessage := range expectedMarkers {
		assert.True(t, testFramework.tangle.Storage.MessageMetadata(testFramework.Message(messageID).ID()).Consume(func(messageMetadata *MessageMetadata) {
			assert.Equal(t, expectedMarkersOfMessage, messageMetadata.StructureDetails().PastMarkers, "Markers of %s are wrong", messageID)
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
			assert.Equal(t, expectedBranchID, messageMetadata.BranchID(), "BranchID of %s is wrong in the Metadata", messageID)
		}))
	}
}
