//nolint:dupl
package tangle

import (
	"testing"

	"github.com/iotaledger/hive.go/objectstorage"
	"github.com/stretchr/testify/assert"

	"github.com/iotaledger/goshimmer/packages/ledgerstate"
	"github.com/iotaledger/goshimmer/packages/markers"
)

func TestScenario_1(t *testing.T) {
	tangle := NewTestTangle()
	defer tangle.Shutdown()

	testFramework := NewMessageTestFramework(
		tangle,
		WithGenesisOutput("G", 3),
	)

	tangle.Setup()

	testFramework.CreateMessage("Message1", WithStrongParents("Genesis"), WithInputs("G"), WithOutput("A", 1), WithOutput("B", 1), WithOutput("C", 1))
	testFramework.CreateMessage("Message2", WithStrongParents("Genesis", "Message1"), WithInputs("B", "C"), WithOutput("E", 2))
	testFramework.CreateMessage("Message3", WithStrongParents("Message1", "Message2"), WithReattachment("Message2"))
	testFramework.CreateMessage("Message4", WithStrongParents("Genesis", "Message1"), WithInputs("A"), WithOutput("D", 1))
	testFramework.CreateMessage("Message5", WithStrongParents("Message1", "Message2"), WithInputs("A"), WithOutput("F", 1))
	testFramework.CreateMessage("Message6", WithStrongParents("Message2", "Message5"), WithInputs("E", "F"), WithOutput("H", 3))
	testFramework.CreateMessage("Message7", WithStrongParents("Message4", "Message5"), WithReattachment("Message2"))
	testFramework.CreateMessage("Message8", WithStrongParents("Message4", "Message5"), WithInputs("F", "D"), WithOutput("I", 2))
	testFramework.CreateMessage("Message9", WithStrongParents("Message4", "Message6"), WithInputs("H"), WithOutput("J", 1))

	testFramework.RegisterBranchID("red", "Message4")
	testFramework.RegisterBranchID("yellow", "Message5")

	testFramework.IssueMessages("Message1", "Message2", "Message3", "Message4", "Message5", "Message6").WaitMessagesBooked()
	testFramework.IssueMessages("Message7", "Message9").WaitMessagesBooked()
	testFramework.IssueMessages("Message8").WaitMessagesBooked()

	for _, messageAlias := range []string{"Message7", "Message8", "Message9"} {
		assert.Truef(t, testFramework.MessageMetadata(messageAlias).invalid, "%s not invalid", messageAlias)
	}

	checkBranchIDs(t, testFramework, map[string]ledgerstate.BranchID{
		"Message1": ledgerstate.MasterBranchID,
		"Message3": ledgerstate.MasterBranchID,
		"Message2": ledgerstate.MasterBranchID,
		"Message4": testFramework.BranchID("red"),
		"Message5": testFramework.BranchID("yellow"),
	})
}

func TestScenario_2(t *testing.T) {
	tangle := NewTestTangle()
	defer tangle.Shutdown()

	testFramework := NewMessageTestFramework(
		tangle,
		WithGenesisOutput("G", 3),
	)

	tangle.Setup()

	testFramework.CreateMessage("Message1", WithStrongParents("Genesis"), WithInputs("G"), WithOutput("A", 1), WithOutput("B", 1), WithOutput("C", 1))
	testFramework.CreateMessage("Message2", WithStrongParents("Genesis", "Message1"), WithInputs("B", "C"), WithOutput("E", 2))
	testFramework.CreateMessage("Message3", WithStrongParents("Message1", "Message2"), WithReattachment("Message2"))
	testFramework.CreateMessage("Message4", WithStrongParents("Genesis", "Message1"), WithInputs("A"), WithOutput("D", 1))
	testFramework.CreateMessage("Message5", WithStrongParents("Message1", "Message2"), WithInputs("A"), WithOutput("F", 1))
	testFramework.CreateMessage("Message6", WithStrongParents("Message2", "Message5"), WithInputs("E", "F"), WithOutput("L", 3))
	testFramework.CreateMessage("Message7", WithStrongParents("Message1", "Message4"), WithInputs("C"), WithOutput("H", 1))
	testFramework.CreateMessage("Message8", WithStrongParents("Message4", "Message7"), WithInputs("H", "D"), WithOutput("I", 2))
	testFramework.CreateMessage("Message9", WithStrongParents("Message4", "Message7"), WithInputs("B"), WithOutput("J", 1))

	testFramework.IssueMessages("Message1").WaitMessagesBooked()
	testFramework.IssueMessages("Message2").WaitMessagesBooked()
	testFramework.IssueMessages("Message3", "Message4").WaitMessagesBooked()
	testFramework.IssueMessages("Message5").WaitMessagesBooked()
	testFramework.IssueMessages("Message6").WaitMessagesBooked()
	testFramework.IssueMessages("Message7").WaitMessagesBooked()
	testFramework.IssueMessages("Message8").WaitMessagesBooked()
	testFramework.IssueMessages("Message9").WaitMessagesBooked()

	testFramework.RegisterBranchID("purple", "Message2")
	testFramework.RegisterBranchID("red", "Message4")
	testFramework.RegisterBranchID("yellow+purple", "Message2", "Message5")
	testFramework.RegisterBranchID("red+orange", "Message4", "Message7")
	testFramework.RegisterBranchID("red+orange+blue", "Message4", "Message7", "Message9")

	checkBranchIDs(t, testFramework, map[string]ledgerstate.BranchID{
		"Message1": ledgerstate.MasterBranchID,
		"Message2": testFramework.BranchID("purple"),
		"Message3": testFramework.BranchID("purple"),
		"Message4": testFramework.BranchID("red"),
		"Message5": testFramework.BranchID("yellow+purple"),
		"Message6": testFramework.BranchID("yellow+purple"),
		"Message7": testFramework.BranchID("red+orange"),
		"Message8": testFramework.BranchID("red+orange"),
		"Message9": testFramework.BranchID("red+orange+blue"),
	})

	checkMarkers(t, testFramework, map[string]*markers.Markers{
		"Message1": markers.NewMarkers(markers.NewMarker(1, 1)),
		"Message2": markers.NewMarkers(markers.NewMarker(1, 2)),
		"Message3": markers.NewMarkers(markers.NewMarker(1, 3)),
		"Message4": markers.NewMarkers(markers.NewMarker(1, 1)),
		"Message5": markers.NewMarkers(markers.NewMarker(2, 3)),
		"Message6": markers.NewMarkers(markers.NewMarker(2, 4)),
		"Message7": markers.NewMarkers(markers.NewMarker(3, 2)),
		"Message8": markers.NewMarkers(markers.NewMarker(3, 3)),
		"Message9": markers.NewMarkers(markers.NewMarker(4, 3)),
	})
}

func TestScenario_3(t *testing.T) {
	tangle := NewTestTangle()
	defer tangle.Shutdown()

	testFramework := NewMessageTestFramework(
		tangle,
		WithGenesisOutput("G", 3),
	)

	tangle.Setup()

	testFramework.CreateMessage("Message1", WithStrongParents("Genesis"), WithInputs("G"), WithOutput("A", 1), WithOutput("B", 1), WithOutput("C", 1))
	testFramework.CreateMessage("Message2", WithStrongParents("Genesis", "Message1"), WithInputs("B", "C"), WithOutput("E", 2))
	testFramework.CreateMessage("Message3", WithStrongParents("Message1", "Message2"), WithReattachment("Message2"))
	testFramework.CreateMessage("Message4", WithStrongParents("Genesis", "Message1"), WithInputs("A"), WithOutput("D", 1))
	testFramework.CreateMessage("Message5", WithStrongParents("Message1"), WithWeakParents("Message2"), WithInputs("A"), WithOutput("F", 1))
	testFramework.CreateMessage("Message6", WithStrongParents("Message2", "Message5"), WithInputs("E", "F"), WithOutput("L", 3))
	testFramework.CreateMessage("Message7", WithStrongParents("Message1", "Message4"), WithInputs("C"), WithOutput("H", 1))
	testFramework.CreateMessage("Message8", WithStrongParents("Message4", "Message7"), WithInputs("H", "D"), WithOutput("I", 2))
	testFramework.CreateMessage("Message9", WithStrongParents("Message4", "Message7"), WithInputs("B"), WithOutput("J", 1))

	testFramework.IssueMessages("Message1").WaitMessagesBooked()
	testFramework.IssueMessages("Message2").WaitMessagesBooked()
	testFramework.IssueMessages("Message3", "Message4").WaitMessagesBooked()
	testFramework.IssueMessages("Message5").WaitMessagesBooked()
	testFramework.IssueMessages("Message6").WaitMessagesBooked()
	testFramework.IssueMessages("Message7").WaitMessagesBooked()
	testFramework.IssueMessages("Message8").WaitMessagesBooked()
	testFramework.IssueMessages("Message9").WaitMessagesBooked()

	testFramework.RegisterBranchID("purple", "Message2")
	testFramework.RegisterBranchID("red", "Message4")
	testFramework.RegisterBranchID("yellow", "Message5")
	testFramework.RegisterBranchID("yellow+purple", "Message2", "Message5")
	testFramework.RegisterBranchID("red+orange", "Message4", "Message7")
	testFramework.RegisterBranchID("red+orange+blue", "Message4", "Message7", "Message9")

	checkBranchIDs(t, testFramework, map[string]ledgerstate.BranchID{
		"Message1": ledgerstate.MasterBranchID,
		"Message2": testFramework.BranchID("purple"),
		"Message3": testFramework.BranchID("purple"),
		"Message4": testFramework.BranchID("red"),
		"Message5": testFramework.BranchID("yellow"),
		"Message6": testFramework.BranchID("yellow+purple"),
		"Message7": testFramework.BranchID("red+orange"),
		"Message8": testFramework.BranchID("red+orange"),
		"Message9": testFramework.BranchID("red+orange+blue"),
	})
}

func TestBookerMarkerGap(t *testing.T) {
	tangle := NewTestTangle()
	defer tangle.Shutdown()

	testFramework := NewMessageTestFramework(
		tangle,
		WithGenesisOutput("A", 500),
		WithGenesisOutput("B", 500),
		WithGenesisOutput("C", 500),
		WithGenesisOutput("L", 500),
	)

	tangle.Setup()

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
	}

	// ISSUE Message1.5
	{
		testFramework.CreateMessage("Message1.5", WithStrongParents("Message1"))
		testFramework.IssueMessages("Message1.5").WaitMessagesBooked()

		checkMarkers(t, testFramework, map[string]*markers.Markers{
			"Message1":   markers.NewMarkers(markers.NewMarker(1, 1)),
			"Message1.5": markers.NewMarkers(markers.NewMarker(1, 2)),
		})
		checkMessageMetadataBranchIDs(t, testFramework, map[string]ledgerstate.BranchID{
			"Message1":   ledgerstate.UndefinedBranchID,
			"Message1.5": ledgerstate.UndefinedBranchID,
		})
		checkBranchIDs(t, testFramework, map[string]ledgerstate.BranchID{
			"Message1":   ledgerstate.MasterBranchID,
			"Message1.5": ledgerstate.MasterBranchID,
		})
	}

	// ISSUE Message2
	{
		testFramework.CreateMessage("Message2", WithStrongParents("Message1.5"), WithInputs("B"), WithOutput("E", 500))

		testFramework.IssueMessages("Message2").WaitMessagesBooked()

		checkMarkers(t, testFramework, map[string]*markers.Markers{
			"Message1":   markers.NewMarkers(markers.NewMarker(1, 1)),
			"Message1.5": markers.NewMarkers(markers.NewMarker(1, 2)),
			"Message2":   markers.NewMarkers(markers.NewMarker(1, 3)),
		})
		checkMessageMetadataBranchIDs(t, testFramework, map[string]ledgerstate.BranchID{
			"Message1":   ledgerstate.UndefinedBranchID,
			"Message1.5": ledgerstate.UndefinedBranchID,
			"Message2":   ledgerstate.UndefinedBranchID,
		})
		checkBranchIDs(t, testFramework, map[string]ledgerstate.BranchID{
			"Message1":   ledgerstate.MasterBranchID,
			"Message1.5": ledgerstate.MasterBranchID,
			"Message2":   ledgerstate.MasterBranchID,
		})
	}

	// ISSUE Message3
	{
		testFramework.CreateMessage("Message3", WithStrongParents("Genesis"), WithInputs("B"), WithOutput("F", 500))

		testFramework.RegisterBranchID("Message2", "Message2")
		testFramework.RegisterBranchID("Message3", "Message3")

		testFramework.IssueMessages("Message3").WaitMessagesBooked()

		checkMarkers(t, testFramework, map[string]*markers.Markers{
			"Message1":   markers.NewMarkers(markers.NewMarker(1, 1)),
			"Message1.5": markers.NewMarkers(markers.NewMarker(1, 2)),
			"Message2":   markers.NewMarkers(markers.NewMarker(1, 3)),
			"Message3":   markers.NewMarkers(markers.NewMarker(2, 1)),
		})
		checkMessageMetadataBranchIDs(t, testFramework, map[string]ledgerstate.BranchID{
			"Message1":   ledgerstate.UndefinedBranchID,
			"Message1.5": ledgerstate.UndefinedBranchID,
			"Message2":   ledgerstate.UndefinedBranchID,
			"Message3":   ledgerstate.UndefinedBranchID,
		})
		checkBranchIDs(t, testFramework, map[string]ledgerstate.BranchID{
			"Message1":   ledgerstate.MasterBranchID,
			"Message1.5": ledgerstate.MasterBranchID,
			"Message2":   testFramework.BranchID("Message2"),
			"Message3":   testFramework.BranchID("Message3"),
		})
	}

	// ISSUE Message4
	{
		testFramework.CreateMessage("Message4", WithStrongParents("Message3"), WithInputs("A"), WithOutput("X", 500))

		testFramework.RegisterBranchID("Message1", "Message1")
		testFramework.RegisterBranchID("Message4", "Message4")

		testFramework.RegisterBranchID("Message1+Message2", "Message1", "Message2")
		testFramework.RegisterBranchID("Message3+Message4", "Message3", "Message4")

		testFramework.IssueMessages("Message4").WaitMessagesBooked()

		checkMarkers(t, testFramework, map[string]*markers.Markers{
			"Message1":   markers.NewMarkers(markers.NewMarker(1, 1)),
			"Message1.5": markers.NewMarkers(markers.NewMarker(1, 2)),
			"Message2":   markers.NewMarkers(markers.NewMarker(1, 3)),
			"Message3":   markers.NewMarkers(markers.NewMarker(2, 1)),
			"Message4":   markers.NewMarkers(markers.NewMarker(3, 2)),
		})
		checkMessageMetadataBranchIDs(t, testFramework, map[string]ledgerstate.BranchID{
			"Message1":   ledgerstate.UndefinedBranchID,
			"Message1.5": ledgerstate.UndefinedBranchID,
			"Message2":   ledgerstate.UndefinedBranchID,
			"Message3":   ledgerstate.UndefinedBranchID,
			"Message4":   ledgerstate.UndefinedBranchID,
		})
		checkBranchIDs(t, testFramework, map[string]ledgerstate.BranchID{
			"Message1":   testFramework.BranchID("Message1"),
			"Message1.5": testFramework.BranchID("Message1"),
			"Message2":   testFramework.BranchID("Message1+Message2"),
			"Message3":   testFramework.BranchID("Message3"),
			"Message4":   testFramework.BranchID("Message3+Message4"),
		})
	}
}

