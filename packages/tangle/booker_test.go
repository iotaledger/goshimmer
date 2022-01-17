//nolint:dupl
package tangle

import (
	"fmt"
	"testing"

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

	for _, messageAlias := range []string{"Message9"} {
		assert.Truef(t, testFramework.MessageMetadata(messageAlias).objectivelyInvalid, "%s not invalid", messageAlias)
	}

	checkBranchIDs(t, testFramework, map[string]ledgerstate.BranchIDs{
		"Message1": ledgerstate.NewBranchIDs(ledgerstate.MasterBranchID),
		"Message3": ledgerstate.NewBranchIDs(ledgerstate.MasterBranchID),
		"Message2": ledgerstate.NewBranchIDs(ledgerstate.MasterBranchID),
		"Message4": testFramework.BranchIDs("red"),
		"Message5": testFramework.BranchIDs("yellow"),
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

	testFramework.RegisterBranchID("purple", "Message5")
	testFramework.RegisterBranchID("red", "Message4")
	testFramework.RegisterBranchID("yellow", "Message2")
	testFramework.RegisterBranchID("orange", "Message7")
	testFramework.RegisterBranchID("blue", "Message9")

	checkBranchIDs(t, testFramework, map[string]ledgerstate.BranchIDs{
		"Message1": ledgerstate.NewBranchIDs(ledgerstate.MasterBranchID),
		"Message2": testFramework.BranchIDs("yellow"),
		"Message3": testFramework.BranchIDs("yellow"),
		"Message4": testFramework.BranchIDs("red"),
		"Message5": testFramework.BranchIDs("yellow", "purple"),
		"Message6": testFramework.BranchIDs("yellow", "purple"),
		"Message7": testFramework.BranchIDs("red", "orange"),
		"Message8": testFramework.BranchIDs("red", "orange"),
		"Message9": testFramework.BranchIDs("red", "orange", "blue"),
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
	testFramework.RegisterBranchID("orange", "Message7")
	testFramework.RegisterBranchID("blue", "Message9")

	checkBranchIDs(t, testFramework, map[string]ledgerstate.BranchIDs{
		"Message1": ledgerstate.NewBranchIDs(ledgerstate.MasterBranchID),
		"Message2": testFramework.BranchIDs("purple"),
		"Message3": testFramework.BranchIDs("purple"),
		"Message4": testFramework.BranchIDs("red"),
		"Message5": testFramework.BranchIDs("yellow"),
		"Message6": testFramework.BranchIDs("yellow", "purple"),
		"Message7": testFramework.BranchIDs("red", "orange"),
		"Message8": testFramework.BranchIDs("red", "orange"),
		"Message9": testFramework.BranchIDs("red", "orange", "blue"),
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
		checkMessageMetadataDiffBranchIDs(t, testFramework, map[string][]ledgerstate.BranchID{
			"Message1": {ledgerstate.UndefinedBranchID, ledgerstate.UndefinedBranchID},
		})
		checkBranchIDs(t, testFramework, map[string]ledgerstate.BranchIDs{
			"Message1": ledgerstate.NewBranchIDs(ledgerstate.MasterBranchID),
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
		checkMessageMetadataDiffBranchIDs(t, testFramework, map[string][]ledgerstate.BranchID{
			"Message1":   {ledgerstate.UndefinedBranchID, ledgerstate.UndefinedBranchID},
			"Message1.5": {ledgerstate.UndefinedBranchID, ledgerstate.UndefinedBranchID},
		})
		checkBranchIDs(t, testFramework, map[string]ledgerstate.BranchIDs{
			"Message1":   ledgerstate.NewBranchIDs(ledgerstate.MasterBranchID),
			"Message1.5": ledgerstate.NewBranchIDs(ledgerstate.MasterBranchID),
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
		checkMessageMetadataDiffBranchIDs(t, testFramework, map[string][]ledgerstate.BranchID{
			"Message1":   {ledgerstate.UndefinedBranchID, ledgerstate.UndefinedBranchID},
			"Message1.5": {ledgerstate.UndefinedBranchID, ledgerstate.UndefinedBranchID},
			"Message2":   {ledgerstate.UndefinedBranchID, ledgerstate.UndefinedBranchID},
		})
		checkBranchIDs(t, testFramework, map[string]ledgerstate.BranchIDs{
			"Message1":   ledgerstate.NewBranchIDs(ledgerstate.MasterBranchID),
			"Message1.5": ledgerstate.NewBranchIDs(ledgerstate.MasterBranchID),
			"Message2":   ledgerstate.NewBranchIDs(ledgerstate.MasterBranchID),
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
		checkMessageMetadataDiffBranchIDs(t, testFramework, map[string][]ledgerstate.BranchID{
			"Message1":   {ledgerstate.UndefinedBranchID, ledgerstate.UndefinedBranchID},
			"Message1.5": {ledgerstate.UndefinedBranchID, ledgerstate.UndefinedBranchID},
			"Message2":   {ledgerstate.UndefinedBranchID, ledgerstate.UndefinedBranchID},
			"Message3":   {ledgerstate.UndefinedBranchID, ledgerstate.UndefinedBranchID},
		})
		checkBranchIDs(t, testFramework, map[string]ledgerstate.BranchIDs{
			"Message1":   ledgerstate.NewBranchIDs(ledgerstate.MasterBranchID),
			"Message1.5": ledgerstate.NewBranchIDs(ledgerstate.MasterBranchID),
			"Message2":   testFramework.BranchIDs("Message2"),
			"Message3":   testFramework.BranchIDs("Message3"),
		})
	}

	// ISSUE Message4
	{
		testFramework.CreateMessage("Message4", WithStrongParents("Message3"), WithInputs("A"), WithOutput("X", 500))

		testFramework.RegisterBranchID("Message1", "Message1")
		testFramework.RegisterBranchID("Message4", "Message4")

		testFramework.IssueMessages("Message4").WaitMessagesBooked()

		checkMarkers(t, testFramework, map[string]*markers.Markers{
			"Message1":   markers.NewMarkers(markers.NewMarker(1, 1)),
			"Message1.5": markers.NewMarkers(markers.NewMarker(1, 2)),
			"Message2":   markers.NewMarkers(markers.NewMarker(1, 3)),
			"Message3":   markers.NewMarkers(markers.NewMarker(2, 1)),
			"Message4":   markers.NewMarkers(markers.NewMarker(3, 2)),
		})
		checkMessageMetadataDiffBranchIDs(t, testFramework, map[string][]ledgerstate.BranchID{
			"Message1":   {ledgerstate.UndefinedBranchID, ledgerstate.UndefinedBranchID},
			"Message1.5": {ledgerstate.UndefinedBranchID, ledgerstate.UndefinedBranchID},
			"Message2":   {ledgerstate.UndefinedBranchID, ledgerstate.UndefinedBranchID},
			"Message3":   {ledgerstate.UndefinedBranchID, ledgerstate.UndefinedBranchID},
			"Message4":   {ledgerstate.UndefinedBranchID, ledgerstate.UndefinedBranchID},
		})
		checkBranchIDs(t, testFramework, map[string]ledgerstate.BranchIDs{
			"Message1":   testFramework.BranchIDs("Message1"),
			"Message1.5": testFramework.BranchIDs("Message1"),
			"Message2":   testFramework.BranchIDs("Message1", "Message2"),
			"Message3":   testFramework.BranchIDs("Message3"),
			"Message4":   testFramework.BranchIDs("Message3", "Message4"),
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
		checkMessageMetadataDiffBranchIDs(t, testFramework, map[string][]ledgerstate.BranchID{
			"Message1": {ledgerstate.UndefinedBranchID, ledgerstate.UndefinedBranchID},
		})
		checkBranchIDs(t, testFramework, map[string]ledgerstate.BranchIDs{
			"Message1": ledgerstate.NewBranchIDs(ledgerstate.MasterBranchID),
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
		checkMessageMetadataDiffBranchIDs(t, testFramework, map[string][]ledgerstate.BranchID{
			"Message1": {ledgerstate.UndefinedBranchID, ledgerstate.UndefinedBranchID},
			"Message2": {ledgerstate.UndefinedBranchID, ledgerstate.UndefinedBranchID},
		})
		checkBranchIDs(t, testFramework, map[string]ledgerstate.BranchIDs{
			"Message1": testFramework.BranchIDs("Message1"),
			"Message2": testFramework.BranchIDs("Message2"),
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
		checkMessageMetadataDiffBranchIDs(t, testFramework, map[string][]ledgerstate.BranchID{
			"Message1": {ledgerstate.UndefinedBranchID, ledgerstate.UndefinedBranchID},
			"Message2": {ledgerstate.UndefinedBranchID, ledgerstate.UndefinedBranchID},
			"Message3": {ledgerstate.UndefinedBranchID, ledgerstate.UndefinedBranchID},
		})
		checkBranchIDs(t, testFramework, map[string]ledgerstate.BranchIDs{
			"Message1": testFramework.BranchIDs("Message1"),
			"Message2": testFramework.BranchIDs("Message2"),
			"Message3": ledgerstate.NewBranchIDs(ledgerstate.MasterBranchID),
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
		checkMessageMetadataDiffBranchIDs(t, testFramework, map[string][]ledgerstate.BranchID{
			"Message1": {ledgerstate.UndefinedBranchID, ledgerstate.UndefinedBranchID},
			"Message2": {ledgerstate.UndefinedBranchID, ledgerstate.UndefinedBranchID},
			"Message3": {ledgerstate.UndefinedBranchID, ledgerstate.UndefinedBranchID},
			"Message4": {ledgerstate.UndefinedBranchID, ledgerstate.UndefinedBranchID},
		})
		checkBranchIDs(t, testFramework, map[string]ledgerstate.BranchIDs{
			"Message1": testFramework.BranchIDs("Message1"),
			"Message2": testFramework.BranchIDs("Message2"),
			"Message3": testFramework.BranchIDs("Message3"),
			"Message4": testFramework.BranchIDs("Message4"),
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
		checkMessageMetadataDiffBranchIDs(t, testFramework, map[string][]ledgerstate.BranchID{
			"Message1": {ledgerstate.UndefinedBranchID, ledgerstate.UndefinedBranchID},
			"Message2": {ledgerstate.UndefinedBranchID, ledgerstate.UndefinedBranchID},
			"Message3": {ledgerstate.UndefinedBranchID, ledgerstate.UndefinedBranchID},
			"Message4": {ledgerstate.UndefinedBranchID, ledgerstate.UndefinedBranchID},
			"Message5": {ledgerstate.UndefinedBranchID, ledgerstate.UndefinedBranchID},
		})
		checkBranchIDs(t, testFramework, map[string]ledgerstate.BranchIDs{
			"Message1": testFramework.BranchIDs("Message1"),
			"Message2": testFramework.BranchIDs("Message2"),
			"Message3": testFramework.BranchIDs("Message3"),
			"Message4": testFramework.BranchIDs("Message4"),
			"Message5": testFramework.BranchIDs("Message1"),
		})
	}

	// ISSUE Message6
	{
		testFramework.CreateMessage("Message6", WithStrongParents("Message1", "Message3"))
		testFramework.IssueMessages("Message6").WaitMessagesBooked()

		checkMarkers(t, testFramework, map[string]*markers.Markers{
			"Message1": markers.NewMarkers(markers.NewMarker(1, 1)),
			"Message2": markers.NewMarkers(markers.NewMarker(2, 1)),
			"Message3": markers.NewMarkers(markers.NewMarker(3, 1)),
			"Message4": markers.NewMarkers(markers.NewMarker(4, 1)),
			"Message5": markers.NewMarkers(markers.NewMarker(1, 2)),
			"Message6": markers.NewMarkers(markers.NewMarker(5, 2)),
		})
		checkMessageMetadataDiffBranchIDs(t, testFramework, map[string][]ledgerstate.BranchID{
			"Message1": {ledgerstate.UndefinedBranchID, ledgerstate.UndefinedBranchID},
			"Message2": {ledgerstate.UndefinedBranchID, ledgerstate.UndefinedBranchID},
			"Message3": {ledgerstate.UndefinedBranchID, ledgerstate.UndefinedBranchID},
			"Message4": {ledgerstate.UndefinedBranchID, ledgerstate.UndefinedBranchID},
			"Message5": {ledgerstate.UndefinedBranchID, ledgerstate.UndefinedBranchID},
			"Message6": {ledgerstate.UndefinedBranchID, ledgerstate.UndefinedBranchID},
		})
		checkBranchIDs(t, testFramework, map[string]ledgerstate.BranchIDs{
			"Message1": testFramework.BranchIDs("Message1"),
			"Message2": testFramework.BranchIDs("Message2"),
			"Message3": testFramework.BranchIDs("Message3"),
			"Message4": testFramework.BranchIDs("Message4"),
			"Message5": testFramework.BranchIDs("Message1"),
			"Message6": testFramework.BranchIDs("Message1", "Message3"),
		})
	}

	// ISSUE Message7
	{
		testFramework.CreateMessage("Message7", WithStrongParents("Message3", "Message5"))
		testFramework.IssueMessages("Message7").WaitMessagesBooked()

		checkMarkers(t, testFramework, map[string]*markers.Markers{
			"Message1": markers.NewMarkers(markers.NewMarker(1, 1)),
			"Message2": markers.NewMarkers(markers.NewMarker(2, 1)),
			"Message3": markers.NewMarkers(markers.NewMarker(3, 1)),
			"Message4": markers.NewMarkers(markers.NewMarker(4, 1)),
			"Message5": markers.NewMarkers(markers.NewMarker(1, 2)),
			"Message6": markers.NewMarkers(markers.NewMarker(5, 2)),
			"Message7": markers.NewMarkers(markers.NewMarker(1, 2), markers.NewMarker(3, 1)),
		})
		checkMessageMetadataDiffBranchIDs(t, testFramework, map[string][]ledgerstate.BranchID{
			"Message1": {ledgerstate.UndefinedBranchID, ledgerstate.UndefinedBranchID},
			"Message2": {ledgerstate.UndefinedBranchID, ledgerstate.UndefinedBranchID},
			"Message3": {ledgerstate.UndefinedBranchID, ledgerstate.UndefinedBranchID},
			"Message4": {ledgerstate.UndefinedBranchID, ledgerstate.UndefinedBranchID},
			"Message5": {ledgerstate.UndefinedBranchID, ledgerstate.UndefinedBranchID},
			"Message6": {ledgerstate.UndefinedBranchID, ledgerstate.UndefinedBranchID},
			"Message7": {ledgerstate.UndefinedBranchID, ledgerstate.UndefinedBranchID},
		})
		checkBranchIDs(t, testFramework, map[string]ledgerstate.BranchIDs{
			"Message1": testFramework.BranchIDs("Message1"),
			"Message2": testFramework.BranchIDs("Message2"),
			"Message3": testFramework.BranchIDs("Message3"),
			"Message4": testFramework.BranchIDs("Message4"),
			"Message5": testFramework.BranchIDs("Message1"),
			"Message6": testFramework.BranchIDs("Message1", "Message3"),
			"Message7": testFramework.BranchIDs("Message1", "Message3"),
		})
	}

	// ISSUE Message8
	{
		testFramework.CreateMessage("Message8", WithStrongParents("Genesis"), WithInputs("Genesis3"), WithOutput("Message8", 500))

		testFramework.RegisterBranchID("Message5", "Message5")
		testFramework.RegisterBranchID("Message8", "Message8")

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
		checkMessageMetadataDiffBranchIDs(t, testFramework, map[string][]ledgerstate.BranchID{
			"Message1": {ledgerstate.UndefinedBranchID, ledgerstate.UndefinedBranchID},
			"Message2": {ledgerstate.UndefinedBranchID, ledgerstate.UndefinedBranchID},
			"Message3": {ledgerstate.UndefinedBranchID, ledgerstate.UndefinedBranchID},
			"Message4": {ledgerstate.UndefinedBranchID, ledgerstate.UndefinedBranchID},
			"Message5": {ledgerstate.UndefinedBranchID, ledgerstate.UndefinedBranchID},
			"Message6": {ledgerstate.UndefinedBranchID, ledgerstate.UndefinedBranchID},
			"Message7": {ledgerstate.UndefinedBranchID, ledgerstate.UndefinedBranchID},
			"Message8": {ledgerstate.UndefinedBranchID, ledgerstate.UndefinedBranchID},
		})

		checkBranchIDs(t, testFramework, map[string]ledgerstate.BranchIDs{
			"Message1": testFramework.BranchIDs("Message1"),
			"Message2": testFramework.BranchIDs("Message2"),
			"Message3": testFramework.BranchIDs("Message3"),
			"Message4": testFramework.BranchIDs("Message4"),
			"Message5": testFramework.BranchIDs("Message1", "Message5"),
			"Message6": testFramework.BranchIDs("Message1", "Message3"),
			"Message7": testFramework.BranchIDs("Message1", "Message3", "Message5"),
			"Message8": testFramework.BranchIDs("Message8"),
		})
	}
}

func TestBookerIndividuallyMappedMessagesSameSequence(t *testing.T) {
	tangle := NewTestTangle()
	defer tangle.Shutdown()

	testFramework := NewMessageTestFramework(
		tangle,
		WithGenesisOutput("A", 500),
		WithGenesisOutput("B", 500),
	)

	tangle.Setup()

	// ISSUE A1
	{
		testFramework.CreateMessage("A1", WithStrongParents("Genesis"), WithInputs("A"), WithOutput("A1", 500))
		testFramework.IssueMessages("A1").WaitMessagesBooked()

		checkMarkers(t, testFramework, map[string]*markers.Markers{
			"A1": markers.NewMarkers(markers.NewMarker(1, 1)),
		})
		checkMessageMetadataDiffBranchIDs(t, testFramework, map[string][]ledgerstate.BranchID{
			"A1": {ledgerstate.UndefinedBranchID, ledgerstate.UndefinedBranchID},
		})
		checkBranchIDs(t, testFramework, map[string]ledgerstate.BranchIDs{
			"A1": ledgerstate.NewBranchIDs(ledgerstate.MasterBranchID),
		})
	}

	// ISSUE A2
	{
		testFramework.CreateMessage("A2", WithStrongParents("A1"))
		testFramework.IssueMessages("A2").WaitMessagesBooked()

		checkMarkers(t, testFramework, map[string]*markers.Markers{
			"A1": markers.NewMarkers(markers.NewMarker(1, 1)),
			"A2": markers.NewMarkers(markers.NewMarker(1, 2)),
		})
		checkMessageMetadataDiffBranchIDs(t, testFramework, map[string][]ledgerstate.BranchID{
			"A1": {ledgerstate.UndefinedBranchID, ledgerstate.UndefinedBranchID},
			"A2": {ledgerstate.UndefinedBranchID, ledgerstate.UndefinedBranchID},
		})
		checkBranchIDs(t, testFramework, map[string]ledgerstate.BranchIDs{
			"A1": ledgerstate.NewBranchIDs(ledgerstate.MasterBranchID),
			"A2": ledgerstate.NewBranchIDs(ledgerstate.MasterBranchID),
		})
	}

	// ISSUE A3
	{
		testFramework.PreventNewMarkers(true).CreateMessage("A3", WithStrongParents("A2"), WithInputs("B"), WithOutput("B1", 500))
		testFramework.IssueMessages("A3").WaitMessagesBooked().PreventNewMarkers(false)

		checkMarkers(t, testFramework, map[string]*markers.Markers{
			"A1": markers.NewMarkers(markers.NewMarker(1, 1)),
			"A2": markers.NewMarkers(markers.NewMarker(1, 2)),
			"A3": markers.NewMarkers(markers.NewMarker(1, 2)),
		})
		checkMessageMetadataDiffBranchIDs(t, testFramework, map[string][]ledgerstate.BranchID{
			"A1": {ledgerstate.UndefinedBranchID, ledgerstate.UndefinedBranchID},
			"A2": {ledgerstate.UndefinedBranchID, ledgerstate.UndefinedBranchID},
			"A3": {ledgerstate.UndefinedBranchID, ledgerstate.UndefinedBranchID},
		})
		checkBranchIDs(t, testFramework, map[string]ledgerstate.BranchIDs{
			"A1": ledgerstate.NewBranchIDs(ledgerstate.MasterBranchID),
			"A2": ledgerstate.NewBranchIDs(ledgerstate.MasterBranchID),
			"A3": ledgerstate.NewBranchIDs(ledgerstate.MasterBranchID),
		})
	}

	// ISSUE A4
	{
		testFramework.CreateMessage("A4", WithStrongParents("A3"))
		testFramework.IssueMessages("A4").WaitMessagesBooked()

		checkMarkers(t, testFramework, map[string]*markers.Markers{
			"A1": markers.NewMarkers(markers.NewMarker(1, 1)),
			"A2": markers.NewMarkers(markers.NewMarker(1, 2)),
			"A3": markers.NewMarkers(markers.NewMarker(1, 2)),
			"A4": markers.NewMarkers(markers.NewMarker(1, 3)),
		})
		checkMessageMetadataDiffBranchIDs(t, testFramework, map[string][]ledgerstate.BranchID{
			"A1": {ledgerstate.UndefinedBranchID, ledgerstate.UndefinedBranchID},
			"A2": {ledgerstate.UndefinedBranchID, ledgerstate.UndefinedBranchID},
			"A3": {ledgerstate.UndefinedBranchID, ledgerstate.UndefinedBranchID},
			"A4": {ledgerstate.UndefinedBranchID, ledgerstate.UndefinedBranchID},
		})
		checkBranchIDs(t, testFramework, map[string]ledgerstate.BranchIDs{
			"A1": ledgerstate.NewBranchIDs(ledgerstate.MasterBranchID),
			"A2": ledgerstate.NewBranchIDs(ledgerstate.MasterBranchID),
			"A3": ledgerstate.NewBranchIDs(ledgerstate.MasterBranchID),
			"A4": ledgerstate.NewBranchIDs(ledgerstate.MasterBranchID),
		})
	}

	// ISSUE A3*
	{
		testFramework.CreateMessage("A3*", WithStrongParents("Genesis"), WithInputs("B"), WithOutput("A3*", 500))

		testFramework.RegisterBranchID("A3", "A3")
		testFramework.RegisterBranchID("A3*", "A3*")

		testFramework.IssueMessages("A3*").WaitMessagesBooked()

		checkMarkers(t, testFramework, map[string]*markers.Markers{
			"A1":  markers.NewMarkers(markers.NewMarker(1, 1)),
			"A2":  markers.NewMarkers(markers.NewMarker(1, 2)),
			"A3":  markers.NewMarkers(markers.NewMarker(1, 2)),
			"A4":  markers.NewMarkers(markers.NewMarker(1, 3)),
			"A3*": markers.NewMarkers(markers.NewMarker(2, 1)),
		})
		checkMessageMetadataDiffBranchIDs(t, testFramework, map[string][]ledgerstate.BranchID{
			"A1":  {ledgerstate.UndefinedBranchID, ledgerstate.UndefinedBranchID},
			"A2":  {ledgerstate.UndefinedBranchID, ledgerstate.UndefinedBranchID},
			"A3":  {testFramework.BranchID("A3"), ledgerstate.UndefinedBranchID},
			"A4":  {ledgerstate.UndefinedBranchID, ledgerstate.UndefinedBranchID},
			"A3*": {ledgerstate.UndefinedBranchID, ledgerstate.UndefinedBranchID},
		})
		checkBranchIDs(t, testFramework, map[string]ledgerstate.BranchIDs{
			"A1":  ledgerstate.NewBranchIDs(ledgerstate.MasterBranchID),
			"A2":  ledgerstate.NewBranchIDs(ledgerstate.MasterBranchID),
			"A3":  testFramework.BranchIDs("A3"),
			"A4":  testFramework.BranchIDs("A3"),
			"A3*": testFramework.BranchIDs("A3*"),
		})
	}

	// ISSUE A1*
	{
		testFramework.CreateMessage("A1*", WithStrongParents("Genesis"), WithInputs("A"), WithOutput("A1*", 500))

		testFramework.RegisterBranchID("A1", "A1")
		testFramework.RegisterBranchID("A1*", "A1*")

		testFramework.IssueMessages("A1*").WaitMessagesBooked()

		checkMarkers(t, testFramework, map[string]*markers.Markers{
			"A1":  markers.NewMarkers(markers.NewMarker(1, 1)),
			"A2":  markers.NewMarkers(markers.NewMarker(1, 2)),
			"A3":  markers.NewMarkers(markers.NewMarker(1, 2)),
			"A4":  markers.NewMarkers(markers.NewMarker(1, 3)),
			"A3*": markers.NewMarkers(markers.NewMarker(2, 1)),
			"A1*": markers.NewMarkers(markers.NewMarker(3, 1)),
		})
		checkMessageMetadataDiffBranchIDs(t, testFramework, map[string][]ledgerstate.BranchID{
			"A1":  {ledgerstate.UndefinedBranchID, ledgerstate.UndefinedBranchID},
			"A2":  {ledgerstate.UndefinedBranchID, ledgerstate.UndefinedBranchID},
			"A3":  {testFramework.BranchID("A3"), ledgerstate.UndefinedBranchID},
			"A4":  {ledgerstate.UndefinedBranchID, ledgerstate.UndefinedBranchID},
			"A3*": {ledgerstate.UndefinedBranchID, ledgerstate.UndefinedBranchID},
			"A1*": {ledgerstate.UndefinedBranchID, ledgerstate.UndefinedBranchID},
		})
		checkBranchIDs(t, testFramework, map[string]ledgerstate.BranchIDs{
			"A1":  testFramework.BranchIDs("A1"),
			"A2":  testFramework.BranchIDs("A1"),
			"A3":  testFramework.BranchIDs("A1", "A3"),
			"A4":  testFramework.BranchIDs("A1", "A3"),
			"A3*": testFramework.BranchIDs("A3*"),
			"A1*": testFramework.BranchIDs("A1*"),
		})
	}
}

func TestBookerMarkerMappingsGap(t *testing.T) {
	tangle := NewTestTangle()
	defer tangle.Shutdown()

	testFramework := NewMessageTestFramework(
		tangle,
		WithGenesisOutput("A", 500),
		WithGenesisOutput("B", 500),
		WithGenesisOutput("C", 500),
		WithGenesisOutput("D", 500),
		WithGenesisOutput("L", 500),
	)

	tangle.Setup()

	// ISSUE A1
	{
		testFramework.CreateMessage("A1", WithStrongParents("Genesis"), WithInputs("A"), WithOutput("A1", 500))
		testFramework.IssueMessages("A1").WaitMessagesBooked()

		checkMarkers(t, testFramework, map[string]*markers.Markers{
			"A1": markers.NewMarkers(markers.NewMarker(1, 1)),
		})
		checkMessageMetadataDiffBranchIDs(t, testFramework, map[string][]ledgerstate.BranchID{
			"A1": {ledgerstate.UndefinedBranchID, ledgerstate.UndefinedBranchID},
		})
		checkBranchIDs(t, testFramework, map[string]ledgerstate.BranchIDs{
			"A1": ledgerstate.NewBranchIDs(ledgerstate.MasterBranchID),
		})
	}

	// ISSUE B1
	{
		testFramework.CreateMessage("B1", WithStrongParents("Genesis"), WithInputs("A"), WithOutput("B1", 500))

		testFramework.RegisterBranchID("A", "A1")
		testFramework.RegisterBranchID("B", "B1")

		testFramework.IssueMessages("B1").WaitMessagesBooked()

		checkMarkers(t, testFramework, map[string]*markers.Markers{
			"A1": markers.NewMarkers(markers.NewMarker(1, 1)),
			"B1": markers.NewMarkers(markers.NewMarker(2, 1)),
		})
		checkMessageMetadataDiffBranchIDs(t, testFramework, map[string][]ledgerstate.BranchID{
			"A1": {ledgerstate.UndefinedBranchID, ledgerstate.UndefinedBranchID},
			"B1": {ledgerstate.UndefinedBranchID, ledgerstate.UndefinedBranchID},
		})
		checkBranchIDs(t, testFramework, map[string]ledgerstate.BranchIDs{
			"A1": testFramework.BranchIDs("A"),
			"B1": testFramework.BranchIDs("B"),
		})
	}

	// ISSUE C1
	{
		testFramework.CreateMessage("C1", WithStrongParents("Genesis"), WithInputs("C"), WithOutput("C1", 500))

		testFramework.RegisterBranchID("A", "A1")
		testFramework.RegisterBranchID("B", "B1")

		testFramework.IssueMessages("C1").WaitMessagesBooked()

		checkMarkers(t, testFramework, map[string]*markers.Markers{
			"A1": markers.NewMarkers(markers.NewMarker(1, 1)),
			"B1": markers.NewMarkers(markers.NewMarker(2, 1)),
			"C1": markers.NewMarkers(markers.NewMarker(3, 1)),
		})
		checkMessageMetadataDiffBranchIDs(t, testFramework, map[string][]ledgerstate.BranchID{
			"A1": {ledgerstate.UndefinedBranchID, ledgerstate.UndefinedBranchID},
			"B1": {ledgerstate.UndefinedBranchID, ledgerstate.UndefinedBranchID},
			"C1": {ledgerstate.UndefinedBranchID, ledgerstate.UndefinedBranchID},
		})
		checkBranchIDs(t, testFramework, map[string]ledgerstate.BranchIDs{
			"A1": testFramework.BranchIDs("A"),
			"B1": testFramework.BranchIDs("B"),
			"C1": ledgerstate.NewBranchIDs(ledgerstate.MasterBranchID),
		})
	}

	// ISSUE D1
	{
		testFramework.CreateMessage("D1", WithStrongParents("Genesis"), WithInputs("C"), WithOutput("D1", 500))

		testFramework.RegisterBranchID("C", "C1")
		testFramework.RegisterBranchID("D", "D1")

		testFramework.IssueMessages("D1").WaitMessagesBooked()

		checkMarkers(t, testFramework, map[string]*markers.Markers{
			"A1": markers.NewMarkers(markers.NewMarker(1, 1)),
			"B1": markers.NewMarkers(markers.NewMarker(2, 1)),
			"C1": markers.NewMarkers(markers.NewMarker(3, 1)),
			"D1": markers.NewMarkers(markers.NewMarker(4, 1)),
		})
		checkMessageMetadataDiffBranchIDs(t, testFramework, map[string][]ledgerstate.BranchID{
			"A1": {ledgerstate.UndefinedBranchID, ledgerstate.UndefinedBranchID},
			"B1": {ledgerstate.UndefinedBranchID, ledgerstate.UndefinedBranchID},
			"C1": {ledgerstate.UndefinedBranchID, ledgerstate.UndefinedBranchID},
			"D1": {ledgerstate.UndefinedBranchID, ledgerstate.UndefinedBranchID},
		})
		checkBranchIDs(t, testFramework, map[string]ledgerstate.BranchIDs{
			"A1": testFramework.BranchIDs("A"),
			"B1": testFramework.BranchIDs("B"),
			"C1": testFramework.BranchIDs("C"),
			"D1": testFramework.BranchIDs("D"),
		})
	}

	// ISSUE A2
	{
		testFramework.CreateMessage("A2", WithStrongParents("A1"), WithInputs("A1"), WithOutput("A2", 500))

		testFramework.IssueMessages("A2").WaitMessagesBooked()

		checkMarkers(t, testFramework, map[string]*markers.Markers{
			"A1": markers.NewMarkers(markers.NewMarker(1, 1)),
			"B1": markers.NewMarkers(markers.NewMarker(2, 1)),
			"C1": markers.NewMarkers(markers.NewMarker(3, 1)),
			"D1": markers.NewMarkers(markers.NewMarker(4, 1)),
			"A2": markers.NewMarkers(markers.NewMarker(1, 2)),
		})
		checkMessageMetadataDiffBranchIDs(t, testFramework, map[string][]ledgerstate.BranchID{
			"A1": {ledgerstate.UndefinedBranchID, ledgerstate.UndefinedBranchID},
			"B1": {ledgerstate.UndefinedBranchID, ledgerstate.UndefinedBranchID},
			"C1": {ledgerstate.UndefinedBranchID, ledgerstate.UndefinedBranchID},
			"D1": {ledgerstate.UndefinedBranchID, ledgerstate.UndefinedBranchID},
			"A2": {ledgerstate.UndefinedBranchID, ledgerstate.UndefinedBranchID},
		})
		checkBranchIDs(t, testFramework, map[string]ledgerstate.BranchIDs{
			"A1": testFramework.BranchIDs("A"),
			"B1": testFramework.BranchIDs("B"),
			"C1": testFramework.BranchIDs("C"),
			"D1": testFramework.BranchIDs("D"),
			"A2": testFramework.BranchIDs("A"),
		})
	}

	// ISSUE A3
	{
		testFramework.CreateMessage("A3", WithStrongParents("A2"))

		testFramework.IssueMessages("A3").WaitMessagesBooked()

		checkMarkers(t, testFramework, map[string]*markers.Markers{
			"A1": markers.NewMarkers(markers.NewMarker(1, 1)),
			"B1": markers.NewMarkers(markers.NewMarker(2, 1)),
			"C1": markers.NewMarkers(markers.NewMarker(3, 1)),
			"D1": markers.NewMarkers(markers.NewMarker(4, 1)),
			"A2": markers.NewMarkers(markers.NewMarker(1, 2)),
			"A3": markers.NewMarkers(markers.NewMarker(1, 3)),
		})
		checkMessageMetadataDiffBranchIDs(t, testFramework, map[string][]ledgerstate.BranchID{
			"A1": {ledgerstate.UndefinedBranchID, ledgerstate.UndefinedBranchID},
			"B1": {ledgerstate.UndefinedBranchID, ledgerstate.UndefinedBranchID},
			"C1": {ledgerstate.UndefinedBranchID, ledgerstate.UndefinedBranchID},
			"D1": {ledgerstate.UndefinedBranchID, ledgerstate.UndefinedBranchID},
			"A2": {ledgerstate.UndefinedBranchID, ledgerstate.UndefinedBranchID},
			"A3": {ledgerstate.UndefinedBranchID, ledgerstate.UndefinedBranchID},
		})
		checkBranchIDs(t, testFramework, map[string]ledgerstate.BranchIDs{
			"A1": testFramework.BranchIDs("A"),
			"B1": testFramework.BranchIDs("B"),
			"C1": testFramework.BranchIDs("C"),
			"D1": testFramework.BranchIDs("D"),
			"A2": testFramework.BranchIDs("A"),
			"A3": testFramework.BranchIDs("A"),
		})
	}

	// ISSUE A+C1
	{
		testFramework.CreateMessage("A+C1", WithStrongParents("A3", "C1"))

		testFramework.RegisterBranchID("A+C", "A1", "C1")

		testFramework.IssueMessages("A+C1").WaitMessagesBooked()

		checkMarkers(t, testFramework, map[string]*markers.Markers{
			"A1":   markers.NewMarkers(markers.NewMarker(1, 1)),
			"B1":   markers.NewMarkers(markers.NewMarker(2, 1)),
			"C1":   markers.NewMarkers(markers.NewMarker(3, 1)),
			"D1":   markers.NewMarkers(markers.NewMarker(4, 1)),
			"A2":   markers.NewMarkers(markers.NewMarker(1, 2)),
			"A3":   markers.NewMarkers(markers.NewMarker(1, 3)),
			"A+C1": markers.NewMarkers(markers.NewMarker(5, 4)),
		})
		checkMessageMetadataDiffBranchIDs(t, testFramework, map[string][]ledgerstate.BranchID{
			"A1":   {ledgerstate.UndefinedBranchID, ledgerstate.UndefinedBranchID},
			"B1":   {ledgerstate.UndefinedBranchID, ledgerstate.UndefinedBranchID},
			"C1":   {ledgerstate.UndefinedBranchID, ledgerstate.UndefinedBranchID},
			"D1":   {ledgerstate.UndefinedBranchID, ledgerstate.UndefinedBranchID},
			"A2":   {ledgerstate.UndefinedBranchID, ledgerstate.UndefinedBranchID},
			"A3":   {ledgerstate.UndefinedBranchID, ledgerstate.UndefinedBranchID},
			"A+C1": {ledgerstate.UndefinedBranchID, ledgerstate.UndefinedBranchID},
		})
		checkBranchIDs(t, testFramework, map[string]ledgerstate.BranchIDs{
			"A1":   testFramework.BranchIDs("A"),
			"B1":   testFramework.BranchIDs("B"),
			"C1":   testFramework.BranchIDs("C"),
			"D1":   testFramework.BranchIDs("D"),
			"A2":   testFramework.BranchIDs("A"),
			"A3":   testFramework.BranchIDs("A"),
			"A+C1": testFramework.BranchIDs("A", "C"),
		})
	}

	// ISSUE A+C2
	{
		testFramework.CreateMessage("A+C2", WithStrongParents("A3", "C1"))

		testFramework.IssueMessages("A+C2").WaitMessagesBooked()

		checkMarkers(t, testFramework, map[string]*markers.Markers{
			"A1":   markers.NewMarkers(markers.NewMarker(1, 1)),
			"B1":   markers.NewMarkers(markers.NewMarker(2, 1)),
			"C1":   markers.NewMarkers(markers.NewMarker(3, 1)),
			"D1":   markers.NewMarkers(markers.NewMarker(4, 1)),
			"A2":   markers.NewMarkers(markers.NewMarker(1, 2)),
			"A3":   markers.NewMarkers(markers.NewMarker(1, 3)),
			"A+C1": markers.NewMarkers(markers.NewMarker(5, 4)),
			"A+C2": markers.NewMarkers(markers.NewMarker(1, 3), markers.NewMarker(3, 1)),
		})
		checkMessageMetadataDiffBranchIDs(t, testFramework, map[string][]ledgerstate.BranchID{
			"A1":   {ledgerstate.UndefinedBranchID, ledgerstate.UndefinedBranchID},
			"B1":   {ledgerstate.UndefinedBranchID, ledgerstate.UndefinedBranchID},
			"C1":   {ledgerstate.UndefinedBranchID, ledgerstate.UndefinedBranchID},
			"D1":   {ledgerstate.UndefinedBranchID, ledgerstate.UndefinedBranchID},
			"A2":   {ledgerstate.UndefinedBranchID, ledgerstate.UndefinedBranchID},
			"A3":   {ledgerstate.UndefinedBranchID, ledgerstate.UndefinedBranchID},
			"A+C1": {ledgerstate.UndefinedBranchID, ledgerstate.UndefinedBranchID},
			"A+C2": {ledgerstate.UndefinedBranchID, ledgerstate.UndefinedBranchID},
		})
		checkBranchIDs(t, testFramework, map[string]ledgerstate.BranchIDs{
			"A1":   testFramework.BranchIDs("A"),
			"B1":   testFramework.BranchIDs("B"),
			"C1":   testFramework.BranchIDs("C"),
			"D1":   testFramework.BranchIDs("D"),
			"A2":   testFramework.BranchIDs("A"),
			"A3":   testFramework.BranchIDs("A"),
			"A+C1": testFramework.BranchIDs("A", "C"),
			"A+C2": testFramework.BranchIDs("A", "C"),
		})
	}

	// ISSUE A2*
	{

		testFramework.CreateMessage("A2*", WithStrongParents("A1"), WithInputs("A1"), WithOutput("A2*", 500))

		testFramework.RegisterBranchID("A2", "A2")
		testFramework.RegisterBranchID("A2*", "A2*")

		testFramework.RegisterBranchID("A2+C", "A2", "C1")

		testFramework.IssueMessages("A2*").WaitMessagesBooked()

		checkMarkers(t, testFramework, map[string]*markers.Markers{
			"A1":   markers.NewMarkers(markers.NewMarker(1, 1)),
			"B1":   markers.NewMarkers(markers.NewMarker(2, 1)),
			"C1":   markers.NewMarkers(markers.NewMarker(3, 1)),
			"D1":   markers.NewMarkers(markers.NewMarker(4, 1)),
			"A2":   markers.NewMarkers(markers.NewMarker(1, 2)),
			"A3":   markers.NewMarkers(markers.NewMarker(1, 3)),
			"A+C1": markers.NewMarkers(markers.NewMarker(5, 4)),
			"A+C2": markers.NewMarkers(markers.NewMarker(1, 3), markers.NewMarker(3, 1)),
			"A2*":  markers.NewMarkers(markers.NewMarker(6, 2)),
		})
		checkMessageMetadataDiffBranchIDs(t, testFramework, map[string][]ledgerstate.BranchID{
			"A1":   {ledgerstate.UndefinedBranchID, ledgerstate.UndefinedBranchID},
			"B1":   {ledgerstate.UndefinedBranchID, ledgerstate.UndefinedBranchID},
			"C1":   {ledgerstate.UndefinedBranchID, ledgerstate.UndefinedBranchID},
			"D1":   {ledgerstate.UndefinedBranchID, ledgerstate.UndefinedBranchID},
			"A2":   {ledgerstate.UndefinedBranchID, ledgerstate.UndefinedBranchID},
			"A3":   {ledgerstate.UndefinedBranchID, ledgerstate.UndefinedBranchID},
			"A+C1": {ledgerstate.UndefinedBranchID, ledgerstate.UndefinedBranchID},
			"A+C2": {ledgerstate.UndefinedBranchID, ledgerstate.UndefinedBranchID},
			"A2*":  {ledgerstate.UndefinedBranchID, ledgerstate.UndefinedBranchID},
		})
		checkBranchIDs(t, testFramework, map[string]ledgerstate.BranchIDs{
			"A1":   testFramework.BranchIDs("A"),
			"B1":   testFramework.BranchIDs("B"),
			"C1":   testFramework.BranchIDs("C"),
			"D1":   testFramework.BranchIDs("D"),
			"A2":   testFramework.BranchIDs("A", "A2"),
			"A3":   testFramework.BranchIDs("A", "A2"),
			"A+C1": testFramework.BranchIDs("A", "A2", "C"),
			"A+C2": testFramework.BranchIDs("A", "A2", "C"),
			"A2*":  testFramework.BranchIDs("A", "A2*"),
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
		checkMessageMetadataDiffBranchIDs(t, testFramework, map[string][]ledgerstate.BranchID{
			"Message1": {ledgerstate.UndefinedBranchID, ledgerstate.UndefinedBranchID},
		})
		checkBranchIDs(t, testFramework, map[string]ledgerstate.BranchIDs{
			"Message1": ledgerstate.NewBranchIDs(ledgerstate.MasterBranchID),
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
		checkMessageMetadataDiffBranchIDs(t, testFramework, map[string][]ledgerstate.BranchID{
			"Message1": {ledgerstate.UndefinedBranchID, ledgerstate.UndefinedBranchID},
			"Message2": {ledgerstate.UndefinedBranchID, ledgerstate.UndefinedBranchID},
		})
		checkBranchIDs(t, testFramework, map[string]ledgerstate.BranchIDs{
			"Message1": testFramework.BranchIDs("A"),
			"Message2": testFramework.BranchIDs("B"),
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
		checkMessageMetadataDiffBranchIDs(t, testFramework, map[string][]ledgerstate.BranchID{
			"Message1": {ledgerstate.UndefinedBranchID, ledgerstate.UndefinedBranchID},
			"Message2": {ledgerstate.UndefinedBranchID, ledgerstate.UndefinedBranchID},
			"Message3": {ledgerstate.UndefinedBranchID, ledgerstate.UndefinedBranchID},
		})
		checkBranchIDs(t, testFramework, map[string]ledgerstate.BranchIDs{
			"Message1": testFramework.BranchIDs("A"),
			"Message2": testFramework.BranchIDs("B"),
			"Message3": testFramework.BranchIDs("C"),
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
		checkMessageMetadataDiffBranchIDs(t, testFramework, map[string][]ledgerstate.BranchID{
			"Message1": {ledgerstate.UndefinedBranchID, ledgerstate.UndefinedBranchID},
			"Message2": {ledgerstate.UndefinedBranchID, ledgerstate.UndefinedBranchID},
			"Message3": {ledgerstate.UndefinedBranchID, ledgerstate.UndefinedBranchID},
			"Message4": {ledgerstate.UndefinedBranchID, ledgerstate.UndefinedBranchID},
		})
		checkBranchIDs(t, testFramework, map[string]ledgerstate.BranchIDs{
			"Message1": testFramework.BranchIDs("A"),
			"Message2": testFramework.BranchIDs("B"),
			"Message3": testFramework.BranchIDs("C"),
			"Message4": ledgerstate.NewBranchIDs(ledgerstate.MasterBranchID),
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
		checkMessageMetadataDiffBranchIDs(t, testFramework, map[string][]ledgerstate.BranchID{
			"Message1": {ledgerstate.UndefinedBranchID, ledgerstate.UndefinedBranchID},
			"Message2": {ledgerstate.UndefinedBranchID, ledgerstate.UndefinedBranchID},
			"Message3": {ledgerstate.UndefinedBranchID, ledgerstate.UndefinedBranchID},
			"Message4": {ledgerstate.UndefinedBranchID, ledgerstate.UndefinedBranchID},
			"Message5": {ledgerstate.UndefinedBranchID, ledgerstate.UndefinedBranchID},
		})
		checkBranchIDs(t, testFramework, map[string]ledgerstate.BranchIDs{
			"Message1": testFramework.BranchIDs("A"),
			"Message2": testFramework.BranchIDs("B"),
			"Message3": testFramework.BranchIDs("C"),
			"Message4": ledgerstate.NewBranchIDs(ledgerstate.MasterBranchID),
			"Message5": ledgerstate.NewBranchIDs(ledgerstate.MasterBranchID),
		})
	}

	// ISSUE Message6
	{
		testFramework.CreateMessage("Message6", WithStrongParents("Message1", "Message2"), WithShallowLikeParents("Message2"))
		testFramework.IssueMessages("Message6").WaitMessagesBooked()

		checkMarkers(t, testFramework, map[string]*markers.Markers{
			"Message1": markers.NewMarkers(markers.NewMarker(1, 1)),
			"Message2": markers.NewMarkers(markers.NewMarker(2, 1)),
			"Message3": markers.NewMarkers(markers.NewMarker(3, 1)),
			"Message4": markers.NewMarkers(markers.NewMarker(4, 1)),
			"Message5": markers.NewMarkers(markers.NewMarker(4, 2)),
			"Message6": markers.NewMarkers(markers.NewMarker(2, 2)),
		})
		checkMessageMetadataDiffBranchIDs(t, testFramework, map[string][]ledgerstate.BranchID{
			"Message1": {ledgerstate.UndefinedBranchID, ledgerstate.UndefinedBranchID},
			"Message2": {ledgerstate.UndefinedBranchID, ledgerstate.UndefinedBranchID},
			"Message3": {ledgerstate.UndefinedBranchID, ledgerstate.UndefinedBranchID},
			"Message4": {ledgerstate.UndefinedBranchID, ledgerstate.UndefinedBranchID},
			"Message5": {ledgerstate.UndefinedBranchID, ledgerstate.UndefinedBranchID},
			"Message6": {ledgerstate.UndefinedBranchID, ledgerstate.UndefinedBranchID},
		})
		checkBranchIDs(t, testFramework, map[string]ledgerstate.BranchIDs{
			"Message1": testFramework.BranchIDs("A"),
			"Message2": testFramework.BranchIDs("B"),
			"Message3": testFramework.BranchIDs("C"),
			"Message4": ledgerstate.NewBranchIDs(ledgerstate.MasterBranchID),
			"Message5": ledgerstate.NewBranchIDs(ledgerstate.MasterBranchID),
			"Message6": testFramework.BranchIDs("B"),
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
		checkMessageMetadataDiffBranchIDs(t, testFramework, map[string][]ledgerstate.BranchID{
			"Message1": {ledgerstate.UndefinedBranchID, ledgerstate.UndefinedBranchID},
			"Message2": {ledgerstate.UndefinedBranchID, ledgerstate.UndefinedBranchID},
			"Message3": {ledgerstate.UndefinedBranchID, ledgerstate.UndefinedBranchID},
			"Message4": {ledgerstate.UndefinedBranchID, ledgerstate.UndefinedBranchID},
			"Message5": {ledgerstate.UndefinedBranchID, ledgerstate.UndefinedBranchID},
			"Message6": {ledgerstate.UndefinedBranchID, ledgerstate.UndefinedBranchID},
			"Message7": {ledgerstate.UndefinedBranchID, ledgerstate.UndefinedBranchID},
		})
		checkBranchIDs(t, testFramework, map[string]ledgerstate.BranchIDs{
			"Message1": testFramework.BranchIDs("A"),
			"Message2": testFramework.BranchIDs("B"),
			"Message3": testFramework.BranchIDs("C"),
			"Message4": ledgerstate.NewBranchIDs(ledgerstate.MasterBranchID),
			"Message5": ledgerstate.NewBranchIDs(ledgerstate.MasterBranchID),
			"Message6": testFramework.BranchIDs("B"),
			"Message7": testFramework.BranchIDs("B"),
		})
	}

	// ISSUE Message8
	{
		testFramework.CreateMessage("Message8", WithStrongParents("Message5", "Message7", "Message3"), WithShallowLikeParents("Message1", "Message3"))

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
		checkMessageMetadataDiffBranchIDs(t, testFramework, map[string][]ledgerstate.BranchID{
			"Message1": {ledgerstate.UndefinedBranchID, ledgerstate.UndefinedBranchID},
			"Message2": {ledgerstate.UndefinedBranchID, ledgerstate.UndefinedBranchID},
			"Message3": {ledgerstate.UndefinedBranchID, ledgerstate.UndefinedBranchID},
			"Message4": {ledgerstate.UndefinedBranchID, ledgerstate.UndefinedBranchID},
			"Message5": {ledgerstate.UndefinedBranchID, ledgerstate.UndefinedBranchID},
			"Message6": {ledgerstate.UndefinedBranchID, ledgerstate.UndefinedBranchID},
			"Message7": {ledgerstate.UndefinedBranchID, ledgerstate.UndefinedBranchID},
			"Message8": {ledgerstate.UndefinedBranchID, ledgerstate.UndefinedBranchID},
		})
		checkBranchIDs(t, testFramework, map[string]ledgerstate.BranchIDs{
			"Message1": testFramework.BranchIDs("A"),
			"Message2": testFramework.BranchIDs("B"),
			"Message3": testFramework.BranchIDs("C"),
			"Message4": ledgerstate.NewBranchIDs(ledgerstate.MasterBranchID),
			"Message5": ledgerstate.NewBranchIDs(ledgerstate.MasterBranchID),
			"Message6": testFramework.BranchIDs("B"),
			"Message7": testFramework.BranchIDs("B"),
			"Message8": testFramework.BranchIDs("A", "C"),
		})
	}

	// ISSUE Message9
	{
		testFramework.CreateMessage("Message9", WithStrongParents("Message1", "Message7", "Message3"), WithShallowLikeParents("Message1", "Message3"), WithInputs("F"), WithOutput("N", 500))
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
		checkMessageMetadataDiffBranchIDs(t, testFramework, map[string][]ledgerstate.BranchID{
			"Message1": {ledgerstate.UndefinedBranchID, ledgerstate.UndefinedBranchID},
			"Message2": {ledgerstate.UndefinedBranchID, ledgerstate.UndefinedBranchID},
			"Message3": {ledgerstate.UndefinedBranchID, ledgerstate.UndefinedBranchID},
			"Message4": {ledgerstate.UndefinedBranchID, ledgerstate.UndefinedBranchID},
			"Message5": {ledgerstate.UndefinedBranchID, ledgerstate.UndefinedBranchID},
			"Message6": {ledgerstate.UndefinedBranchID, ledgerstate.UndefinedBranchID},
			"Message7": {ledgerstate.UndefinedBranchID, ledgerstate.UndefinedBranchID},
			"Message8": {ledgerstate.UndefinedBranchID, ledgerstate.UndefinedBranchID},
			"Message9": {testFramework.BranchID("A"), testFramework.BranchID("B")},
		})
		checkBranchIDs(t, testFramework, map[string]ledgerstate.BranchIDs{
			"Message1": testFramework.BranchIDs("A"),
			"Message2": testFramework.BranchIDs("B"),
			"Message3": testFramework.BranchIDs("C"),
			"Message4": ledgerstate.NewBranchIDs(ledgerstate.MasterBranchID),
			"Message5": ledgerstate.NewBranchIDs(ledgerstate.MasterBranchID),
			"Message6": testFramework.BranchIDs("B"),
			"Message7": testFramework.BranchIDs("B"),
			"Message8": testFramework.BranchIDs("A", "C"),
			"Message9": testFramework.BranchIDs("A", "C"),
		})
	}

	// ISSUE Message10
	{
		testFramework.CreateMessage("Message10", WithStrongParents("Message9"), WithShallowLikeParents("Message2"))
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
		checkMessageMetadataDiffBranchIDs(t, testFramework, map[string][]ledgerstate.BranchID{
			"Message1":  {ledgerstate.UndefinedBranchID, ledgerstate.UndefinedBranchID},
			"Message2":  {ledgerstate.UndefinedBranchID, ledgerstate.UndefinedBranchID},
			"Message3":  {ledgerstate.UndefinedBranchID, ledgerstate.UndefinedBranchID},
			"Message4":  {ledgerstate.UndefinedBranchID, ledgerstate.UndefinedBranchID},
			"Message5":  {ledgerstate.UndefinedBranchID, ledgerstate.UndefinedBranchID},
			"Message6":  {ledgerstate.UndefinedBranchID, ledgerstate.UndefinedBranchID},
			"Message7":  {ledgerstate.UndefinedBranchID, ledgerstate.UndefinedBranchID},
			"Message8":  {ledgerstate.UndefinedBranchID, ledgerstate.UndefinedBranchID},
			"Message9":  {testFramework.BranchID("A"), testFramework.BranchID("B")},
			"Message10": {ledgerstate.UndefinedBranchID, ledgerstate.UndefinedBranchID},
		})
		checkBranchIDs(t, testFramework, map[string]ledgerstate.BranchIDs{
			"Message1":  testFramework.BranchIDs("A"),
			"Message2":  testFramework.BranchIDs("B"),
			"Message3":  testFramework.BranchIDs("C"),
			"Message4":  ledgerstate.NewBranchIDs(ledgerstate.MasterBranchID),
			"Message5":  ledgerstate.NewBranchIDs(ledgerstate.MasterBranchID),
			"Message6":  testFramework.BranchIDs("B"),
			"Message7":  testFramework.BranchIDs("B"),
			"Message8":  testFramework.BranchIDs("A", "C"),
			"Message9":  testFramework.BranchIDs("A", "C"),
			"Message10": testFramework.BranchIDs("B"),
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
		checkMessageMetadataDiffBranchIDs(t, testFramework, map[string][]ledgerstate.BranchID{
			"Message1":    {ledgerstate.UndefinedBranchID, ledgerstate.UndefinedBranchID},
			"Message2":    {ledgerstate.UndefinedBranchID, ledgerstate.UndefinedBranchID},
			"Message3":    {ledgerstate.UndefinedBranchID, ledgerstate.UndefinedBranchID},
			"Message4":    {ledgerstate.UndefinedBranchID, ledgerstate.UndefinedBranchID},
			"Message5":    {ledgerstate.UndefinedBranchID, ledgerstate.UndefinedBranchID},
			"Message6":    {ledgerstate.UndefinedBranchID, ledgerstate.UndefinedBranchID},
			"Message7":    {ledgerstate.UndefinedBranchID, ledgerstate.UndefinedBranchID},
			"Message8":    {ledgerstate.UndefinedBranchID, ledgerstate.UndefinedBranchID},
			"Message9":    {testFramework.BranchID("A"), testFramework.BranchID("B")},
			"Message10":   {ledgerstate.UndefinedBranchID, ledgerstate.UndefinedBranchID},
			"Message11":   {ledgerstate.UndefinedBranchID, ledgerstate.UndefinedBranchID},
			"Message11.5": {testFramework.BranchID("A"), testFramework.BranchID("B")},
		})
		checkBranchIDs(t, testFramework, map[string]ledgerstate.BranchIDs{
			"Message1":    testFramework.BranchIDs("A"),
			"Message2":    testFramework.BranchIDs("B"),
			"Message3":    testFramework.BranchIDs("C"),
			"Message4":    ledgerstate.NewBranchIDs(ledgerstate.MasterBranchID),
			"Message5":    ledgerstate.NewBranchIDs(ledgerstate.MasterBranchID),
			"Message6":    testFramework.BranchIDs("B"),
			"Message7":    testFramework.BranchIDs("B"),
			"Message8":    testFramework.BranchIDs("A", "C"),
			"Message9":    testFramework.BranchIDs("A", "C"),
			"Message10":   testFramework.BranchIDs("B"),
			"Message11":   testFramework.BranchIDs("A", "C"),
			"Message11.5": testFramework.BranchIDs("A", "C"),
		})
	}

	// ISSUE Message12
	{
		testFramework.CreateMessage("Message12", WithStrongParents("Message1"), WithInputs("C"), WithOutput("H", 500))

		testFramework.RegisterBranchID("D", "Message5")
		testFramework.RegisterBranchID("E", "Message12")

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
		checkMessageMetadataDiffBranchIDs(t, testFramework, map[string][]ledgerstate.BranchID{
			"Message1":    {ledgerstate.UndefinedBranchID, ledgerstate.UndefinedBranchID},
			"Message2":    {ledgerstate.UndefinedBranchID, ledgerstate.UndefinedBranchID},
			"Message3":    {ledgerstate.UndefinedBranchID, ledgerstate.UndefinedBranchID},
			"Message4":    {ledgerstate.UndefinedBranchID, ledgerstate.UndefinedBranchID},
			"Message5":    {ledgerstate.UndefinedBranchID, ledgerstate.UndefinedBranchID},
			"Message6":    {ledgerstate.UndefinedBranchID, ledgerstate.UndefinedBranchID},
			"Message7":    {ledgerstate.UndefinedBranchID, ledgerstate.UndefinedBranchID},
			"Message8":    {ledgerstate.UndefinedBranchID, ledgerstate.UndefinedBranchID},
			"Message9":    {testFramework.BranchID("A"), testFramework.BranchID("B")},
			"Message10":   {ledgerstate.UndefinedBranchID, ledgerstate.UndefinedBranchID},
			"Message11":   {ledgerstate.UndefinedBranchID, ledgerstate.UndefinedBranchID},
			"Message11.5": {testFramework.BranchID("A"), testFramework.BranchID("B")},
			"Message12":   {ledgerstate.UndefinedBranchID, ledgerstate.UndefinedBranchID},
		})
		checkBranchIDs(t, testFramework, map[string]ledgerstate.BranchIDs{
			"Message1":    testFramework.BranchIDs("A"),
			"Message2":    testFramework.BranchIDs("B"),
			"Message3":    testFramework.BranchIDs("C"),
			"Message4":    ledgerstate.NewBranchIDs(ledgerstate.MasterBranchID),
			"Message5":    testFramework.BranchIDs("D"),
			"Message6":    testFramework.BranchIDs("B"),
			"Message7":    testFramework.BranchIDs("B", "D"),
			"Message8":    testFramework.BranchIDs("A", "C", "D"),
			"Message9":    testFramework.BranchIDs("A", "C", "D"),
			"Message10":   testFramework.BranchIDs("B", "D"),
			"Message11":   testFramework.BranchIDs("A", "C", "D"),
			"Message11.5": testFramework.BranchIDs("A", "C", "D"),
			"Message12":   testFramework.BranchIDs("A", "E"),
		})
	}

	// ISSUE Message13
	{
		testFramework.CreateMessage("Message13", WithStrongParents("Message9"), WithShallowLikeParents("Message2", "Message12"))

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
		checkMessageMetadataDiffBranchIDs(t, testFramework, map[string][]ledgerstate.BranchID{
			"Message1":    {ledgerstate.UndefinedBranchID, ledgerstate.UndefinedBranchID},
			"Message2":    {ledgerstate.UndefinedBranchID, ledgerstate.UndefinedBranchID},
			"Message3":    {ledgerstate.UndefinedBranchID, ledgerstate.UndefinedBranchID},
			"Message4":    {ledgerstate.UndefinedBranchID, ledgerstate.UndefinedBranchID},
			"Message5":    {ledgerstate.UndefinedBranchID, ledgerstate.UndefinedBranchID},
			"Message6":    {ledgerstate.UndefinedBranchID, ledgerstate.UndefinedBranchID},
			"Message7":    {ledgerstate.UndefinedBranchID, ledgerstate.UndefinedBranchID},
			"Message8":    {ledgerstate.UndefinedBranchID, ledgerstate.UndefinedBranchID},
			"Message9":    {testFramework.BranchID("A"), testFramework.BranchID("B")},
			"Message10":   {ledgerstate.UndefinedBranchID, ledgerstate.UndefinedBranchID},
			"Message11":   {ledgerstate.UndefinedBranchID, ledgerstate.UndefinedBranchID},
			"Message11.5": {testFramework.BranchID("A"), testFramework.BranchID("B")},
			"Message12":   {ledgerstate.UndefinedBranchID, ledgerstate.UndefinedBranchID},
			"Message13":   {ledgerstate.UndefinedBranchID, ledgerstate.UndefinedBranchID},
		})
		checkBranchIDs(t, testFramework, map[string]ledgerstate.BranchIDs{
			"Message1":    testFramework.BranchIDs("A"),
			"Message2":    testFramework.BranchIDs("B"),
			"Message3":    testFramework.BranchIDs("C"),
			"Message4":    ledgerstate.NewBranchIDs(ledgerstate.MasterBranchID),
			"Message5":    testFramework.BranchIDs("D"),
			"Message6":    testFramework.BranchIDs("B"),
			"Message7":    testFramework.BranchIDs("B", "D"),
			"Message8":    testFramework.BranchIDs("A", "C", "D"),
			"Message9":    testFramework.BranchIDs("A", "C", "D"),
			"Message10":   testFramework.BranchIDs("B", "D"),
			"Message11":   testFramework.BranchIDs("A", "C", "D"),
			"Message11.5": testFramework.BranchIDs("A", "C", "D"),
			"Message12":   testFramework.BranchIDs("A", "E"),
			"Message13":   testFramework.BranchIDs("B", "E"),
		})
	}

	// ISSUE Message13.1
	{
		testFramework.CreateMessage("Message13.1", WithStrongParents("Message9"), WithShallowLikeParents("Message2", "Message12"))

		testFramework.RegisterBranchID("C+D", "Message3", "Message5")

		testFramework.IssueMessages("Message13.1").WaitMessagesBooked()

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
			"Message13.1": markers.NewMarkers(markers.NewMarker(2, 3), markers.NewMarker(3, 1)),
		})
		checkMessageMetadataDiffBranchIDs(t, testFramework, map[string][]ledgerstate.BranchID{
			"Message1":    {ledgerstate.UndefinedBranchID, ledgerstate.UndefinedBranchID},
			"Message2":    {ledgerstate.UndefinedBranchID, ledgerstate.UndefinedBranchID},
			"Message3":    {ledgerstate.UndefinedBranchID, ledgerstate.UndefinedBranchID},
			"Message4":    {ledgerstate.UndefinedBranchID, ledgerstate.UndefinedBranchID},
			"Message5":    {ledgerstate.UndefinedBranchID, ledgerstate.UndefinedBranchID},
			"Message6":    {ledgerstate.UndefinedBranchID, ledgerstate.UndefinedBranchID},
			"Message7":    {ledgerstate.UndefinedBranchID, ledgerstate.UndefinedBranchID},
			"Message8":    {ledgerstate.UndefinedBranchID, ledgerstate.UndefinedBranchID},
			"Message9":    {testFramework.BranchID("A"), testFramework.BranchID("B")},
			"Message10":   {ledgerstate.UndefinedBranchID, ledgerstate.UndefinedBranchID},
			"Message11":   {ledgerstate.UndefinedBranchID, ledgerstate.UndefinedBranchID},
			"Message11.5": {testFramework.BranchID("A"), testFramework.BranchID("B")},
			"Message12":   {ledgerstate.UndefinedBranchID, ledgerstate.UndefinedBranchID},
			"Message13":   {ledgerstate.UndefinedBranchID, ledgerstate.UndefinedBranchID},
			"Message13.1": {testFramework.BranchID("E"), testFramework.BranchID("C+D")},
		})
		checkBranchIDs(t, testFramework, map[string]ledgerstate.BranchIDs{
			"Message1":    testFramework.BranchIDs("A"),
			"Message2":    testFramework.BranchIDs("B"),
			"Message3":    testFramework.BranchIDs("C"),
			"Message4":    ledgerstate.NewBranchIDs(ledgerstate.MasterBranchID),
			"Message5":    testFramework.BranchIDs("D"),
			"Message6":    testFramework.BranchIDs("B"),
			"Message7":    testFramework.BranchIDs("B", "D"),
			"Message8":    testFramework.BranchIDs("A", "C", "D"),
			"Message9":    testFramework.BranchIDs("A", "C", "D"),
			"Message10":   testFramework.BranchIDs("B", "D"),
			"Message11":   testFramework.BranchIDs("A", "C", "D"),
			"Message11.5": testFramework.BranchIDs("A", "C", "D"),
			"Message12":   testFramework.BranchIDs("A", "E"),
			"Message13":   testFramework.BranchIDs("B", "E"),
			"Message13.1": testFramework.BranchIDs("B", "E"),
		})
	}

	// ISSUE Message14
	{
		testFramework.CreateMessage("Message14", WithStrongParents("Message10"), WithShallowLikeParents("Message12"))
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
			"Message13.1": markers.NewMarkers(markers.NewMarker(2, 3), markers.NewMarker(3, 1)),
			"Message14":   markers.NewMarkers(markers.NewMarker(2, 4)),
		})
		checkMessageMetadataDiffBranchIDs(t, testFramework, map[string][]ledgerstate.BranchID{
			"Message1":    {ledgerstate.UndefinedBranchID, ledgerstate.UndefinedBranchID},
			"Message2":    {ledgerstate.UndefinedBranchID, ledgerstate.UndefinedBranchID},
			"Message3":    {ledgerstate.UndefinedBranchID, ledgerstate.UndefinedBranchID},
			"Message4":    {ledgerstate.UndefinedBranchID, ledgerstate.UndefinedBranchID},
			"Message5":    {ledgerstate.UndefinedBranchID, ledgerstate.UndefinedBranchID},
			"Message6":    {ledgerstate.UndefinedBranchID, ledgerstate.UndefinedBranchID},
			"Message7":    {ledgerstate.UndefinedBranchID, ledgerstate.UndefinedBranchID},
			"Message8":    {ledgerstate.UndefinedBranchID, ledgerstate.UndefinedBranchID},
			"Message9":    {testFramework.BranchID("A"), testFramework.BranchID("B")},
			"Message10":   {ledgerstate.UndefinedBranchID, ledgerstate.UndefinedBranchID},
			"Message11":   {ledgerstate.UndefinedBranchID, ledgerstate.UndefinedBranchID},
			"Message11.5": {testFramework.BranchID("A"), testFramework.BranchID("B")},
			"Message12":   {ledgerstate.UndefinedBranchID, ledgerstate.UndefinedBranchID},
			"Message13":   {ledgerstate.UndefinedBranchID, ledgerstate.UndefinedBranchID},
			"Message13.1": {testFramework.BranchID("E"), testFramework.BranchID("C+D")},
			"Message14":   {testFramework.BranchID("E"), testFramework.BranchID("D")},
		})
		checkBranchIDs(t, testFramework, map[string]ledgerstate.BranchIDs{
			"Message1":    testFramework.BranchIDs("A"),
			"Message2":    testFramework.BranchIDs("B"),
			"Message3":    testFramework.BranchIDs("C"),
			"Message4":    ledgerstate.NewBranchIDs(ledgerstate.MasterBranchID),
			"Message5":    testFramework.BranchIDs("D"),
			"Message6":    testFramework.BranchIDs("B"),
			"Message7":    testFramework.BranchIDs("B", "D"),
			"Message8":    testFramework.BranchIDs("A", "C", "D"),
			"Message9":    testFramework.BranchIDs("A", "C", "D"),
			"Message10":   testFramework.BranchIDs("B", "D"),
			"Message11":   testFramework.BranchIDs("A", "C", "D"),
			"Message11.5": testFramework.BranchIDs("A", "C", "D"),
			"Message12":   testFramework.BranchIDs("A", "E"),
			"Message13":   testFramework.BranchIDs("B", "E"),
			"Message13.1": testFramework.BranchIDs("B", "E"),
			"Message14":   testFramework.BranchIDs("B", "E"),
		})
	}

	// ISSUE Message15
	{
		testFramework.CreateMessage("Message15", WithStrongParents("Message9"), WithShallowDislikeParents("Message2", "Message5"))

		testFramework.RegisterBranchID("B+C+D", "Message2", "Message3", "Message5")

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
			"Message13.1": markers.NewMarkers(markers.NewMarker(2, 3), markers.NewMarker(3, 1)),
			"Message14":   markers.NewMarkers(markers.NewMarker(2, 4)),
			"Message15":   markers.NewMarkers(markers.NewMarker(2, 3), markers.NewMarker(3, 1)),
		})
		checkMessageMetadataDiffBranchIDs(t, testFramework, map[string][]ledgerstate.BranchID{
			"Message1":    {ledgerstate.UndefinedBranchID, ledgerstate.UndefinedBranchID},
			"Message2":    {ledgerstate.UndefinedBranchID, ledgerstate.UndefinedBranchID},
			"Message3":    {ledgerstate.UndefinedBranchID, ledgerstate.UndefinedBranchID},
			"Message4":    {ledgerstate.UndefinedBranchID, ledgerstate.UndefinedBranchID},
			"Message5":    {ledgerstate.UndefinedBranchID, ledgerstate.UndefinedBranchID},
			"Message6":    {ledgerstate.UndefinedBranchID, ledgerstate.UndefinedBranchID},
			"Message7":    {ledgerstate.UndefinedBranchID, ledgerstate.UndefinedBranchID},
			"Message8":    {ledgerstate.UndefinedBranchID, ledgerstate.UndefinedBranchID},
			"Message9":    {testFramework.BranchID("A"), testFramework.BranchID("B")},
			"Message10":   {ledgerstate.UndefinedBranchID, ledgerstate.UndefinedBranchID},
			"Message11":   {ledgerstate.UndefinedBranchID, ledgerstate.UndefinedBranchID},
			"Message11.5": {testFramework.BranchID("A"), testFramework.BranchID("B")},
			"Message12":   {ledgerstate.UndefinedBranchID, ledgerstate.UndefinedBranchID},
			"Message13":   {ledgerstate.UndefinedBranchID, ledgerstate.UndefinedBranchID},
			"Message13.1": {testFramework.BranchID("E"), testFramework.BranchID("C+D")},
			"Message14":   {testFramework.BranchID("E"), testFramework.BranchID("D")},
			"Message15":   {ledgerstate.UndefinedBranchID, testFramework.BranchID("B+C+D")},
		})
		checkBranchIDs(t, testFramework, map[string]ledgerstate.BranchIDs{
			"Message1":    testFramework.BranchIDs("A"),
			"Message2":    testFramework.BranchIDs("B"),
			"Message3":    testFramework.BranchIDs("C"),
			"Message4":    ledgerstate.NewBranchIDs(ledgerstate.MasterBranchID),
			"Message5":    testFramework.BranchIDs("D"),
			"Message6":    testFramework.BranchIDs("B"),
			"Message7":    testFramework.BranchIDs("B", "D"),
			"Message8":    testFramework.BranchIDs("A", "C", "D"),
			"Message9":    testFramework.BranchIDs("A", "C", "D"),
			"Message10":   testFramework.BranchIDs("B", "D"),
			"Message11":   testFramework.BranchIDs("A", "C", "D"),
			"Message11.5": testFramework.BranchIDs("A", "C", "D"),
			"Message12":   testFramework.BranchIDs("A", "E"),
			"Message13":   testFramework.BranchIDs("B", "E"),
			"Message13.1": testFramework.BranchIDs("B", "E"),
			"Message14":   testFramework.BranchIDs("B", "E"),
			"Message15":   ledgerstate.NewBranchIDs(ledgerstate.MasterBranchID),
		})
	}

	// ISSUE Message16
	{
		testFramework.CreateMessage("Message16", WithStrongParents("Message12"), WithInputs("H"), WithOutput("Z", 500))

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
			"Message13.1": markers.NewMarkers(markers.NewMarker(2, 3), markers.NewMarker(3, 1)),
			"Message14":   markers.NewMarkers(markers.NewMarker(2, 4)),
			"Message15":   markers.NewMarkers(markers.NewMarker(2, 3), markers.NewMarker(3, 1)),
			"Message16":   markers.NewMarkers(markers.NewMarker(6, 3)),
		})
		checkMessageMetadataDiffBranchIDs(t, testFramework, map[string][]ledgerstate.BranchID{
			"Message1":    {ledgerstate.UndefinedBranchID, ledgerstate.UndefinedBranchID},
			"Message2":    {ledgerstate.UndefinedBranchID, ledgerstate.UndefinedBranchID},
			"Message3":    {ledgerstate.UndefinedBranchID, ledgerstate.UndefinedBranchID},
			"Message4":    {ledgerstate.UndefinedBranchID, ledgerstate.UndefinedBranchID},
			"Message5":    {ledgerstate.UndefinedBranchID, ledgerstate.UndefinedBranchID},
			"Message6":    {ledgerstate.UndefinedBranchID, ledgerstate.UndefinedBranchID},
			"Message7":    {ledgerstate.UndefinedBranchID, ledgerstate.UndefinedBranchID},
			"Message8":    {ledgerstate.UndefinedBranchID, ledgerstate.UndefinedBranchID},
			"Message9":    {testFramework.BranchID("A"), testFramework.BranchID("B")},
			"Message10":   {ledgerstate.UndefinedBranchID, ledgerstate.UndefinedBranchID},
			"Message11":   {ledgerstate.UndefinedBranchID, ledgerstate.UndefinedBranchID},
			"Message11.5": {testFramework.BranchID("A"), testFramework.BranchID("B")},
			"Message12":   {ledgerstate.UndefinedBranchID, ledgerstate.UndefinedBranchID},
			"Message13":   {ledgerstate.UndefinedBranchID, ledgerstate.UndefinedBranchID},
			"Message13.1": {testFramework.BranchID("E"), testFramework.BranchID("C+D")},
			"Message14":   {testFramework.BranchID("E"), testFramework.BranchID("D")},
			"Message15":   {ledgerstate.UndefinedBranchID, testFramework.BranchID("B+C+D")},
			"Message16":   {ledgerstate.UndefinedBranchID, ledgerstate.UndefinedBranchID},
		})
		checkBranchIDs(t, testFramework, map[string]ledgerstate.BranchIDs{
			"Message1":    testFramework.BranchIDs("A"),
			"Message2":    testFramework.BranchIDs("B"),
			"Message3":    testFramework.BranchIDs("C"),
			"Message4":    ledgerstate.NewBranchIDs(ledgerstate.MasterBranchID),
			"Message5":    testFramework.BranchIDs("D"),
			"Message6":    testFramework.BranchIDs("B"),
			"Message7":    testFramework.BranchIDs("B", "D"),
			"Message8":    testFramework.BranchIDs("A", "C", "D"),
			"Message9":    testFramework.BranchIDs("A", "C", "D"),
			"Message10":   testFramework.BranchIDs("B", "D"),
			"Message11":   testFramework.BranchIDs("A", "C", "D"),
			"Message11.5": testFramework.BranchIDs("A", "C", "D"),
			"Message12":   testFramework.BranchIDs("A", "E"),
			"Message13":   testFramework.BranchIDs("B", "E"),
			"Message13.1": testFramework.BranchIDs("B", "E"),
			"Message14":   testFramework.BranchIDs("B", "E"),
			"Message15":   ledgerstate.NewBranchIDs(ledgerstate.MasterBranchID),
			"Message16":   testFramework.BranchIDs("A", "E"),
		})
	}

	// ISSUE Message17
	{
		testFramework.CreateMessage("Message17", WithStrongParents("Message12"), WithInputs("H"), WithOutput("Y", 500))

		testFramework.RegisterBranchID("Z", "Message16")
		testFramework.RegisterBranchID("Y", "Message17")

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
			"Message13.1": markers.NewMarkers(markers.NewMarker(2, 3), markers.NewMarker(3, 1)),
			"Message14":   markers.NewMarkers(markers.NewMarker(2, 4)),
			"Message15":   markers.NewMarkers(markers.NewMarker(2, 3), markers.NewMarker(3, 1)),
			"Message16":   markers.NewMarkers(markers.NewMarker(6, 3)),
			"Message17":   markers.NewMarkers(markers.NewMarker(8, 3)),
		})
		checkMessageMetadataDiffBranchIDs(t, testFramework, map[string][]ledgerstate.BranchID{
			"Message1":    {ledgerstate.UndefinedBranchID, ledgerstate.UndefinedBranchID},
			"Message2":    {ledgerstate.UndefinedBranchID, ledgerstate.UndefinedBranchID},
			"Message3":    {ledgerstate.UndefinedBranchID, ledgerstate.UndefinedBranchID},
			"Message4":    {ledgerstate.UndefinedBranchID, ledgerstate.UndefinedBranchID},
			"Message5":    {ledgerstate.UndefinedBranchID, ledgerstate.UndefinedBranchID},
			"Message6":    {ledgerstate.UndefinedBranchID, ledgerstate.UndefinedBranchID},
			"Message7":    {ledgerstate.UndefinedBranchID, ledgerstate.UndefinedBranchID},
			"Message8":    {ledgerstate.UndefinedBranchID, ledgerstate.UndefinedBranchID},
			"Message9":    {testFramework.BranchID("A"), testFramework.BranchID("B")},
			"Message10":   {ledgerstate.UndefinedBranchID, ledgerstate.UndefinedBranchID},
			"Message11":   {ledgerstate.UndefinedBranchID, ledgerstate.UndefinedBranchID},
			"Message11.5": {testFramework.BranchID("A"), testFramework.BranchID("B")},
			"Message12":   {ledgerstate.UndefinedBranchID, ledgerstate.UndefinedBranchID},
			"Message13":   {ledgerstate.UndefinedBranchID, ledgerstate.UndefinedBranchID},
			"Message13.1": {testFramework.BranchID("E"), testFramework.BranchID("C+D")},
			"Message14":   {testFramework.BranchID("E"), testFramework.BranchID("D")},
			"Message15":   {ledgerstate.UndefinedBranchID, testFramework.BranchID("B+C+D")},
			"Message16":   {ledgerstate.UndefinedBranchID, ledgerstate.UndefinedBranchID},
			"Message17":   {ledgerstate.UndefinedBranchID, ledgerstate.UndefinedBranchID},
		})
		checkBranchIDs(t, testFramework, map[string]ledgerstate.BranchIDs{
			"Message1":    testFramework.BranchIDs("A"),
			"Message2":    testFramework.BranchIDs("B"),
			"Message3":    testFramework.BranchIDs("C"),
			"Message4":    ledgerstate.NewBranchIDs(ledgerstate.MasterBranchID),
			"Message5":    testFramework.BranchIDs("D"),
			"Message6":    testFramework.BranchIDs("B"),
			"Message7":    testFramework.BranchIDs("B", "D"),
			"Message8":    testFramework.BranchIDs("A", "C", "D"),
			"Message9":    testFramework.BranchIDs("A", "C", "D"),
			"Message10":   testFramework.BranchIDs("B", "D"),
			"Message11":   testFramework.BranchIDs("A", "C", "D"),
			"Message11.5": testFramework.BranchIDs("A", "C", "D"),
			"Message12":   testFramework.BranchIDs("A", "E"),
			"Message13":   testFramework.BranchIDs("B", "E"),
			"Message13.1": testFramework.BranchIDs("B", "E"),
			"Message14":   testFramework.BranchIDs("B", "E"),
			"Message15":   ledgerstate.NewBranchIDs(ledgerstate.MasterBranchID),
			"Message16":   testFramework.BranchIDs("Z", "A", "E"),
			"Message17":   testFramework.BranchIDs("Y", "A", "E"),
		})
	}

	// ISSUE Message18
	{
		msg := testFramework.CreateMessage("Message18", WithStrongParents("Message17", "Message7"))

		testFramework.IssueMessages("Message18").WaitMessagesBooked()

		tangle.Storage.MessageMetadata(msg.ID()).Consume(func(messageMetadata *MessageMetadata) {
			assert.True(t, messageMetadata.subjectivelyInvalid)
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
			"Message13.1": markers.NewMarkers(markers.NewMarker(2, 3), markers.NewMarker(3, 1)),
			"Message14":   markers.NewMarkers(markers.NewMarker(2, 4)),
			"Message15":   markers.NewMarkers(markers.NewMarker(2, 3), markers.NewMarker(3, 1)),
			"Message16":   markers.NewMarkers(markers.NewMarker(6, 3)),
			"Message17":   markers.NewMarkers(markers.NewMarker(8, 3)),
			"Message18":   markers.NewMarkers(markers.NewMarker(9, 4)),
		})
		checkMessageMetadataDiffBranchIDs(t, testFramework, map[string][]ledgerstate.BranchID{
			"Message1":    {ledgerstate.UndefinedBranchID, ledgerstate.UndefinedBranchID},
			"Message2":    {ledgerstate.UndefinedBranchID, ledgerstate.UndefinedBranchID},
			"Message3":    {ledgerstate.UndefinedBranchID, ledgerstate.UndefinedBranchID},
			"Message4":    {ledgerstate.UndefinedBranchID, ledgerstate.UndefinedBranchID},
			"Message5":    {ledgerstate.UndefinedBranchID, ledgerstate.UndefinedBranchID},
			"Message6":    {ledgerstate.UndefinedBranchID, ledgerstate.UndefinedBranchID},
			"Message7":    {ledgerstate.UndefinedBranchID, ledgerstate.UndefinedBranchID},
			"Message8":    {ledgerstate.UndefinedBranchID, ledgerstate.UndefinedBranchID},
			"Message9":    {testFramework.BranchID("A"), testFramework.BranchID("B")},
			"Message10":   {ledgerstate.UndefinedBranchID, ledgerstate.UndefinedBranchID},
			"Message11":   {ledgerstate.UndefinedBranchID, ledgerstate.UndefinedBranchID},
			"Message11.5": {testFramework.BranchID("A"), testFramework.BranchID("B")},
			"Message12":   {ledgerstate.UndefinedBranchID, ledgerstate.UndefinedBranchID},
			"Message13":   {ledgerstate.UndefinedBranchID, ledgerstate.UndefinedBranchID},
			"Message13.1": {testFramework.BranchID("E"), testFramework.BranchID("C+D")},
			"Message14":   {testFramework.BranchID("E"), testFramework.BranchID("D")},
			"Message15":   {ledgerstate.UndefinedBranchID, testFramework.BranchID("B+C+D")},
			"Message16":   {ledgerstate.UndefinedBranchID, ledgerstate.UndefinedBranchID},
			"Message17":   {ledgerstate.UndefinedBranchID, ledgerstate.UndefinedBranchID},
			"Message18":   {ledgerstate.UndefinedBranchID, ledgerstate.UndefinedBranchID},
		})
		checkBranchIDs(t, testFramework, map[string]ledgerstate.BranchIDs{
			"Message1":    testFramework.BranchIDs("A"),
			"Message2":    testFramework.BranchIDs("B"),
			"Message3":    testFramework.BranchIDs("C"),
			"Message4":    ledgerstate.NewBranchIDs(ledgerstate.MasterBranchID),
			"Message5":    testFramework.BranchIDs("D"),
			"Message6":    testFramework.BranchIDs("B"),
			"Message7":    testFramework.BranchIDs("B", "D"),
			"Message8":    testFramework.BranchIDs("A", "C", "D"),
			"Message9":    testFramework.BranchIDs("A", "C", "D"),
			"Message10":   testFramework.BranchIDs("B", "D"),
			"Message11":   testFramework.BranchIDs("A", "C", "D"),
			"Message11.5": testFramework.BranchIDs("A", "C", "D"),
			"Message12":   testFramework.BranchIDs("A", "E"),
			"Message13":   testFramework.BranchIDs("B", "E"),
			"Message13.1": testFramework.BranchIDs("B", "E"),
			"Message14":   testFramework.BranchIDs("B", "E"),
			"Message15":   ledgerstate.NewBranchIDs(ledgerstate.MasterBranchID),
			"Message16":   testFramework.BranchIDs("Z", "A", "E"),
			"Message17":   testFramework.BranchIDs("Y", "A", "E"),
			"Message18":   testFramework.BranchIDs("Y", "A", "E", "B", "D"),
		})
	}

	// ISSUE Message19
	{
		msg := testFramework.CreateMessage("Message19", WithStrongParents("Message17"), WithWeakParents("Message7"))

		testFramework.IssueMessages("Message19").WaitMessagesBooked()

		tangle.Storage.MessageMetadata(msg.ID()).Consume(func(messageMetadata *MessageMetadata) {
			assert.False(t, messageMetadata.subjectivelyInvalid)
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
			"Message13.1": markers.NewMarkers(markers.NewMarker(2, 3), markers.NewMarker(3, 1)),
			"Message14":   markers.NewMarkers(markers.NewMarker(2, 4)),
			"Message15":   markers.NewMarkers(markers.NewMarker(2, 3), markers.NewMarker(3, 1)),
			"Message16":   markers.NewMarkers(markers.NewMarker(6, 3)),
			"Message17":   markers.NewMarkers(markers.NewMarker(8, 3)),
			"Message18":   markers.NewMarkers(markers.NewMarker(9, 4)),
			"Message19":   markers.NewMarkers(markers.NewMarker(8, 4)),
		})
		checkMessageMetadataDiffBranchIDs(t, testFramework, map[string][]ledgerstate.BranchID{
			"Message1":    {ledgerstate.UndefinedBranchID, ledgerstate.UndefinedBranchID},
			"Message2":    {ledgerstate.UndefinedBranchID, ledgerstate.UndefinedBranchID},
			"Message3":    {ledgerstate.UndefinedBranchID, ledgerstate.UndefinedBranchID},
			"Message4":    {ledgerstate.UndefinedBranchID, ledgerstate.UndefinedBranchID},
			"Message5":    {ledgerstate.UndefinedBranchID, ledgerstate.UndefinedBranchID},
			"Message6":    {ledgerstate.UndefinedBranchID, ledgerstate.UndefinedBranchID},
			"Message7":    {ledgerstate.UndefinedBranchID, ledgerstate.UndefinedBranchID},
			"Message8":    {ledgerstate.UndefinedBranchID, ledgerstate.UndefinedBranchID},
			"Message9":    {testFramework.BranchID("A"), testFramework.BranchID("B")},
			"Message10":   {ledgerstate.UndefinedBranchID, ledgerstate.UndefinedBranchID},
			"Message11":   {ledgerstate.UndefinedBranchID, ledgerstate.UndefinedBranchID},
			"Message11.5": {testFramework.BranchID("A"), testFramework.BranchID("B")},
			"Message12":   {ledgerstate.UndefinedBranchID, ledgerstate.UndefinedBranchID},
			"Message13":   {ledgerstate.UndefinedBranchID, ledgerstate.UndefinedBranchID},
			"Message13.1": {testFramework.BranchID("E"), testFramework.BranchID("C+D")},
			"Message14":   {testFramework.BranchID("E"), testFramework.BranchID("D")},
			"Message15":   {ledgerstate.UndefinedBranchID, testFramework.BranchID("B+C+D")},
			"Message16":   {ledgerstate.UndefinedBranchID, ledgerstate.UndefinedBranchID},
			"Message17":   {ledgerstate.UndefinedBranchID, ledgerstate.UndefinedBranchID},
			"Message18":   {ledgerstate.UndefinedBranchID, ledgerstate.UndefinedBranchID},
			"Message19":   {ledgerstate.UndefinedBranchID, ledgerstate.UndefinedBranchID},
		})
		checkBranchIDs(t, testFramework, map[string]ledgerstate.BranchIDs{
			"Message1":    testFramework.BranchIDs("A"),
			"Message2":    testFramework.BranchIDs("B"),
			"Message3":    testFramework.BranchIDs("C"),
			"Message4":    ledgerstate.NewBranchIDs(ledgerstate.MasterBranchID),
			"Message5":    testFramework.BranchIDs("D"),
			"Message6":    testFramework.BranchIDs("B"),
			"Message7":    testFramework.BranchIDs("B", "D"),
			"Message8":    testFramework.BranchIDs("A", "C", "D"),
			"Message9":    testFramework.BranchIDs("A", "C", "D"),
			"Message10":   testFramework.BranchIDs("B", "D"),
			"Message11":   testFramework.BranchIDs("A", "C", "D"),
			"Message11.5": testFramework.BranchIDs("A", "C", "D"),
			"Message12":   testFramework.BranchIDs("A", "E"),
			"Message13":   testFramework.BranchIDs("B", "E"),
			"Message13.1": testFramework.BranchIDs("B", "E"),
			"Message14":   testFramework.BranchIDs("B", "E"),
			"Message15":   ledgerstate.NewBranchIDs(ledgerstate.MasterBranchID),
			"Message16":   testFramework.BranchIDs("Z", "A", "E"),
			"Message17":   testFramework.BranchIDs("Y", "A", "E"),
			"Message18":   testFramework.BranchIDs("Y", "A", "E", "B", "D"),
			"Message19":   testFramework.BranchIDs("Y", "A", "E"),
		})
	}

	// ISSUE Message20
	{
		msg := testFramework.CreateMessage("Message20", WithStrongParents("Message17"), WithWeakParents("Message2"))

		testFramework.IssueMessages("Message20").WaitMessagesBooked()

		tangle.Storage.MessageMetadata(msg.ID()).Consume(func(messageMetadata *MessageMetadata) {
			assert.True(t, messageMetadata.subjectivelyInvalid)
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
			"Message13.1": markers.NewMarkers(markers.NewMarker(2, 3), markers.NewMarker(3, 1)),
			"Message14":   markers.NewMarkers(markers.NewMarker(2, 4)),
			"Message15":   markers.NewMarkers(markers.NewMarker(2, 3), markers.NewMarker(3, 1)),
			"Message16":   markers.NewMarkers(markers.NewMarker(6, 3)),
			"Message17":   markers.NewMarkers(markers.NewMarker(8, 3)),
			"Message18":   markers.NewMarkers(markers.NewMarker(9, 4)),
			"Message19":   markers.NewMarkers(markers.NewMarker(8, 4)),
			"Message20":   markers.NewMarkers(markers.NewMarker(10, 4)),
		})
		checkMessageMetadataDiffBranchIDs(t, testFramework, map[string][]ledgerstate.BranchID{
			"Message1":    {ledgerstate.UndefinedBranchID, ledgerstate.UndefinedBranchID},
			"Message2":    {ledgerstate.UndefinedBranchID, ledgerstate.UndefinedBranchID},
			"Message3":    {ledgerstate.UndefinedBranchID, ledgerstate.UndefinedBranchID},
			"Message4":    {ledgerstate.UndefinedBranchID, ledgerstate.UndefinedBranchID},
			"Message5":    {ledgerstate.UndefinedBranchID, ledgerstate.UndefinedBranchID},
			"Message6":    {ledgerstate.UndefinedBranchID, ledgerstate.UndefinedBranchID},
			"Message7":    {ledgerstate.UndefinedBranchID, ledgerstate.UndefinedBranchID},
			"Message8":    {ledgerstate.UndefinedBranchID, ledgerstate.UndefinedBranchID},
			"Message9":    {testFramework.BranchID("A"), testFramework.BranchID("B")},
			"Message10":   {ledgerstate.UndefinedBranchID, ledgerstate.UndefinedBranchID},
			"Message11":   {ledgerstate.UndefinedBranchID, ledgerstate.UndefinedBranchID},
			"Message11.5": {testFramework.BranchID("A"), testFramework.BranchID("B")},
			"Message12":   {ledgerstate.UndefinedBranchID, ledgerstate.UndefinedBranchID},
			"Message13":   {ledgerstate.UndefinedBranchID, ledgerstate.UndefinedBranchID},
			"Message13.1": {testFramework.BranchID("E"), testFramework.BranchID("C+D")},
			"Message14":   {testFramework.BranchID("E"), testFramework.BranchID("D")},
			"Message15":   {ledgerstate.UndefinedBranchID, testFramework.BranchID("B+C+D")},
			"Message16":   {ledgerstate.UndefinedBranchID, ledgerstate.UndefinedBranchID},
			"Message17":   {ledgerstate.UndefinedBranchID, ledgerstate.UndefinedBranchID},
			"Message18":   {ledgerstate.UndefinedBranchID, ledgerstate.UndefinedBranchID},
			"Message19":   {ledgerstate.UndefinedBranchID, ledgerstate.UndefinedBranchID},
			"Message20":   {ledgerstate.UndefinedBranchID, ledgerstate.UndefinedBranchID},
		})
		checkBranchIDs(t, testFramework, map[string]ledgerstate.BranchIDs{
			"Message1":    testFramework.BranchIDs("A"),
			"Message2":    testFramework.BranchIDs("B"),
			"Message3":    testFramework.BranchIDs("C"),
			"Message4":    ledgerstate.NewBranchIDs(ledgerstate.MasterBranchID),
			"Message5":    testFramework.BranchIDs("D"),
			"Message6":    testFramework.BranchIDs("B"),
			"Message7":    testFramework.BranchIDs("B", "D"),
			"Message8":    testFramework.BranchIDs("A", "C", "D"),
			"Message9":    testFramework.BranchIDs("A", "C", "D"),
			"Message10":   testFramework.BranchIDs("B", "D"),
			"Message11":   testFramework.BranchIDs("A", "C", "D"),
			"Message11.5": testFramework.BranchIDs("A", "C", "D"),
			"Message12":   testFramework.BranchIDs("A", "E"),
			"Message13":   testFramework.BranchIDs("B", "E"),
			"Message13.1": testFramework.BranchIDs("B", "E"),
			"Message14":   testFramework.BranchIDs("B", "E"),
			"Message15":   ledgerstate.NewBranchIDs(ledgerstate.MasterBranchID),
			"Message16":   testFramework.BranchIDs("Z", "A", "E"),
			"Message17":   testFramework.BranchIDs("Y", "A", "E"),
			"Message18":   testFramework.BranchIDs("Y", "A", "E", "B", "D"),
			"Message19":   testFramework.BranchIDs("Y", "A", "E"),
			"Message20":   testFramework.BranchIDs("Y", "A", "E", "B"),
		})
	}

	// ISSUE Message21
	{
		msg := testFramework.CreateMessage("Message21", WithStrongParents("Message17"), WithWeakParents("Message2"), WithShallowDislikeParents("Message12"))

		testFramework.IssueMessages("Message21").WaitMessagesBooked()

		tangle.Storage.MessageMetadata(msg.ID()).Consume(func(messageMetadata *MessageMetadata) {
			assert.True(t, messageMetadata.subjectivelyInvalid)
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
			"Message13.1": markers.NewMarkers(markers.NewMarker(2, 3), markers.NewMarker(3, 1)),
			"Message14":   markers.NewMarkers(markers.NewMarker(2, 4)),
			"Message15":   markers.NewMarkers(markers.NewMarker(2, 3), markers.NewMarker(3, 1)),
			"Message16":   markers.NewMarkers(markers.NewMarker(6, 3)),
			"Message17":   markers.NewMarkers(markers.NewMarker(8, 3)),
			"Message18":   markers.NewMarkers(markers.NewMarker(9, 4)),
			"Message19":   markers.NewMarkers(markers.NewMarker(8, 4)),
			"Message20":   markers.NewMarkers(markers.NewMarker(10, 4)),
			"Message21":   markers.NewMarkers(markers.NewMarker(11, 4)),
		})
		checkMessageMetadataDiffBranchIDs(t, testFramework, map[string][]ledgerstate.BranchID{
			"Message1":    {ledgerstate.UndefinedBranchID, ledgerstate.UndefinedBranchID},
			"Message2":    {ledgerstate.UndefinedBranchID, ledgerstate.UndefinedBranchID},
			"Message3":    {ledgerstate.UndefinedBranchID, ledgerstate.UndefinedBranchID},
			"Message4":    {ledgerstate.UndefinedBranchID, ledgerstate.UndefinedBranchID},
			"Message5":    {ledgerstate.UndefinedBranchID, ledgerstate.UndefinedBranchID},
			"Message6":    {ledgerstate.UndefinedBranchID, ledgerstate.UndefinedBranchID},
			"Message7":    {ledgerstate.UndefinedBranchID, ledgerstate.UndefinedBranchID},
			"Message8":    {ledgerstate.UndefinedBranchID, ledgerstate.UndefinedBranchID},
			"Message9":    {testFramework.BranchID("A"), testFramework.BranchID("B")},
			"Message10":   {ledgerstate.UndefinedBranchID, ledgerstate.UndefinedBranchID},
			"Message11":   {ledgerstate.UndefinedBranchID, ledgerstate.UndefinedBranchID},
			"Message11.5": {testFramework.BranchID("A"), testFramework.BranchID("B")},
			"Message12":   {ledgerstate.UndefinedBranchID, ledgerstate.UndefinedBranchID},
			"Message13":   {ledgerstate.UndefinedBranchID, ledgerstate.UndefinedBranchID},
			"Message13.1": {testFramework.BranchID("E"), testFramework.BranchID("C+D")},
			"Message14":   {testFramework.BranchID("E"), testFramework.BranchID("D")},
			"Message15":   {ledgerstate.UndefinedBranchID, testFramework.BranchID("B+C+D")},
			"Message16":   {ledgerstate.UndefinedBranchID, ledgerstate.UndefinedBranchID},
			"Message17":   {ledgerstate.UndefinedBranchID, ledgerstate.UndefinedBranchID},
			"Message18":   {ledgerstate.UndefinedBranchID, ledgerstate.UndefinedBranchID},
			"Message19":   {ledgerstate.UndefinedBranchID, ledgerstate.UndefinedBranchID},
			"Message20":   {ledgerstate.UndefinedBranchID, ledgerstate.UndefinedBranchID},
			"Message21":   {ledgerstate.UndefinedBranchID, ledgerstate.UndefinedBranchID},
		})
		checkBranchIDs(t, testFramework, map[string]ledgerstate.BranchIDs{
			"Message1":    testFramework.BranchIDs("A"),
			"Message2":    testFramework.BranchIDs("B"),
			"Message3":    testFramework.BranchIDs("C"),
			"Message4":    ledgerstate.NewBranchIDs(ledgerstate.MasterBranchID),
			"Message5":    testFramework.BranchIDs("D"),
			"Message6":    testFramework.BranchIDs("B"),
			"Message7":    testFramework.BranchIDs("B", "D"),
			"Message8":    testFramework.BranchIDs("A", "C", "D"),
			"Message9":    testFramework.BranchIDs("A", "C", "D"),
			"Message10":   testFramework.BranchIDs("B", "D"),
			"Message11":   testFramework.BranchIDs("A", "C", "D"),
			"Message11.5": testFramework.BranchIDs("A", "C", "D"),
			"Message12":   testFramework.BranchIDs("A", "E"),
			"Message13":   testFramework.BranchIDs("B", "E"),
			"Message13.1": testFramework.BranchIDs("B", "E"),
			"Message14":   testFramework.BranchIDs("B", "E"),
			"Message15":   ledgerstate.NewBranchIDs(ledgerstate.MasterBranchID),
			"Message16":   testFramework.BranchIDs("Z", "A", "E"),
			"Message17":   testFramework.BranchIDs("Y", "A", "E"),
			"Message18":   testFramework.BranchIDs("Y", "A", "E", "B", "D"),
			"Message19":   testFramework.BranchIDs("Y", "A", "E"),
			"Message20":   testFramework.BranchIDs("Y", "A", "E", "B"),
			"Message21":   testFramework.BranchIDs("Y", "A", "B"),
		})
	}

	// ISSUE Message22
	{
		msg := testFramework.CreateMessage("Message22", WithStrongParents("Message17"), WithWeakParents("Message2"), WithShallowDislikeParents("Message12", "Message1"))

		testFramework.IssueMessages("Message22").WaitMessagesBooked()

		tangle.Storage.MessageMetadata(msg.ID()).Consume(func(messageMetadata *MessageMetadata) {
			assert.False(t, messageMetadata.subjectivelyInvalid)
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
			"Message13.1": markers.NewMarkers(markers.NewMarker(2, 3), markers.NewMarker(3, 1)),
			"Message14":   markers.NewMarkers(markers.NewMarker(2, 4)),
			"Message15":   markers.NewMarkers(markers.NewMarker(2, 3), markers.NewMarker(3, 1)),
			"Message16":   markers.NewMarkers(markers.NewMarker(6, 3)),
			"Message17":   markers.NewMarkers(markers.NewMarker(8, 3)),
			"Message18":   markers.NewMarkers(markers.NewMarker(9, 4)),
			"Message19":   markers.NewMarkers(markers.NewMarker(8, 4)),
			"Message20":   markers.NewMarkers(markers.NewMarker(10, 4)),
			"Message21":   markers.NewMarkers(markers.NewMarker(11, 4)),
			"Message22":   markers.NewMarkers(markers.NewMarker(12, 4)),
		})
		checkMessageMetadataDiffBranchIDs(t, testFramework, map[string][]ledgerstate.BranchID{
			"Message1":    {ledgerstate.UndefinedBranchID, ledgerstate.UndefinedBranchID},
			"Message2":    {ledgerstate.UndefinedBranchID, ledgerstate.UndefinedBranchID},
			"Message3":    {ledgerstate.UndefinedBranchID, ledgerstate.UndefinedBranchID},
			"Message4":    {ledgerstate.UndefinedBranchID, ledgerstate.UndefinedBranchID},
			"Message5":    {ledgerstate.UndefinedBranchID, ledgerstate.UndefinedBranchID},
			"Message6":    {ledgerstate.UndefinedBranchID, ledgerstate.UndefinedBranchID},
			"Message7":    {ledgerstate.UndefinedBranchID, ledgerstate.UndefinedBranchID},
			"Message8":    {ledgerstate.UndefinedBranchID, ledgerstate.UndefinedBranchID},
			"Message9":    {testFramework.BranchID("A"), testFramework.BranchID("B")},
			"Message10":   {ledgerstate.UndefinedBranchID, ledgerstate.UndefinedBranchID},
			"Message11":   {ledgerstate.UndefinedBranchID, ledgerstate.UndefinedBranchID},
			"Message11.5": {testFramework.BranchID("A"), testFramework.BranchID("B")},
			"Message12":   {ledgerstate.UndefinedBranchID, ledgerstate.UndefinedBranchID},
			"Message13":   {ledgerstate.UndefinedBranchID, ledgerstate.UndefinedBranchID},
			"Message13.1": {testFramework.BranchID("E"), testFramework.BranchID("C+D")},
			"Message14":   {testFramework.BranchID("E"), testFramework.BranchID("D")},
			"Message15":   {ledgerstate.UndefinedBranchID, testFramework.BranchID("B+C+D")},
			"Message16":   {ledgerstate.UndefinedBranchID, ledgerstate.UndefinedBranchID},
			"Message17":   {ledgerstate.UndefinedBranchID, ledgerstate.UndefinedBranchID},
			"Message18":   {ledgerstate.UndefinedBranchID, ledgerstate.UndefinedBranchID},
			"Message19":   {ledgerstate.UndefinedBranchID, ledgerstate.UndefinedBranchID},
			"Message20":   {ledgerstate.UndefinedBranchID, ledgerstate.UndefinedBranchID},
			"Message21":   {ledgerstate.UndefinedBranchID, ledgerstate.UndefinedBranchID},
			"Message22":   {ledgerstate.UndefinedBranchID, ledgerstate.UndefinedBranchID},
		})
		checkBranchIDs(t, testFramework, map[string]ledgerstate.BranchIDs{
			"Message1":    testFramework.BranchIDs("A"),
			"Message2":    testFramework.BranchIDs("B"),
			"Message3":    testFramework.BranchIDs("C"),
			"Message4":    ledgerstate.NewBranchIDs(ledgerstate.MasterBranchID),
			"Message5":    testFramework.BranchIDs("D"),
			"Message6":    testFramework.BranchIDs("B"),
			"Message7":    testFramework.BranchIDs("B", "D"),
			"Message8":    testFramework.BranchIDs("A", "C", "D"),
			"Message9":    testFramework.BranchIDs("A", "C", "D"),
			"Message10":   testFramework.BranchIDs("B", "D"),
			"Message11":   testFramework.BranchIDs("A", "C", "D"),
			"Message11.5": testFramework.BranchIDs("A", "C", "D"),
			"Message12":   testFramework.BranchIDs("A", "E"),
			"Message13":   testFramework.BranchIDs("B", "E"),
			"Message13.1": testFramework.BranchIDs("B", "E"),
			"Message14":   testFramework.BranchIDs("B", "E"),
			"Message15":   ledgerstate.NewBranchIDs(ledgerstate.MasterBranchID),
			"Message16":   testFramework.BranchIDs("Z", "A", "E"),
			"Message17":   testFramework.BranchIDs("Y", "A", "E"),
			"Message18":   testFramework.BranchIDs("Y", "A", "E", "B", "D"),
			"Message19":   testFramework.BranchIDs("Y", "A", "E"),
			"Message20":   testFramework.BranchIDs("Y", "A", "E", "B"),
			"Message21":   testFramework.BranchIDs("Y", "A", "B"),
			"Message22":   testFramework.BranchIDs("Y"),
		})
	}

	// ISSUE Message23
	{
		msg := testFramework.CreateMessage("Message23", WithStrongParents("Message22"), WithShallowLikeParents("Message2"))

		testFramework.IssueMessages("Message23").WaitMessagesBooked()

		tangle.Storage.MessageMetadata(msg.ID()).Consume(func(messageMetadata *MessageMetadata) {
			assert.False(t, messageMetadata.subjectivelyInvalid)
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
			"Message13.1": markers.NewMarkers(markers.NewMarker(2, 3), markers.NewMarker(3, 1)),
			"Message14":   markers.NewMarkers(markers.NewMarker(2, 4)),
			"Message15":   markers.NewMarkers(markers.NewMarker(2, 3), markers.NewMarker(3, 1)),
			"Message16":   markers.NewMarkers(markers.NewMarker(6, 3)),
			"Message17":   markers.NewMarkers(markers.NewMarker(8, 3)),
			"Message18":   markers.NewMarkers(markers.NewMarker(9, 4)),
			"Message19":   markers.NewMarkers(markers.NewMarker(8, 4)),
			"Message20":   markers.NewMarkers(markers.NewMarker(10, 4)),
			"Message21":   markers.NewMarkers(markers.NewMarker(11, 4)),
			"Message22":   markers.NewMarkers(markers.NewMarker(12, 4)),
			"Message23":   markers.NewMarkers(markers.NewMarker(13, 5)),
		})
		checkMessageMetadataDiffBranchIDs(t, testFramework, map[string][]ledgerstate.BranchID{
			"Message1":    {ledgerstate.UndefinedBranchID, ledgerstate.UndefinedBranchID},
			"Message2":    {ledgerstate.UndefinedBranchID, ledgerstate.UndefinedBranchID},
			"Message3":    {ledgerstate.UndefinedBranchID, ledgerstate.UndefinedBranchID},
			"Message4":    {ledgerstate.UndefinedBranchID, ledgerstate.UndefinedBranchID},
			"Message5":    {ledgerstate.UndefinedBranchID, ledgerstate.UndefinedBranchID},
			"Message6":    {ledgerstate.UndefinedBranchID, ledgerstate.UndefinedBranchID},
			"Message7":    {ledgerstate.UndefinedBranchID, ledgerstate.UndefinedBranchID},
			"Message8":    {ledgerstate.UndefinedBranchID, ledgerstate.UndefinedBranchID},
			"Message9":    {testFramework.BranchID("A"), testFramework.BranchID("B")},
			"Message10":   {ledgerstate.UndefinedBranchID, ledgerstate.UndefinedBranchID},
			"Message11":   {ledgerstate.UndefinedBranchID, ledgerstate.UndefinedBranchID},
			"Message11.5": {testFramework.BranchID("A"), testFramework.BranchID("B")},
			"Message12":   {ledgerstate.UndefinedBranchID, ledgerstate.UndefinedBranchID},
			"Message13":   {ledgerstate.UndefinedBranchID, ledgerstate.UndefinedBranchID},
			"Message13.1": {testFramework.BranchID("E"), testFramework.BranchID("C+D")},
			"Message14":   {testFramework.BranchID("E"), testFramework.BranchID("D")},
			"Message15":   {ledgerstate.UndefinedBranchID, testFramework.BranchID("B+C+D")},
			"Message16":   {ledgerstate.UndefinedBranchID, ledgerstate.UndefinedBranchID},
			"Message17":   {ledgerstate.UndefinedBranchID, ledgerstate.UndefinedBranchID},
			"Message18":   {ledgerstate.UndefinedBranchID, ledgerstate.UndefinedBranchID},
			"Message19":   {ledgerstate.UndefinedBranchID, ledgerstate.UndefinedBranchID},
			"Message20":   {ledgerstate.UndefinedBranchID, ledgerstate.UndefinedBranchID},
			"Message21":   {ledgerstate.UndefinedBranchID, ledgerstate.UndefinedBranchID},
			"Message22":   {ledgerstate.UndefinedBranchID, ledgerstate.UndefinedBranchID},
			"Message23":   {ledgerstate.UndefinedBranchID, ledgerstate.UndefinedBranchID},
		})
		checkBranchIDs(t, testFramework, map[string]ledgerstate.BranchIDs{
			"Message1":    testFramework.BranchIDs("A"),
			"Message2":    testFramework.BranchIDs("B"),
			"Message3":    testFramework.BranchIDs("C"),
			"Message4":    ledgerstate.NewBranchIDs(ledgerstate.MasterBranchID),
			"Message5":    testFramework.BranchIDs("D"),
			"Message6":    testFramework.BranchIDs("B"),
			"Message7":    testFramework.BranchIDs("B", "D"),
			"Message8":    testFramework.BranchIDs("A", "C", "D"),
			"Message9":    testFramework.BranchIDs("A", "C", "D"),
			"Message10":   testFramework.BranchIDs("B", "D"),
			"Message11":   testFramework.BranchIDs("A", "C", "D"),
			"Message11.5": testFramework.BranchIDs("A", "C", "D"),
			"Message12":   testFramework.BranchIDs("A", "E"),
			"Message13":   testFramework.BranchIDs("B", "E"),
			"Message13.1": testFramework.BranchIDs("B", "E"),
			"Message14":   testFramework.BranchIDs("B", "E"),
			"Message15":   ledgerstate.NewBranchIDs(ledgerstate.MasterBranchID),
			"Message16":   testFramework.BranchIDs("Z", "A", "E"),
			"Message17":   testFramework.BranchIDs("Y", "A", "E"),
			"Message18":   testFramework.BranchIDs("Y", "A", "E", "B", "D"),
			"Message19":   testFramework.BranchIDs("Y", "A", "E"),
			"Message20":   testFramework.BranchIDs("Y", "A", "E", "B"),
			"Message21":   testFramework.BranchIDs("Y", "A", "B"),
			"Message22":   testFramework.BranchIDs("Y"),
			"Message23":   testFramework.BranchIDs("B", "Y"),
		})
	}

	// ISSUE Message24
	{
		msg := testFramework.CreateMessage("Message24", WithStrongParents("Message23"), WithShallowLikeParents("Message5"))

		testFramework.IssueMessages("Message24").WaitMessagesBooked()

		tangle.Storage.MessageMetadata(msg.ID()).Consume(func(messageMetadata *MessageMetadata) {
			assert.True(t, messageMetadata.subjectivelyInvalid)
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
			"Message13.1": markers.NewMarkers(markers.NewMarker(2, 3), markers.NewMarker(3, 1)),
			"Message14":   markers.NewMarkers(markers.NewMarker(2, 4)),
			"Message15":   markers.NewMarkers(markers.NewMarker(2, 3), markers.NewMarker(3, 1)),
			"Message16":   markers.NewMarkers(markers.NewMarker(6, 3)),
			"Message17":   markers.NewMarkers(markers.NewMarker(8, 3)),
			"Message18":   markers.NewMarkers(markers.NewMarker(9, 4)),
			"Message19":   markers.NewMarkers(markers.NewMarker(8, 4)),
			"Message20":   markers.NewMarkers(markers.NewMarker(10, 4)),
			"Message21":   markers.NewMarkers(markers.NewMarker(11, 4)),
			"Message22":   markers.NewMarkers(markers.NewMarker(12, 4)),
			"Message23":   markers.NewMarkers(markers.NewMarker(13, 5)),
			"Message24":   markers.NewMarkers(markers.NewMarker(14, 6)),
		})
		checkMessageMetadataDiffBranchIDs(t, testFramework, map[string][]ledgerstate.BranchID{
			"Message1":    {ledgerstate.UndefinedBranchID, ledgerstate.UndefinedBranchID},
			"Message2":    {ledgerstate.UndefinedBranchID, ledgerstate.UndefinedBranchID},
			"Message3":    {ledgerstate.UndefinedBranchID, ledgerstate.UndefinedBranchID},
			"Message4":    {ledgerstate.UndefinedBranchID, ledgerstate.UndefinedBranchID},
			"Message5":    {ledgerstate.UndefinedBranchID, ledgerstate.UndefinedBranchID},
			"Message6":    {ledgerstate.UndefinedBranchID, ledgerstate.UndefinedBranchID},
			"Message7":    {ledgerstate.UndefinedBranchID, ledgerstate.UndefinedBranchID},
			"Message8":    {ledgerstate.UndefinedBranchID, ledgerstate.UndefinedBranchID},
			"Message9":    {testFramework.BranchID("A"), testFramework.BranchID("B")},
			"Message10":   {ledgerstate.UndefinedBranchID, ledgerstate.UndefinedBranchID},
			"Message11":   {ledgerstate.UndefinedBranchID, ledgerstate.UndefinedBranchID},
			"Message11.5": {testFramework.BranchID("A"), testFramework.BranchID("B")},
			"Message12":   {ledgerstate.UndefinedBranchID, ledgerstate.UndefinedBranchID},
			"Message13":   {ledgerstate.UndefinedBranchID, ledgerstate.UndefinedBranchID},
			"Message13.1": {testFramework.BranchID("E"), testFramework.BranchID("C+D")},
			"Message14":   {testFramework.BranchID("E"), testFramework.BranchID("D")},
			"Message15":   {ledgerstate.UndefinedBranchID, testFramework.BranchID("B+C+D")},
			"Message16":   {ledgerstate.UndefinedBranchID, ledgerstate.UndefinedBranchID},
			"Message17":   {ledgerstate.UndefinedBranchID, ledgerstate.UndefinedBranchID},
			"Message18":   {ledgerstate.UndefinedBranchID, ledgerstate.UndefinedBranchID},
			"Message19":   {ledgerstate.UndefinedBranchID, ledgerstate.UndefinedBranchID},
			"Message20":   {ledgerstate.UndefinedBranchID, ledgerstate.UndefinedBranchID},
			"Message21":   {ledgerstate.UndefinedBranchID, ledgerstate.UndefinedBranchID},
			"Message22":   {ledgerstate.UndefinedBranchID, ledgerstate.UndefinedBranchID},
			"Message23":   {ledgerstate.UndefinedBranchID, ledgerstate.UndefinedBranchID},
			"Message24":   {ledgerstate.UndefinedBranchID, ledgerstate.UndefinedBranchID},
		})
		checkBranchIDs(t, testFramework, map[string]ledgerstate.BranchIDs{
			"Message1":    testFramework.BranchIDs("A"),
			"Message2":    testFramework.BranchIDs("B"),
			"Message3":    testFramework.BranchIDs("C"),
			"Message4":    ledgerstate.NewBranchIDs(ledgerstate.MasterBranchID),
			"Message5":    testFramework.BranchIDs("D"),
			"Message6":    testFramework.BranchIDs("B"),
			"Message7":    testFramework.BranchIDs("B", "D"),
			"Message8":    testFramework.BranchIDs("A", "C", "D"),
			"Message9":    testFramework.BranchIDs("A", "C", "D"),
			"Message10":   testFramework.BranchIDs("B", "D"),
			"Message11":   testFramework.BranchIDs("A", "C", "D"),
			"Message11.5": testFramework.BranchIDs("A", "C", "D"),
			"Message12":   testFramework.BranchIDs("A", "E"),
			"Message13":   testFramework.BranchIDs("B", "E"),
			"Message13.1": testFramework.BranchIDs("B", "E"),
			"Message14":   testFramework.BranchIDs("B", "E"),
			"Message15":   ledgerstate.NewBranchIDs(ledgerstate.MasterBranchID),
			"Message16":   testFramework.BranchIDs("Z", "A", "E"),
			"Message17":   testFramework.BranchIDs("Y", "A", "E"),
			"Message18":   testFramework.BranchIDs("Y", "A", "E", "B", "D"),
			"Message19":   testFramework.BranchIDs("Y", "A", "E"),
			"Message20":   testFramework.BranchIDs("Y", "A", "E", "B"),
			"Message21":   testFramework.BranchIDs("Y", "A", "B"),
			"Message22":   testFramework.BranchIDs("Y"),
			"Message23":   testFramework.BranchIDs("B", "Y"),
			"Message24":   testFramework.BranchIDs("B", "Y", "D"),
		})
	}

	// ISSUE Message25
	{
		msg := testFramework.CreateMessage("Message25", WithStrongParents("Message22"), WithWeakParents("Message5"))

		testFramework.IssueMessages("Message25").WaitMessagesBooked()

		tangle.Storage.MessageMetadata(msg.ID()).Consume(func(messageMetadata *MessageMetadata) {
			assert.True(t, messageMetadata.subjectivelyInvalid)
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
			"Message13.1": markers.NewMarkers(markers.NewMarker(2, 3), markers.NewMarker(3, 1)),
			"Message14":   markers.NewMarkers(markers.NewMarker(2, 4)),
			"Message15":   markers.NewMarkers(markers.NewMarker(2, 3), markers.NewMarker(3, 1)),
			"Message16":   markers.NewMarkers(markers.NewMarker(6, 3)),
			"Message17":   markers.NewMarkers(markers.NewMarker(8, 3)),
			"Message18":   markers.NewMarkers(markers.NewMarker(9, 4)),
			"Message19":   markers.NewMarkers(markers.NewMarker(8, 4)),
			"Message20":   markers.NewMarkers(markers.NewMarker(10, 4)),
			"Message21":   markers.NewMarkers(markers.NewMarker(11, 4)),
			"Message22":   markers.NewMarkers(markers.NewMarker(12, 4)),
			"Message23":   markers.NewMarkers(markers.NewMarker(13, 5)),
			"Message24":   markers.NewMarkers(markers.NewMarker(14, 6)),
			"Message25":   markers.NewMarkers(markers.NewMarker(15, 5)),
		})
		checkMessageMetadataDiffBranchIDs(t, testFramework, map[string][]ledgerstate.BranchID{
			"Message1":    {ledgerstate.UndefinedBranchID, ledgerstate.UndefinedBranchID},
			"Message2":    {ledgerstate.UndefinedBranchID, ledgerstate.UndefinedBranchID},
			"Message3":    {ledgerstate.UndefinedBranchID, ledgerstate.UndefinedBranchID},
			"Message4":    {ledgerstate.UndefinedBranchID, ledgerstate.UndefinedBranchID},
			"Message5":    {ledgerstate.UndefinedBranchID, ledgerstate.UndefinedBranchID},
			"Message6":    {ledgerstate.UndefinedBranchID, ledgerstate.UndefinedBranchID},
			"Message7":    {ledgerstate.UndefinedBranchID, ledgerstate.UndefinedBranchID},
			"Message8":    {ledgerstate.UndefinedBranchID, ledgerstate.UndefinedBranchID},
			"Message9":    {testFramework.BranchID("A"), testFramework.BranchID("B")},
			"Message10":   {ledgerstate.UndefinedBranchID, ledgerstate.UndefinedBranchID},
			"Message11":   {ledgerstate.UndefinedBranchID, ledgerstate.UndefinedBranchID},
			"Message11.5": {testFramework.BranchID("A"), testFramework.BranchID("B")},
			"Message12":   {ledgerstate.UndefinedBranchID, ledgerstate.UndefinedBranchID},
			"Message13":   {ledgerstate.UndefinedBranchID, ledgerstate.UndefinedBranchID},
			"Message13.1": {testFramework.BranchID("E"), testFramework.BranchID("C+D")},
			"Message14":   {testFramework.BranchID("E"), testFramework.BranchID("D")},
			"Message15":   {ledgerstate.UndefinedBranchID, testFramework.BranchID("B+C+D")},
			"Message16":   {ledgerstate.UndefinedBranchID, ledgerstate.UndefinedBranchID},
			"Message17":   {ledgerstate.UndefinedBranchID, ledgerstate.UndefinedBranchID},
			"Message18":   {ledgerstate.UndefinedBranchID, ledgerstate.UndefinedBranchID},
			"Message19":   {ledgerstate.UndefinedBranchID, ledgerstate.UndefinedBranchID},
			"Message20":   {ledgerstate.UndefinedBranchID, ledgerstate.UndefinedBranchID},
			"Message21":   {ledgerstate.UndefinedBranchID, ledgerstate.UndefinedBranchID},
			"Message22":   {ledgerstate.UndefinedBranchID, ledgerstate.UndefinedBranchID},
			"Message23":   {ledgerstate.UndefinedBranchID, ledgerstate.UndefinedBranchID},
			"Message24":   {ledgerstate.UndefinedBranchID, ledgerstate.UndefinedBranchID},
			"Message25":   {ledgerstate.UndefinedBranchID, ledgerstate.UndefinedBranchID},
		})
		checkBranchIDs(t, testFramework, map[string]ledgerstate.BranchIDs{
			"Message1":    testFramework.BranchIDs("A"),
			"Message2":    testFramework.BranchIDs("B"),
			"Message3":    testFramework.BranchIDs("C"),
			"Message4":    ledgerstate.NewBranchIDs(ledgerstate.MasterBranchID),
			"Message5":    testFramework.BranchIDs("D"),
			"Message6":    testFramework.BranchIDs("B"),
			"Message7":    testFramework.BranchIDs("B", "D"),
			"Message8":    testFramework.BranchIDs("A", "C", "D"),
			"Message9":    testFramework.BranchIDs("A", "C", "D"),
			"Message10":   testFramework.BranchIDs("B", "D"),
			"Message11":   testFramework.BranchIDs("A", "C", "D"),
			"Message11.5": testFramework.BranchIDs("A", "C", "D"),
			"Message12":   testFramework.BranchIDs("A", "E"),
			"Message13":   testFramework.BranchIDs("B", "E"),
			"Message13.1": testFramework.BranchIDs("B", "E"),
			"Message14":   testFramework.BranchIDs("B", "E"),
			"Message15":   ledgerstate.NewBranchIDs(ledgerstate.MasterBranchID),
			"Message16":   testFramework.BranchIDs("Z", "A", "E"),
			"Message17":   testFramework.BranchIDs("Y", "A", "E"),
			"Message18":   testFramework.BranchIDs("Y", "A", "E", "B", "D"),
			"Message19":   testFramework.BranchIDs("Y", "A", "E"),
			"Message20":   testFramework.BranchIDs("Y", "A", "E", "B"),
			"Message21":   testFramework.BranchIDs("Y", "A", "B"),
			"Message22":   testFramework.BranchIDs("Y"),
			"Message23":   testFramework.BranchIDs("B", "Y"),
			"Message24":   testFramework.BranchIDs("B", "Y", "D"),
			"Message25":   testFramework.BranchIDs("Y", "D"),
		})
	}
}

func TestArithmeticBranchIDs_Add(t *testing.T) {
	branchID1 := ledgerstate.BranchIDFromRandomness()
	branchID2 := ledgerstate.BranchIDFromRandomness()
	branchID3 := ledgerstate.BranchIDFromRandomness()

	ledgerstate.RegisterBranchIDAlias(branchID1, "branchID1")
	ledgerstate.RegisterBranchIDAlias(branchID2, "branchID2")
	ledgerstate.RegisterBranchIDAlias(branchID3, "branchID3")

	arithmeticBranchIDs := NewArithmeticBranchIDs()
	fmt.Println(arithmeticBranchIDs)

	arithmeticBranchIDs.Add(ledgerstate.NewBranchIDs(branchID1, branchID2))
	arithmeticBranchIDs.Add(ledgerstate.NewBranchIDs(branchID1, branchID3))
	arithmeticBranchIDs.Subtract(ledgerstate.NewBranchIDs(branchID2, branchID2))

	fmt.Println(arithmeticBranchIDs)
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

func checkBranchIDs(t *testing.T, testFramework *MessageTestFramework, expectedBranchIDs map[string]ledgerstate.BranchIDs) {
	for messageID, messageExpectedBranchIDs := range expectedBranchIDs {
		fmt.Println(">>", messageID)
		retrievedBranchIDs, errRetrieve := testFramework.tangle.Booker.MessageBranchIDs(testFramework.Message(messageID).ID())
		assert.NoError(t, errRetrieve)

		assert.Equal(t, messageExpectedBranchIDs, retrievedBranchIDs, "BranchID of %s should be %s but is %s", messageID, messageExpectedBranchIDs, retrievedBranchIDs)
	}
}

func checkMessageMetadataDiffBranchIDs(t *testing.T, testFramework *MessageTestFramework, expectedDiffBranchIDs map[string][]ledgerstate.BranchID) {
	for messageID, expectedDiffBranchID := range expectedDiffBranchIDs {
		assert.True(t, testFramework.tangle.Storage.MessageMetadata(testFramework.Message(messageID).ID()).Consume(func(messageMetadata *MessageMetadata) {
			assert.Equal(t, expectedDiffBranchID[0], messageMetadata.AddedBranchIDs(), "AddBranchIDs of %s should be %s but is %s in the Metadata", messageID, expectedDiffBranchID[0], messageMetadata.AddedBranchIDs())
		}))
		assert.True(t, testFramework.tangle.Storage.MessageMetadata(testFramework.Message(messageID).ID()).Consume(func(messageMetadata *MessageMetadata) {
			assert.Equal(t, expectedDiffBranchID[1], messageMetadata.SubtractedBranchIDs(), "SubtractedBranchIDs of %s should be %s but is %s in the Metadata", messageID, expectedDiffBranchID[1], messageMetadata.SubtractedBranchIDs())
		}))
	}
}