func TestBookerMarkerGap2(t *testing.T) {
	tangle := NewTestTangle()
	defer tangle.Shutdown()

	testFramework := NewMessageTestFramework(
		tangle,
		WithGenesisOutput("Genesis1", 500),
		WithGenesisOutput("Genesis2", 500),
		WithGenesisOutput("Genesis3", 500),
	)

	tangle.Setup()

	// ISSUE Message1
	{
		testFramework.CreateMessage("Message1", WithStrongParents("Genesis"), WithInputs("Genesis1"), WithOutput("Message1", 500))
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
	}

	// ISSUE Message2
	{
		testFramework.CreateMessage("Message2", WithStrongParents("Genesis"), WithInputs("Genesis1"), WithOutput("Message2", 500))
		testFramework.IssueMessages("Message2").WaitMessagesBooked()

		testFramework.RegisterBranchID("Message1", "Message1")
		testFramework.RegisterBranchID("Message2", "Message2")

		checkMarkers(t, testFramework, map[string]*markers.Markers{
			"Message1": markers.NewMarkers(markers.NewMarker(1, 1)),
			"Message2": markers.NewMarkers(markers.NewMarker(2, 1)),
		})
		checkMessageMetadataBranchIDs(t, testFramework, map[string]ledgerstate.BranchID{
			"Message1": ledgerstate.UndefinedBranchID,
			"Message2": ledgerstate.UndefinedBranchID,
		})
		checkBranchIDs(t, testFramework, map[string]ledgerstate.BranchID{
			"Message1": testFramework.BranchID("Message1"),
			"Message2": testFramework.BranchID("Message2"),
		})
	}

	// ISSUE Message3
	{
		testFramework.CreateMessage("Message3", WithStrongParents("Genesis"), WithInputs("Genesis2"), WithOutput("Message3", 500))
		testFramework.IssueMessages("Message3").WaitMessagesBooked()

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
			"Message1": testFramework.BranchID("Message1"),
			"Message2": testFramework.BranchID("Message2"),
			"Message3": ledgerstate.MasterBranchID,
		})
	}

	// ISSUE Message4
	{
		testFramework.CreateMessage("Message4", WithStrongParents("Genesis"), WithInputs("Genesis2"), WithOutput("Message4", 500))
		testFramework.IssueMessages("Message4").WaitMessagesBooked()

		testFramework.RegisterBranchID("Message3", "Message3")
		testFramework.RegisterBranchID("Message4", "Message4")

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
			"Message1": testFramework.BranchID("Message1"),
			"Message2": testFramework.BranchID("Message2"),
			"Message3": testFramework.BranchID("Message3"),
			"Message4": testFramework.BranchID("Message4"),
		})
	}

	// ISSUE Message5
	{
		testFramework.CreateMessage("Message5", WithStrongParents("Message1"), WithInputs("Genesis3"), WithOutput("Message5", 500))
		testFramework.IssueMessages("Message5").WaitMessagesBooked()

		checkMarkers(t, testFramework, map[string]*markers.Markers{
			"Message1": markers.NewMarkers(markers.NewMarker(1, 1)),
			"Message2": markers.NewMarkers(markers.NewMarker(2, 1)),
			"Message3": markers.NewMarkers(markers.NewMarker(3, 1)),
			"Message4": markers.NewMarkers(markers.NewMarker(4, 1)),
			"Message5": markers.NewMarkers(markers.NewMarker(1, 2)),
		})
		checkMessageMetadataBranchIDs(t, testFramework, map[string]ledgerstate.BranchID{
			"Message1": ledgerstate.UndefinedBranchID,
			"Message2": ledgerstate.UndefinedBranchID,
			"Message3": ledgerstate.UndefinedBranchID,
			"Message4": ledgerstate.UndefinedBranchID,
			"Message5": ledgerstate.UndefinedBranchID,
		})
		checkBranchIDs(t, testFramework, map[string]ledgerstate.BranchID{
			"Message1": testFramework.BranchID("Message1"),
			"Message2": testFramework.BranchID("Message2"),
			"Message3": testFramework.BranchID("Message3"),
			"Message4": testFramework.BranchID("Message4"),
			"Message5": testFramework.BranchID("Message1"),
		})
	}

	// ISSUE Message6
	{
		testFramework.CreateMessage("Message6", WithStrongParents("Message1", "Message3"))
		testFramework.IssueMessages("Message6").WaitMessagesBooked()

		testFramework.RegisterBranchID("Message1+Message3", "Message1", "Message3")

		checkMarkers(t, testFramework, map[string]*markers.Markers{
			"Message1": markers.NewMarkers(markers.NewMarker(1, 1)),
			"Message2": markers.NewMarkers(markers.NewMarker(2, 1)),
			"Message3": markers.NewMarkers(markers.NewMarker(3, 1)),
			"Message4": markers.NewMarkers(markers.NewMarker(4, 1)),
			"Message5": markers.NewMarkers(markers.NewMarker(1, 2)),
			"Message6": markers.NewMarkers(markers.NewMarker(5, 2)),
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
			"Message1": testFramework.BranchID("Message1"),
			"Message2": testFramework.BranchID("Message2"),
			"Message3": testFramework.BranchID("Message3"),
			"Message4": testFramework.BranchID("Message4"),
			"Message5": testFramework.BranchID("Message1"),
			"Message6": testFramework.BranchID("Message1+Message3"),
		})
	}

	// ISSUE Message7
	{
		testFramework.CreateMessage("Message7", WithStrongParents("Message3", "Message5"))
		testFramework.IssueMessages("Message7").WaitMessagesBooked()

		testFramework.RegisterBranchID("Message1+Message3", "Message1", "Message3")

		checkMarkers(t, testFramework, map[string]*markers.Markers{
			"Message1": markers.NewMarkers(markers.NewMarker(1, 1)),
			"Message2": markers.NewMarkers(markers.NewMarker(2, 1)),
			"Message3": markers.NewMarkers(markers.NewMarker(3, 1)),
			"Message4": markers.NewMarkers(markers.NewMarker(4, 1)),
			"Message5": markers.NewMarkers(markers.NewMarker(1, 2)),
			"Message6": markers.NewMarkers(markers.NewMarker(5, 2)),
			"Message7": markers.NewMarkers(markers.NewMarker(1, 2), markers.NewMarker(3, 1)),
		})
		checkMessageMetadataBranchIDs(t, testFramework, map[string]ledgerstate.BranchID{
			"Message1": ledgerstate.UndefinedBranchID,
			"Message2": ledgerstate.UndefinedBranchID,
			"Message3": ledgerstate.UndefinedBranchID,
			"Message4": ledgerstate.UndefinedBranchID,
			"Message5": ledgerstate.UndefinedBranchID,
			"Message6": ledgerstate.UndefinedBranchID,
			"Message7": testFramework.BranchID("Message1+Message3"),
		})
		checkBranchIDs(t, testFramework, map[string]ledgerstate.BranchID{
			"Message1": testFramework.BranchID("Message1"),
			"Message2": testFramework.BranchID("Message2"),
			"Message3": testFramework.BranchID("Message3"),
			"Message4": testFramework.BranchID("Message4"),
			"Message5": testFramework.BranchID("Message1"),
			"Message6": testFramework.BranchID("Message1+Message3"),
			"Message7": testFramework.BranchID("Message1+Message3"),
		})
	}

	// ISSUE Message8
	{
		testFramework.CreateMessage("Message8", WithStrongParents("Genesis"), WithInputs("Genesis3"), WithOutput("Message8", 500))

		testFramework.RegisterBranchID("Message1+Message5", "Message1", "Message5")
		testFramework.RegisterBranchID("Message8", "Message8")
		testFramework.RegisterBranchID("Message1+Message3+Message5", "Message1", "Message3", "Message5")

		testFramework.IssueMessages("Message8").WaitMessagesBooked()

		checkMarkers(t, testFramework, map[string]*markers.Markers{
			"Message1": markers.NewMarkers(markers.NewMarker(1, 1)),
			"Message2": markers.NewMarkers(markers.NewMarker(2, 1)),
			"Message3": markers.NewMarkers(markers.NewMarker(3, 1)),
			"Message4": markers.NewMarkers(markers.NewMarker(4, 1)),
			"Message5": markers.NewMarkers(markers.NewMarker(1, 2)),
			"Message6": markers.NewMarkers(markers.NewMarker(5, 2)),
			"Message7": markers.NewMarkers(markers.NewMarker(1, 2), markers.NewMarker(3, 1)),
			"Message8": markers.NewMarkers(markers.NewMarker(6, 1)),
		})
		checkMessageMetadataBranchIDs(t, testFramework, map[string]ledgerstate.BranchID{
			"Message1": ledgerstate.UndefinedBranchID,
			"Message2": ledgerstate.UndefinedBranchID,
			"Message3": ledgerstate.UndefinedBranchID,
			"Message4": ledgerstate.UndefinedBranchID,
			"Message5": ledgerstate.UndefinedBranchID,
			"Message6": ledgerstate.UndefinedBranchID,
			"Message7": testFramework.BranchID("Message1+Message3+Message5"),
			"Message8": ledgerstate.UndefinedBranchID,
		})
		checkBranchIDs(t, testFramework, map[string]ledgerstate.BranchID{
			"Message1": testFramework.BranchID("Message1"),
			"Message2": testFramework.BranchID("Message2"),
			"Message3": testFramework.BranchID("Message3"),
			"Message4": testFramework.BranchID("Message4"),
			"Message5": testFramework.BranchID("Message1+Message5"),
			"Message6": testFramework.BranchID("Message1+Message3"),
			"Message7": testFramework.BranchID("Message1+Message3+Message5"),
			"Message8": testFramework.BranchID("Message8"),
		})
	}
}

// Please refer to packages/tangle/images/TestBookerMarkerMappings.html for a diagram of this test.
func TestBookerMarkerMappings(t *testing.T) {
	tangle := NewTestTangle()
	defer tangle.Shutdown()

	testFramework := NewMessageTestFramework(
		tangle,
		WithGenesisOutput("A", 500),
		WithGenesisOutput("B", 500),
		WithGenesisOutput("C", 500),
		WithGenesisOutput("L", 500),
	)

	tangle.Setup()

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
	}

	// ISSUE Message2
	{
		testFramework.CreateMessage("Message2", WithStrongParents("Genesis"), WithInputs("A", "B"), WithOutput("E", 1000))

		testFramework.RegisterBranchID("A", "Message1")
		testFramework.RegisterBranchID("B", "Message2")

		testFramework.IssueMessages("Message2").WaitMessagesBooked()

		checkMarkers(t, testFramework, map[string]*markers.Markers{
			"Message1": markers.NewMarkers(markers.NewMarker(1, 1)),
			"Message2": markers.NewMarkers(markers.NewMarker(2, 1)),
		})
		checkMessageMetadataBranchIDs(t, testFramework, map[string]ledgerstate.BranchID{
			"Message1": ledgerstate.UndefinedBranchID,
			"Message2": ledgerstate.UndefinedBranchID,
		})
		checkBranchIDs(t, testFramework, map[string]ledgerstate.BranchID{
			"Message1": testFramework.BranchID("A"),
			"Message2": testFramework.BranchID("B"),
		})
	}

	// ISSUE Message3
	{
		testFramework.CreateMessage("Message3", WithStrongParents("Genesis"), WithInputs("B"), WithOutput("F", 500))

		testFramework.RegisterBranchID("C", "Message3")

		testFramework.IssueMessages("Message3").WaitMessagesBooked()

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
			"Message1": testFramework.BranchID("A"),
			"Message2": testFramework.BranchID("B"),
			"Message3": testFramework.BranchID("C"),
		})
	}

	// ISSUE Message4
	{
		testFramework.CreateMessage("Message4", WithStrongParents("Genesis"), WithInputs("L"), WithOutput("K", 500))
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
			"Message1": testFramework.BranchID("A"),
			"Message2": testFramework.BranchID("B"),
			"Message3": testFramework.BranchID("C"),
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
			"Message1": testFramework.BranchID("A"),
			"Message2": testFramework.BranchID("B"),
			"Message3": testFramework.BranchID("C"),
			"Message4": ledgerstate.MasterBranchID,
			"Message5": ledgerstate.MasterBranchID,
		})
	}

	// ISSUE Message6
	{
		testFramework.CreateMessage("Message6", WithStrongParents("Message1", "Message2"), WithLikeParents("Message2"))
		testFramework.IssueMessages("Message6").WaitMessagesBooked()

		checkMarkers(t, testFramework, map[string]*markers.Markers{
			"Message1": markers.NewMarkers(markers.NewMarker(1, 1)),
			"Message2": markers.NewMarkers(markers.NewMarker(2, 1)),
			"Message3": markers.NewMarkers(markers.NewMarker(3, 1)),
			"Message4": markers.NewMarkers(markers.NewMarker(4, 1)),
			"Message5": markers.NewMarkers(markers.NewMarker(4, 2)),
			"Message6": markers.NewMarkers(markers.NewMarker(2, 2)),
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
			"Message1": testFramework.BranchID("A"),
			"Message2": testFramework.BranchID("B"),
			"Message3": testFramework.BranchID("C"),
			"Message4": ledgerstate.MasterBranchID,
			"Message5": ledgerstate.MasterBranchID,
			"Message6": testFramework.BranchID("B"),
		})
	}

	// ISSUE Message7
	{
		testFramework.CreateMessage("Message7", WithStrongParents("Message6", "Message5"))
		testFramework.IssueMessages("Message7").WaitMessagesBooked()

		checkMarkers(t, testFramework, map[string]*markers.Markers{
			"Message1": markers.NewMarkers(markers.NewMarker(1, 1)),
			"Message2": markers.NewMarkers(markers.NewMarker(2, 1)),
			"Message3": markers.NewMarkers(markers.NewMarker(3, 1)),
			"Message4": markers.NewMarkers(markers.NewMarker(4, 1)),
			"Message5": markers.NewMarkers(markers.NewMarker(4, 2)),
			"Message6": markers.NewMarkers(markers.NewMarker(2, 2)),
			"Message7": markers.NewMarkers(markers.NewMarker(2, 3)),
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
			"Message1": testFramework.BranchID("A"),
			"Message2": testFramework.BranchID("B"),
			"Message3": testFramework.BranchID("C"),
			"Message4": ledgerstate.MasterBranchID,
			"Message5": ledgerstate.MasterBranchID,
			"Message6": testFramework.BranchID("B"),
			"Message7": testFramework.BranchID("B"),
		})
	}

	// ISSUE Message8
	{
		testFramework.CreateMessage("Message8", WithStrongParents("Message5", "Message7", "Message3"), WithLikeParents("Message1", "Message3"))

		testFramework.RegisterBranchID("A+C", "Message1", "Message3")

		testFramework.IssueMessages("Message8").WaitMessagesBooked()

		checkMarkers(t, testFramework, map[string]*markers.Markers{
			"Message1": markers.NewMarkers(markers.NewMarker(1, 1)),
			"Message2": markers.NewMarkers(markers.NewMarker(2, 1)),
			"Message3": markers.NewMarkers(markers.NewMarker(3, 1)),
			"Message4": markers.NewMarkers(markers.NewMarker(4, 1)),
			"Message5": markers.NewMarkers(markers.NewMarker(4, 2)),
			"Message6": markers.NewMarkers(markers.NewMarker(2, 2)),
			"Message7": markers.NewMarkers(markers.NewMarker(2, 3)),
			"Message8": markers.NewMarkers(markers.NewMarker(5, 4)),
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
			"Message1": testFramework.BranchID("A"),
			"Message2": testFramework.BranchID("B"),
			"Message3": testFramework.BranchID("C"),
			"Message4": ledgerstate.MasterBranchID,
			"Message5": ledgerstate.MasterBranchID,
			"Message6": testFramework.BranchID("B"),
			"Message7": testFramework.BranchID("B"),
			"Message8": testFramework.BranchID("A+C"),
		})
	}

	// ISSUE Message9
	{
		testFramework.CreateMessage("Message9", WithStrongParents("Message1", "Message7", "Message3"), WithLikeParents("Message1", "Message3"), WithInputs("F"), WithOutput("N", 500))
		testFramework.IssueMessages("Message9").WaitMessagesBooked()

		checkMarkers(t, testFramework, map[string]*markers.Markers{
			"Message1": markers.NewMarkers(markers.NewMarker(1, 1)),
			"Message2": markers.NewMarkers(markers.NewMarker(2, 1)),
			"Message3": markers.NewMarkers(markers.NewMarker(3, 1)),
			"Message4": markers.NewMarkers(markers.NewMarker(4, 1)),
			"Message5": markers.NewMarkers(markers.NewMarker(4, 2)),
			"Message6": markers.NewMarkers(markers.NewMarker(2, 2)),
			"Message7": markers.NewMarkers(markers.NewMarker(2, 3)),
			"Message8": markers.NewMarkers(markers.NewMarker(5, 4)),
			"Message9": markers.NewMarkers(markers.NewMarker(2, 3), markers.NewMarker(3, 1)),
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
			"Message9": testFramework.BranchID("A+C"),
		})
		checkIndividuallyMappedMessages(t, testFramework, map[ledgerstate.BranchID][]string{
			testFramework.BranchID("A+C"): {"Message9"},
		})
		checkBranchIDs(t, testFramework, map[string]ledgerstate.BranchID{
			"Message1": testFramework.BranchID("A"),
			"Message2": testFramework.BranchID("B"),
			"Message3": testFramework.BranchID("C"),
			"Message4": ledgerstate.MasterBranchID,
			"Message5": ledgerstate.MasterBranchID,
			"Message6": testFramework.BranchID("B"),
			"Message7": testFramework.BranchID("B"),
			"Message8": testFramework.BranchID("A+C"),
			"Message9": testFramework.BranchID("A+C"),
		})
	}

	// ISSUE Message10
	{
		testFramework.CreateMessage("Message10", WithStrongParents("Message9"), WithLikeParents("Message2"))
		testFramework.IssueMessages("Message10").WaitMessagesBooked()

		checkMarkers(t, testFramework, map[string]*markers.Markers{
			"Message1":  markers.NewMarkers(markers.NewMarker(1, 1)),
			"Message2":  markers.NewMarkers(markers.NewMarker(2, 1)),
			"Message3":  markers.NewMarkers(markers.NewMarker(3, 1)),
			"Message4":  markers.NewMarkers(markers.NewMarker(4, 1)),
			"Message5":  markers.NewMarkers(markers.NewMarker(4, 2)),
			"Message6":  markers.NewMarkers(markers.NewMarker(2, 2)),
			"Message7":  markers.NewMarkers(markers.NewMarker(2, 3)),
			"Message8":  markers.NewMarkers(markers.NewMarker(5, 4)),
			"Message9":  markers.NewMarkers(markers.NewMarker(2, 3), markers.NewMarker(3, 1)),
			"Message10": markers.NewMarkers(markers.NewMarker(2, 4)),
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
			"Message9":  testFramework.BranchID("A+C"),
			"Message10": ledgerstate.UndefinedBranchID,
		})
		checkIndividuallyMappedMessages(t, testFramework, map[ledgerstate.BranchID][]string{
			testFramework.BranchID("A+C"): {"Message9"},
		})
		checkBranchIDs(t, testFramework, map[string]ledgerstate.BranchID{
			"Message1":  testFramework.BranchID("A"),
			"Message2":  testFramework.BranchID("B"),
			"Message3":  testFramework.BranchID("C"),
			"Message4":  ledgerstate.MasterBranchID,
			"Message5":  ledgerstate.MasterBranchID,
			"Message6":  testFramework.BranchID("B"),
			"Message7":  testFramework.BranchID("B"),
			"Message8":  testFramework.BranchID("A+C"),
			"Message9":  testFramework.BranchID("A+C"),
			"Message10": testFramework.BranchID("B"),
		})
	}

	// ISSUE Message11
	{
		testFramework.CreateMessage("Message11", WithStrongParents("Message8", "Message9"))
		testFramework.CreateMessage("Message11.5", WithStrongParents("Message9"))
		testFramework.IssueMessages("Message11", "Message11.5").WaitMessagesBooked()

		checkMarkers(t, testFramework, map[string]*markers.Markers{
			"Message1":    markers.NewMarkers(markers.NewMarker(1, 1)),
			"Message2":    markers.NewMarkers(markers.NewMarker(2, 1)),
			"Message3":    markers.NewMarkers(markers.NewMarker(3, 1)),
			"Message4":    markers.NewMarkers(markers.NewMarker(4, 1)),
			"Message5":    markers.NewMarkers(markers.NewMarker(4, 2)),
			"Message6":    markers.NewMarkers(markers.NewMarker(2, 2)),
			"Message7":    markers.NewMarkers(markers.NewMarker(2, 3)),
			"Message8":    markers.NewMarkers(markers.NewMarker(5, 4)),
			"Message9":    markers.NewMarkers(markers.NewMarker(2, 3), markers.NewMarker(3, 1)),
			"Message10":   markers.NewMarkers(markers.NewMarker(2, 4)),
			"Message11":   markers.NewMarkers(markers.NewMarker(5, 5)),
			"Message11.5": markers.NewMarkers(markers.NewMarker(2, 3), markers.NewMarker(3, 1)),
		})
		checkMessageMetadataBranchIDs(t, testFramework, map[string]ledgerstate.BranchID{
			"Message1":    ledgerstate.UndefinedBranchID,
			"Message2":    ledgerstate.UndefinedBranchID,
			"Message3":    ledgerstate.UndefinedBranchID,
			"Message4":    ledgerstate.UndefinedBranchID,
			"Message5":    ledgerstate.UndefinedBranchID,
			"Message6":    ledgerstate.UndefinedBranchID,
			"Message7":    ledgerstate.UndefinedBranchID,
			"Message8":    ledgerstate.UndefinedBranchID,
			"Message9":    testFramework.BranchID("A+C"),
			"Message10":   ledgerstate.UndefinedBranchID,
			"Message11":   ledgerstate.UndefinedBranchID,
			"Message11.5": testFramework.BranchID("A+C"),
		})
		checkIndividuallyMappedMessages(t, testFramework, map[ledgerstate.BranchID][]string{
			testFramework.BranchID("A+C"): {"Message9", "Message11.5"},
		})
		checkBranchIDs(t, testFramework, map[string]ledgerstate.BranchID{
			"Message1":    testFramework.BranchID("A"),
			"Message2":    testFramework.BranchID("B"),
			"Message3":    testFramework.BranchID("C"),
			"Message4":    ledgerstate.MasterBranchID,
			"Message5":    ledgerstate.MasterBranchID,
			"Message6":    testFramework.BranchID("B"),
			"Message7":    testFramework.BranchID("B"),
			"Message8":    testFramework.BranchID("A+C"),
			"Message9":    testFramework.BranchID("A+C"),
			"Message10":   testFramework.BranchID("B"),
			"Message11":   testFramework.BranchID("A+C"),
			"Message11.5": testFramework.BranchID("A+C"),
		})
	}

	// ISSUE Message12
	{
		testFramework.CreateMessage("Message12", WithStrongParents("Message1"), WithInputs("C"), WithOutput("H", 500))

		testFramework.RegisterBranchID("D", "Message5")
		testFramework.RegisterBranchID("E", "Message12")
		testFramework.RegisterBranchID("A+E", "Message1", "Message12")
		testFramework.RegisterBranchID("B+D", "Message2", "Message5")
		testFramework.RegisterBranchID("A+C+D", "Message1", "Message3", "Message5")

		testFramework.IssueMessages("Message12").WaitMessagesBooked()

		checkMarkers(t, testFramework, map[string]*markers.Markers{
			"Message1":    markers.NewMarkers(markers.NewMarker(1, 1)),
			"Message2":    markers.NewMarkers(markers.NewMarker(2, 1)),
			"Message3":    markers.NewMarkers(markers.NewMarker(3, 1)),
			"Message4":    markers.NewMarkers(markers.NewMarker(4, 1)),
			"Message5":    markers.NewMarkers(markers.NewMarker(4, 2)),
			"Message6":    markers.NewMarkers(markers.NewMarker(2, 2)),
			"Message7":    markers.NewMarkers(markers.NewMarker(2, 3)),
			"Message8":    markers.NewMarkers(markers.NewMarker(5, 4)),
			"Message9":    markers.NewMarkers(markers.NewMarker(2, 3), markers.NewMarker(3, 1)),
			"Message10":   markers.NewMarkers(markers.NewMarker(2, 4)),
			"Message11":   markers.NewMarkers(markers.NewMarker(5, 5)),
			"Message11.5": markers.NewMarkers(markers.NewMarker(2, 3), markers.NewMarker(3, 1)),
			"Message12":   markers.NewMarkers(markers.NewMarker(6, 2)),
		})
		checkMessageMetadataBranchIDs(t, testFramework, map[string]ledgerstate.BranchID{
			"Message1":    ledgerstate.UndefinedBranchID,
			"Message2":    ledgerstate.UndefinedBranchID,
			"Message3":    ledgerstate.UndefinedBranchID,
			"Message4":    ledgerstate.UndefinedBranchID,
			"Message5":    ledgerstate.UndefinedBranchID,
			"Message6":    ledgerstate.UndefinedBranchID,
			"Message7":    ledgerstate.UndefinedBranchID,
			"Message8":    ledgerstate.UndefinedBranchID,
			"Message9":    testFramework.BranchID("A+C+D"),
			"Message10":   ledgerstate.UndefinedBranchID,
			"Message11":   ledgerstate.UndefinedBranchID,
			"Message11.5": testFramework.BranchID("A+C+D"),
			"Message12":   ledgerstate.UndefinedBranchID,
		})
		checkIndividuallyMappedMessages(t, testFramework, map[ledgerstate.BranchID][]string{
			testFramework.BranchID("A+C+D"): {"Message9", "Message11.5"},
		})
		checkBranchIDs(t, testFramework, map[string]ledgerstate.BranchID{
			"Message1":    testFramework.BranchID("A"),
			"Message2":    testFramework.BranchID("B"),
			"Message3":    testFramework.BranchID("C"),
			"Message4":    ledgerstate.MasterBranchID,
			"Message5":    testFramework.BranchID("D"),
			"Message6":    testFramework.BranchID("B"),
			"Message7":    testFramework.BranchID("B+D"),
			"Message8":    testFramework.BranchID("A+C+D"),
			"Message9":    testFramework.BranchID("A+C+D"),
			"Message10":   testFramework.BranchID("B+D"),
			"Message11":   testFramework.BranchID("A+C+D"),
			"Message11.5": testFramework.BranchID("A+C+D"),
			"Message12":   testFramework.BranchID("A+E"),
		})
	}

	// ISSUE Message13
	{
		testFramework.CreateMessage("Message13", WithStrongParents("Message9"), WithLikeParents("Message2", "Message12"))

		testFramework.RegisterBranchID("B+E", "Message2", "Message12")

		testFramework.IssueMessages("Message13").WaitMessagesBooked()

		checkMarkers(t, testFramework, map[string]*markers.Markers{
			"Message1":    markers.NewMarkers(markers.NewMarker(1, 1)),
			"Message2":    markers.NewMarkers(markers.NewMarker(2, 1)),
			"Message3":    markers.NewMarkers(markers.NewMarker(3, 1)),
			"Message4":    markers.NewMarkers(markers.NewMarker(4, 1)),
			"Message5":    markers.NewMarkers(markers.NewMarker(4, 2)),
			"Message6":    markers.NewMarkers(markers.NewMarker(2, 2)),
			"Message7":    markers.NewMarkers(markers.NewMarker(2, 3)),
			"Message8":    markers.NewMarkers(markers.NewMarker(5, 4)),
			"Message9":    markers.NewMarkers(markers.NewMarker(2, 3), markers.NewMarker(3, 1)),
			"Message10":   markers.NewMarkers(markers.NewMarker(2, 4)),
			"Message11":   markers.NewMarkers(markers.NewMarker(5, 5)),
			"Message11.5": markers.NewMarkers(markers.NewMarker(2, 3), markers.NewMarker(3, 1)),
			"Message12":   markers.NewMarkers(markers.NewMarker(6, 2)),
			"Message13":   markers.NewMarkers(markers.NewMarker(7, 4)),
		})
		checkMessageMetadataBranchIDs(t, testFramework, map[string]ledgerstate.BranchID{
			"Message1":    ledgerstate.UndefinedBranchID,
			"Message2":    ledgerstate.UndefinedBranchID,
			"Message3":    ledgerstate.UndefinedBranchID,
			"Message4":    ledgerstate.UndefinedBranchID,
			"Message5":    ledgerstate.UndefinedBranchID,
			"Message6":    ledgerstate.UndefinedBranchID,
			"Message7":    ledgerstate.UndefinedBranchID,
			"Message8":    ledgerstate.UndefinedBranchID,
			"Message9":    testFramework.BranchID("A+C+D"),
			"Message10":   ledgerstate.UndefinedBranchID,
			"Message11":   ledgerstate.UndefinedBranchID,
			"Message11.5": testFramework.BranchID("A+C+D"),
			"Message12":   ledgerstate.UndefinedBranchID,
			"Message13":   ledgerstate.UndefinedBranchID,
		})
		checkIndividuallyMappedMessages(t, testFramework, map[ledgerstate.BranchID][]string{
			testFramework.BranchID("A+C+D"): {"Message9", "Message11.5"},
		})
		checkBranchIDs(t, testFramework, map[string]ledgerstate.BranchID{
			"Message1":    testFramework.BranchID("A"),
			"Message2":    testFramework.BranchID("B"),
			"Message3":    testFramework.BranchID("C"),
			"Message4":    ledgerstate.MasterBranchID,
			"Message5":    testFramework.BranchID("D"),
			"Message6":    testFramework.BranchID("B"),
			"Message7":    testFramework.BranchID("B+D"),
			"Message8":    testFramework.BranchID("A+C+D"),
			"Message9":    testFramework.BranchID("A+C+D"),
			"Message10":   testFramework.BranchID("B+D"),
			"Message11":   testFramework.BranchID("A+C+D"),
			"Message11.5": testFramework.BranchID("A+C+D"),
			"Message12":   testFramework.BranchID("A+E"),
			"Message13":   testFramework.BranchID("B+E"),
		})
	}

	// ISSUE Message14
	{
		testFramework.CreateMessage("Message14", WithStrongParents("Message10"), WithLikeParents("Message12"))
		testFramework.IssueMessages("Message14").WaitMessagesBooked()

		checkMarkers(t, testFramework, map[string]*markers.Markers{
			"Message1":    markers.NewMarkers(markers.NewMarker(1, 1)),
			"Message2":    markers.NewMarkers(markers.NewMarker(2, 1)),
			"Message3":    markers.NewMarkers(markers.NewMarker(3, 1)),
			"Message4":    markers.NewMarkers(markers.NewMarker(4, 1)),
			"Message5":    markers.NewMarkers(markers.NewMarker(4, 2)),
			"Message6":    markers.NewMarkers(markers.NewMarker(2, 2)),
			"Message7":    markers.NewMarkers(markers.NewMarker(2, 3)),
			"Message8":    markers.NewMarkers(markers.NewMarker(5, 4)),
			"Message9":    markers.NewMarkers(markers.NewMarker(2, 3), markers.NewMarker(3, 1)),
			"Message10":   markers.NewMarkers(markers.NewMarker(2, 4)),
			"Message11":   markers.NewMarkers(markers.NewMarker(5, 5)),
			"Message11.5": markers.NewMarkers(markers.NewMarker(2, 3), markers.NewMarker(3, 1)),
			"Message12":   markers.NewMarkers(markers.NewMarker(6, 2)),
			"Message13":   markers.NewMarkers(markers.NewMarker(7, 4)),
			"Message14":   markers.NewMarkers(markers.NewMarker(2, 4), markers.NewMarker(6, 2)),
		})
		checkMessageMetadataBranchIDs(t, testFramework, map[string]ledgerstate.BranchID{
			"Message1":    ledgerstate.UndefinedBranchID,
			"Message2":    ledgerstate.UndefinedBranchID,
			"Message3":    ledgerstate.UndefinedBranchID,
			"Message4":    ledgerstate.UndefinedBranchID,
			"Message5":    ledgerstate.UndefinedBranchID,
			"Message6":    ledgerstate.UndefinedBranchID,
			"Message7":    ledgerstate.UndefinedBranchID,
			"Message8":    ledgerstate.UndefinedBranchID,
			"Message9":    testFramework.BranchID("A+C+D"),
			"Message10":   ledgerstate.UndefinedBranchID,
			"Message11":   ledgerstate.UndefinedBranchID,
			"Message11.5": testFramework.BranchID("A+C+D"),
			"Message12":   ledgerstate.UndefinedBranchID,
			"Message13":   ledgerstate.UndefinedBranchID,
			"Message14":   testFramework.BranchID("B+E"),
		})
		checkIndividuallyMappedMessages(t, testFramework, map[ledgerstate.BranchID][]string{
			testFramework.BranchID("A+C+D"): {"Message9", "Message11.5"},
			testFramework.BranchID("B+E"):   {"Message14"},
		})
		checkBranchIDs(t, testFramework, map[string]ledgerstate.BranchID{
			"Message1":    testFramework.BranchID("A"),
			"Message2":    testFramework.BranchID("B"),
			"Message3":    testFramework.BranchID("C"),
			"Message4":    ledgerstate.MasterBranchID,
			"Message5":    testFramework.BranchID("D"),
			"Message6":    testFramework.BranchID("B"),
			"Message7":    testFramework.BranchID("B+D"),
			"Message8":    testFramework.BranchID("A+C+D"),
			"Message9":    testFramework.BranchID("A+C+D"),
			"Message10":   testFramework.BranchID("B+D"),
			"Message11":   testFramework.BranchID("A+C+D"),
			"Message11.5": testFramework.BranchID("A+C+D"),
			"Message12":   testFramework.BranchID("A+E"),
			"Message13":   testFramework.BranchID("B+E"),
			"Message14":   testFramework.BranchID("B+E"),
		})
	}

	// ISSUE Message15
	{
		testFramework.CreateMessage("Message15", WithStrongParents("Message5", "Message14"), WithLikeParents("Message5"), WithInputs("D"), WithOutput("I", 500))

		testFramework.RegisterBranchID("B+D", "Message2", "Message5")

		testFramework.IssueMessages("Message15").WaitMessagesBooked()

		checkMarkers(t, testFramework, map[string]*markers.Markers{
			"Message1":    markers.NewMarkers(markers.NewMarker(1, 1)),
			"Message2":    markers.NewMarkers(markers.NewMarker(2, 1)),
			"Message3":    markers.NewMarkers(markers.NewMarker(3, 1)),
			"Message4":    markers.NewMarkers(markers.NewMarker(4, 1)),
			"Message5":    markers.NewMarkers(markers.NewMarker(4, 2)),
			"Message6":    markers.NewMarkers(markers.NewMarker(2, 2)),
			"Message7":    markers.NewMarkers(markers.NewMarker(2, 3)),
			"Message8":    markers.NewMarkers(markers.NewMarker(5, 4)),
			"Message9":    markers.NewMarkers(markers.NewMarker(2, 3), markers.NewMarker(3, 1)),
			"Message10":   markers.NewMarkers(markers.NewMarker(2, 4)),
			"Message11":   markers.NewMarkers(markers.NewMarker(5, 5)),
			"Message11.5": markers.NewMarkers(markers.NewMarker(2, 3), markers.NewMarker(3, 1)),
			"Message12":   markers.NewMarkers(markers.NewMarker(6, 2)),
			"Message13":   markers.NewMarkers(markers.NewMarker(7, 4)),
			"Message14":   markers.NewMarkers(markers.NewMarker(2, 4), markers.NewMarker(6, 2)),
			"Message15":   markers.NewMarkers(markers.NewMarker(2, 5)),
		})
		checkMessageMetadataBranchIDs(t, testFramework, map[string]ledgerstate.BranchID{
			"Message1":    ledgerstate.UndefinedBranchID,
			"Message2":    ledgerstate.UndefinedBranchID,
			"Message3":    ledgerstate.UndefinedBranchID,
			"Message4":    ledgerstate.UndefinedBranchID,
			"Message5":    ledgerstate.UndefinedBranchID,
			"Message6":    ledgerstate.UndefinedBranchID,
			"Message7":    ledgerstate.UndefinedBranchID,
			"Message8":    ledgerstate.UndefinedBranchID,
			"Message9":    testFramework.BranchID("A+C+D"),
			"Message10":   ledgerstate.UndefinedBranchID,
			"Message11":   ledgerstate.UndefinedBranchID,
			"Message11.5": testFramework.BranchID("A+C+D"),
			"Message12":   ledgerstate.UndefinedBranchID,
			"Message13":   ledgerstate.UndefinedBranchID,
			"Message14":   testFramework.BranchID("B+E"),
			"Message15":   ledgerstate.UndefinedBranchID,
		})
		checkIndividuallyMappedMessages(t, testFramework, map[ledgerstate.BranchID][]string{
			testFramework.BranchID("A+C+D"): {"Message9", "Message11.5"},
			testFramework.BranchID("B+E"):   {"Message14"},
		})
		checkBranchIDs(t, testFramework, map[string]ledgerstate.BranchID{
			"Message1":    testFramework.BranchID("A"),
			"Message2":    testFramework.BranchID("B"),
			"Message3":    testFramework.BranchID("C"),
			"Message4":    ledgerstate.MasterBranchID,
			"Message5":    testFramework.BranchID("D"),
			"Message6":    testFramework.BranchID("B"),
			"Message7":    testFramework.BranchID("B+D"),
			"Message8":    testFramework.BranchID("A+C+D"),
			"Message9":    testFramework.BranchID("A+C+D"),
			"Message10":   testFramework.BranchID("B+D"),
			"Message11":   testFramework.BranchID("A+C+D"),
			"Message11.5": testFramework.BranchID("A+C+D"),
			"Message12":   testFramework.BranchID("A+E"),
			"Message13":   testFramework.BranchID("B+E"),
			"Message14":   testFramework.BranchID("B+E"),
			"Message15":   testFramework.BranchID("B+D"),
		})
	}

	// ISSUE Message16
	{
		testFramework.CreateMessage("Message16", WithStrongParents("Genesis"), WithInputs("L"), WithOutput("J", 500))

		testFramework.RegisterBranchID("F", "Message4")
		testFramework.RegisterBranchID("G", "Message16")
		testFramework.RegisterBranchID("D+F", "Message5", "Message4")
		testFramework.RegisterBranchID("B+D+F", "Message2", "Message5", "Message4")
		testFramework.RegisterBranchID("A+C+D+F", "Message1", "Message3", "Message5", "Message4")
		testFramework.RegisterBranchID("B+E+F", "Message2", "Message12", "Message4")

		testFramework.IssueMessages("Message16").WaitMessagesBooked()

		checkMarkers(t, testFramework, map[string]*markers.Markers{
			"Message1":    markers.NewMarkers(markers.NewMarker(1, 1)),
			"Message2":    markers.NewMarkers(markers.NewMarker(2, 1)),
			"Message3":    markers.NewMarkers(markers.NewMarker(3, 1)),
			"Message4":    markers.NewMarkers(markers.NewMarker(4, 1)),
			"Message5":    markers.NewMarkers(markers.NewMarker(4, 2)),
			"Message6":    markers.NewMarkers(markers.NewMarker(2, 2)),
			"Message7":    markers.NewMarkers(markers.NewMarker(2, 3)),
			"Message8":    markers.NewMarkers(markers.NewMarker(5, 4)),
			"Message9":    markers.NewMarkers(markers.NewMarker(2, 3), markers.NewMarker(3, 1)),
			"Message10":   markers.NewMarkers(markers.NewMarker(2, 4)),
			"Message11":   markers.NewMarkers(markers.NewMarker(5, 5)),
			"Message11.5": markers.NewMarkers(markers.NewMarker(2, 3), markers.NewMarker(3, 1)),
			"Message12":   markers.NewMarkers(markers.NewMarker(6, 2)),
			"Message13":   markers.NewMarkers(markers.NewMarker(7, 4)),
			"Message14":   markers.NewMarkers(markers.NewMarker(2, 4), markers.NewMarker(6, 2)),
			"Message15":   markers.NewMarkers(markers.NewMarker(2, 5)),
			"Message16":   markers.NewMarkers(markers.NewMarker(8, 1)),
		})
		checkMessageMetadataBranchIDs(t, testFramework, map[string]ledgerstate.BranchID{
			"Message1":    ledgerstate.UndefinedBranchID,
			"Message2":    ledgerstate.UndefinedBranchID,
			"Message3":    ledgerstate.UndefinedBranchID,
			"Message4":    ledgerstate.UndefinedBranchID,
			"Message5":    ledgerstate.UndefinedBranchID,
			"Message6":    ledgerstate.UndefinedBranchID,
			"Message7":    ledgerstate.UndefinedBranchID,
			"Message8":    ledgerstate.UndefinedBranchID,
			"Message9":    testFramework.BranchID("A+C+D+F"),
			"Message10":   ledgerstate.UndefinedBranchID,
			"Message11":   ledgerstate.UndefinedBranchID,
			"Message11.5": testFramework.BranchID("A+C+D+F"),
			"Message12":   ledgerstate.UndefinedBranchID,
			"Message13":   ledgerstate.UndefinedBranchID,
			"Message14":   testFramework.BranchID("B+E+F"),
			"Message15":   ledgerstate.UndefinedBranchID,
			"Message16":   ledgerstate.UndefinedBranchID,
		})
		checkIndividuallyMappedMessages(t, testFramework, map[ledgerstate.BranchID][]string{
			testFramework.BranchID("A+C+D+F"): {"Message9", "Message11.5"},
			testFramework.BranchID("B+E+F"):   {"Message14"},
		})
		checkBranchIDs(t, testFramework, map[string]ledgerstate.BranchID{
			"Message1":    testFramework.BranchID("A"),
			"Message2":    testFramework.BranchID("B"),
			"Message3":    testFramework.BranchID("C"),
			"Message4":    testFramework.BranchID("F"),
			"Message5":    testFramework.BranchID("D+F"),
			"Message6":    testFramework.BranchID("B"),
			"Message7":    testFramework.BranchID("B+D+F"),
			"Message8":    testFramework.BranchID("A+C+D+F"),
			"Message9":    testFramework.BranchID("A+C+D+F"),
			"Message10":   testFramework.BranchID("B+D+F"),
			"Message11":   testFramework.BranchID("A+C+D+F"),
			"Message11.5": testFramework.BranchID("A+C+D+F"),
			"Message12":   testFramework.BranchID("A+E"),
			"Message13":   testFramework.BranchID("B+E+F"),
			"Message14":   testFramework.BranchID("B+E+F"),
			"Message15":   testFramework.BranchID("B+D+F"),
			"Message16":   testFramework.BranchID("G"),
		})
	}

	// ISSUE Message17
	{
		testFramework.CreateMessage("Message17", WithStrongParents("Message14"))
		testFramework.IssueMessages("Message17").WaitMessagesBooked()

		checkMarkers(t, testFramework, map[string]*markers.Markers{
			"Message1":    markers.NewMarkers(markers.NewMarker(1, 1)),
			"Message2":    markers.NewMarkers(markers.NewMarker(2, 1)),
			"Message3":    markers.NewMarkers(markers.NewMarker(3, 1)),
			"Message4":    markers.NewMarkers(markers.NewMarker(4, 1)),
			"Message5":    markers.NewMarkers(markers.NewMarker(4, 2)),
			"Message6":    markers.NewMarkers(markers.NewMarker(2, 2)),
			"Message7":    markers.NewMarkers(markers.NewMarker(2, 3)),
			"Message8":    markers.NewMarkers(markers.NewMarker(5, 4)),
			"Message9":    markers.NewMarkers(markers.NewMarker(2, 3), markers.NewMarker(3, 1)),
			"Message10":   markers.NewMarkers(markers.NewMarker(2, 4)),
			"Message11":   markers.NewMarkers(markers.NewMarker(5, 5)),
			"Message11.5": markers.NewMarkers(markers.NewMarker(2, 3), markers.NewMarker(3, 1)),
			"Message12":   markers.NewMarkers(markers.NewMarker(6, 2)),
			"Message13":   markers.NewMarkers(markers.NewMarker(7, 4)),
			"Message14":   markers.NewMarkers(markers.NewMarker(2, 4), markers.NewMarker(6, 2)),
			"Message15":   markers.NewMarkers(markers.NewMarker(2, 5)),
			"Message16":   markers.NewMarkers(markers.NewMarker(8, 1)),
			"Message17":   markers.NewMarkers(markers.NewMarker(2, 4), markers.NewMarker(6, 2)),
		})
		checkMessageMetadataBranchIDs(t, testFramework, map[string]ledgerstate.BranchID{
			"Message1":    ledgerstate.UndefinedBranchID,
			"Message2":    ledgerstate.UndefinedBranchID,
			"Message3":    ledgerstate.UndefinedBranchID,
			"Message4":    ledgerstate.UndefinedBranchID,
			"Message5":    ledgerstate.UndefinedBranchID,
			"Message6":    ledgerstate.UndefinedBranchID,
			"Message7":    ledgerstate.UndefinedBranchID,
			"Message8":    ledgerstate.UndefinedBranchID,
			"Message9":    testFramework.BranchID("A+C+D+F"),
			"Message10":   ledgerstate.UndefinedBranchID,
			"Message11":   ledgerstate.UndefinedBranchID,
			"Message11.5": testFramework.BranchID("A+C+D+F"),
			"Message12":   ledgerstate.UndefinedBranchID,
			"Message13":   ledgerstate.UndefinedBranchID,
			"Message14":   testFramework.BranchID("B+E+F"),
			"Message15":   ledgerstate.UndefinedBranchID,
			"Message16":   ledgerstate.UndefinedBranchID,
			"Message17":   testFramework.BranchID("B+E+F"),
		})
		checkIndividuallyMappedMessages(t, testFramework, map[ledgerstate.BranchID][]string{
			testFramework.BranchID("A+C+D+F"): {"Message9", "Message11.5"},
			testFramework.BranchID("B+E+F"):   {"Message14", "Message17"},
		})
		checkBranchIDs(t, testFramework, map[string]ledgerstate.BranchID{
			"Message1":    testFramework.BranchID("A"),
			"Message2":    testFramework.BranchID("B"),
			"Message3":    testFramework.BranchID("C"),
			"Message4":    testFramework.BranchID("F"),
			"Message5":    testFramework.BranchID("D+F"),
			"Message6":    testFramework.BranchID("B"),
			"Message7":    testFramework.BranchID("B+D+F"),
			"Message8":    testFramework.BranchID("A+C+D+F"),
			"Message9":    testFramework.BranchID("A+C+D+F"),
			"Message10":   testFramework.BranchID("B+D+F"),
			"Message11":   testFramework.BranchID("A+C+D+F"),
			"Message11.5": testFramework.BranchID("A+C+D+F"),
			"Message12":   testFramework.BranchID("A+E"),
			"Message13":   testFramework.BranchID("B+E+F"),
			"Message14":   testFramework.BranchID("B+E+F"),
			"Message15":   testFramework.BranchID("B+D+F"),
			"Message16":   testFramework.BranchID("G"),
			"Message17":   testFramework.BranchID("B+E+F"),
		})
	}

	// ISSUE Message18
	{
		testFramework.CreateMessage("Message18", WithStrongParents("Message17", "Message13"))
		testFramework.IssueMessages("Message18").WaitMessagesBooked()

		checkMarkers(t, testFramework, map[string]*markers.Markers{
			"Message1":    markers.NewMarkers(markers.NewMarker(1, 1)),
			"Message2":    markers.NewMarkers(markers.NewMarker(2, 1)),
			"Message3":    markers.NewMarkers(markers.NewMarker(3, 1)),
			"Message4":    markers.NewMarkers(markers.NewMarker(4, 1)),
			"Message5":    markers.NewMarkers(markers.NewMarker(4, 2)),
			"Message6":    markers.NewMarkers(markers.NewMarker(2, 2)),
			"Message7":    markers.NewMarkers(markers.NewMarker(2, 3)),
			"Message8":    markers.NewMarkers(markers.NewMarker(5, 4)),
			"Message9":    markers.NewMarkers(markers.NewMarker(2, 3), markers.NewMarker(3, 1)),
			"Message10":   markers.NewMarkers(markers.NewMarker(2, 4)),
			"Message11":   markers.NewMarkers(markers.NewMarker(5, 5)),
			"Message11.5": markers.NewMarkers(markers.NewMarker(2, 3), markers.NewMarker(3, 1)),
			"Message12":   markers.NewMarkers(markers.NewMarker(6, 2)),
			"Message13":   markers.NewMarkers(markers.NewMarker(7, 4)),
			"Message14":   markers.NewMarkers(markers.NewMarker(2, 4), markers.NewMarker(6, 2)),
			"Message15":   markers.NewMarkers(markers.NewMarker(2, 5)),
			"Message16":   markers.NewMarkers(markers.NewMarker(8, 1)),
			"Message17":   markers.NewMarkers(markers.NewMarker(2, 4), markers.NewMarker(6, 2)),
			"Message18":   markers.NewMarkers(markers.NewMarker(7, 5)),
		})
		checkMessageMetadataBranchIDs(t, testFramework, map[string]ledgerstate.BranchID{
			"Message1":    ledgerstate.UndefinedBranchID,
			"Message2":    ledgerstate.UndefinedBranchID,
			"Message3":    ledgerstate.UndefinedBranchID,
			"Message4":    ledgerstate.UndefinedBranchID,
			"Message5":    ledgerstate.UndefinedBranchID,
			"Message6":    ledgerstate.UndefinedBranchID,
			"Message7":    ledgerstate.UndefinedBranchID,
			"Message8":    ledgerstate.UndefinedBranchID,
			"Message9":    testFramework.BranchID("A+C+D+F"),
			"Message10":   ledgerstate.UndefinedBranchID,
			"Message11":   ledgerstate.UndefinedBranchID,
			"Message11.5": testFramework.BranchID("A+C+D+F"),
			"Message12":   ledgerstate.UndefinedBranchID,
			"Message13":   ledgerstate.UndefinedBranchID,
			"Message14":   testFramework.BranchID("B+E+F"),
			"Message15":   ledgerstate.UndefinedBranchID,
			"Message16":   ledgerstate.UndefinedBranchID,
			"Message17":   testFramework.BranchID("B+E+F"),
			"Message18":   ledgerstate.UndefinedBranchID,
		})
		checkIndividuallyMappedMessages(t, testFramework, map[ledgerstate.BranchID][]string{
			testFramework.BranchID("A+C+D+F"): {"Message9", "Message11.5"},
			testFramework.BranchID("B+E+F"):   {"Message14", "Message17"},
		})
		checkBranchIDs(t, testFramework, map[string]ledgerstate.BranchID{
			"Message1":    testFramework.BranchID("A"),
			"Message2":    testFramework.BranchID("B"),
			"Message3":    testFramework.BranchID("C"),
			"Message4":    testFramework.BranchID("F"),
			"Message5":    testFramework.BranchID("D+F"),
			"Message6":    testFramework.BranchID("B"),
			"Message7":    testFramework.BranchID("B+D+F"),
			"Message8":    testFramework.BranchID("A+C+D+F"),
			"Message9":    testFramework.BranchID("A+C+D+F"),
			"Message10":   testFramework.BranchID("B+D+F"),
			"Message11":   testFramework.BranchID("A+C+D+F"),
			"Message11.5": testFramework.BranchID("A+C+D+F"),
			"Message12":   testFramework.BranchID("A+E"),
			"Message13":   testFramework.BranchID("B+E+F"),
			"Message14":   testFramework.BranchID("B+E+F"),
			"Message15":   testFramework.BranchID("B+D+F"),
			"Message16":   testFramework.BranchID("G"),
			"Message17":   testFramework.BranchID("B+E+F"),
			"Message18":   testFramework.BranchID("B+E+F"),
		})
	}

	// ISSUE Message19
	{
		testFramework.CreateMessage("Message19", WithStrongParents("Message3", "Message7"), WithLikeParents("Message3"), WithInputs("F"), WithOutput("M", 500))

		testFramework.RegisterBranchID("H", "Message9")
		testFramework.RegisterBranchID("I", "Message19")
		testFramework.RegisterBranchID("D+F+I", "Message5", "Message4", "Message19")
		testFramework.RegisterBranchID("A+D+F+H", "Message1", "Message5", "Message4", "Message9")

		testFramework.IssueMessages("Message19").WaitMessagesBooked()

		checkMarkers(t, testFramework, map[string]*markers.Markers{
			"Message1":    markers.NewMarkers(markers.NewMarker(1, 1)),
			"Message2":    markers.NewMarkers(markers.NewMarker(2, 1)),
			"Message3":    markers.NewMarkers(markers.NewMarker(3, 1)),
			"Message4":    markers.NewMarkers(markers.NewMarker(4, 1)),
			"Message5":    markers.NewMarkers(markers.NewMarker(4, 2)),
			"Message6":    markers.NewMarkers(markers.NewMarker(2, 2)),
			"Message7":    markers.NewMarkers(markers.NewMarker(2, 3)),
			"Message8":    markers.NewMarkers(markers.NewMarker(5, 4)),
			"Message9":    markers.NewMarkers(markers.NewMarker(2, 3), markers.NewMarker(3, 1)),
			"Message10":   markers.NewMarkers(markers.NewMarker(2, 4)),
			"Message11":   markers.NewMarkers(markers.NewMarker(5, 5)),
			"Message11.5": markers.NewMarkers(markers.NewMarker(2, 3), markers.NewMarker(3, 1)),
			"Message12":   markers.NewMarkers(markers.NewMarker(6, 2)),
			"Message13":   markers.NewMarkers(markers.NewMarker(7, 4)),
			"Message14":   markers.NewMarkers(markers.NewMarker(2, 4), markers.NewMarker(6, 2)),
			"Message15":   markers.NewMarkers(markers.NewMarker(2, 5)),
			"Message16":   markers.NewMarkers(markers.NewMarker(8, 1)),
			"Message17":   markers.NewMarkers(markers.NewMarker(2, 4), markers.NewMarker(6, 2)),
			"Message18":   markers.NewMarkers(markers.NewMarker(7, 5)),
			"Message19":   markers.NewMarkers(markers.NewMarker(9, 4)),
		})
		checkMessageMetadataBranchIDs(t, testFramework, map[string]ledgerstate.BranchID{
			"Message1":    ledgerstate.UndefinedBranchID,
			"Message2":    ledgerstate.UndefinedBranchID,
			"Message3":    ledgerstate.UndefinedBranchID,
			"Message4":    ledgerstate.UndefinedBranchID,
			"Message5":    ledgerstate.UndefinedBranchID,
			"Message6":    ledgerstate.UndefinedBranchID,
			"Message7":    ledgerstate.UndefinedBranchID,
			"Message8":    ledgerstate.UndefinedBranchID,
			"Message9":    testFramework.BranchID("A+D+F+H"),
			"Message10":   ledgerstate.UndefinedBranchID,
			"Message11":   ledgerstate.UndefinedBranchID,
			"Message11.5": testFramework.BranchID("A+D+F+H"),
			"Message12":   ledgerstate.UndefinedBranchID,
			"Message13":   ledgerstate.UndefinedBranchID,
			"Message14":   testFramework.BranchID("B+E+F"),
			"Message15":   ledgerstate.UndefinedBranchID,
			"Message16":   ledgerstate.UndefinedBranchID,
			"Message17":   testFramework.BranchID("B+E+F"),
			"Message18":   ledgerstate.UndefinedBranchID,
			"Message19":   ledgerstate.UndefinedBranchID,
		})
		checkIndividuallyMappedMessages(t, testFramework, map[ledgerstate.BranchID][]string{
			testFramework.BranchID("A+D+F+H"): {"Message9", "Message11.5"},
			testFramework.BranchID("B+E+F"):   {"Message14", "Message17"},
		})
		checkBranchIDs(t, testFramework, map[string]ledgerstate.BranchID{
			"Message1":    testFramework.BranchID("A"),
			"Message2":    testFramework.BranchID("B"),
			"Message3":    testFramework.BranchID("C"),
			"Message4":    testFramework.BranchID("F"),
			"Message5":    testFramework.BranchID("D+F"),
			"Message6":    testFramework.BranchID("B"),
			"Message7":    testFramework.BranchID("B+D+F"),
			"Message8":    testFramework.BranchID("A+C+D+F"),
			"Message9":    testFramework.BranchID("A+D+F+H"),
			"Message10":   testFramework.BranchID("B+D+F"),
			"Message11":   testFramework.BranchID("A+D+F+H"),
			"Message11.5": testFramework.BranchID("A+D+F+H"),
			"Message12":   testFramework.BranchID("A+E"),
			"Message13":   testFramework.BranchID("B+E+F"),
			"Message14":   testFramework.BranchID("B+E+F"),
			"Message15":   testFramework.BranchID("B+D+F"),
			"Message16":   testFramework.BranchID("G"),
			"Message17":   testFramework.BranchID("B+E+F"),
			"Message18":   testFramework.BranchID("B+E+F"),
			"Message19":   testFramework.BranchID("D+F+I"),
		})
	}

	// ISSUE Message20 - 23
	{
		testFramework.CreateMessage("Message20", WithStrongParents("Message4"))
		testFramework.CreateMessage("Message21", WithStrongParents("Message4"))
		testFramework.CreateMessage("Message22", WithStrongParents("Message20", "Message21"))
		testFramework.CreateMessage("Message23", WithStrongParents("Message5", "Message22"))
		testFramework.IssueMessages("Message20").WaitMessagesBooked()
		testFramework.IssueMessages("Message21").WaitMessagesBooked()
		testFramework.IssueMessages("Message22").WaitMessagesBooked()
		testFramework.IssueMessages("Message23").WaitMessagesBooked()

		checkMarkers(t, testFramework, map[string]*markers.Markers{
			"Message1":    markers.NewMarkers(markers.NewMarker(1, 1)),
			"Message2":    markers.NewMarkers(markers.NewMarker(2, 1)),
			"Message3":    markers.NewMarkers(markers.NewMarker(3, 1)),
			"Message4":    markers.NewMarkers(markers.NewMarker(4, 1)),
			"Message5":    markers.NewMarkers(markers.NewMarker(4, 2)),
			"Message6":    markers.NewMarkers(markers.NewMarker(2, 2)),
			"Message7":    markers.NewMarkers(markers.NewMarker(2, 3)),
			"Message8":    markers.NewMarkers(markers.NewMarker(5, 4)),
			"Message9":    markers.NewMarkers(markers.NewMarker(2, 3), markers.NewMarker(3, 1)),
			"Message10":   markers.NewMarkers(markers.NewMarker(2, 4)),
			"Message11":   markers.NewMarkers(markers.NewMarker(5, 5)),
			"Message11.5": markers.NewMarkers(markers.NewMarker(2, 3), markers.NewMarker(3, 1)),
			"Message12":   markers.NewMarkers(markers.NewMarker(6, 2)),
			"Message13":   markers.NewMarkers(markers.NewMarker(7, 4)),
			"Message14":   markers.NewMarkers(markers.NewMarker(2, 4), markers.NewMarker(6, 2)),
			"Message15":   markers.NewMarkers(markers.NewMarker(2, 5)),
			"Message16":   markers.NewMarkers(markers.NewMarker(8, 1)),
			"Message17":   markers.NewMarkers(markers.NewMarker(2, 4), markers.NewMarker(6, 2)),
			"Message18":   markers.NewMarkers(markers.NewMarker(7, 5)),
			"Message19":   markers.NewMarkers(markers.NewMarker(9, 4)),
			"Message20":   markers.NewMarkers(markers.NewMarker(10, 2)),
			"Message21":   markers.NewMarkers(markers.NewMarker(4, 1)),
			"Message22":   markers.NewMarkers(markers.NewMarker(10, 3)),
			"Message23":   markers.NewMarkers(markers.NewMarker(4, 4)),
		})
		checkMessageMetadataBranchIDs(t, testFramework, map[string]ledgerstate.BranchID{
			"Message1":    ledgerstate.UndefinedBranchID,
			"Message2":    ledgerstate.UndefinedBranchID,
			"Message3":    ledgerstate.UndefinedBranchID,
			"Message4":    ledgerstate.UndefinedBranchID,
			"Message5":    ledgerstate.UndefinedBranchID,
			"Message6":    ledgerstate.UndefinedBranchID,
			"Message7":    ledgerstate.UndefinedBranchID,
			"Message8":    ledgerstate.UndefinedBranchID,
			"Message9":    testFramework.BranchID("A+D+F+H"),
			"Message10":   ledgerstate.UndefinedBranchID,
			"Message11":   ledgerstate.UndefinedBranchID,
			"Message11.5": testFramework.BranchID("A+D+F+H"),
			"Message12":   ledgerstate.UndefinedBranchID,
			"Message13":   ledgerstate.UndefinedBranchID,
			"Message14":   testFramework.BranchID("B+E+F"),
			"Message15":   ledgerstate.UndefinedBranchID,
			"Message16":   ledgerstate.UndefinedBranchID,
			"Message17":   testFramework.BranchID("B+E+F"),
			"Message18":   ledgerstate.UndefinedBranchID,
			"Message19":   ledgerstate.UndefinedBranchID,
			"Message20":   ledgerstate.UndefinedBranchID,
			"Message21":   ledgerstate.UndefinedBranchID,
			"Message22":   ledgerstate.UndefinedBranchID,
			"Message23":   ledgerstate.UndefinedBranchID,
		})
		checkIndividuallyMappedMessages(t, testFramework, map[ledgerstate.BranchID][]string{
			testFramework.BranchID("A+D+F+H"): {"Message9", "Message11.5"},
			testFramework.BranchID("B+E+F"):   {"Message14", "Message17"},
		})
		checkBranchIDs(t, testFramework, map[string]ledgerstate.BranchID{
			"Message1":    testFramework.BranchID("A"),
			"Message2":    testFramework.BranchID("B"),
			"Message3":    testFramework.BranchID("C"),
			"Message4":    testFramework.BranchID("F"),
			"Message5":    testFramework.BranchID("D+F"),
			"Message6":    testFramework.BranchID("B"),
			"Message7":    testFramework.BranchID("B+D+F"),
			"Message8":    testFramework.BranchID("A+C+D+F"),
			"Message9":    testFramework.BranchID("A+D+F+H"),
			"Message10":   testFramework.BranchID("B+D+F"),
			"Message11":   testFramework.BranchID("A+D+F+H"),
			"Message11.5": testFramework.BranchID("A+D+F+H"),
			"Message12":   testFramework.BranchID("A+E"),
			"Message13":   testFramework.BranchID("B+E+F"),
			"Message14":   testFramework.BranchID("B+E+F"),
			"Message15":   testFramework.BranchID("B+D+F"),
			"Message16":   testFramework.BranchID("G"),
			"Message17":   testFramework.BranchID("B+E+F"),
			"Message18":   testFramework.BranchID("B+E+F"),
			"Message19":   testFramework.BranchID("D+F+I"),
			"Message20":   testFramework.BranchID("F"),
			"Message21":   testFramework.BranchID("F"),
			"Message22":   testFramework.BranchID("F"),
			"Message23":   testFramework.BranchID("D+F"),
		})
	}

	// ISSUE Message30
	{
		msg := testFramework.CreateMessage("Message30", WithStrongParents("Message1", "Message2"))
		testFramework.IssueMessages("Message30").WaitMessagesBooked()
		tangle.Storage.MessageMetadata(msg.ID()).Consume(func(messageMetadata *MessageMetadata) {
			assert.True(t, messageMetadata.invalid)
		})

		checkMarkers(t, testFramework, map[string]*markers.Markers{
			"Message1":    markers.NewMarkers(markers.NewMarker(1, 1)),
			"Message2":    markers.NewMarkers(markers.NewMarker(2, 1)),
			"Message3":    markers.NewMarkers(markers.NewMarker(3, 1)),
			"Message4":    markers.NewMarkers(markers.NewMarker(4, 1)),
			"Message5":    markers.NewMarkers(markers.NewMarker(4, 2)),
			"Message6":    markers.NewMarkers(markers.NewMarker(2, 2)),
			"Message7":    markers.NewMarkers(markers.NewMarker(2, 3)),
			"Message8":    markers.NewMarkers(markers.NewMarker(5, 4)),
			"Message9":    markers.NewMarkers(markers.NewMarker(2, 3), markers.NewMarker(3, 1)),
			"Message10":   markers.NewMarkers(markers.NewMarker(2, 4)),
			"Message11":   markers.NewMarkers(markers.NewMarker(5, 5)),
			"Message11.5": markers.NewMarkers(markers.NewMarker(2, 3), markers.NewMarker(3, 1)),
			"Message12":   markers.NewMarkers(markers.NewMarker(6, 2)),
			"Message13":   markers.NewMarkers(markers.NewMarker(7, 4)),
			"Message14":   markers.NewMarkers(markers.NewMarker(2, 4), markers.NewMarker(6, 2)),
			"Message15":   markers.NewMarkers(markers.NewMarker(2, 5)),
			"Message16":   markers.NewMarkers(markers.NewMarker(8, 1)),
			"Message17":   markers.NewMarkers(markers.NewMarker(2, 4), markers.NewMarker(6, 2)),
			"Message18":   markers.NewMarkers(markers.NewMarker(7, 5)),
			"Message19":   markers.NewMarkers(markers.NewMarker(9, 4)),
		})
		checkMessageMetadataBranchIDs(t, testFramework, map[string]ledgerstate.BranchID{
			"Message1":    ledgerstate.UndefinedBranchID,
			"Message2":    ledgerstate.UndefinedBranchID,
			"Message3":    ledgerstate.UndefinedBranchID,
			"Message4":    ledgerstate.UndefinedBranchID,
			"Message5":    ledgerstate.UndefinedBranchID,
			"Message6":    ledgerstate.UndefinedBranchID,
			"Message7":    ledgerstate.UndefinedBranchID,
			"Message8":    ledgerstate.UndefinedBranchID,
			"Message9":    testFramework.BranchID("A+D+F+H"),
			"Message10":   ledgerstate.UndefinedBranchID,
			"Message11":   ledgerstate.UndefinedBranchID,
			"Message11.5": testFramework.BranchID("A+D+F+H"),
			"Message12":   ledgerstate.UndefinedBranchID,
			"Message13":   ledgerstate.UndefinedBranchID,
			"Message14":   testFramework.BranchID("B+E+F"),
			"Message15":   ledgerstate.UndefinedBranchID,
			"Message16":   ledgerstate.UndefinedBranchID,
			"Message17":   testFramework.BranchID("B+E+F"),
			"Message18":   ledgerstate.UndefinedBranchID,
			"Message19":   ledgerstate.UndefinedBranchID,
		})
		checkBranchIDs(t, testFramework, map[string]ledgerstate.BranchID{
			"Message1":    testFramework.BranchID("A"),
			"Message2":    testFramework.BranchID("B"),
			"Message3":    testFramework.BranchID("C"),
			"Message4":    testFramework.BranchID("F"),
			"Message5":    testFramework.BranchID("D+F"),
			"Message6":    testFramework.BranchID("B"),
			"Message7":    testFramework.BranchID("B+D+F"),
			"Message8":    testFramework.BranchID("A+C+D+F"),
			"Message9":    testFramework.BranchID("A+D+F+H"),
			"Message10":   testFramework.BranchID("B+D+F"),
			"Message11":   testFramework.BranchID("A+D+F+H"),
			"Message11.5": testFramework.BranchID("A+D+F+H"),
			"Message12":   testFramework.BranchID("A+E"),
			"Message13":   testFramework.BranchID("B+E+F"),
			"Message14":   testFramework.BranchID("B+E+F"),
			"Message15":   testFramework.BranchID("B+D+F"),
			"Message16":   testFramework.BranchID("G"),
			"Message17":   testFramework.BranchID("B+E+F"),
			"Message18":   testFramework.BranchID("B+E+F"),
			"Message19":   testFramework.BranchID("D+F+I"),
		})
	}

	// ISSUE Message31
	{
		msg := testFramework.CreateMessage("Message31", WithStrongParents("Message1", "Message2"), WithLikeParents("Message2"), WithInputs("G"), WithOutput("O", 500))
		testFramework.IssueMessages("Message31").WaitMessagesBooked()
		tangle.Storage.MessageMetadata(msg.ID()).Consume(func(messageMetadata *MessageMetadata) {
			assert.True(t, messageMetadata.invalid)
		})

		checkMarkers(t, testFramework, map[string]*markers.Markers{
			"Message1":    markers.NewMarkers(markers.NewMarker(1, 1)),
			"Message2":    markers.NewMarkers(markers.NewMarker(2, 1)),
			"Message3":    markers.NewMarkers(markers.NewMarker(3, 1)),
			"Message4":    markers.NewMarkers(markers.NewMarker(4, 1)),
			"Message5":    markers.NewMarkers(markers.NewMarker(4, 2)),
			"Message6":    markers.NewMarkers(markers.NewMarker(2, 2)),
			"Message7":    markers.NewMarkers(markers.NewMarker(2, 3)),
			"Message8":    markers.NewMarkers(markers.NewMarker(5, 4)),
			"Message9":    markers.NewMarkers(markers.NewMarker(2, 3), markers.NewMarker(3, 1)),
			"Message10":   markers.NewMarkers(markers.NewMarker(2, 4)),
			"Message11":   markers.NewMarkers(markers.NewMarker(5, 5)),
			"Message11.5": markers.NewMarkers(markers.NewMarker(2, 3), markers.NewMarker(3, 1)),
			"Message12":   markers.NewMarkers(markers.NewMarker(6, 2)),
			"Message13":   markers.NewMarkers(markers.NewMarker(7, 4)),
			"Message14":   markers.NewMarkers(markers.NewMarker(2, 4), markers.NewMarker(6, 2)),
			"Message15":   markers.NewMarkers(markers.NewMarker(2, 5)),
			"Message16":   markers.NewMarkers(markers.NewMarker(8, 1)),
			"Message17":   markers.NewMarkers(markers.NewMarker(2, 4), markers.NewMarker(6, 2)),
			"Message18":   markers.NewMarkers(markers.NewMarker(7, 5)),
			"Message19":   markers.NewMarkers(markers.NewMarker(9, 4)),
		})
		checkMessageMetadataBranchIDs(t, testFramework, map[string]ledgerstate.BranchID{
			"Message1":    ledgerstate.UndefinedBranchID,
			"Message2":    ledgerstate.UndefinedBranchID,
			"Message3":    ledgerstate.UndefinedBranchID,
			"Message4":    ledgerstate.UndefinedBranchID,
			"Message5":    ledgerstate.UndefinedBranchID,
			"Message6":    ledgerstate.UndefinedBranchID,
			"Message7":    ledgerstate.UndefinedBranchID,
			"Message8":    ledgerstate.UndefinedBranchID,
			"Message9":    testFramework.BranchID("A+D+F+H"),
			"Message10":   ledgerstate.UndefinedBranchID,
			"Message11":   ledgerstate.UndefinedBranchID,
			"Message11.5": testFramework.BranchID("A+D+F+H"),
			"Message12":   ledgerstate.UndefinedBranchID,
			"Message13":   ledgerstate.UndefinedBranchID,
			"Message14":   testFramework.BranchID("B+E+F"),
			"Message15":   ledgerstate.UndefinedBranchID,
			"Message16":   ledgerstate.UndefinedBranchID,
			"Message17":   testFramework.BranchID("B+E+F"),
			"Message18":   ledgerstate.UndefinedBranchID,
			"Message19":   ledgerstate.UndefinedBranchID,
		})
		checkBranchIDs(t, testFramework, map[string]ledgerstate.BranchID{
			"Message1":    testFramework.BranchID("A"),
			"Message2":    testFramework.BranchID("B"),
			"Message3":    testFramework.BranchID("C"),
			"Message4":    testFramework.BranchID("F"),
			"Message5":    testFramework.BranchID("D+F"),
			"Message6":    testFramework.BranchID("B"),
			"Message7":    testFramework.BranchID("B+D+F"),
			"Message8":    testFramework.BranchID("A+C+D+F"),
			"Message9":    testFramework.BranchID("A+D+F+H"),
			"Message10":   testFramework.BranchID("B+D+F"),
			"Message11":   testFramework.BranchID("A+D+F+H"),
			"Message11.5": testFramework.BranchID("A+D+F+H"),
			"Message12":   testFramework.BranchID("A+E"),
			"Message13":   testFramework.BranchID("B+E+F"),
			"Message14":   testFramework.BranchID("B+E+F"),
			"Message15":   testFramework.BranchID("B+D+F"),
			"Message16":   testFramework.BranchID("G"),
			"Message17":   testFramework.BranchID("B+E+F"),
			"Message18":   testFramework.BranchID("B+E+F"),
			"Message19":   testFramework.BranchID("D+F+I"),
		})
	}

	// ISSUE Message32
	{
		msg := testFramework.CreateMessage("Message32", WithStrongParents("Message5"), WithLikeParents("Message6"))
		testFramework.IssueMessages("Message32").WaitMessagesBooked()
		tangle.Storage.MessageMetadata(msg.ID()).Consume(func(messageMetadata *MessageMetadata) {
			assert.True(t, messageMetadata.invalid)
		})

		checkMarkers(t, testFramework, map[string]*markers.Markers{
			"Message1":    markers.NewMarkers(markers.NewMarker(1, 1)),
			"Message2":    markers.NewMarkers(markers.NewMarker(2, 1)),
			"Message3":    markers.NewMarkers(markers.NewMarker(3, 1)),
			"Message4":    markers.NewMarkers(markers.NewMarker(4, 1)),
			"Message5":    markers.NewMarkers(markers.NewMarker(4, 2)),
			"Message6":    markers.NewMarkers(markers.NewMarker(2, 2)),
			"Message7":    markers.NewMarkers(markers.NewMarker(2, 3)),
			"Message8":    markers.NewMarkers(markers.NewMarker(5, 4)),
			"Message9":    markers.NewMarkers(markers.NewMarker(2, 3), markers.NewMarker(3, 1)),
			"Message10":   markers.NewMarkers(markers.NewMarker(2, 4)),
			"Message11":   markers.NewMarkers(markers.NewMarker(5, 5)),
			"Message11.5": markers.NewMarkers(markers.NewMarker(2, 3), markers.NewMarker(3, 1)),
			"Message12":   markers.NewMarkers(markers.NewMarker(6, 2)),
			"Message13":   markers.NewMarkers(markers.NewMarker(7, 4)),
			"Message14":   markers.NewMarkers(markers.NewMarker(2, 4), markers.NewMarker(6, 2)),
			"Message15":   markers.NewMarkers(markers.NewMarker(2, 5)),
			"Message16":   markers.NewMarkers(markers.NewMarker(8, 1)),
			"Message17":   markers.NewMarkers(markers.NewMarker(2, 4), markers.NewMarker(6, 2)),
			"Message18":   markers.NewMarkers(markers.NewMarker(7, 5)),
			"Message19":   markers.NewMarkers(markers.NewMarker(9, 4)),
		})
		checkMessageMetadataBranchIDs(t, testFramework, map[string]ledgerstate.BranchID{
			"Message1":    ledgerstate.UndefinedBranchID,
			"Message2":    ledgerstate.UndefinedBranchID,
			"Message3":    ledgerstate.UndefinedBranchID,
			"Message4":    ledgerstate.UndefinedBranchID,
			"Message5":    ledgerstate.UndefinedBranchID,
			"Message6":    ledgerstate.UndefinedBranchID,
			"Message7":    ledgerstate.UndefinedBranchID,
			"Message8":    ledgerstate.UndefinedBranchID,
			"Message9":    testFramework.BranchID("A+D+F+H"),
			"Message10":   ledgerstate.UndefinedBranchID,
			"Message11":   ledgerstate.UndefinedBranchID,
			"Message11.5": testFramework.BranchID("A+D+F+H"),
			"Message12":   ledgerstate.UndefinedBranchID,
			"Message13":   ledgerstate.UndefinedBranchID,
			"Message14":   testFramework.BranchID("B+E+F"),
			"Message15":   ledgerstate.UndefinedBranchID,
			"Message16":   ledgerstate.UndefinedBranchID,
			"Message17":   testFramework.BranchID("B+E+F"),
			"Message18":   ledgerstate.UndefinedBranchID,
			"Message19":   ledgerstate.UndefinedBranchID,
		})
		checkBranchIDs(t, testFramework, map[string]ledgerstate.BranchID{
			"Message1":    testFramework.BranchID("A"),
			"Message2":    testFramework.BranchID("B"),
			"Message3":    testFramework.BranchID("C"),
			"Message4":    testFramework.BranchID("F"),
			"Message5":    testFramework.BranchID("D+F"),
			"Message6":    testFramework.BranchID("B"),
			"Message7":    testFramework.BranchID("B+D+F"),
			"Message8":    testFramework.BranchID("A+C+D+F"),
			"Message9":    testFramework.BranchID("A+D+F+H"),
			"Message10":   testFramework.BranchID("B+D+F"),
			"Message11":   testFramework.BranchID("A+D+F+H"),
			"Message11.5": testFramework.BranchID("A+D+F+H"),
			"Message12":   testFramework.BranchID("A+E"),
			"Message13":   testFramework.BranchID("B+E+F"),
			"Message14":   testFramework.BranchID("B+E+F"),
			"Message15":   testFramework.BranchID("B+D+F"),
			"Message16":   testFramework.BranchID("G"),
			"Message17":   testFramework.BranchID("B+E+F"),
			"Message18":   testFramework.BranchID("B+E+F"),
			"Message19":   testFramework.BranchID("D+F+I"),
		})
	}

	// ISSUE Message33
	{
		msg := testFramework.CreateMessage("Message33", WithStrongParents("Message15", "Message11"))
		testFramework.IssueMessages("Message33").WaitMessagesBooked()
		tangle.Storage.MessageMetadata(msg.ID()).Consume(func(messageMetadata *MessageMetadata) {
			assert.True(t, messageMetadata.invalid)
		})

		checkMarkers(t, testFramework, map[string]*markers.Markers{
			"Message1":    markers.NewMarkers(markers.NewMarker(1, 1)),
			"Message2":    markers.NewMarkers(markers.NewMarker(2, 1)),
			"Message3":    markers.NewMarkers(markers.NewMarker(3, 1)),
			"Message4":    markers.NewMarkers(markers.NewMarker(4, 1)),
			"Message5":    markers.NewMarkers(markers.NewMarker(4, 2)),
			"Message6":    markers.NewMarkers(markers.NewMarker(2, 2)),
			"Message7":    markers.NewMarkers(markers.NewMarker(2, 3)),
			"Message8":    markers.NewMarkers(markers.NewMarker(5, 4)),
			"Message9":    markers.NewMarkers(markers.NewMarker(2, 3), markers.NewMarker(3, 1)),
			"Message10":   markers.NewMarkers(markers.NewMarker(2, 4)),
			"Message11":   markers.NewMarkers(markers.NewMarker(5, 5)),
			"Message11.5": markers.NewMarkers(markers.NewMarker(2, 3), markers.NewMarker(3, 1)),
			"Message12":   markers.NewMarkers(markers.NewMarker(6, 2)),
			"Message13":   markers.NewMarkers(markers.NewMarker(7, 4)),
			"Message14":   markers.NewMarkers(markers.NewMarker(2, 4), markers.NewMarker(6, 2)),
			"Message15":   markers.NewMarkers(markers.NewMarker(2, 5)),
			"Message16":   markers.NewMarkers(markers.NewMarker(8, 1)),
			"Message17":   markers.NewMarkers(markers.NewMarker(2, 4), markers.NewMarker(6, 2)),
			"Message18":   markers.NewMarkers(markers.NewMarker(7, 5)),
			"Message19":   markers.NewMarkers(markers.NewMarker(9, 4)),
		})
		checkMessageMetadataBranchIDs(t, testFramework, map[string]ledgerstate.BranchID{
			"Message1":    ledgerstate.UndefinedBranchID,
			"Message2":    ledgerstate.UndefinedBranchID,
			"Message3":    ledgerstate.UndefinedBranchID,
			"Message4":    ledgerstate.UndefinedBranchID,
			"Message5":    ledgerstate.UndefinedBranchID,
			"Message6":    ledgerstate.UndefinedBranchID,
			"Message7":    ledgerstate.UndefinedBranchID,
			"Message8":    ledgerstate.UndefinedBranchID,
			"Message9":    testFramework.BranchID("A+D+F+H"),
			"Message10":   ledgerstate.UndefinedBranchID,
			"Message11":   ledgerstate.UndefinedBranchID,
			"Message11.5": testFramework.BranchID("A+D+F+H"),
			"Message12":   ledgerstate.UndefinedBranchID,
			"Message13":   ledgerstate.UndefinedBranchID,
			"Message14":   testFramework.BranchID("B+E+F"),
			"Message15":   ledgerstate.UndefinedBranchID,
			"Message16":   ledgerstate.UndefinedBranchID,
			"Message17":   testFramework.BranchID("B+E+F"),
			"Message18":   ledgerstate.UndefinedBranchID,
			"Message19":   ledgerstate.UndefinedBranchID,
		})
		checkBranchIDs(t, testFramework, map[string]ledgerstate.BranchID{
			"Message1":    testFramework.BranchID("A"),
			"Message2":    testFramework.BranchID("B"),
			"Message3":    testFramework.BranchID("C"),
			"Message4":    testFramework.BranchID("F"),
			"Message5":    testFramework.BranchID("D+F"),
			"Message6":    testFramework.BranchID("B"),
			"Message7":    testFramework.BranchID("B+D+F"),
			"Message8":    testFramework.BranchID("A+C+D+F"),
			"Message9":    testFramework.BranchID("A+D+F+H"),
			"Message10":   testFramework.BranchID("B+D+F"),
			"Message11":   testFramework.BranchID("A+D+F+H"),
			"Message11.5": testFramework.BranchID("A+D+F+H"),
			"Message12":   testFramework.BranchID("A+E"),
			"Message13":   testFramework.BranchID("B+E+F"),
			"Message14":   testFramework.BranchID("B+E+F"),
			"Message15":   testFramework.BranchID("B+D+F"),
			"Message16":   testFramework.BranchID("G"),
			"Message17":   testFramework.BranchID("B+E+F"),
			"Message18":   testFramework.BranchID("B+E+F"),
			"Message19":   testFramework.BranchID("D+F+I"),
		})
	}

	// ISSUE Message34
	{
		msg := testFramework.CreateMessage("Message34", WithStrongParents("Message14", "Message9"))
		testFramework.IssueMessages("Message34").WaitMessagesBooked()
		tangle.Storage.MessageMetadata(msg.ID()).Consume(func(messageMetadata *MessageMetadata) {
			assert.True(t, messageMetadata.invalid)
		})

		checkMarkers(t, testFramework, map[string]*markers.Markers{
			"Message1":    markers.NewMarkers(markers.NewMarker(1, 1)),
			"Message2":    markers.NewMarkers(markers.NewMarker(2, 1)),
			"Message3":    markers.NewMarkers(markers.NewMarker(3, 1)),
			"Message4":    markers.NewMarkers(markers.NewMarker(4, 1)),
			"Message5":    markers.NewMarkers(markers.NewMarker(4, 2)),
			"Message6":    markers.NewMarkers(markers.NewMarker(2, 2)),
			"Message7":    markers.NewMarkers(markers.NewMarker(2, 3)),
			"Message8":    markers.NewMarkers(markers.NewMarker(5, 4)),
			"Message9":    markers.NewMarkers(markers.NewMarker(2, 3), markers.NewMarker(3, 1)),
			"Message10":   markers.NewMarkers(markers.NewMarker(2, 4)),
			"Message11":   markers.NewMarkers(markers.NewMarker(5, 5)),
			"Message11.5": markers.NewMarkers(markers.NewMarker(2, 3), markers.NewMarker(3, 1)),
			"Message12":   markers.NewMarkers(markers.NewMarker(6, 2)),
			"Message13":   markers.NewMarkers(markers.NewMarker(7, 4)),
			"Message14":   markers.NewMarkers(markers.NewMarker(2, 4), markers.NewMarker(6, 2)),
			"Message15":   markers.NewMarkers(markers.NewMarker(2, 5)),
			"Message16":   markers.NewMarkers(markers.NewMarker(8, 1)),
			"Message17":   markers.NewMarkers(markers.NewMarker(2, 4), markers.NewMarker(6, 2)),
			"Message18":   markers.NewMarkers(markers.NewMarker(7, 5)),
			"Message19":   markers.NewMarkers(markers.NewMarker(9, 4)),
		})
		checkMessageMetadataBranchIDs(t, testFramework, map[string]ledgerstate.BranchID{
			"Message1":    ledgerstate.UndefinedBranchID,
			"Message2":    ledgerstate.UndefinedBranchID,
			"Message3":    ledgerstate.UndefinedBranchID,
			"Message4":    ledgerstate.UndefinedBranchID,
			"Message5":    ledgerstate.UndefinedBranchID,
			"Message6":    ledgerstate.UndefinedBranchID,
			"Message7":    ledgerstate.UndefinedBranchID,
			"Message8":    ledgerstate.UndefinedBranchID,
			"Message9":    testFramework.BranchID("A+D+F+H"),
			"Message10":   ledgerstate.UndefinedBranchID,
			"Message11":   ledgerstate.UndefinedBranchID,
			"Message11.5": testFramework.BranchID("A+D+F+H"),
			"Message12":   ledgerstate.UndefinedBranchID,
			"Message13":   ledgerstate.UndefinedBranchID,
			"Message14":   testFramework.BranchID("B+E+F"),
			"Message15":   ledgerstate.UndefinedBranchID,
			"Message16":   ledgerstate.UndefinedBranchID,
			"Message17":   testFramework.BranchID("B+E+F"),
			"Message18":   ledgerstate.UndefinedBranchID,
			"Message19":   ledgerstate.UndefinedBranchID,
		})
		checkBranchIDs(t, testFramework, map[string]ledgerstate.BranchID{
			"Message1":    testFramework.BranchID("A"),
			"Message2":    testFramework.BranchID("B"),
			"Message3":    testFramework.BranchID("C"),
			"Message4":    testFramework.BranchID("F"),
			"Message5":    testFramework.BranchID("D+F"),
			"Message6":    testFramework.BranchID("B"),
			"Message7":    testFramework.BranchID("B+D+F"),
			"Message8":    testFramework.BranchID("A+C+D+F"),
			"Message9":    testFramework.BranchID("A+D+F+H"),
			"Message10":   testFramework.BranchID("B+D+F"),
			"Message11":   testFramework.BranchID("A+D+F+H"),
			"Message11.5": testFramework.BranchID("A+D+F+H"),
			"Message12":   testFramework.BranchID("A+E"),
			"Message13":   testFramework.BranchID("B+E+F"),
			"Message14":   testFramework.BranchID("B+E+F"),
			"Message15":   testFramework.BranchID("B+D+F"),
			"Message16":   testFramework.BranchID("G"),
			"Message17":   testFramework.BranchID("B+E+F"),
			"Message18":   testFramework.BranchID("B+E+F"),
			"Message19":   testFramework.BranchID("D+F+I"),
		})
	}

	// ISSUE Message35
	{
		testFramework.CreateMessage("Message35", WithStrongParents("Message15", "Message16"))
		testFramework.IssueMessages("Message35").WaitMessagesBooked()

		checkMarkers(t, testFramework, map[string]*markers.Markers{
			"Message1":    markers.NewMarkers(markers.NewMarker(1, 1)),
			"Message2":    markers.NewMarkers(markers.NewMarker(2, 1)),
			"Message3":    markers.NewMarkers(markers.NewMarker(3, 1)),
			"Message4":    markers.NewMarkers(markers.NewMarker(4, 1)),
			"Message5":    markers.NewMarkers(markers.NewMarker(4, 2)),
			"Message6":    markers.NewMarkers(markers.NewMarker(2, 2)),
			"Message7":    markers.NewMarkers(markers.NewMarker(2, 3)),
			"Message8":    markers.NewMarkers(markers.NewMarker(5, 4)),
			"Message9":    markers.NewMarkers(markers.NewMarker(2, 3), markers.NewMarker(3, 1)),
			"Message10":   markers.NewMarkers(markers.NewMarker(2, 4)),
			"Message11":   markers.NewMarkers(markers.NewMarker(5, 5)),
			"Message11.5": markers.NewMarkers(markers.NewMarker(2, 3), markers.NewMarker(3, 1)),
			"Message12":   markers.NewMarkers(markers.NewMarker(6, 2)),
			"Message13":   markers.NewMarkers(markers.NewMarker(7, 4)),
			"Message14":   markers.NewMarkers(markers.NewMarker(2, 4), markers.NewMarker(6, 2)),
			"Message15":   markers.NewMarkers(markers.NewMarker(2, 5)),
			"Message16":   markers.NewMarkers(markers.NewMarker(8, 1)),
			"Message17":   markers.NewMarkers(markers.NewMarker(2, 4), markers.NewMarker(6, 2)),
			"Message18":   markers.NewMarkers(markers.NewMarker(7, 5)),
			"Message19":   markers.NewMarkers(markers.NewMarker(9, 4)),
		})
		checkMessageMetadataBranchIDs(t, testFramework, map[string]ledgerstate.BranchID{
			"Message1":    ledgerstate.UndefinedBranchID,
			"Message2":    ledgerstate.UndefinedBranchID,
			"Message3":    ledgerstate.UndefinedBranchID,
			"Message4":    ledgerstate.UndefinedBranchID,
			"Message5":    ledgerstate.UndefinedBranchID,
			"Message6":    ledgerstate.UndefinedBranchID,
			"Message7":    ledgerstate.UndefinedBranchID,
			"Message8":    ledgerstate.UndefinedBranchID,
			"Message9":    testFramework.BranchID("A+D+F+H"),
			"Message10":   ledgerstate.UndefinedBranchID,
			"Message11":   ledgerstate.UndefinedBranchID,
			"Message11.5": testFramework.BranchID("A+D+F+H"),
			"Message12":   ledgerstate.UndefinedBranchID,
			"Message13":   ledgerstate.UndefinedBranchID,
			"Message14":   testFramework.BranchID("B+E+F"),
			"Message15":   ledgerstate.UndefinedBranchID,
			"Message16":   ledgerstate.UndefinedBranchID,
			"Message17":   testFramework.BranchID("B+E+F"),
			"Message18":   ledgerstate.UndefinedBranchID,
			"Message19":   ledgerstate.UndefinedBranchID,
		})
		checkBranchIDs(t, testFramework, map[string]ledgerstate.BranchID{
			"Message1":    testFramework.BranchID("A"),
			"Message2":    testFramework.BranchID("B"),
			"Message3":    testFramework.BranchID("C"),
			"Message4":    testFramework.BranchID("F"),
			"Message5":    testFramework.BranchID("D+F"),
			"Message6":    testFramework.BranchID("B"),
			"Message7":    testFramework.BranchID("B+D+F"),
			"Message8":    testFramework.BranchID("A+C+D+F"),
			"Message9":    testFramework.BranchID("A+D+F+H"),
			"Message10":   testFramework.BranchID("B+D+F"),
			"Message11":   testFramework.BranchID("A+D+F+H"),
			"Message11.5": testFramework.BranchID("A+D+F+H"),
			"Message12":   testFramework.BranchID("A+E"),
			"Message13":   testFramework.BranchID("B+E+F"),
			"Message14":   testFramework.BranchID("B+E+F"),
			"Message15":   testFramework.BranchID("B+D+F"),
			"Message16":   testFramework.BranchID("G"),
			"Message17":   testFramework.BranchID("B+E+F"),
			"Message18":   testFramework.BranchID("B+E+F"),
			"Message19":   testFramework.BranchID("D+F+I"),
		})
	}
}

func checkIndividuallyMappedMessages(t *testing.T, testFramework *MessageTestFramework, expectedIndividuallyMappedMessages map[ledgerstate.BranchID][]string) {
	expectedMappings := 0

	for branchID, expectedMessageAliases := range expectedIndividuallyMappedMessages {
		for _, alias := range expectedMessageAliases {
			expectedMappings++
			assert.True(t, testFramework.tangle.Storage.IndividuallyMappedMessage(branchID, testFramework.Message(alias).ID()).Consume(func(individuallyMappedMessage *IndividuallyMappedMessage) {}))
		}
	}

	// check that there's only exactly as many individually mapped messages as expected (ie old stuff gets cleaned up)
	testFramework.tangle.Storage.individuallyMappedMessageStorage.ForEach(func(key []byte, cachedObject objectstorage.CachedObject) bool {
		defer cachedObject.Release()

		expectedMappings--

		return true
	})

	assert.Zero(t, expectedMappings)
}

func checkMarkers(t *testing.T, testFramework *MessageTestFramework, expectedMarkers map[string]*markers.Markers) {
	for messageID, expectedMarkersOfMessage := range expectedMarkers {
		assert.True(t, testFramework.tangle.Storage.MessageMetadata(testFramework.Message(messageID).ID()).Consume(func(messageMetadata *MessageMetadata) {
			assert.True(t, expectedMarkersOfMessage.Equals(messageMetadata.StructureDetails().PastMarkers), "Markers of %s are wrong.\n"+
				"Expected: %+v\nActual: %+v", messageID, expectedMarkersOfMessage, messageMetadata.StructureDetails().PastMarkers)
		}))

		// if we have only a single marker - check if the marker is mapped to this message (or its inherited past marker)
		if expectedMarkersOfMessage.Size() == 1 {
			currentMarker := expectedMarkersOfMessage.Marker()

			mappedMessageIDOfMarker := testFramework.tangle.Booker.MarkersManager.MessageID(currentMarker)
			currentMessageID := testFramework.Message(messageID).ID()

			if mappedMessageIDOfMarker == currentMessageID {
				continue
			}

			assert.True(t, testFramework.tangle.Storage.MessageMetadata(mappedMessageIDOfMarker).Consume(func(messageMetadata *MessageMetadata) {
				assert.True(t, messageMetadata.StructureDetails().IsPastMarker && *messageMetadata.StructureDetails().PastMarkers.Marker() == *currentMarker, "%s was mapped to wrong %s", currentMarker, messageMetadata.ID())
			}), "failed to load Message with %s", mappedMessageIDOfMarker)
		}
	}
}

func checkBranchIDs(t *testing.T, testFramework *MessageTestFramework, expectedBranchIDs map[string]ledgerstate.BranchID) {
	for messageID, expectedBranchID := range expectedBranchIDs {
		retrievedBranchID, err := testFramework.tangle.Booker.MessageBranchID(testFramework.Message(messageID).ID())
		assert.NoError(t, err)

		assert.Equal(t, expectedBranchID, retrievedBranchID, "BranchID of %s should be %s but is %s", messageID, expectedBranchID, retrievedBranchID)
	}
}

func checkMessageMetadataBranchIDs(t *testing.T, testFramework *MessageTestFramework, expectedBranchIDs map[string]ledgerstate.BranchID) {
	for messageID, expectedBranchID := range expectedBranchIDs {
		assert.True(t, testFramework.tangle.Storage.MessageMetadata(testFramework.Message(messageID).ID()).Consume(func(messageMetadata *MessageMetadata) {
			assert.Equal(t, expectedBranchID, messageMetadata.BranchID(), "BranchID of %s should be %s but is %s in the Metadata", messageID, expectedBranchID, messageMetadata.BranchID())
		}))
	}
}
