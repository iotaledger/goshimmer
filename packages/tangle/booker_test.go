//nolint:dupl,whitespace
package tangle

import (
	"fmt"
	"math/rand"
	"sync"
	"testing"
	"time"

	"github.com/iotaledger/hive.go/debug"
	"github.com/stretchr/testify/assert"

	"github.com/iotaledger/goshimmer/packages/ledger/branchdag"
	"github.com/iotaledger/goshimmer/packages/markers"
)

func TestScenario_1(t *testing.T) {
	debug.Enabled = true

	tangle := NewTestTangle(WithBranchDAGOptions(branchdag.WithMergeToMaster(false)))
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
	testFramework.CreateMessage("Message9", WithStrongParents("Message4", "Message6"), WithInputs("H"), WithOutput("J", 3))

	testFramework.RegisterBranchID("Branch4", "Message4")
	testFramework.RegisterBranchID("Branch5", "Message5")

	testFramework.RegisterBranchID("Branch6", "Message6")
	testFramework.RegisterBranchID("Branch8", "Message8")

	testFramework.IssueMessages("Message1", "Message2", "Message3", "Message4", "Message5", "Message6").WaitUntilAllTasksProcessed()
	testFramework.IssueMessages("Message8").WaitUntilAllTasksProcessed()
	testFramework.IssueMessages("Message7", "Message9").WaitUntilAllTasksProcessed()

	// Message8 combines conflicting branches on UTXO level
	for _, messageAlias := range []string{"Message8"} {
		assert.Truef(t, testFramework.MessageMetadata(messageAlias).IsSubjectivelyInvalid(), "%s not subjectively invalid", messageAlias)
	}

	// Message9 combines conflicting branches on message level
	for _, messageAlias := range []string{"Message9"} {
		assert.Truef(t, testFramework.MessageMetadata(messageAlias).IsSubjectivelyInvalid(), "%s not subjectively invalid", messageAlias)
	}

	checkBranchIDs(t, testFramework, map[string]branchdag.BranchIDs{
		"Message1": branchdag.NewBranchIDs(branchdag.MasterBranchID),
		"Message3": branchdag.NewBranchIDs(branchdag.MasterBranchID),
		"Message2": branchdag.NewBranchIDs(branchdag.MasterBranchID),
		"Message4": testFramework.BranchIDs("Branch4"),
		"Message5": testFramework.BranchIDs("Branch5"),
		"Message6": testFramework.BranchIDs("Branch6"),
		"Message7": testFramework.BranchIDs("Branch4", "Branch5"),
		"Message8": testFramework.BranchIDs("Branch4", "Branch5", "Branch8"),
		"Message9": testFramework.BranchIDs("Branch4", "Branch6"),
	})
}

func TestScenario_2(t *testing.T) {
	tangle := NewTestTangle(WithBranchDAGOptions(branchdag.WithMergeToMaster(false)))
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

	testFramework.RegisterBranchID("green", "Message5")
	testFramework.RegisterBranchID("red", "Message4")
	testFramework.RegisterBranchID("yellow", "Message2")
	testFramework.RegisterBranchID("black", "Message7")
	testFramework.RegisterBranchID("blue", "Message9")

	testFramework.IssueMessages("Message1").WaitUntilAllTasksProcessed()
	testFramework.IssueMessages("Message2").WaitUntilAllTasksProcessed()
	testFramework.IssueMessages("Message3", "Message4").WaitUntilAllTasksProcessed()
	testFramework.IssueMessages("Message5").WaitUntilAllTasksProcessed()
	testFramework.IssueMessages("Message6").WaitUntilAllTasksProcessed()
	testFramework.IssueMessages("Message7").WaitUntilAllTasksProcessed()
	testFramework.IssueMessages("Message8").WaitUntilAllTasksProcessed()
	testFramework.IssueMessages("Message9").WaitUntilAllTasksProcessed()

	checkBranchIDs(t, testFramework, map[string]branchdag.BranchIDs{
		"Message1": branchdag.NewBranchIDs(branchdag.MasterBranchID),
		"Message2": testFramework.BranchIDs("yellow"),
		"Message3": testFramework.BranchIDs("yellow"),
		"Message4": testFramework.BranchIDs("red"),
		"Message5": testFramework.BranchIDs("yellow", "green"),
		"Message6": testFramework.BranchIDs("yellow", "green"),
		"Message7": testFramework.BranchIDs("red", "black"),
		"Message8": testFramework.BranchIDs("red", "black"),
		"Message9": testFramework.BranchIDs("red", "black", "blue"),
	})

	checkMarkers(t, testFramework, map[string]*markers.Markers{
		"Message1": markers.NewMarkers(markers.NewMarker(0, 1)),
		"Message2": markers.NewMarkers(markers.NewMarker(0, 2)),
		"Message3": markers.NewMarkers(markers.NewMarker(0, 3)),
		"Message4": markers.NewMarkers(markers.NewMarker(0, 1)),
		"Message5": markers.NewMarkers(markers.NewMarker(0, 2)),
		"Message6": markers.NewMarkers(markers.NewMarker(0, 2)),
		"Message7": markers.NewMarkers(markers.NewMarker(0, 1)),
		"Message8": markers.NewMarkers(markers.NewMarker(0, 1)),
		"Message9": markers.NewMarkers(markers.NewMarker(0, 1)),
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

	testFramework.IssueMessages("Message1").WaitUntilAllTasksProcessed()
	testFramework.IssueMessages("Message2").WaitUntilAllTasksProcessed()
	testFramework.IssueMessages("Message3", "Message4").WaitUntilAllTasksProcessed()
	testFramework.IssueMessages("Message5").WaitUntilAllTasksProcessed()
	testFramework.IssueMessages("Message6").WaitUntilAllTasksProcessed()
	testFramework.IssueMessages("Message7").WaitUntilAllTasksProcessed()
	testFramework.IssueMessages("Message8").WaitUntilAllTasksProcessed()
	testFramework.IssueMessages("Message9").WaitUntilAllTasksProcessed()

	testFramework.RegisterBranchID("purple", "Message2")
	testFramework.RegisterBranchID("red", "Message4")
	testFramework.RegisterBranchID("yellow", "Message5")
	testFramework.RegisterBranchID("orange", "Message7")
	testFramework.RegisterBranchID("blue", "Message9")
	testFramework.RegisterBranchID("blue", "Message9")

	checkBranchIDs(t, testFramework, map[string]branchdag.BranchIDs{
		"Message1": branchdag.NewBranchIDs(branchdag.MasterBranchID),
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
		testFramework.IssueMessages("Message1").WaitUntilAllTasksProcessed()

		checkMarkers(t, testFramework, map[string]*markers.Markers{
			"Message1": markers.NewMarkers(markers.NewMarker(0, 1)),
		})
		checkMessageMetadataDiffBranchIDs(t, testFramework, map[string][]branchdag.BranchIDs{
			"Message1": {branchdag.NewBranchIDs(), branchdag.NewBranchIDs()},
		})
		checkBranchIDs(t, testFramework, map[string]branchdag.BranchIDs{
			"Message1": branchdag.NewBranchIDs(branchdag.MasterBranchID),
		})
	}

	// ISSUE Message1.5
	{
		testFramework.CreateMessage("Message1.5", WithStrongParents("Message1"))
		testFramework.IssueMessages("Message1.5").WaitUntilAllTasksProcessed()

		checkMarkers(t, testFramework, map[string]*markers.Markers{
			"Message1":   markers.NewMarkers(markers.NewMarker(0, 1)),
			"Message1.5": markers.NewMarkers(markers.NewMarker(0, 2)),
		})
		checkMessageMetadataDiffBranchIDs(t, testFramework, map[string][]branchdag.BranchIDs{
			"Message1":   {branchdag.NewBranchIDs(), branchdag.NewBranchIDs()},
			"Message1.5": {branchdag.NewBranchIDs(), branchdag.NewBranchIDs()},
		})
		checkBranchIDs(t, testFramework, map[string]branchdag.BranchIDs{
			"Message1":   branchdag.NewBranchIDs(branchdag.MasterBranchID),
			"Message1.5": branchdag.NewBranchIDs(branchdag.MasterBranchID),
		})
	}

	// ISSUE Message2
	{
		testFramework.CreateMessage("Message2", WithStrongParents("Message1.5"), WithInputs("B"), WithOutput("E", 500))

		testFramework.IssueMessages("Message2").WaitUntilAllTasksProcessed()

		checkMarkers(t, testFramework, map[string]*markers.Markers{
			"Message1":   markers.NewMarkers(markers.NewMarker(0, 1)),
			"Message1.5": markers.NewMarkers(markers.NewMarker(0, 2)),
			"Message2":   markers.NewMarkers(markers.NewMarker(0, 3)),
		})
		checkMessageMetadataDiffBranchIDs(t, testFramework, map[string][]branchdag.BranchIDs{
			"Message1":   {branchdag.NewBranchIDs(), branchdag.NewBranchIDs()},
			"Message1.5": {branchdag.NewBranchIDs(), branchdag.NewBranchIDs()},
			"Message2":   {branchdag.NewBranchIDs(), branchdag.NewBranchIDs()},
		})
		checkBranchIDs(t, testFramework, map[string]branchdag.BranchIDs{
			"Message1":   branchdag.NewBranchIDs(branchdag.MasterBranchID),
			"Message1.5": branchdag.NewBranchIDs(branchdag.MasterBranchID),
			"Message2":   branchdag.NewBranchIDs(branchdag.MasterBranchID),
		})
	}
	// ISSUE Message3
	{
		testFramework.CreateMessage("Message3", WithStrongParents("Genesis"), WithInputs("B"), WithOutput("F", 500))

		testFramework.RegisterBranchID("Message2", "Message2")
		testFramework.RegisterBranchID("Message3", "Message3")

		testFramework.IssueMessages("Message3").WaitUntilAllTasksProcessed()

		checkMarkers(t, testFramework, map[string]*markers.Markers{
			"Message1":   markers.NewMarkers(markers.NewMarker(0, 1)),
			"Message1.5": markers.NewMarkers(markers.NewMarker(0, 2)),
			"Message2":   markers.NewMarkers(markers.NewMarker(0, 3)),
			"Message3":   markers.NewMarkers(markers.NewMarker(0, 0)),
		})
		checkMessageMetadataDiffBranchIDs(t, testFramework, map[string][]branchdag.BranchIDs{
			"Message1":   {branchdag.NewBranchIDs(), branchdag.NewBranchIDs()},
			"Message1.5": {branchdag.NewBranchIDs(), branchdag.NewBranchIDs()},
			"Message2":   {branchdag.NewBranchIDs(), branchdag.NewBranchIDs()},
			"Message3":   {testFramework.BranchIDs("Message3"), branchdag.NewBranchIDs()},
		})
		checkBranchIDs(t, testFramework, map[string]branchdag.BranchIDs{
			"Message1":   branchdag.NewBranchIDs(branchdag.MasterBranchID),
			"Message1.5": branchdag.NewBranchIDs(branchdag.MasterBranchID),
			"Message2":   testFramework.BranchIDs("Message2"),
			"Message3":   testFramework.BranchIDs("Message3"),
		})
	}

	// ISSUE Message4
	{
		testFramework.CreateMessage("Message4", WithStrongParents("Message3"), WithInputs("A"), WithOutput("X", 500))

		testFramework.RegisterBranchID("Message1", "Message1")
		testFramework.RegisterBranchID("Message4", "Message4")

		testFramework.IssueMessages("Message4").WaitUntilAllTasksProcessed()

		checkMarkers(t, testFramework, map[string]*markers.Markers{
			"Message1":   markers.NewMarkers(markers.NewMarker(0, 1)),
			"Message1.5": markers.NewMarkers(markers.NewMarker(0, 2)),
			"Message2":   markers.NewMarkers(markers.NewMarker(0, 3)),
			"Message3":   markers.NewMarkers(markers.NewMarker(0, 0)),
			"Message4":   markers.NewMarkers(markers.NewMarker(0, 0)),
		})
		checkMessageMetadataDiffBranchIDs(t, testFramework, map[string][]branchdag.BranchIDs{
			"Message1":   {branchdag.NewBranchIDs(), branchdag.NewBranchIDs()},
			"Message1.5": {branchdag.NewBranchIDs(), branchdag.NewBranchIDs()},
			"Message2":   {branchdag.NewBranchIDs(), branchdag.NewBranchIDs()},
			"Message3":   {testFramework.BranchIDs("Message3"), branchdag.NewBranchIDs()},
			"Message4":   {testFramework.BranchIDs("Message3", "Message4"), branchdag.NewBranchIDs()},
		})
		checkBranchIDs(t, testFramework, map[string]branchdag.BranchIDs{
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
		testFramework.IssueMessages("Message1").WaitUntilAllTasksProcessed()

		checkMarkers(t, testFramework, map[string]*markers.Markers{
			"Message1": markers.NewMarkers(markers.NewMarker(0, 1)),
		})
		checkMessageMetadataDiffBranchIDs(t, testFramework, map[string][]branchdag.BranchIDs{
			"Message1": {branchdag.NewBranchIDs(), branchdag.NewBranchIDs()},
		})
		checkBranchIDs(t, testFramework, map[string]branchdag.BranchIDs{
			"Message1": branchdag.NewBranchIDs(branchdag.MasterBranchID),
		})
	}

	// ISSUE Message2
	{
		testFramework.CreateMessage("Message2", WithStrongParents("Genesis"), WithInputs("Genesis1"), WithOutput("Message2", 500))
		testFramework.IssueMessages("Message2").WaitUntilAllTasksProcessed()

		testFramework.RegisterBranchID("Message1", "Message1")
		testFramework.RegisterBranchID("Message2", "Message2")

		checkMarkers(t, testFramework, map[string]*markers.Markers{
			"Message1": markers.NewMarkers(markers.NewMarker(0, 1)),
			"Message2": markers.NewMarkers(markers.NewMarker(0, 0)),
		})
		checkMessageMetadataDiffBranchIDs(t, testFramework, map[string][]branchdag.BranchIDs{
			"Message1": {branchdag.NewBranchIDs(), branchdag.NewBranchIDs()},
			"Message2": {testFramework.BranchIDs("Message2"), branchdag.NewBranchIDs()},
		})
		checkBranchIDs(t, testFramework, map[string]branchdag.BranchIDs{
			"Message1": testFramework.BranchIDs("Message1"),
			"Message2": testFramework.BranchIDs("Message2"),
		})
	}

	// ISSUE Message3
	{
		testFramework.CreateMessage("Message3", WithStrongParents("Genesis"), WithInputs("Genesis2"), WithOutput("Message3", 500))
		testFramework.IssueMessages("Message3").WaitUntilAllTasksProcessed()

		checkMarkers(t, testFramework, map[string]*markers.Markers{
			"Message1": markers.NewMarkers(markers.NewMarker(0, 1)),
			"Message2": markers.NewMarkers(markers.NewMarker(0, 0)),
			"Message3": markers.NewMarkers(markers.NewMarker(0, 0)),
		})
		checkMessageMetadataDiffBranchIDs(t, testFramework, map[string][]branchdag.BranchIDs{
			"Message1": {branchdag.NewBranchIDs(), branchdag.NewBranchIDs()},
			"Message2": {testFramework.BranchIDs("Message2"), branchdag.NewBranchIDs()},
			"Message3": {branchdag.NewBranchIDs(), branchdag.NewBranchIDs()},
		})
		checkBranchIDs(t, testFramework, map[string]branchdag.BranchIDs{
			"Message1": testFramework.BranchIDs("Message1"),
			"Message2": testFramework.BranchIDs("Message2"),
			"Message3": branchdag.NewBranchIDs(branchdag.MasterBranchID),
		})
	}

	// ISSUE Message4
	{
		testFramework.CreateMessage("Message4", WithStrongParents("Genesis"), WithInputs("Genesis2"), WithOutput("Message4", 500))
		testFramework.IssueMessages("Message4").WaitUntilAllTasksProcessed()

		testFramework.RegisterBranchID("Message3", "Message3")
		testFramework.RegisterBranchID("Message4", "Message4")

		checkMarkers(t, testFramework, map[string]*markers.Markers{
			"Message1": markers.NewMarkers(markers.NewMarker(0, 1)),
			"Message2": markers.NewMarkers(markers.NewMarker(0, 0)),
			"Message3": markers.NewMarkers(markers.NewMarker(0, 0)),
			"Message4": markers.NewMarkers(markers.NewMarker(0, 0)),
		})
		checkMessageMetadataDiffBranchIDs(t, testFramework, map[string][]branchdag.BranchIDs{
			"Message1": {branchdag.NewBranchIDs(), branchdag.NewBranchIDs()},
			"Message2": {testFramework.BranchIDs("Message2"), branchdag.NewBranchIDs()},
			"Message3": {testFramework.BranchIDs("Message3"), branchdag.NewBranchIDs()},
			"Message4": {testFramework.BranchIDs("Message4"), branchdag.NewBranchIDs()},
		})
		checkBranchIDs(t, testFramework, map[string]branchdag.BranchIDs{
			"Message1": testFramework.BranchIDs("Message1"),
			"Message2": testFramework.BranchIDs("Message2"),
			"Message3": testFramework.BranchIDs("Message3"),
			"Message4": testFramework.BranchIDs("Message4"),
		})
	}

	// ISSUE Message5
	{
		testFramework.CreateMessage("Message5", WithStrongParents("Message1"), WithInputs("Genesis3"), WithOutput("Message5", 500))
		testFramework.IssueMessages("Message5").WaitUntilAllTasksProcessed()

		checkMarkers(t, testFramework, map[string]*markers.Markers{
			"Message1": markers.NewMarkers(markers.NewMarker(0, 1)),
			"Message2": markers.NewMarkers(markers.NewMarker(0, 0)),
			"Message3": markers.NewMarkers(markers.NewMarker(0, 0)),
			"Message4": markers.NewMarkers(markers.NewMarker(0, 0)),
			"Message5": markers.NewMarkers(markers.NewMarker(0, 2)),
		})
		checkMessageMetadataDiffBranchIDs(t, testFramework, map[string][]branchdag.BranchIDs{
			"Message1": {branchdag.NewBranchIDs(), branchdag.NewBranchIDs()},
			"Message2": {testFramework.BranchIDs("Message2"), branchdag.NewBranchIDs()},
			"Message3": {testFramework.BranchIDs("Message3"), branchdag.NewBranchIDs()},
			"Message4": {testFramework.BranchIDs("Message4"), branchdag.NewBranchIDs()},
			"Message5": {branchdag.NewBranchIDs(), branchdag.NewBranchIDs()},
		})
		checkBranchIDs(t, testFramework, map[string]branchdag.BranchIDs{
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
		testFramework.IssueMessages("Message6").WaitUntilAllTasksProcessed()

		checkMarkers(t, testFramework, map[string]*markers.Markers{
			"Message1": markers.NewMarkers(markers.NewMarker(0, 1)),
			"Message2": markers.NewMarkers(markers.NewMarker(0, 0)),
			"Message3": markers.NewMarkers(markers.NewMarker(0, 0)),
			"Message4": markers.NewMarkers(markers.NewMarker(0, 0)),
			"Message5": markers.NewMarkers(markers.NewMarker(0, 2)),
			"Message6": markers.NewMarkers(markers.NewMarker(0, 1)),
		})
		checkMessageMetadataDiffBranchIDs(t, testFramework, map[string][]branchdag.BranchIDs{
			"Message1": {branchdag.NewBranchIDs(), branchdag.NewBranchIDs()},
			"Message2": {testFramework.BranchIDs("Message2"), branchdag.NewBranchIDs()},
			"Message3": {testFramework.BranchIDs("Message3"), branchdag.NewBranchIDs()},
			"Message4": {testFramework.BranchIDs("Message4"), branchdag.NewBranchIDs()},
			"Message5": {branchdag.NewBranchIDs(), branchdag.NewBranchIDs()},
			"Message6": {testFramework.BranchIDs("Message3"), branchdag.NewBranchIDs()},
		})
		checkBranchIDs(t, testFramework, map[string]branchdag.BranchIDs{
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
		testFramework.IssueMessages("Message7").WaitUntilAllTasksProcessed()

		checkMarkers(t, testFramework, map[string]*markers.Markers{
			"Message1": markers.NewMarkers(markers.NewMarker(0, 1)),
			"Message2": markers.NewMarkers(markers.NewMarker(0, 0)),
			"Message3": markers.NewMarkers(markers.NewMarker(0, 0)),
			"Message4": markers.NewMarkers(markers.NewMarker(0, 0)),
			"Message5": markers.NewMarkers(markers.NewMarker(0, 2)),
			"Message6": markers.NewMarkers(markers.NewMarker(0, 1)),
			"Message7": markers.NewMarkers(markers.NewMarker(0, 3)),
		})
		checkMessageMetadataDiffBranchIDs(t, testFramework, map[string][]branchdag.BranchIDs{
			"Message1": {branchdag.NewBranchIDs(), branchdag.NewBranchIDs()},
			"Message2": {testFramework.BranchIDs("Message2"), branchdag.NewBranchIDs()},
			"Message3": {testFramework.BranchIDs("Message3"), branchdag.NewBranchIDs()},
			"Message4": {testFramework.BranchIDs("Message4"), branchdag.NewBranchIDs()},
			"Message5": {branchdag.NewBranchIDs(), branchdag.NewBranchIDs()},
			"Message6": {testFramework.BranchIDs("Message3"), branchdag.NewBranchIDs()},
			"Message7": {branchdag.NewBranchIDs(), branchdag.NewBranchIDs()},
		})
		checkBranchIDs(t, testFramework, map[string]branchdag.BranchIDs{
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

		testFramework.IssueMessages("Message8").WaitUntilAllTasksProcessed()

		checkMarkers(t, testFramework, map[string]*markers.Markers{
			"Message1": markers.NewMarkers(markers.NewMarker(0, 1)),
			"Message2": markers.NewMarkers(markers.NewMarker(0, 0)),
			"Message3": markers.NewMarkers(markers.NewMarker(0, 0)),
			"Message4": markers.NewMarkers(markers.NewMarker(0, 0)),
			"Message5": markers.NewMarkers(markers.NewMarker(0, 2)),
			"Message6": markers.NewMarkers(markers.NewMarker(0, 1)),
			"Message7": markers.NewMarkers(markers.NewMarker(0, 3)),
			"Message8": markers.NewMarkers(markers.NewMarker(0, 0)),
		})
		checkMessageMetadataDiffBranchIDs(t, testFramework, map[string][]branchdag.BranchIDs{
			"Message1": {branchdag.NewBranchIDs(), branchdag.NewBranchIDs()},
			"Message2": {testFramework.BranchIDs("Message2"), branchdag.NewBranchIDs()},
			"Message3": {testFramework.BranchIDs("Message3"), branchdag.NewBranchIDs()},
			"Message4": {testFramework.BranchIDs("Message4"), branchdag.NewBranchIDs()},
			"Message5": {branchdag.NewBranchIDs(), branchdag.NewBranchIDs()},
			"Message6": {testFramework.BranchIDs("Message3"), branchdag.NewBranchIDs()},
			"Message7": {branchdag.NewBranchIDs(), branchdag.NewBranchIDs()},
			"Message8": {testFramework.BranchIDs("Message8"), branchdag.NewBranchIDs()},
		})
		checkBranchIDs(t, testFramework, map[string]branchdag.BranchIDs{
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
		testFramework.IssueMessages("A1").WaitUntilAllTasksProcessed()

		checkMarkers(t, testFramework, map[string]*markers.Markers{
			"A1": markers.NewMarkers(markers.NewMarker(0, 1)),
		})
		checkMessageMetadataDiffBranchIDs(t, testFramework, map[string][]branchdag.BranchIDs{
			"A1": {branchdag.NewBranchIDs(), branchdag.NewBranchIDs()},
		})
		checkBranchIDs(t, testFramework, map[string]branchdag.BranchIDs{
			"A1": branchdag.NewBranchIDs(branchdag.MasterBranchID),
		})
	}

	// ISSUE A2
	{
		testFramework.CreateMessage("A2", WithStrongParents("A1"))
		testFramework.IssueMessages("A2").WaitUntilAllTasksProcessed()

		checkMarkers(t, testFramework, map[string]*markers.Markers{
			"A1": markers.NewMarkers(markers.NewMarker(0, 1)),
			"A2": markers.NewMarkers(markers.NewMarker(0, 2)),
		})
		checkMessageMetadataDiffBranchIDs(t, testFramework, map[string][]branchdag.BranchIDs{
			"A1": {branchdag.NewBranchIDs(), branchdag.NewBranchIDs()},
			"A2": {branchdag.NewBranchIDs(), branchdag.NewBranchIDs()},
		})
		checkBranchIDs(t, testFramework, map[string]branchdag.BranchIDs{
			"A1": branchdag.NewBranchIDs(branchdag.MasterBranchID),
			"A2": branchdag.NewBranchIDs(branchdag.MasterBranchID),
		})
	}

	// ISSUE A3
	{
		testFramework.PreventNewMarkers(true).CreateMessage("A3", WithStrongParents("A2"), WithInputs("B"), WithOutput("B1", 500))
		testFramework.IssueMessages("A3").WaitUntilAllTasksProcessed().PreventNewMarkers(false)

		checkMarkers(t, testFramework, map[string]*markers.Markers{
			"A1": markers.NewMarkers(markers.NewMarker(0, 1)),
			"A2": markers.NewMarkers(markers.NewMarker(0, 2)),
			"A3": markers.NewMarkers(markers.NewMarker(0, 2)),
		})
		checkMessageMetadataDiffBranchIDs(t, testFramework, map[string][]branchdag.BranchIDs{
			"A1": {branchdag.NewBranchIDs(), branchdag.NewBranchIDs()},
			"A2": {branchdag.NewBranchIDs(), branchdag.NewBranchIDs()},
			"A3": {branchdag.NewBranchIDs(), branchdag.NewBranchIDs()},
		})
		checkBranchIDs(t, testFramework, map[string]branchdag.BranchIDs{
			"A1": branchdag.NewBranchIDs(branchdag.MasterBranchID),
			"A2": branchdag.NewBranchIDs(branchdag.MasterBranchID),
			"A3": branchdag.NewBranchIDs(branchdag.MasterBranchID),
		})
	}

	// ISSUE A4
	{
		testFramework.CreateMessage("A4", WithStrongParents("A3"))
		testFramework.IssueMessages("A4").WaitUntilAllTasksProcessed()

		checkMarkers(t, testFramework, map[string]*markers.Markers{
			"A1": markers.NewMarkers(markers.NewMarker(0, 1)),
			"A2": markers.NewMarkers(markers.NewMarker(0, 2)),
			"A3": markers.NewMarkers(markers.NewMarker(0, 2)),
			"A4": markers.NewMarkers(markers.NewMarker(0, 3)),
		})
		checkMessageMetadataDiffBranchIDs(t, testFramework, map[string][]branchdag.BranchIDs{
			"A1": {branchdag.NewBranchIDs(), branchdag.NewBranchIDs()},
			"A2": {branchdag.NewBranchIDs(), branchdag.NewBranchIDs()},
			"A3": {branchdag.NewBranchIDs(), branchdag.NewBranchIDs()},
			"A4": {branchdag.NewBranchIDs(), branchdag.NewBranchIDs()},
		})
		checkBranchIDs(t, testFramework, map[string]branchdag.BranchIDs{
			"A1": branchdag.NewBranchIDs(branchdag.MasterBranchID),
			"A2": branchdag.NewBranchIDs(branchdag.MasterBranchID),
			"A3": branchdag.NewBranchIDs(branchdag.MasterBranchID),
			"A4": branchdag.NewBranchIDs(branchdag.MasterBranchID),
		})
	}

	// ISSUE A3*
	{
		testFramework.CreateMessage("A3*", WithStrongParents("Genesis"), WithInputs("B"), WithOutput("A3*", 500))

		testFramework.RegisterBranchID("A3", "A3")
		testFramework.RegisterBranchID("A3*", "A3*")

		testFramework.IssueMessages("A3*").WaitUntilAllTasksProcessed()

		checkMarkers(t, testFramework, map[string]*markers.Markers{
			"A1":  markers.NewMarkers(markers.NewMarker(0, 1)),
			"A2":  markers.NewMarkers(markers.NewMarker(0, 2)),
			"A3":  markers.NewMarkers(markers.NewMarker(0, 2)),
			"A4":  markers.NewMarkers(markers.NewMarker(0, 3)),
			"A3*": markers.NewMarkers(markers.NewMarker(0, 0)),
		})
		checkMessageMetadataDiffBranchIDs(t, testFramework, map[string][]branchdag.BranchIDs{
			"A1":  {branchdag.NewBranchIDs(), branchdag.NewBranchIDs()},
			"A2":  {branchdag.NewBranchIDs(), branchdag.NewBranchIDs()},
			"A3":  {testFramework.BranchIDs("A3"), branchdag.NewBranchIDs()},
			"A4":  {branchdag.NewBranchIDs(), branchdag.NewBranchIDs()},
			"A3*": {testFramework.BranchIDs("A3*"), branchdag.NewBranchIDs()},
		})
		checkBranchIDs(t, testFramework, map[string]branchdag.BranchIDs{
			"A1":  branchdag.NewBranchIDs(branchdag.MasterBranchID),
			"A2":  branchdag.NewBranchIDs(branchdag.MasterBranchID),
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

		testFramework.IssueMessages("A1*").WaitUntilAllTasksProcessed()

		checkMarkers(t, testFramework, map[string]*markers.Markers{
			"A1":  markers.NewMarkers(markers.NewMarker(0, 1)),
			"A2":  markers.NewMarkers(markers.NewMarker(0, 2)),
			"A3":  markers.NewMarkers(markers.NewMarker(0, 2)),
			"A4":  markers.NewMarkers(markers.NewMarker(0, 3)),
			"A3*": markers.NewMarkers(markers.NewMarker(0, 0)),
			"A1*": markers.NewMarkers(markers.NewMarker(0, 0)),
		})
		checkMessageMetadataDiffBranchIDs(t, testFramework, map[string][]branchdag.BranchIDs{
			"A1":  {branchdag.NewBranchIDs(), branchdag.NewBranchIDs()},
			"A2":  {branchdag.NewBranchIDs(), branchdag.NewBranchIDs()},
			"A3":  {testFramework.BranchIDs("A3"), branchdag.NewBranchIDs()},
			"A4":  {branchdag.NewBranchIDs(), branchdag.NewBranchIDs()},
			"A3*": {testFramework.BranchIDs("A3*"), branchdag.NewBranchIDs()},
			"A1*": {testFramework.BranchIDs("A1*"), branchdag.NewBranchIDs()},
		})
		checkBranchIDs(t, testFramework, map[string]branchdag.BranchIDs{
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
		testFramework.IssueMessages("A1").WaitUntilAllTasksProcessed()

		checkMarkers(t, testFramework, map[string]*markers.Markers{
			"A1": markers.NewMarkers(markers.NewMarker(0, 1)),
		})
		checkMessageMetadataDiffBranchIDs(t, testFramework, map[string][]branchdag.BranchIDs{
			"A1": {branchdag.NewBranchIDs(), branchdag.NewBranchIDs()},
		})
		checkBranchIDs(t, testFramework, map[string]branchdag.BranchIDs{
			"A1": branchdag.NewBranchIDs(branchdag.MasterBranchID),
		})
	}

	// ISSUE B1
	{
		testFramework.CreateMessage("B1", WithStrongParents("Genesis"), WithInputs("A"), WithOutput("B1", 500))

		testFramework.RegisterBranchID("A", "A1")
		testFramework.RegisterBranchID("B", "B1")

		testFramework.IssueMessages("B1").WaitUntilAllTasksProcessed()

		checkMarkers(t, testFramework, map[string]*markers.Markers{
			"A1": markers.NewMarkers(markers.NewMarker(0, 1)),
			"B1": markers.NewMarkers(markers.NewMarker(0, 0)),
		})
		checkMessageMetadataDiffBranchIDs(t, testFramework, map[string][]branchdag.BranchIDs{
			"A1": {branchdag.NewBranchIDs(), branchdag.NewBranchIDs()},
			"B1": {testFramework.BranchIDs("B"), branchdag.NewBranchIDs()},
		})
		checkBranchIDs(t, testFramework, map[string]branchdag.BranchIDs{
			"A1": testFramework.BranchIDs("A"),
			"B1": testFramework.BranchIDs("B"),
		})
	}

	// ISSUE C1
	{
		testFramework.CreateMessage("C1", WithStrongParents("Genesis"), WithInputs("C"), WithOutput("C1", 500))

		testFramework.RegisterBranchID("A", "A1")
		testFramework.RegisterBranchID("B", "B1")

		testFramework.IssueMessages("C1").WaitUntilAllTasksProcessed()

		checkMarkers(t, testFramework, map[string]*markers.Markers{
			"A1": markers.NewMarkers(markers.NewMarker(0, 1)),
			"B1": markers.NewMarkers(markers.NewMarker(0, 0)),
			"C1": markers.NewMarkers(markers.NewMarker(0, 0)),
		})
		checkMessageMetadataDiffBranchIDs(t, testFramework, map[string][]branchdag.BranchIDs{
			"A1": {branchdag.NewBranchIDs(), branchdag.NewBranchIDs()},
			"B1": {testFramework.BranchIDs("B"), branchdag.NewBranchIDs()},
			"C1": {branchdag.NewBranchIDs(), branchdag.NewBranchIDs()},
		})
		checkBranchIDs(t, testFramework, map[string]branchdag.BranchIDs{
			"A1": testFramework.BranchIDs("A"),
			"B1": testFramework.BranchIDs("B"),
			"C1": branchdag.NewBranchIDs(branchdag.MasterBranchID),
		})
	}

	// ISSUE D1
	{
		testFramework.CreateMessage("D1", WithStrongParents("Genesis"), WithInputs("C"), WithOutput("D1", 500))

		testFramework.RegisterBranchID("C", "C1")
		testFramework.RegisterBranchID("D", "D1")

		testFramework.IssueMessages("D1").WaitUntilAllTasksProcessed()

		checkMarkers(t, testFramework, map[string]*markers.Markers{
			"A1": markers.NewMarkers(markers.NewMarker(0, 1)),
			"B1": markers.NewMarkers(markers.NewMarker(0, 0)),
			"C1": markers.NewMarkers(markers.NewMarker(0, 0)),
			"D1": markers.NewMarkers(markers.NewMarker(0, 0)),
		})
		checkMessageMetadataDiffBranchIDs(t, testFramework, map[string][]branchdag.BranchIDs{
			"A1": {branchdag.NewBranchIDs(), branchdag.NewBranchIDs()},
			"B1": {testFramework.BranchIDs("B"), branchdag.NewBranchIDs()},
			"C1": {testFramework.BranchIDs("C"), branchdag.NewBranchIDs()},
			"D1": {testFramework.BranchIDs("D"), branchdag.NewBranchIDs()},
		})
		checkBranchIDs(t, testFramework, map[string]branchdag.BranchIDs{
			"A1": testFramework.BranchIDs("A"),
			"B1": testFramework.BranchIDs("B"),
			"C1": testFramework.BranchIDs("C"),
			"D1": testFramework.BranchIDs("D"),
		})
	}

	// ISSUE A2
	{
		testFramework.CreateMessage("A2", WithStrongParents("A1"), WithInputs("A1"), WithOutput("A2", 500))

		testFramework.IssueMessages("A2").WaitUntilAllTasksProcessed()

		checkMarkers(t, testFramework, map[string]*markers.Markers{
			"A1": markers.NewMarkers(markers.NewMarker(0, 1)),
			"B1": markers.NewMarkers(markers.NewMarker(0, 0)),
			"C1": markers.NewMarkers(markers.NewMarker(0, 0)),
			"D1": markers.NewMarkers(markers.NewMarker(0, 0)),
			"A2": markers.NewMarkers(markers.NewMarker(0, 2)),
		})
		checkMessageMetadataDiffBranchIDs(t, testFramework, map[string][]branchdag.BranchIDs{
			"A1": {branchdag.NewBranchIDs(), branchdag.NewBranchIDs()},
			"B1": {testFramework.BranchIDs("B"), branchdag.NewBranchIDs()},
			"C1": {testFramework.BranchIDs("C"), branchdag.NewBranchIDs()},
			"D1": {testFramework.BranchIDs("D"), branchdag.NewBranchIDs()},
			"A2": {branchdag.NewBranchIDs(), branchdag.NewBranchIDs()},
		})
		checkBranchIDs(t, testFramework, map[string]branchdag.BranchIDs{
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

		testFramework.IssueMessages("A3").WaitUntilAllTasksProcessed()

		checkMarkers(t, testFramework, map[string]*markers.Markers{
			"A1": markers.NewMarkers(markers.NewMarker(0, 1)),
			"B1": markers.NewMarkers(markers.NewMarker(0, 0)),
			"C1": markers.NewMarkers(markers.NewMarker(0, 0)),
			"D1": markers.NewMarkers(markers.NewMarker(0, 0)),
			"A2": markers.NewMarkers(markers.NewMarker(0, 2)),
			"A3": markers.NewMarkers(markers.NewMarker(0, 3)),
		})
		checkMessageMetadataDiffBranchIDs(t, testFramework, map[string][]branchdag.BranchIDs{
			"A1": {branchdag.NewBranchIDs(), branchdag.NewBranchIDs()},
			"B1": {testFramework.BranchIDs("B"), branchdag.NewBranchIDs()},
			"C1": {testFramework.BranchIDs("C"), branchdag.NewBranchIDs()},
			"D1": {testFramework.BranchIDs("D"), branchdag.NewBranchIDs()},
			"A2": {branchdag.NewBranchIDs(), branchdag.NewBranchIDs()},
			"A3": {branchdag.NewBranchIDs(), branchdag.NewBranchIDs()},
		})
		checkBranchIDs(t, testFramework, map[string]branchdag.BranchIDs{
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

		testFramework.IssueMessages("A+C1").WaitUntilAllTasksProcessed()

		checkMarkers(t, testFramework, map[string]*markers.Markers{
			"A1":   markers.NewMarkers(markers.NewMarker(0, 1)),
			"B1":   markers.NewMarkers(markers.NewMarker(0, 0)),
			"C1":   markers.NewMarkers(markers.NewMarker(0, 0)),
			"D1":   markers.NewMarkers(markers.NewMarker(0, 0)),
			"A2":   markers.NewMarkers(markers.NewMarker(0, 2)),
			"A3":   markers.NewMarkers(markers.NewMarker(0, 3)),
			"A+C1": markers.NewMarkers(markers.NewMarker(0, 4)),
		})
		checkMessageMetadataDiffBranchIDs(t, testFramework, map[string][]branchdag.BranchIDs{
			"A1":   {branchdag.NewBranchIDs(), branchdag.NewBranchIDs()},
			"B1":   {testFramework.BranchIDs("B"), branchdag.NewBranchIDs()},
			"C1":   {testFramework.BranchIDs("C"), branchdag.NewBranchIDs()},
			"D1":   {testFramework.BranchIDs("D"), branchdag.NewBranchIDs()},
			"A2":   {branchdag.NewBranchIDs(), branchdag.NewBranchIDs()},
			"A3":   {branchdag.NewBranchIDs(), branchdag.NewBranchIDs()},
			"A+C1": {branchdag.NewBranchIDs(), branchdag.NewBranchIDs()},
		})
		checkBranchIDs(t, testFramework, map[string]branchdag.BranchIDs{
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

		testFramework.IssueMessages("A+C2").WaitUntilAllTasksProcessed()

		checkMarkers(t, testFramework, map[string]*markers.Markers{
			"A1":   markers.NewMarkers(markers.NewMarker(0, 1)),
			"B1":   markers.NewMarkers(markers.NewMarker(0, 0)),
			"C1":   markers.NewMarkers(markers.NewMarker(0, 0)),
			"D1":   markers.NewMarkers(markers.NewMarker(0, 0)),
			"A2":   markers.NewMarkers(markers.NewMarker(0, 2)),
			"A3":   markers.NewMarkers(markers.NewMarker(0, 3)),
			"A+C1": markers.NewMarkers(markers.NewMarker(0, 4)),
			"A+C2": markers.NewMarkers(markers.NewMarker(0, 0), markers.NewMarker(0, 3)),
		})
		checkMessageMetadataDiffBranchIDs(t, testFramework, map[string][]branchdag.BranchIDs{
			"A1":   {branchdag.NewBranchIDs(), branchdag.NewBranchIDs()},
			"B1":   {testFramework.BranchIDs("B"), branchdag.NewBranchIDs()},
			"C1":   {testFramework.BranchIDs("C"), branchdag.NewBranchIDs()},
			"D1":   {testFramework.BranchIDs("D"), branchdag.NewBranchIDs()},
			"A2":   {branchdag.NewBranchIDs(), branchdag.NewBranchIDs()},
			"A3":   {branchdag.NewBranchIDs(), branchdag.NewBranchIDs()},
			"A+C1": {branchdag.NewBranchIDs(), branchdag.NewBranchIDs()},
			"A+C2": {testFramework.BranchIDs("C"), branchdag.NewBranchIDs()},
		})
		checkBranchIDs(t, testFramework, map[string]branchdag.BranchIDs{
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

		testFramework.IssueMessages("A2*").WaitUntilAllTasksProcessed()

		checkMarkers(t, testFramework, map[string]*markers.Markers{
			"A1":   markers.NewMarkers(markers.NewMarker(0, 1)),
			"B1":   markers.NewMarkers(markers.NewMarker(0, 0)),
			"C1":   markers.NewMarkers(markers.NewMarker(0, 0)),
			"D1":   markers.NewMarkers(markers.NewMarker(0, 0)),
			"A2":   markers.NewMarkers(markers.NewMarker(0, 2)),
			"A3":   markers.NewMarkers(markers.NewMarker(0, 3)),
			"A+C1": markers.NewMarkers(markers.NewMarker(0, 4)),
			"A+C2": markers.NewMarkers(markers.NewMarker(0, 0), markers.NewMarker(0, 3)),
			"A2*":  markers.NewMarkers(markers.NewMarker(0, 1)),
		})
		checkMessageMetadataDiffBranchIDs(t, testFramework, map[string][]branchdag.BranchIDs{
			"A1":   {branchdag.NewBranchIDs(), branchdag.NewBranchIDs()},
			"B1":   {testFramework.BranchIDs("B"), branchdag.NewBranchIDs()},
			"C1":   {testFramework.BranchIDs("C"), branchdag.NewBranchIDs()},
			"D1":   {testFramework.BranchIDs("D"), branchdag.NewBranchIDs()},
			"A2":   {branchdag.NewBranchIDs(), branchdag.NewBranchIDs()},
			"A3":   {branchdag.NewBranchIDs(), branchdag.NewBranchIDs()},
			"A+C1": {branchdag.NewBranchIDs(), branchdag.NewBranchIDs()},
			"A+C2": {testFramework.BranchIDs("C"), branchdag.NewBranchIDs()},
			"A2*":  {testFramework.BranchIDs("A2*"), branchdag.NewBranchIDs()},
		})
		checkBranchIDs(t, testFramework, map[string]branchdag.BranchIDs{
			"A1":   testFramework.BranchIDs("A"),
			"B1":   testFramework.BranchIDs("B"),
			"C1":   testFramework.BranchIDs("C"),
			"D1":   testFramework.BranchIDs("D"),
			"A2":   testFramework.BranchIDs("A2"),
			"A3":   testFramework.BranchIDs("A2"),
			"A+C1": testFramework.BranchIDs("A2", "C"),
			"A+C2": testFramework.BranchIDs("A2", "C"),
			"A2*":  testFramework.BranchIDs("A", "A2*"),
		})
	}
}

// Please refer to packages/tangle/images/TestBookerMarkerMappings.html for a diagram of this test.
func TestBookerMarkerMappings(t *testing.T) {
	tangle := NewTestTangle()

	testFramework := NewMessageTestFramework(
		tangle,
		WithGenesisOutput("A", 500),
		WithGenesisOutput("B", 500),
		WithGenesisOutput("C", 500),
		WithGenesisOutput("L", 500),
		WithGenesisOutput("L2", 500),
	)

	tangle.Setup()

	// ISSUE Message1
	{
		testFramework.CreateMessage("Message1", WithStrongParents("Genesis"), WithInputs("A"), WithOutput("G", 500))
		testFramework.IssueMessages("Message1").WaitUntilAllTasksProcessed()

		checkMarkers(t, testFramework, map[string]*markers.Markers{
			"Message1": markers.NewMarkers(markers.NewMarker(0, 1)),
		})
		checkMessageMetadataDiffBranchIDs(t, testFramework, map[string][]branchdag.BranchIDs{
			"Message1": {branchdag.NewBranchIDs(), branchdag.NewBranchIDs()},
		})
		checkBranchIDs(t, testFramework, map[string]branchdag.BranchIDs{
			"Message1": branchdag.NewBranchIDs(branchdag.MasterBranchID),
		})
	}

	// ISSUE Message2
	{
		testFramework.CreateMessage("Message2", WithStrongParents("Genesis"), WithInputs("A", "B"), WithOutput("E", 1000))

		testFramework.RegisterBranchID("A", "Message1")
		testFramework.RegisterBranchID("B", "Message2")

		testFramework.IssueMessages("Message2").WaitUntilAllTasksProcessed()

		checkMarkers(t, testFramework, map[string]*markers.Markers{
			"Message1": markers.NewMarkers(markers.NewMarker(0, 1)),
			"Message2": markers.NewMarkers(markers.NewMarker(0, 0)),
		})
		checkMessageMetadataDiffBranchIDs(t, testFramework, map[string][]branchdag.BranchIDs{
			"Message1": {branchdag.NewBranchIDs(), branchdag.NewBranchIDs()},
			"Message2": {testFramework.BranchIDs("B"), branchdag.NewBranchIDs()},
		})
		checkBranchIDs(t, testFramework, map[string]branchdag.BranchIDs{
			"Message1": testFramework.BranchIDs("A"),
			"Message2": testFramework.BranchIDs("B"),
		})
	}

	// ISSUE Message3
	{
		testFramework.CreateMessage("Message3", WithStrongParents("Genesis"), WithInputs("B"), WithOutput("F", 500))

		testFramework.RegisterBranchID("C", "Message3")

		testFramework.IssueMessages("Message3").WaitUntilAllTasksProcessed()

		checkMarkers(t, testFramework, map[string]*markers.Markers{
			"Message1": markers.NewMarkers(markers.NewMarker(0, 1)),
			"Message2": markers.NewMarkers(markers.NewMarker(0, 0)),
			"Message3": markers.NewMarkers(markers.NewMarker(0, 0)),
		})
		checkMessageMetadataDiffBranchIDs(t, testFramework, map[string][]branchdag.BranchIDs{
			"Message1": {branchdag.NewBranchIDs(), branchdag.NewBranchIDs()},
			"Message2": {testFramework.BranchIDs("B"), branchdag.NewBranchIDs()},
			"Message3": {testFramework.BranchIDs("C"), branchdag.NewBranchIDs()},
		})
		checkBranchIDs(t, testFramework, map[string]branchdag.BranchIDs{
			"Message1": testFramework.BranchIDs("A"),
			"Message2": testFramework.BranchIDs("B"),
			"Message3": testFramework.BranchIDs("C"),
		})
	}

	// ISSUE Message4
	{
		testFramework.CreateMessage("Message4", WithStrongParents("Genesis"), WithInputs("L"), WithOutput("K", 500))
		testFramework.IssueMessages("Message4").WaitUntilAllTasksProcessed()

		checkMarkers(t, testFramework, map[string]*markers.Markers{
			"Message1": markers.NewMarkers(markers.NewMarker(0, 1)),
			"Message2": markers.NewMarkers(markers.NewMarker(0, 0)),
			"Message3": markers.NewMarkers(markers.NewMarker(0, 0)),
			"Message4": markers.NewMarkers(markers.NewMarker(0, 0)),
		})
		checkMessageMetadataDiffBranchIDs(t, testFramework, map[string][]branchdag.BranchIDs{
			"Message1": {branchdag.NewBranchIDs(), branchdag.NewBranchIDs()},
			"Message2": {testFramework.BranchIDs("B"), branchdag.NewBranchIDs()},
			"Message3": {testFramework.BranchIDs("C"), branchdag.NewBranchIDs()},
			"Message4": {branchdag.NewBranchIDs(), branchdag.NewBranchIDs()},
		})
		checkBranchIDs(t, testFramework, map[string]branchdag.BranchIDs{
			"Message1": testFramework.BranchIDs("A"),
			"Message2": testFramework.BranchIDs("B"),
			"Message3": testFramework.BranchIDs("C"),
			"Message4": branchdag.NewBranchIDs(branchdag.MasterBranchID),
		})
	}

	// ISSUE Message5
	{
		testFramework.CreateMessage("Message5", WithStrongParents("Message4"), WithInputs("C"), WithOutput("D", 500))
		testFramework.IssueMessages("Message5").WaitUntilAllTasksProcessed()

		checkMarkers(t, testFramework, map[string]*markers.Markers{
			"Message1": markers.NewMarkers(markers.NewMarker(0, 1)),
			"Message2": markers.NewMarkers(markers.NewMarker(0, 0)),
			"Message3": markers.NewMarkers(markers.NewMarker(0, 0)),
			"Message4": markers.NewMarkers(markers.NewMarker(0, 0)),
			"Message5": markers.NewMarkers(markers.NewMarker(0, 0)),
		})
		checkMessageMetadataDiffBranchIDs(t, testFramework, map[string][]branchdag.BranchIDs{
			"Message1": {branchdag.NewBranchIDs(), branchdag.NewBranchIDs()},
			"Message2": {testFramework.BranchIDs("B"), branchdag.NewBranchIDs()},
			"Message3": {testFramework.BranchIDs("C"), branchdag.NewBranchIDs()},
			"Message4": {branchdag.NewBranchIDs(), branchdag.NewBranchIDs()},
			"Message5": {branchdag.NewBranchIDs(), branchdag.NewBranchIDs()},
		})
		checkBranchIDs(t, testFramework, map[string]branchdag.BranchIDs{
			"Message1": testFramework.BranchIDs("A"),
			"Message2": testFramework.BranchIDs("B"),
			"Message3": testFramework.BranchIDs("C"),
			"Message4": branchdag.NewBranchIDs(branchdag.MasterBranchID),
			"Message5": branchdag.NewBranchIDs(branchdag.MasterBranchID),
		})
	}

	// ISSUE Message6
	{
		testFramework.CreateMessage("Message6", WithStrongParents("Message1", "Message2"), WithShallowLikeParents("Message2"))
		testFramework.IssueMessages("Message6").WaitUntilAllTasksProcessed()

		checkMarkers(t, testFramework, map[string]*markers.Markers{
			"Message1": markers.NewMarkers(markers.NewMarker(0, 1)),
			"Message2": markers.NewMarkers(markers.NewMarker(0, 0)),
			"Message3": markers.NewMarkers(markers.NewMarker(0, 0)),
			"Message4": markers.NewMarkers(markers.NewMarker(0, 0)),
			"Message5": markers.NewMarkers(markers.NewMarker(0, 0)),
			"Message6": markers.NewMarkers(markers.NewMarker(0, 2)),
		})
		checkMessageMetadataDiffBranchIDs(t, testFramework, map[string][]branchdag.BranchIDs{
			"Message1": {branchdag.NewBranchIDs(), branchdag.NewBranchIDs()},
			"Message2": {testFramework.BranchIDs("B"), branchdag.NewBranchIDs()},
			"Message3": {testFramework.BranchIDs("C"), branchdag.NewBranchIDs()},
			"Message4": {branchdag.NewBranchIDs(), branchdag.NewBranchIDs()},
			"Message5": {branchdag.NewBranchIDs(), branchdag.NewBranchIDs()},
			"Message6": {branchdag.NewBranchIDs(), branchdag.NewBranchIDs()},
		})
		checkBranchIDs(t, testFramework, map[string]branchdag.BranchIDs{
			"Message1": testFramework.BranchIDs("A"),
			"Message2": testFramework.BranchIDs("B"),
			"Message3": testFramework.BranchIDs("C"),
			"Message4": branchdag.NewBranchIDs(branchdag.MasterBranchID),
			"Message5": branchdag.NewBranchIDs(branchdag.MasterBranchID),
			"Message6": testFramework.BranchIDs("B"),
		})
	}

	// ISSUE Message7
	{
		testFramework.CreateMessage("Message7", WithStrongParents("Message6", "Message5"))
		testFramework.IssueMessages("Message7").WaitUntilAllTasksProcessed()

		checkMarkers(t, testFramework, map[string]*markers.Markers{
			"Message1": markers.NewMarkers(markers.NewMarker(0, 1)),
			"Message2": markers.NewMarkers(markers.NewMarker(0, 0)),
			"Message3": markers.NewMarkers(markers.NewMarker(0, 0)),
			"Message4": markers.NewMarkers(markers.NewMarker(0, 0)),
			"Message5": markers.NewMarkers(markers.NewMarker(0, 0)),
			"Message6": markers.NewMarkers(markers.NewMarker(0, 2)),
			"Message7": markers.NewMarkers(markers.NewMarker(0, 3)),
		})
		checkMessageMetadataDiffBranchIDs(t, testFramework, map[string][]branchdag.BranchIDs{
			"Message1": {branchdag.NewBranchIDs(), branchdag.NewBranchIDs()},
			"Message2": {testFramework.BranchIDs("B"), branchdag.NewBranchIDs()},
			"Message3": {testFramework.BranchIDs("C"), branchdag.NewBranchIDs()},
			"Message4": {branchdag.NewBranchIDs(), branchdag.NewBranchIDs()},
			"Message5": {branchdag.NewBranchIDs(), branchdag.NewBranchIDs()},
			"Message6": {branchdag.NewBranchIDs(), branchdag.NewBranchIDs()},
			"Message7": {branchdag.NewBranchIDs(), branchdag.NewBranchIDs()},
		})
		checkBranchIDs(t, testFramework, map[string]branchdag.BranchIDs{
			"Message1": testFramework.BranchIDs("A"),
			"Message2": testFramework.BranchIDs("B"),
			"Message3": testFramework.BranchIDs("C"),
			"Message4": branchdag.NewBranchIDs(branchdag.MasterBranchID),
			"Message5": branchdag.NewBranchIDs(branchdag.MasterBranchID),
			"Message6": testFramework.BranchIDs("B"),
			"Message7": testFramework.BranchIDs("B"),
		})
	}

	// ISSUE Message8
	{
		testFramework.CreateMessage("Message8", WithStrongParents("Message5", "Message7", "Message3"), WithShallowLikeParents("Message1", "Message3"))

		testFramework.IssueMessages("Message8").WaitUntilAllTasksProcessed()

		checkMarkers(t, testFramework, map[string]*markers.Markers{
			"Message1": markers.NewMarkers(markers.NewMarker(0, 1)),
			"Message2": markers.NewMarkers(markers.NewMarker(0, 0)),
			"Message3": markers.NewMarkers(markers.NewMarker(0, 0)),
			"Message4": markers.NewMarkers(markers.NewMarker(0, 0)),
			"Message5": markers.NewMarkers(markers.NewMarker(0, 0)),
			"Message6": markers.NewMarkers(markers.NewMarker(0, 2)),
			"Message7": markers.NewMarkers(markers.NewMarker(0, 3)),
			"Message8": markers.NewMarkers(markers.NewMarker(0, 4)),
		})
		checkMessageMetadataDiffBranchIDs(t, testFramework, map[string][]branchdag.BranchIDs{
			"Message1": {branchdag.NewBranchIDs(), branchdag.NewBranchIDs()},
			"Message2": {testFramework.BranchIDs("B"), branchdag.NewBranchIDs()},
			"Message3": {testFramework.BranchIDs("C"), branchdag.NewBranchIDs()},
			"Message4": {branchdag.NewBranchIDs(), branchdag.NewBranchIDs()},
			"Message5": {branchdag.NewBranchIDs(), branchdag.NewBranchIDs()},
			"Message6": {branchdag.NewBranchIDs(), branchdag.NewBranchIDs()},
			"Message7": {branchdag.NewBranchIDs(), branchdag.NewBranchIDs()},
			"Message8": {branchdag.NewBranchIDs(), branchdag.NewBranchIDs()},
		})
		checkBranchIDs(t, testFramework, map[string]branchdag.BranchIDs{
			"Message1": testFramework.BranchIDs("A"),
			"Message2": testFramework.BranchIDs("B"),
			"Message3": testFramework.BranchIDs("C"),
			"Message4": branchdag.NewBranchIDs(branchdag.MasterBranchID),
			"Message5": branchdag.NewBranchIDs(branchdag.MasterBranchID),
			"Message6": testFramework.BranchIDs("B"),
			"Message7": testFramework.BranchIDs("B"),
			"Message8": testFramework.BranchIDs("A", "C"),
		})
	}

	// ISSUE Message9
	{
		testFramework.CreateMessage("Message9", WithStrongParents("Message1", "Message7", "Message3"), WithShallowLikeParents("Message1", "Message3"), WithInputs("F"), WithOutput("N", 500))
		testFramework.IssueMessages("Message9").WaitUntilAllTasksProcessed()

		checkMarkers(t, testFramework, map[string]*markers.Markers{
			"Message1": markers.NewMarkers(markers.NewMarker(0, 1)),
			"Message2": markers.NewMarkers(markers.NewMarker(0, 0)),
			"Message3": markers.NewMarkers(markers.NewMarker(0, 0)),
			"Message4": markers.NewMarkers(markers.NewMarker(0, 0)),
			"Message5": markers.NewMarkers(markers.NewMarker(0, 0)),
			"Message6": markers.NewMarkers(markers.NewMarker(0, 2)),
			"Message7": markers.NewMarkers(markers.NewMarker(0, 3)),
			"Message8": markers.NewMarkers(markers.NewMarker(0, 4)),
			"Message9": markers.NewMarkers(markers.NewMarker(0, 3)),
		})
		checkMessageMetadataDiffBranchIDs(t, testFramework, map[string][]branchdag.BranchIDs{
			"Message1": {branchdag.NewBranchIDs(), branchdag.NewBranchIDs()},
			"Message2": {testFramework.BranchIDs("B"), branchdag.NewBranchIDs()},
			"Message3": {testFramework.BranchIDs("C"), branchdag.NewBranchIDs()},
			"Message4": {branchdag.NewBranchIDs(), branchdag.NewBranchIDs()},
			"Message5": {branchdag.NewBranchIDs(), branchdag.NewBranchIDs()},
			"Message6": {branchdag.NewBranchIDs(), branchdag.NewBranchIDs()},
			"Message7": {branchdag.NewBranchIDs(), branchdag.NewBranchIDs()},
			"Message8": {branchdag.NewBranchIDs(), branchdag.NewBranchIDs()},
			"Message9": {testFramework.BranchIDs("A", "C"), testFramework.BranchIDs("B")},
		})
		checkBranchIDs(t, testFramework, map[string]branchdag.BranchIDs{
			"Message1": testFramework.BranchIDs("A"),
			"Message2": testFramework.BranchIDs("B"),
			"Message3": testFramework.BranchIDs("C"),
			"Message4": branchdag.NewBranchIDs(branchdag.MasterBranchID),
			"Message5": branchdag.NewBranchIDs(branchdag.MasterBranchID),
			"Message6": testFramework.BranchIDs("B"),
			"Message7": testFramework.BranchIDs("B"),
			"Message8": testFramework.BranchIDs("A", "C"),
			"Message9": testFramework.BranchIDs("A", "C"),
		})
	}

	// ISSUE Message10
	{
		testFramework.CreateMessage("Message10", WithStrongParents("Message9"), WithShallowLikeParents("Message2"))
		testFramework.IssueMessages("Message10").WaitUntilAllTasksProcessed()

		checkMarkers(t, testFramework, map[string]*markers.Markers{
			"Message1":  markers.NewMarkers(markers.NewMarker(0, 1)),
			"Message2":  markers.NewMarkers(markers.NewMarker(0, 0)),
			"Message3":  markers.NewMarkers(markers.NewMarker(0, 0)),
			"Message4":  markers.NewMarkers(markers.NewMarker(0, 0)),
			"Message5":  markers.NewMarkers(markers.NewMarker(0, 0)),
			"Message6":  markers.NewMarkers(markers.NewMarker(0, 2)),
			"Message7":  markers.NewMarkers(markers.NewMarker(0, 3)),
			"Message8":  markers.NewMarkers(markers.NewMarker(0, 4)),
			"Message9":  markers.NewMarkers(markers.NewMarker(0, 3)),
			"Message10": markers.NewMarkers(markers.NewMarker(0, 3)),
		})
		checkMessageMetadataDiffBranchIDs(t, testFramework, map[string][]branchdag.BranchIDs{
			"Message1":  {branchdag.NewBranchIDs(), branchdag.NewBranchIDs()},
			"Message2":  {testFramework.BranchIDs("B"), branchdag.NewBranchIDs()},
			"Message3":  {testFramework.BranchIDs("C"), branchdag.NewBranchIDs()},
			"Message4":  {branchdag.NewBranchIDs(), branchdag.NewBranchIDs()},
			"Message5":  {branchdag.NewBranchIDs(), branchdag.NewBranchIDs()},
			"Message6":  {branchdag.NewBranchIDs(), branchdag.NewBranchIDs()},
			"Message7":  {branchdag.NewBranchIDs(), branchdag.NewBranchIDs()},
			"Message8":  {branchdag.NewBranchIDs(), branchdag.NewBranchIDs()},
			"Message9":  {testFramework.BranchIDs("A", "C"), testFramework.BranchIDs("B")},
			"Message10": {branchdag.NewBranchIDs(), branchdag.NewBranchIDs()},
		})
		checkBranchIDs(t, testFramework, map[string]branchdag.BranchIDs{
			"Message1":  testFramework.BranchIDs("A"),
			"Message2":  testFramework.BranchIDs("B"),
			"Message3":  testFramework.BranchIDs("C"),
			"Message4":  branchdag.NewBranchIDs(branchdag.MasterBranchID),
			"Message5":  branchdag.NewBranchIDs(branchdag.MasterBranchID),
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
		testFramework.IssueMessages("Message11", "Message11.5").WaitUntilAllTasksProcessed()

		checkMarkers(t, testFramework, map[string]*markers.Markers{
			"Message1":    markers.NewMarkers(markers.NewMarker(0, 1)),
			"Message2":    markers.NewMarkers(markers.NewMarker(0, 0)),
			"Message3":    markers.NewMarkers(markers.NewMarker(0, 0)),
			"Message4":    markers.NewMarkers(markers.NewMarker(0, 0)),
			"Message5":    markers.NewMarkers(markers.NewMarker(0, 0)),
			"Message6":    markers.NewMarkers(markers.NewMarker(0, 2)),
			"Message7":    markers.NewMarkers(markers.NewMarker(0, 3)),
			"Message8":    markers.NewMarkers(markers.NewMarker(0, 4)),
			"Message9":    markers.NewMarkers(markers.NewMarker(0, 3)),
			"Message10":   markers.NewMarkers(markers.NewMarker(0, 3)),
			"Message11":   markers.NewMarkers(markers.NewMarker(0, 5)),
			"Message11.5": markers.NewMarkers(markers.NewMarker(0, 3)),
		})
		checkMessageMetadataDiffBranchIDs(t, testFramework, map[string][]branchdag.BranchIDs{
			"Message1":    {branchdag.NewBranchIDs(), branchdag.NewBranchIDs()},
			"Message2":    {testFramework.BranchIDs("B"), branchdag.NewBranchIDs()},
			"Message3":    {testFramework.BranchIDs("C"), branchdag.NewBranchIDs()},
			"Message4":    {branchdag.NewBranchIDs(), branchdag.NewBranchIDs()},
			"Message5":    {branchdag.NewBranchIDs(), branchdag.NewBranchIDs()},
			"Message6":    {branchdag.NewBranchIDs(), branchdag.NewBranchIDs()},
			"Message7":    {branchdag.NewBranchIDs(), branchdag.NewBranchIDs()},
			"Message8":    {branchdag.NewBranchIDs(), branchdag.NewBranchIDs()},
			"Message9":    {testFramework.BranchIDs("A", "C"), testFramework.BranchIDs("B")},
			"Message10":   {branchdag.NewBranchIDs(), branchdag.NewBranchIDs()},
			"Message11":   {branchdag.NewBranchIDs(), branchdag.NewBranchIDs()},
			"Message11.5": {testFramework.BranchIDs("A", "C"), testFramework.BranchIDs("B")},
		})
		checkBranchIDs(t, testFramework, map[string]branchdag.BranchIDs{
			"Message1":    testFramework.BranchIDs("A"),
			"Message2":    testFramework.BranchIDs("B"),
			"Message3":    testFramework.BranchIDs("C"),
			"Message4":    branchdag.NewBranchIDs(branchdag.MasterBranchID),
			"Message5":    branchdag.NewBranchIDs(branchdag.MasterBranchID),
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

		testFramework.IssueMessages("Message12").WaitUntilAllTasksProcessed()

		checkMarkers(t, testFramework, map[string]*markers.Markers{
			"Message1":    markers.NewMarkers(markers.NewMarker(0, 1)),
			"Message2":    markers.NewMarkers(markers.NewMarker(0, 0)),
			"Message3":    markers.NewMarkers(markers.NewMarker(0, 0)),
			"Message4":    markers.NewMarkers(markers.NewMarker(0, 0)),
			"Message5":    markers.NewMarkers(markers.NewMarker(0, 0)),
			"Message6":    markers.NewMarkers(markers.NewMarker(0, 2)),
			"Message7":    markers.NewMarkers(markers.NewMarker(0, 3)),
			"Message8":    markers.NewMarkers(markers.NewMarker(0, 4)),
			"Message9":    markers.NewMarkers(markers.NewMarker(0, 3)),
			"Message10":   markers.NewMarkers(markers.NewMarker(0, 3)),
			"Message11":   markers.NewMarkers(markers.NewMarker(0, 5)),
			"Message11.5": markers.NewMarkers(markers.NewMarker(0, 3)),
			"Message12":   markers.NewMarkers(markers.NewMarker(0, 1)),
		})
		checkMessageMetadataDiffBranchIDs(t, testFramework, map[string][]branchdag.BranchIDs{
			"Message1":    {branchdag.NewBranchIDs(), branchdag.NewBranchIDs()},
			"Message2":    {testFramework.BranchIDs("B"), branchdag.NewBranchIDs()},
			"Message3":    {testFramework.BranchIDs("C"), branchdag.NewBranchIDs()},
			"Message4":    {branchdag.NewBranchIDs(), branchdag.NewBranchIDs()},
			"Message5":    {testFramework.BranchIDs("D"), branchdag.NewBranchIDs()},
			"Message6":    {branchdag.NewBranchIDs(), branchdag.NewBranchIDs()},
			"Message7":    {branchdag.NewBranchIDs(), branchdag.NewBranchIDs()},
			"Message8":    {branchdag.NewBranchIDs(), branchdag.NewBranchIDs()},
			"Message9":    {testFramework.BranchIDs("A", "C"), testFramework.BranchIDs("B")},
			"Message10":   {branchdag.NewBranchIDs(), branchdag.NewBranchIDs()},
			"Message11":   {branchdag.NewBranchIDs(), branchdag.NewBranchIDs()},
			"Message11.5": {testFramework.BranchIDs("A", "C"), testFramework.BranchIDs("B")},
			"Message12":   {testFramework.BranchIDs("E"), branchdag.NewBranchIDs()},
		})
		checkBranchIDs(t, testFramework, map[string]branchdag.BranchIDs{
			"Message1":    testFramework.BranchIDs("A"),
			"Message2":    testFramework.BranchIDs("B"),
			"Message3":    testFramework.BranchIDs("C"),
			"Message4":    branchdag.NewBranchIDs(branchdag.MasterBranchID),
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

		testFramework.IssueMessages("Message13").WaitUntilAllTasksProcessed()

		checkMarkers(t, testFramework, map[string]*markers.Markers{
			"Message1":    markers.NewMarkers(markers.NewMarker(0, 1)),
			"Message2":    markers.NewMarkers(markers.NewMarker(0, 0)),
			"Message3":    markers.NewMarkers(markers.NewMarker(0, 0)),
			"Message4":    markers.NewMarkers(markers.NewMarker(0, 0)),
			"Message5":    markers.NewMarkers(markers.NewMarker(0, 0)),
			"Message6":    markers.NewMarkers(markers.NewMarker(0, 2)),
			"Message7":    markers.NewMarkers(markers.NewMarker(0, 3)),
			"Message8":    markers.NewMarkers(markers.NewMarker(0, 4)),
			"Message9":    markers.NewMarkers(markers.NewMarker(0, 3)),
			"Message10":   markers.NewMarkers(markers.NewMarker(0, 3)),
			"Message11":   markers.NewMarkers(markers.NewMarker(0, 5)),
			"Message11.5": markers.NewMarkers(markers.NewMarker(0, 3)),
			"Message12":   markers.NewMarkers(markers.NewMarker(0, 1)),
			"Message13":   markers.NewMarkers(markers.NewMarker(0, 3)),
		})
		checkMessageMetadataDiffBranchIDs(t, testFramework, map[string][]branchdag.BranchIDs{
			"Message1":    {branchdag.NewBranchIDs(), branchdag.NewBranchIDs()},
			"Message2":    {testFramework.BranchIDs("B"), branchdag.NewBranchIDs()},
			"Message3":    {testFramework.BranchIDs("C"), branchdag.NewBranchIDs()},
			"Message4":    {branchdag.NewBranchIDs(), branchdag.NewBranchIDs()},
			"Message5":    {testFramework.BranchIDs("D"), branchdag.NewBranchIDs()},
			"Message6":    {branchdag.NewBranchIDs(), branchdag.NewBranchIDs()},
			"Message7":    {branchdag.NewBranchIDs(), branchdag.NewBranchIDs()},
			"Message8":    {branchdag.NewBranchIDs(), branchdag.NewBranchIDs()},
			"Message9":    {testFramework.BranchIDs("A", "C"), testFramework.BranchIDs("B")},
			"Message10":   {branchdag.NewBranchIDs(), branchdag.NewBranchIDs()},
			"Message11":   {branchdag.NewBranchIDs(), branchdag.NewBranchIDs()},
			"Message11.5": {testFramework.BranchIDs("A", "C"), testFramework.BranchIDs("B")},
			"Message12":   {testFramework.BranchIDs("E"), branchdag.NewBranchIDs()},
			"Message13":   {testFramework.BranchIDs("E"), testFramework.BranchIDs("D")},
		})
		checkBranchIDs(t, testFramework, map[string]branchdag.BranchIDs{
			"Message1":    testFramework.BranchIDs("A"),
			"Message2":    testFramework.BranchIDs("B"),
			"Message3":    testFramework.BranchIDs("C"),
			"Message4":    branchdag.NewBranchIDs(branchdag.MasterBranchID),
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

		testFramework.IssueMessages("Message13.1").WaitUntilAllTasksProcessed()

		checkMarkers(t, testFramework, map[string]*markers.Markers{
			"Message1":    markers.NewMarkers(markers.NewMarker(0, 1)),
			"Message2":    markers.NewMarkers(markers.NewMarker(0, 0)),
			"Message3":    markers.NewMarkers(markers.NewMarker(0, 0)),
			"Message4":    markers.NewMarkers(markers.NewMarker(0, 0)),
			"Message5":    markers.NewMarkers(markers.NewMarker(0, 0)),
			"Message6":    markers.NewMarkers(markers.NewMarker(0, 2)),
			"Message7":    markers.NewMarkers(markers.NewMarker(0, 3)),
			"Message8":    markers.NewMarkers(markers.NewMarker(0, 4)),
			"Message9":    markers.NewMarkers(markers.NewMarker(0, 3)),
			"Message10":   markers.NewMarkers(markers.NewMarker(0, 3)),
			"Message11":   markers.NewMarkers(markers.NewMarker(0, 5)),
			"Message11.5": markers.NewMarkers(markers.NewMarker(0, 3)),
			"Message12":   markers.NewMarkers(markers.NewMarker(0, 1)),
			"Message13":   markers.NewMarkers(markers.NewMarker(0, 3)),
			"Message13.1": markers.NewMarkers(markers.NewMarker(0, 3)),
		})
		checkMessageMetadataDiffBranchIDs(t, testFramework, map[string][]branchdag.BranchIDs{
			"Message1":    {branchdag.NewBranchIDs(), branchdag.NewBranchIDs()},
			"Message2":    {testFramework.BranchIDs("B"), branchdag.NewBranchIDs()},
			"Message3":    {testFramework.BranchIDs("C"), branchdag.NewBranchIDs()},
			"Message4":    {branchdag.NewBranchIDs(), branchdag.NewBranchIDs()},
			"Message5":    {testFramework.BranchIDs("D"), branchdag.NewBranchIDs()},
			"Message6":    {branchdag.NewBranchIDs(), branchdag.NewBranchIDs()},
			"Message7":    {branchdag.NewBranchIDs(), branchdag.NewBranchIDs()},
			"Message8":    {branchdag.NewBranchIDs(), branchdag.NewBranchIDs()},
			"Message9":    {testFramework.BranchIDs("A", "C"), testFramework.BranchIDs("B")},
			"Message10":   {branchdag.NewBranchIDs(), branchdag.NewBranchIDs()},
			"Message11":   {branchdag.NewBranchIDs(), branchdag.NewBranchIDs()},
			"Message11.5": {testFramework.BranchIDs("A", "C"), testFramework.BranchIDs("B")},
			"Message12":   {testFramework.BranchIDs("E"), branchdag.NewBranchIDs()},
			"Message13":   {testFramework.BranchIDs("E"), testFramework.BranchIDs("D")},
			"Message13.1": {testFramework.BranchIDs("E"), testFramework.BranchIDs("D")},
		})
		checkBranchIDs(t, testFramework, map[string]branchdag.BranchIDs{
			"Message1":    testFramework.BranchIDs("A"),
			"Message2":    testFramework.BranchIDs("B"),
			"Message3":    testFramework.BranchIDs("C"),
			"Message4":    branchdag.NewBranchIDs(branchdag.MasterBranchID),
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
		testFramework.IssueMessages("Message14").WaitUntilAllTasksProcessed()

		checkMarkers(t, testFramework, map[string]*markers.Markers{
			"Message1":    markers.NewMarkers(markers.NewMarker(0, 1)),
			"Message2":    markers.NewMarkers(markers.NewMarker(0, 0)),
			"Message3":    markers.NewMarkers(markers.NewMarker(0, 0)),
			"Message4":    markers.NewMarkers(markers.NewMarker(0, 0)),
			"Message5":    markers.NewMarkers(markers.NewMarker(0, 0)),
			"Message6":    markers.NewMarkers(markers.NewMarker(0, 2)),
			"Message7":    markers.NewMarkers(markers.NewMarker(0, 3)),
			"Message8":    markers.NewMarkers(markers.NewMarker(0, 4)),
			"Message9":    markers.NewMarkers(markers.NewMarker(0, 3)),
			"Message10":   markers.NewMarkers(markers.NewMarker(0, 3)),
			"Message11":   markers.NewMarkers(markers.NewMarker(0, 5)),
			"Message11.5": markers.NewMarkers(markers.NewMarker(0, 3)),
			"Message12":   markers.NewMarkers(markers.NewMarker(0, 1)),
			"Message13":   markers.NewMarkers(markers.NewMarker(0, 3)),
			"Message13.1": markers.NewMarkers(markers.NewMarker(0, 3)),
			"Message14":   markers.NewMarkers(markers.NewMarker(0, 3)),
		})
		checkMessageMetadataDiffBranchIDs(t, testFramework, map[string][]branchdag.BranchIDs{
			"Message1":    {branchdag.NewBranchIDs(), branchdag.NewBranchIDs()},
			"Message2":    {testFramework.BranchIDs("B"), branchdag.NewBranchIDs()},
			"Message3":    {testFramework.BranchIDs("C"), branchdag.NewBranchIDs()},
			"Message4":    {branchdag.NewBranchIDs(), branchdag.NewBranchIDs()},
			"Message5":    {testFramework.BranchIDs("D"), branchdag.NewBranchIDs()},
			"Message6":    {branchdag.NewBranchIDs(), branchdag.NewBranchIDs()},
			"Message7":    {branchdag.NewBranchIDs(), branchdag.NewBranchIDs()},
			"Message8":    {branchdag.NewBranchIDs(), branchdag.NewBranchIDs()},
			"Message9":    {testFramework.BranchIDs("A", "C"), testFramework.BranchIDs("B")},
			"Message10":   {branchdag.NewBranchIDs(), branchdag.NewBranchIDs()},
			"Message11":   {branchdag.NewBranchIDs(), branchdag.NewBranchIDs()},
			"Message11.5": {testFramework.BranchIDs("A", "C"), testFramework.BranchIDs("B")},
			"Message12":   {testFramework.BranchIDs("E"), branchdag.NewBranchIDs()},
			"Message13":   {testFramework.BranchIDs("E"), testFramework.BranchIDs("D")},
			"Message13.1": {testFramework.BranchIDs("E"), testFramework.BranchIDs("D")},
			"Message14":   {testFramework.BranchIDs("E"), testFramework.BranchIDs("D")},
		})
		checkBranchIDs(t, testFramework, map[string]branchdag.BranchIDs{
			"Message1":    testFramework.BranchIDs("A"),
			"Message2":    testFramework.BranchIDs("B"),
			"Message3":    testFramework.BranchIDs("C"),
			"Message4":    branchdag.NewBranchIDs(branchdag.MasterBranchID),
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

		testFramework.IssueMessages("Message15").WaitUntilAllTasksProcessed()

		checkMarkers(t, testFramework, map[string]*markers.Markers{
			"Message1":    markers.NewMarkers(markers.NewMarker(0, 1)),
			"Message2":    markers.NewMarkers(markers.NewMarker(0, 0)),
			"Message3":    markers.NewMarkers(markers.NewMarker(0, 0)),
			"Message4":    markers.NewMarkers(markers.NewMarker(0, 0)),
			"Message5":    markers.NewMarkers(markers.NewMarker(0, 0)),
			"Message6":    markers.NewMarkers(markers.NewMarker(0, 2)),
			"Message7":    markers.NewMarkers(markers.NewMarker(0, 3)),
			"Message8":    markers.NewMarkers(markers.NewMarker(0, 4)),
			"Message9":    markers.NewMarkers(markers.NewMarker(0, 3)),
			"Message10":   markers.NewMarkers(markers.NewMarker(0, 3)),
			"Message11":   markers.NewMarkers(markers.NewMarker(0, 5)),
			"Message11.5": markers.NewMarkers(markers.NewMarker(0, 3)),
			"Message12":   markers.NewMarkers(markers.NewMarker(0, 1)),
			"Message13":   markers.NewMarkers(markers.NewMarker(0, 3)),
			"Message13.1": markers.NewMarkers(markers.NewMarker(0, 3)),
			"Message14":   markers.NewMarkers(markers.NewMarker(0, 3)),
			"Message15":   markers.NewMarkers(markers.NewMarker(0, 3)),
		})
		checkMessageMetadataDiffBranchIDs(t, testFramework, map[string][]branchdag.BranchIDs{
			"Message1":    {branchdag.NewBranchIDs(), branchdag.NewBranchIDs()},
			"Message2":    {testFramework.BranchIDs("B"), branchdag.NewBranchIDs()},
			"Message3":    {testFramework.BranchIDs("C"), branchdag.NewBranchIDs()},
			"Message4":    {branchdag.NewBranchIDs(), branchdag.NewBranchIDs()},
			"Message5":    {testFramework.BranchIDs("D"), branchdag.NewBranchIDs()},
			"Message6":    {branchdag.NewBranchIDs(), branchdag.NewBranchIDs()},
			"Message7":    {branchdag.NewBranchIDs(), branchdag.NewBranchIDs()},
			"Message8":    {branchdag.NewBranchIDs(), branchdag.NewBranchIDs()},
			"Message9":    {testFramework.BranchIDs("A", "C"), testFramework.BranchIDs("B")},
			"Message10":   {branchdag.NewBranchIDs(), branchdag.NewBranchIDs()},
			"Message11":   {branchdag.NewBranchIDs(), branchdag.NewBranchIDs()},
			"Message11.5": {testFramework.BranchIDs("A", "C"), testFramework.BranchIDs("B")},
			"Message12":   {testFramework.BranchIDs("E"), branchdag.NewBranchIDs()},
			"Message13":   {testFramework.BranchIDs("E"), testFramework.BranchIDs("D")},
			"Message13.1": {testFramework.BranchIDs("E"), testFramework.BranchIDs("D")},
			"Message14":   {testFramework.BranchIDs("E"), testFramework.BranchIDs("D")},
			"Message15":   {branchdag.NewBranchIDs(), testFramework.BranchIDs("B", "D")},
		})
		checkBranchIDs(t, testFramework, map[string]branchdag.BranchIDs{
			"Message1":    testFramework.BranchIDs("A"),
			"Message2":    testFramework.BranchIDs("B"),
			"Message3":    testFramework.BranchIDs("C"),
			"Message4":    branchdag.NewBranchIDs(branchdag.MasterBranchID),
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
			"Message15":   branchdag.NewBranchIDs(branchdag.MasterBranchID),
		})
	}

	// ISSUE Message16
	{
		testFramework.CreateMessage("Message16", WithStrongParents("Message12"), WithInputs("H"), WithOutput("Z", 500))

		testFramework.IssueMessages("Message16").WaitUntilAllTasksProcessed()

		checkMarkers(t, testFramework, map[string]*markers.Markers{
			"Message1":    markers.NewMarkers(markers.NewMarker(0, 1)),
			"Message2":    markers.NewMarkers(markers.NewMarker(0, 0)),
			"Message3":    markers.NewMarkers(markers.NewMarker(0, 0)),
			"Message4":    markers.NewMarkers(markers.NewMarker(0, 0)),
			"Message5":    markers.NewMarkers(markers.NewMarker(0, 0)),
			"Message6":    markers.NewMarkers(markers.NewMarker(0, 2)),
			"Message7":    markers.NewMarkers(markers.NewMarker(0, 3)),
			"Message8":    markers.NewMarkers(markers.NewMarker(0, 4)),
			"Message9":    markers.NewMarkers(markers.NewMarker(0, 3)),
			"Message10":   markers.NewMarkers(markers.NewMarker(0, 3)),
			"Message11":   markers.NewMarkers(markers.NewMarker(0, 5)),
			"Message11.5": markers.NewMarkers(markers.NewMarker(0, 3)),
			"Message12":   markers.NewMarkers(markers.NewMarker(0, 1)),
			"Message13":   markers.NewMarkers(markers.NewMarker(0, 3)),
			"Message13.1": markers.NewMarkers(markers.NewMarker(0, 3)),
			"Message14":   markers.NewMarkers(markers.NewMarker(0, 3)),
			"Message15":   markers.NewMarkers(markers.NewMarker(0, 3)),
			"Message16":   markers.NewMarkers(markers.NewMarker(0, 1)),
		})
		checkMessageMetadataDiffBranchIDs(t, testFramework, map[string][]branchdag.BranchIDs{
			"Message1":    {branchdag.NewBranchIDs(), branchdag.NewBranchIDs()},
			"Message2":    {testFramework.BranchIDs("B"), branchdag.NewBranchIDs()},
			"Message3":    {testFramework.BranchIDs("C"), branchdag.NewBranchIDs()},
			"Message4":    {branchdag.NewBranchIDs(), branchdag.NewBranchIDs()},
			"Message5":    {testFramework.BranchIDs("D"), branchdag.NewBranchIDs()},
			"Message6":    {branchdag.NewBranchIDs(), branchdag.NewBranchIDs()},
			"Message7":    {branchdag.NewBranchIDs(), branchdag.NewBranchIDs()},
			"Message8":    {branchdag.NewBranchIDs(), branchdag.NewBranchIDs()},
			"Message9":    {testFramework.BranchIDs("A", "C"), testFramework.BranchIDs("B")},
			"Message10":   {branchdag.NewBranchIDs(), branchdag.NewBranchIDs()},
			"Message11":   {branchdag.NewBranchIDs(), branchdag.NewBranchIDs()},
			"Message11.5": {testFramework.BranchIDs("A", "C"), testFramework.BranchIDs("B")},
			"Message12":   {testFramework.BranchIDs("E"), branchdag.NewBranchIDs()},
			"Message13":   {testFramework.BranchIDs("E"), testFramework.BranchIDs("D")},
			"Message13.1": {testFramework.BranchIDs("E"), testFramework.BranchIDs("D")},
			"Message14":   {testFramework.BranchIDs("E"), testFramework.BranchIDs("D")},
			"Message15":   {branchdag.NewBranchIDs(), testFramework.BranchIDs("B", "D")},
			"Message16":   {testFramework.BranchIDs("E"), branchdag.NewBranchIDs()},
		})
		checkBranchIDs(t, testFramework, map[string]branchdag.BranchIDs{
			"Message1":    testFramework.BranchIDs("A"),
			"Message2":    testFramework.BranchIDs("B"),
			"Message3":    testFramework.BranchIDs("C"),
			"Message4":    branchdag.NewBranchIDs(branchdag.MasterBranchID),
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
			"Message15":   branchdag.NewBranchIDs(branchdag.MasterBranchID),
			"Message16":   testFramework.BranchIDs("A", "E"),
		})
	}

	// ISSUE Message17
	{
		testFramework.CreateMessage("Message17", WithStrongParents("Message12"), WithInputs("H"), WithOutput("Y", 500))

		testFramework.RegisterBranchID("Z", "Message16")
		testFramework.RegisterBranchID("Y", "Message17")

		testFramework.IssueMessages("Message17").WaitUntilAllTasksProcessed()

		checkMarkers(t, testFramework, map[string]*markers.Markers{
			"Message1":    markers.NewMarkers(markers.NewMarker(0, 1)),
			"Message2":    markers.NewMarkers(markers.NewMarker(0, 0)),
			"Message3":    markers.NewMarkers(markers.NewMarker(0, 0)),
			"Message4":    markers.NewMarkers(markers.NewMarker(0, 0)),
			"Message5":    markers.NewMarkers(markers.NewMarker(0, 0)),
			"Message6":    markers.NewMarkers(markers.NewMarker(0, 2)),
			"Message7":    markers.NewMarkers(markers.NewMarker(0, 3)),
			"Message8":    markers.NewMarkers(markers.NewMarker(0, 4)),
			"Message9":    markers.NewMarkers(markers.NewMarker(0, 3)),
			"Message10":   markers.NewMarkers(markers.NewMarker(0, 3)),
			"Message11":   markers.NewMarkers(markers.NewMarker(0, 5)),
			"Message11.5": markers.NewMarkers(markers.NewMarker(0, 3)),
			"Message12":   markers.NewMarkers(markers.NewMarker(0, 1)),
			"Message13":   markers.NewMarkers(markers.NewMarker(0, 3)),
			"Message13.1": markers.NewMarkers(markers.NewMarker(0, 3)),
			"Message14":   markers.NewMarkers(markers.NewMarker(0, 3)),
			"Message15":   markers.NewMarkers(markers.NewMarker(0, 3)),
			"Message16":   markers.NewMarkers(markers.NewMarker(0, 1)),
			"Message17":   markers.NewMarkers(markers.NewMarker(0, 1)),
		})
		checkMessageMetadataDiffBranchIDs(t, testFramework, map[string][]branchdag.BranchIDs{
			"Message1":    {branchdag.NewBranchIDs(), branchdag.NewBranchIDs()},
			"Message2":    {testFramework.BranchIDs("B"), branchdag.NewBranchIDs()},
			"Message3":    {testFramework.BranchIDs("C"), branchdag.NewBranchIDs()},
			"Message4":    {branchdag.NewBranchIDs(), branchdag.NewBranchIDs()},
			"Message5":    {testFramework.BranchIDs("D"), branchdag.NewBranchIDs()},
			"Message6":    {branchdag.NewBranchIDs(), branchdag.NewBranchIDs()},
			"Message7":    {branchdag.NewBranchIDs(), branchdag.NewBranchIDs()},
			"Message8":    {branchdag.NewBranchIDs(), branchdag.NewBranchIDs()},
			"Message9":    {testFramework.BranchIDs("A", "C"), testFramework.BranchIDs("B")},
			"Message10":   {branchdag.NewBranchIDs(), branchdag.NewBranchIDs()},
			"Message11":   {branchdag.NewBranchIDs(), branchdag.NewBranchIDs()},
			"Message11.5": {testFramework.BranchIDs("A", "C"), testFramework.BranchIDs("B")},
			"Message12":   {testFramework.BranchIDs("E"), branchdag.NewBranchIDs()},
			"Message13":   {testFramework.BranchIDs("E"), testFramework.BranchIDs("D")},
			"Message13.1": {testFramework.BranchIDs("E"), testFramework.BranchIDs("D")},
			"Message14":   {testFramework.BranchIDs("E"), testFramework.BranchIDs("D")},
			"Message15":   {branchdag.NewBranchIDs(), testFramework.BranchIDs("B", "D")},
			"Message16":   {testFramework.BranchIDs("Z"), branchdag.NewBranchIDs()},
			"Message17":   {testFramework.BranchIDs("E", "Y"), branchdag.NewBranchIDs()},
		})
		checkBranchIDs(t, testFramework, map[string]branchdag.BranchIDs{
			"Message1":    testFramework.BranchIDs("A"),
			"Message2":    testFramework.BranchIDs("B"),
			"Message3":    testFramework.BranchIDs("C"),
			"Message4":    branchdag.NewBranchIDs(branchdag.MasterBranchID),
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
			"Message15":   branchdag.NewBranchIDs(branchdag.MasterBranchID),
			"Message16":   testFramework.BranchIDs("Z", "A"),
			"Message17":   testFramework.BranchIDs("Y", "A", "E"),
		})
	}

	// ISSUE Message18
	{
		msg := testFramework.CreateMessage("Message18", WithStrongParents("Message17", "Message7"))

		testFramework.IssueMessages("Message18").WaitUntilAllTasksProcessed()

		tangle.Storage.MessageMetadata(msg.ID()).Consume(func(messageMetadata *MessageMetadata) {
			assert.True(t, messageMetadata.subjectivelyInvalid)
		})

		checkMarkers(t, testFramework, map[string]*markers.Markers{
			"Message1":    markers.NewMarkers(markers.NewMarker(0, 1)),
			"Message2":    markers.NewMarkers(markers.NewMarker(0, 0)),
			"Message3":    markers.NewMarkers(markers.NewMarker(0, 0)),
			"Message4":    markers.NewMarkers(markers.NewMarker(0, 0)),
			"Message5":    markers.NewMarkers(markers.NewMarker(0, 0)),
			"Message6":    markers.NewMarkers(markers.NewMarker(0, 2)),
			"Message7":    markers.NewMarkers(markers.NewMarker(0, 3)),
			"Message8":    markers.NewMarkers(markers.NewMarker(0, 4)),
			"Message9":    markers.NewMarkers(markers.NewMarker(0, 3)),
			"Message10":   markers.NewMarkers(markers.NewMarker(0, 3)),
			"Message11":   markers.NewMarkers(markers.NewMarker(0, 5)),
			"Message11.5": markers.NewMarkers(markers.NewMarker(0, 3)),
			"Message12":   markers.NewMarkers(markers.NewMarker(0, 1)),
			"Message13":   markers.NewMarkers(markers.NewMarker(0, 3)),
			"Message13.1": markers.NewMarkers(markers.NewMarker(0, 3)),
			"Message14":   markers.NewMarkers(markers.NewMarker(0, 3)),
			"Message15":   markers.NewMarkers(markers.NewMarker(0, 3)),
			"Message16":   markers.NewMarkers(markers.NewMarker(0, 1)),
			"Message17":   markers.NewMarkers(markers.NewMarker(0, 1)),
			"Message18":   markers.NewMarkers(markers.NewMarker(0, 3)),
		})
		checkMessageMetadataDiffBranchIDs(t, testFramework, map[string][]branchdag.BranchIDs{
			"Message1":    {branchdag.NewBranchIDs(), branchdag.NewBranchIDs()},
			"Message2":    {testFramework.BranchIDs("B"), branchdag.NewBranchIDs()},
			"Message3":    {testFramework.BranchIDs("C"), branchdag.NewBranchIDs()},
			"Message4":    {branchdag.NewBranchIDs(), branchdag.NewBranchIDs()},
			"Message5":    {testFramework.BranchIDs("D"), branchdag.NewBranchIDs()},
			"Message6":    {branchdag.NewBranchIDs(), branchdag.NewBranchIDs()},
			"Message7":    {branchdag.NewBranchIDs(), branchdag.NewBranchIDs()},
			"Message8":    {branchdag.NewBranchIDs(), branchdag.NewBranchIDs()},
			"Message9":    {testFramework.BranchIDs("A", "C"), testFramework.BranchIDs("B")},
			"Message10":   {branchdag.NewBranchIDs(), branchdag.NewBranchIDs()},
			"Message11":   {branchdag.NewBranchIDs(), branchdag.NewBranchIDs()},
			"Message11.5": {testFramework.BranchIDs("A", "C"), testFramework.BranchIDs("B")},
			"Message12":   {testFramework.BranchIDs("E"), branchdag.NewBranchIDs()},
			"Message13":   {testFramework.BranchIDs("E"), testFramework.BranchIDs("D")},
			"Message13.1": {testFramework.BranchIDs("E"), testFramework.BranchIDs("D")},
			"Message14":   {testFramework.BranchIDs("E"), testFramework.BranchIDs("D")},
			"Message15":   {branchdag.NewBranchIDs(), testFramework.BranchIDs("B", "D")},
			"Message16":   {testFramework.BranchIDs("Z"), branchdag.NewBranchIDs()},
			"Message17":   {testFramework.BranchIDs("E", "Y"), branchdag.NewBranchIDs()},
			"Message18":   {testFramework.BranchIDs("Y", "A", "E"), branchdag.NewBranchIDs()},
		})
		checkBranchIDs(t, testFramework, map[string]branchdag.BranchIDs{
			"Message1":    testFramework.BranchIDs("A"),
			"Message2":    testFramework.BranchIDs("B"),
			"Message3":    testFramework.BranchIDs("C"),
			"Message4":    branchdag.NewBranchIDs(branchdag.MasterBranchID),
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
			"Message15":   branchdag.NewBranchIDs(branchdag.MasterBranchID),
			"Message16":   testFramework.BranchIDs("Z", "A"),
			"Message17":   testFramework.BranchIDs("Y", "A", "E"),
			"Message18":   testFramework.BranchIDs("Y", "A", "E", "B", "D"),
		})
	}

	// ISSUE Message19
	{
		msg := testFramework.CreateMessage("Message19", WithStrongParents("Message17"), WithWeakParents("Message7"))

		testFramework.IssueMessages("Message19").WaitUntilAllTasksProcessed()

		tangle.Storage.MessageMetadata(msg.ID()).Consume(func(messageMetadata *MessageMetadata) {
			assert.False(t, messageMetadata.subjectivelyInvalid)
		})

		checkMarkers(t, testFramework, map[string]*markers.Markers{
			"Message1":    markers.NewMarkers(markers.NewMarker(0, 1)),
			"Message2":    markers.NewMarkers(markers.NewMarker(0, 0)),
			"Message3":    markers.NewMarkers(markers.NewMarker(0, 0)),
			"Message4":    markers.NewMarkers(markers.NewMarker(0, 0)),
			"Message5":    markers.NewMarkers(markers.NewMarker(0, 0)),
			"Message6":    markers.NewMarkers(markers.NewMarker(0, 2)),
			"Message7":    markers.NewMarkers(markers.NewMarker(0, 3)),
			"Message8":    markers.NewMarkers(markers.NewMarker(0, 4)),
			"Message9":    markers.NewMarkers(markers.NewMarker(0, 3)),
			"Message10":   markers.NewMarkers(markers.NewMarker(0, 3)),
			"Message11":   markers.NewMarkers(markers.NewMarker(0, 5)),
			"Message11.5": markers.NewMarkers(markers.NewMarker(0, 3)),
			"Message12":   markers.NewMarkers(markers.NewMarker(0, 1)),
			"Message13":   markers.NewMarkers(markers.NewMarker(0, 3)),
			"Message13.1": markers.NewMarkers(markers.NewMarker(0, 3)),
			"Message14":   markers.NewMarkers(markers.NewMarker(0, 3)),
			"Message15":   markers.NewMarkers(markers.NewMarker(0, 3)),
			"Message16":   markers.NewMarkers(markers.NewMarker(0, 1)),
			"Message17":   markers.NewMarkers(markers.NewMarker(0, 1)),
			"Message18":   markers.NewMarkers(markers.NewMarker(0, 3)),
			"Message19":   markers.NewMarkers(markers.NewMarker(0, 1)),
		})
		checkMessageMetadataDiffBranchIDs(t, testFramework, map[string][]branchdag.BranchIDs{
			"Message1":    {branchdag.NewBranchIDs(), branchdag.NewBranchIDs()},
			"Message2":    {testFramework.BranchIDs("B"), branchdag.NewBranchIDs()},
			"Message3":    {testFramework.BranchIDs("C"), branchdag.NewBranchIDs()},
			"Message4":    {branchdag.NewBranchIDs(), branchdag.NewBranchIDs()},
			"Message5":    {testFramework.BranchIDs("D"), branchdag.NewBranchIDs()},
			"Message6":    {branchdag.NewBranchIDs(), branchdag.NewBranchIDs()},
			"Message7":    {branchdag.NewBranchIDs(), branchdag.NewBranchIDs()},
			"Message8":    {branchdag.NewBranchIDs(), branchdag.NewBranchIDs()},
			"Message9":    {testFramework.BranchIDs("A", "C"), testFramework.BranchIDs("B")},
			"Message10":   {branchdag.NewBranchIDs(), branchdag.NewBranchIDs()},
			"Message11":   {branchdag.NewBranchIDs(), branchdag.NewBranchIDs()},
			"Message11.5": {testFramework.BranchIDs("A", "C"), testFramework.BranchIDs("B")},
			"Message12":   {testFramework.BranchIDs("E"), branchdag.NewBranchIDs()},
			"Message13":   {testFramework.BranchIDs("E"), testFramework.BranchIDs("D")},
			"Message13.1": {testFramework.BranchIDs("E"), testFramework.BranchIDs("D")},
			"Message14":   {testFramework.BranchIDs("E"), testFramework.BranchIDs("D")},
			"Message15":   {branchdag.NewBranchIDs(), testFramework.BranchIDs("B", "D")},
			"Message16":   {testFramework.BranchIDs("Z"), branchdag.NewBranchIDs()},
			"Message17":   {testFramework.BranchIDs("E", "Y"), branchdag.NewBranchIDs()},
			"Message18":   {testFramework.BranchIDs("Y", "A", "E"), branchdag.NewBranchIDs()},
			"Message19":   {testFramework.BranchIDs("E", "Y"), branchdag.NewBranchIDs()},
		})
		checkBranchIDs(t, testFramework, map[string]branchdag.BranchIDs{
			"Message1":    testFramework.BranchIDs("A"),
			"Message2":    testFramework.BranchIDs("B"),
			"Message3":    testFramework.BranchIDs("C"),
			"Message4":    branchdag.NewBranchIDs(branchdag.MasterBranchID),
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
			"Message15":   branchdag.NewBranchIDs(branchdag.MasterBranchID),
			"Message16":   testFramework.BranchIDs("Z", "A"),
			"Message17":   testFramework.BranchIDs("Y", "A", "E"),
			"Message18":   testFramework.BranchIDs("Y", "A", "E", "B", "D"),
			"Message19":   testFramework.BranchIDs("Y", "A", "E"),
		})
	}

	// ISSUE Message20
	{
		msg := testFramework.CreateMessage("Message20", WithStrongParents("Message17"), WithWeakParents("Message2"))

		testFramework.IssueMessages("Message20").WaitUntilAllTasksProcessed()

		tangle.Storage.MessageMetadata(msg.ID()).Consume(func(messageMetadata *MessageMetadata) {
			assert.True(t, messageMetadata.subjectivelyInvalid)
		})

		checkMarkers(t, testFramework, map[string]*markers.Markers{
			"Message1":    markers.NewMarkers(markers.NewMarker(0, 1)),
			"Message2":    markers.NewMarkers(markers.NewMarker(0, 0)),
			"Message3":    markers.NewMarkers(markers.NewMarker(0, 0)),
			"Message4":    markers.NewMarkers(markers.NewMarker(0, 0)),
			"Message5":    markers.NewMarkers(markers.NewMarker(0, 0)),
			"Message6":    markers.NewMarkers(markers.NewMarker(0, 2)),
			"Message7":    markers.NewMarkers(markers.NewMarker(0, 3)),
			"Message8":    markers.NewMarkers(markers.NewMarker(0, 4)),
			"Message9":    markers.NewMarkers(markers.NewMarker(0, 3)),
			"Message10":   markers.NewMarkers(markers.NewMarker(0, 3)),
			"Message11":   markers.NewMarkers(markers.NewMarker(0, 5)),
			"Message11.5": markers.NewMarkers(markers.NewMarker(0, 3)),
			"Message12":   markers.NewMarkers(markers.NewMarker(0, 1)),
			"Message13":   markers.NewMarkers(markers.NewMarker(0, 3)),
			"Message13.1": markers.NewMarkers(markers.NewMarker(0, 3)),
			"Message14":   markers.NewMarkers(markers.NewMarker(0, 3)),
			"Message15":   markers.NewMarkers(markers.NewMarker(0, 3)),
			"Message16":   markers.NewMarkers(markers.NewMarker(0, 1)),
			"Message17":   markers.NewMarkers(markers.NewMarker(0, 1)),
			"Message18":   markers.NewMarkers(markers.NewMarker(0, 3)),
			"Message19":   markers.NewMarkers(markers.NewMarker(0, 1)),
			"Message20":   markers.NewMarkers(markers.NewMarker(0, 1)),
		})
		checkMessageMetadataDiffBranchIDs(t, testFramework, map[string][]branchdag.BranchIDs{
			"Message1":    {branchdag.NewBranchIDs(), branchdag.NewBranchIDs()},
			"Message2":    {testFramework.BranchIDs("B"), branchdag.NewBranchIDs()},
			"Message3":    {testFramework.BranchIDs("C"), branchdag.NewBranchIDs()},
			"Message4":    {branchdag.NewBranchIDs(), branchdag.NewBranchIDs()},
			"Message5":    {testFramework.BranchIDs("D"), branchdag.NewBranchIDs()},
			"Message6":    {branchdag.NewBranchIDs(), branchdag.NewBranchIDs()},
			"Message7":    {branchdag.NewBranchIDs(), branchdag.NewBranchIDs()},
			"Message8":    {branchdag.NewBranchIDs(), branchdag.NewBranchIDs()},
			"Message9":    {testFramework.BranchIDs("A", "C"), testFramework.BranchIDs("B")},
			"Message10":   {branchdag.NewBranchIDs(), branchdag.NewBranchIDs()},
			"Message11":   {branchdag.NewBranchIDs(), branchdag.NewBranchIDs()},
			"Message11.5": {testFramework.BranchIDs("A", "C"), testFramework.BranchIDs("B")},
			"Message12":   {testFramework.BranchIDs("E"), branchdag.NewBranchIDs()},
			"Message13":   {testFramework.BranchIDs("E"), testFramework.BranchIDs("D")},
			"Message13.1": {testFramework.BranchIDs("E"), testFramework.BranchIDs("D")},
			"Message14":   {testFramework.BranchIDs("E"), testFramework.BranchIDs("D")},
			"Message15":   {branchdag.NewBranchIDs(), testFramework.BranchIDs("B", "D")},
			"Message16":   {testFramework.BranchIDs("Z"), branchdag.NewBranchIDs()},
			"Message17":   {testFramework.BranchIDs("E", "Y"), branchdag.NewBranchIDs()},
			"Message18":   {testFramework.BranchIDs("Y", "A", "E"), branchdag.NewBranchIDs()},
			"Message19":   {testFramework.BranchIDs("E", "Y"), branchdag.NewBranchIDs()},
			"Message20":   {testFramework.BranchIDs("Y", "E", "B"), branchdag.NewBranchIDs()},
		})
		checkBranchIDs(t, testFramework, map[string]branchdag.BranchIDs{
			"Message1":    testFramework.BranchIDs("A"),
			"Message2":    testFramework.BranchIDs("B"),
			"Message3":    testFramework.BranchIDs("C"),
			"Message4":    branchdag.NewBranchIDs(branchdag.MasterBranchID),
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
			"Message15":   branchdag.NewBranchIDs(branchdag.MasterBranchID),
			"Message16":   testFramework.BranchIDs("Z", "A"),
			"Message17":   testFramework.BranchIDs("Y", "A", "E"),
			"Message18":   testFramework.BranchIDs("Y", "A", "E", "B", "D"),
			"Message19":   testFramework.BranchIDs("Y", "A", "E"),
			"Message20":   testFramework.BranchIDs("Y", "A", "E", "B"),
		})
	}
	// ISSUE Message21
	{
		msg := testFramework.CreateMessage("Message21", WithStrongParents("Message17"), WithWeakParents("Message2"), WithShallowDislikeParents("Message12"))

		testFramework.IssueMessages("Message21").WaitUntilAllTasksProcessed()

		tangle.Storage.MessageMetadata(msg.ID()).Consume(func(messageMetadata *MessageMetadata) {
			assert.True(t, messageMetadata.subjectivelyInvalid)
		})

		checkMarkers(t, testFramework, map[string]*markers.Markers{
			"Message1":    markers.NewMarkers(markers.NewMarker(0, 1)),
			"Message2":    markers.NewMarkers(markers.NewMarker(0, 0)),
			"Message3":    markers.NewMarkers(markers.NewMarker(0, 0)),
			"Message4":    markers.NewMarkers(markers.NewMarker(0, 0)),
			"Message5":    markers.NewMarkers(markers.NewMarker(0, 0)),
			"Message6":    markers.NewMarkers(markers.NewMarker(0, 2)),
			"Message7":    markers.NewMarkers(markers.NewMarker(0, 3)),
			"Message8":    markers.NewMarkers(markers.NewMarker(0, 4)),
			"Message9":    markers.NewMarkers(markers.NewMarker(0, 3)),
			"Message10":   markers.NewMarkers(markers.NewMarker(0, 3)),
			"Message11":   markers.NewMarkers(markers.NewMarker(0, 5)),
			"Message11.5": markers.NewMarkers(markers.NewMarker(0, 3)),
			"Message12":   markers.NewMarkers(markers.NewMarker(0, 1)),
			"Message13":   markers.NewMarkers(markers.NewMarker(0, 3)),
			"Message13.1": markers.NewMarkers(markers.NewMarker(0, 3)),
			"Message14":   markers.NewMarkers(markers.NewMarker(0, 3)),
			"Message15":   markers.NewMarkers(markers.NewMarker(0, 3)),
			"Message16":   markers.NewMarkers(markers.NewMarker(0, 1)),
			"Message17":   markers.NewMarkers(markers.NewMarker(0, 1)),
			"Message18":   markers.NewMarkers(markers.NewMarker(0, 3)),
			"Message19":   markers.NewMarkers(markers.NewMarker(0, 1)),
			"Message20":   markers.NewMarkers(markers.NewMarker(0, 1)),
			"Message21":   markers.NewMarkers(markers.NewMarker(0, 1)),
		})
		checkMessageMetadataDiffBranchIDs(t, testFramework, map[string][]branchdag.BranchIDs{
			"Message1":    {branchdag.NewBranchIDs(), branchdag.NewBranchIDs()},
			"Message2":    {testFramework.BranchIDs("B"), branchdag.NewBranchIDs()},
			"Message3":    {testFramework.BranchIDs("C"), branchdag.NewBranchIDs()},
			"Message4":    {branchdag.NewBranchIDs(), branchdag.NewBranchIDs()},
			"Message5":    {testFramework.BranchIDs("D"), branchdag.NewBranchIDs()},
			"Message6":    {branchdag.NewBranchIDs(), branchdag.NewBranchIDs()},
			"Message7":    {branchdag.NewBranchIDs(), branchdag.NewBranchIDs()},
			"Message8":    {branchdag.NewBranchIDs(), branchdag.NewBranchIDs()},
			"Message9":    {testFramework.BranchIDs("A", "C"), testFramework.BranchIDs("B")},
			"Message10":   {branchdag.NewBranchIDs(), branchdag.NewBranchIDs()},
			"Message11":   {branchdag.NewBranchIDs(), branchdag.NewBranchIDs()},
			"Message11.5": {testFramework.BranchIDs("A", "C"), testFramework.BranchIDs("B")},
			"Message12":   {testFramework.BranchIDs("E"), branchdag.NewBranchIDs()},
			"Message13":   {testFramework.BranchIDs("E"), testFramework.BranchIDs("D")},
			"Message13.1": {testFramework.BranchIDs("E"), testFramework.BranchIDs("D")},
			"Message14":   {testFramework.BranchIDs("E"), testFramework.BranchIDs("D")},
			"Message15":   {branchdag.NewBranchIDs(), testFramework.BranchIDs("B", "D")},
			"Message16":   {testFramework.BranchIDs("Z"), branchdag.NewBranchIDs()},
			"Message17":   {testFramework.BranchIDs("E", "Y"), branchdag.NewBranchIDs()},
			"Message18":   {testFramework.BranchIDs("Y", "A", "E"), branchdag.NewBranchIDs()},
			"Message19":   {testFramework.BranchIDs("E", "Y"), branchdag.NewBranchIDs()},
			"Message20":   {testFramework.BranchIDs("Y", "E", "B"), branchdag.NewBranchIDs()},
			"Message21":   {testFramework.BranchIDs("B"), branchdag.NewBranchIDs()},
		})
		checkBranchIDs(t, testFramework, map[string]branchdag.BranchIDs{
			"Message1":    testFramework.BranchIDs("A"),
			"Message2":    testFramework.BranchIDs("B"),
			"Message3":    testFramework.BranchIDs("C"),
			"Message4":    branchdag.NewBranchIDs(branchdag.MasterBranchID),
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
			"Message15":   branchdag.NewBranchIDs(branchdag.MasterBranchID),
			"Message16":   testFramework.BranchIDs("Z", "A"),
			"Message17":   testFramework.BranchIDs("Y", "A", "E"),
			"Message18":   testFramework.BranchIDs("Y", "A", "E", "B", "D"),
			"Message19":   testFramework.BranchIDs("Y", "A", "E"),
			"Message20":   testFramework.BranchIDs("Y", "A", "E", "B"),
			"Message21":   testFramework.BranchIDs("A", "B"),
		})
	}

	// ISSUE Message22
	{
		msg := testFramework.CreateMessage("Message22", WithStrongParents("Message17"), WithWeakParents("Message2"), WithShallowDislikeParents("Message12", "Message1"))

		testFramework.IssueMessages("Message22").WaitUntilAllTasksProcessed()

		tangle.Storage.MessageMetadata(msg.ID()).Consume(func(messageMetadata *MessageMetadata) {
			assert.False(t, messageMetadata.subjectivelyInvalid)
		})

		checkMarkers(t, testFramework, map[string]*markers.Markers{
			"Message1":    markers.NewMarkers(markers.NewMarker(0, 1)),
			"Message2":    markers.NewMarkers(markers.NewMarker(0, 0)),
			"Message3":    markers.NewMarkers(markers.NewMarker(0, 0)),
			"Message4":    markers.NewMarkers(markers.NewMarker(0, 0)),
			"Message5":    markers.NewMarkers(markers.NewMarker(0, 0)),
			"Message6":    markers.NewMarkers(markers.NewMarker(0, 2)),
			"Message7":    markers.NewMarkers(markers.NewMarker(0, 3)),
			"Message8":    markers.NewMarkers(markers.NewMarker(0, 4)),
			"Message9":    markers.NewMarkers(markers.NewMarker(0, 3)),
			"Message10":   markers.NewMarkers(markers.NewMarker(0, 3)),
			"Message11":   markers.NewMarkers(markers.NewMarker(0, 5)),
			"Message11.5": markers.NewMarkers(markers.NewMarker(0, 3)),
			"Message12":   markers.NewMarkers(markers.NewMarker(0, 1)),
			"Message13":   markers.NewMarkers(markers.NewMarker(0, 3)),
			"Message13.1": markers.NewMarkers(markers.NewMarker(0, 3)),
			"Message14":   markers.NewMarkers(markers.NewMarker(0, 3)),
			"Message15":   markers.NewMarkers(markers.NewMarker(0, 3)),
			"Message16":   markers.NewMarkers(markers.NewMarker(0, 1)),
			"Message17":   markers.NewMarkers(markers.NewMarker(0, 1)),
			"Message18":   markers.NewMarkers(markers.NewMarker(0, 3)),
			"Message19":   markers.NewMarkers(markers.NewMarker(0, 1)),
			"Message20":   markers.NewMarkers(markers.NewMarker(0, 1)),
			"Message21":   markers.NewMarkers(markers.NewMarker(0, 1)),
			"Message22":   markers.NewMarkers(markers.NewMarker(0, 1)),
		})
		checkMessageMetadataDiffBranchIDs(t, testFramework, map[string][]branchdag.BranchIDs{
			"Message1":    {branchdag.NewBranchIDs(), branchdag.NewBranchIDs()},
			"Message2":    {testFramework.BranchIDs("B"), branchdag.NewBranchIDs()},
			"Message3":    {testFramework.BranchIDs("C"), branchdag.NewBranchIDs()},
			"Message4":    {branchdag.NewBranchIDs(), branchdag.NewBranchIDs()},
			"Message5":    {testFramework.BranchIDs("D"), branchdag.NewBranchIDs()},
			"Message6":    {branchdag.NewBranchIDs(), branchdag.NewBranchIDs()},
			"Message7":    {branchdag.NewBranchIDs(), branchdag.NewBranchIDs()},
			"Message8":    {branchdag.NewBranchIDs(), branchdag.NewBranchIDs()},
			"Message9":    {testFramework.BranchIDs("A", "C"), testFramework.BranchIDs("B")},
			"Message10":   {branchdag.NewBranchIDs(), branchdag.NewBranchIDs()},
			"Message11":   {branchdag.NewBranchIDs(), branchdag.NewBranchIDs()},
			"Message11.5": {testFramework.BranchIDs("A", "C"), testFramework.BranchIDs("B")},
			"Message12":   {testFramework.BranchIDs("E"), branchdag.NewBranchIDs()},
			"Message13":   {testFramework.BranchIDs("E"), testFramework.BranchIDs("D")},
			"Message13.1": {testFramework.BranchIDs("E"), testFramework.BranchIDs("D")},
			"Message14":   {testFramework.BranchIDs("E"), testFramework.BranchIDs("D")},
			"Message15":   {branchdag.NewBranchIDs(), testFramework.BranchIDs("B", "D")},
			"Message16":   {testFramework.BranchIDs("Z"), branchdag.NewBranchIDs()},
			"Message17":   {testFramework.BranchIDs("E", "Y"), branchdag.NewBranchIDs()},
			"Message18":   {testFramework.BranchIDs("Y", "A", "E"), branchdag.NewBranchIDs()},
			"Message19":   {testFramework.BranchIDs("E", "Y"), branchdag.NewBranchIDs()},
			"Message20":   {testFramework.BranchIDs("Y", "E", "B"), branchdag.NewBranchIDs()},
			"Message21":   {testFramework.BranchIDs("B"), branchdag.NewBranchIDs()},
			"Message22":   {testFramework.BranchIDs(), testFramework.BranchIDs("A")},
		})
		checkBranchIDs(t, testFramework, map[string]branchdag.BranchIDs{
			"Message1":    testFramework.BranchIDs("A"),
			"Message2":    testFramework.BranchIDs("B"),
			"Message3":    testFramework.BranchIDs("C"),
			"Message4":    branchdag.NewBranchIDs(branchdag.MasterBranchID),
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
			"Message15":   branchdag.NewBranchIDs(branchdag.MasterBranchID),
			"Message16":   testFramework.BranchIDs("Z", "A"),
			"Message17":   testFramework.BranchIDs("Y", "A", "E"),
			"Message18":   testFramework.BranchIDs("Y", "A", "E", "B", "D"),
			"Message19":   testFramework.BranchIDs("Y", "A", "E"),
			"Message20":   testFramework.BranchIDs("Y", "A", "E", "B"),
			"Message21":   testFramework.BranchIDs("A", "B"),
			"Message22":   branchdag.NewBranchIDs(branchdag.MasterBranchID),
		})
	}

	// ISSUE Message23
	{
		msg := testFramework.CreateMessage("Message23", WithStrongParents("Message22"), WithShallowLikeParents("Message2"))

		testFramework.IssueMessages("Message23").WaitUntilAllTasksProcessed()

		tangle.Storage.MessageMetadata(msg.ID()).Consume(func(messageMetadata *MessageMetadata) {
			assert.False(t, messageMetadata.subjectivelyInvalid)
		})

		checkMarkers(t, testFramework, map[string]*markers.Markers{
			"Message1":    markers.NewMarkers(markers.NewMarker(0, 1)),
			"Message2":    markers.NewMarkers(markers.NewMarker(0, 0)),
			"Message3":    markers.NewMarkers(markers.NewMarker(0, 0)),
			"Message4":    markers.NewMarkers(markers.NewMarker(0, 0)),
			"Message5":    markers.NewMarkers(markers.NewMarker(0, 0)),
			"Message6":    markers.NewMarkers(markers.NewMarker(0, 2)),
			"Message7":    markers.NewMarkers(markers.NewMarker(0, 3)),
			"Message8":    markers.NewMarkers(markers.NewMarker(0, 4)),
			"Message9":    markers.NewMarkers(markers.NewMarker(0, 3)),
			"Message10":   markers.NewMarkers(markers.NewMarker(0, 3)),
			"Message11":   markers.NewMarkers(markers.NewMarker(0, 5)),
			"Message11.5": markers.NewMarkers(markers.NewMarker(0, 3)),
			"Message12":   markers.NewMarkers(markers.NewMarker(0, 1)),
			"Message13":   markers.NewMarkers(markers.NewMarker(0, 3)),
			"Message13.1": markers.NewMarkers(markers.NewMarker(0, 3)),
			"Message14":   markers.NewMarkers(markers.NewMarker(0, 3)),
			"Message15":   markers.NewMarkers(markers.NewMarker(0, 3)),
			"Message16":   markers.NewMarkers(markers.NewMarker(0, 1)),
			"Message17":   markers.NewMarkers(markers.NewMarker(0, 1)),
			"Message18":   markers.NewMarkers(markers.NewMarker(0, 3)),
			"Message19":   markers.NewMarkers(markers.NewMarker(0, 1)),
			"Message20":   markers.NewMarkers(markers.NewMarker(0, 1)),
			"Message21":   markers.NewMarkers(markers.NewMarker(0, 1)),
			"Message22":   markers.NewMarkers(markers.NewMarker(0, 1)),
			"Message23":   markers.NewMarkers(markers.NewMarker(0, 1)),
		})
		checkMessageMetadataDiffBranchIDs(t, testFramework, map[string][]branchdag.BranchIDs{
			"Message1":    {branchdag.NewBranchIDs(), branchdag.NewBranchIDs()},
			"Message2":    {testFramework.BranchIDs("B"), branchdag.NewBranchIDs()},
			"Message3":    {testFramework.BranchIDs("C"), branchdag.NewBranchIDs()},
			"Message4":    {branchdag.NewBranchIDs(), branchdag.NewBranchIDs()},
			"Message5":    {testFramework.BranchIDs("D"), branchdag.NewBranchIDs()},
			"Message6":    {branchdag.NewBranchIDs(), branchdag.NewBranchIDs()},
			"Message7":    {branchdag.NewBranchIDs(), branchdag.NewBranchIDs()},
			"Message8":    {branchdag.NewBranchIDs(), branchdag.NewBranchIDs()},
			"Message9":    {testFramework.BranchIDs("A", "C"), testFramework.BranchIDs("B")},
			"Message10":   {branchdag.NewBranchIDs(), branchdag.NewBranchIDs()},
			"Message11":   {branchdag.NewBranchIDs(), branchdag.NewBranchIDs()},
			"Message11.5": {testFramework.BranchIDs("A", "C"), testFramework.BranchIDs("B")},
			"Message12":   {testFramework.BranchIDs("E"), branchdag.NewBranchIDs()},
			"Message13":   {testFramework.BranchIDs("E"), testFramework.BranchIDs("D")},
			"Message13.1": {testFramework.BranchIDs("E"), testFramework.BranchIDs("D")},
			"Message14":   {testFramework.BranchIDs("E"), testFramework.BranchIDs("D")},
			"Message15":   {branchdag.NewBranchIDs(), testFramework.BranchIDs("B", "D")},
			"Message16":   {testFramework.BranchIDs("Z"), branchdag.NewBranchIDs()},
			"Message17":   {testFramework.BranchIDs("E", "Y"), branchdag.NewBranchIDs()},
			"Message18":   {testFramework.BranchIDs("Y", "A", "E"), branchdag.NewBranchIDs()},
			"Message19":   {testFramework.BranchIDs("E", "Y"), branchdag.NewBranchIDs()},
			"Message20":   {testFramework.BranchIDs("Y", "E", "B"), branchdag.NewBranchIDs()},
			"Message21":   {testFramework.BranchIDs("B"), branchdag.NewBranchIDs()},
			"Message22":   {testFramework.BranchIDs(), testFramework.BranchIDs("A")},
			"Message23":   {testFramework.BranchIDs("B"), testFramework.BranchIDs("A")},
		})
		checkBranchIDs(t, testFramework, map[string]branchdag.BranchIDs{
			"Message1":    testFramework.BranchIDs("A"),
			"Message2":    testFramework.BranchIDs("B"),
			"Message3":    testFramework.BranchIDs("C"),
			"Message4":    branchdag.NewBranchIDs(branchdag.MasterBranchID),
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
			"Message15":   branchdag.NewBranchIDs(branchdag.MasterBranchID),
			"Message16":   testFramework.BranchIDs("Z", "A"),
			"Message17":   testFramework.BranchIDs("Y", "A", "E"),
			"Message18":   testFramework.BranchIDs("Y", "A", "E", "B", "D"),
			"Message19":   testFramework.BranchIDs("Y", "A", "E"),
			"Message20":   testFramework.BranchIDs("Y", "A", "E", "B"),
			"Message21":   testFramework.BranchIDs("A", "B"),
			"Message22":   branchdag.NewBranchIDs(branchdag.MasterBranchID),
			"Message23":   testFramework.BranchIDs("B"),
		})
	}

	// ISSUE Message24
	{
		testFramework.CreateMessage("Message24", WithStrongParents("Message23"), WithShallowLikeParents("Message5"))

		testFramework.IssueMessages("Message24").WaitUntilAllTasksProcessed()

		checkMarkers(t, testFramework, map[string]*markers.Markers{
			"Message1":    markers.NewMarkers(markers.NewMarker(0, 1)),
			"Message2":    markers.NewMarkers(markers.NewMarker(0, 0)),
			"Message3":    markers.NewMarkers(markers.NewMarker(0, 0)),
			"Message4":    markers.NewMarkers(markers.NewMarker(0, 0)),
			"Message5":    markers.NewMarkers(markers.NewMarker(0, 0)),
			"Message6":    markers.NewMarkers(markers.NewMarker(0, 2)),
			"Message7":    markers.NewMarkers(markers.NewMarker(0, 3)),
			"Message8":    markers.NewMarkers(markers.NewMarker(0, 4)),
			"Message9":    markers.NewMarkers(markers.NewMarker(0, 3)),
			"Message10":   markers.NewMarkers(markers.NewMarker(0, 3)),
			"Message11":   markers.NewMarkers(markers.NewMarker(0, 5)),
			"Message11.5": markers.NewMarkers(markers.NewMarker(0, 3)),
			"Message12":   markers.NewMarkers(markers.NewMarker(0, 1)),
			"Message13":   markers.NewMarkers(markers.NewMarker(0, 3)),
			"Message13.1": markers.NewMarkers(markers.NewMarker(0, 3)),
			"Message14":   markers.NewMarkers(markers.NewMarker(0, 3)),
			"Message15":   markers.NewMarkers(markers.NewMarker(0, 3)),
			"Message16":   markers.NewMarkers(markers.NewMarker(0, 1)),
			"Message17":   markers.NewMarkers(markers.NewMarker(0, 1)),
			"Message18":   markers.NewMarkers(markers.NewMarker(0, 3)),
			"Message19":   markers.NewMarkers(markers.NewMarker(0, 1)),
			"Message20":   markers.NewMarkers(markers.NewMarker(0, 1)),
			"Message21":   markers.NewMarkers(markers.NewMarker(0, 1)),
			"Message22":   markers.NewMarkers(markers.NewMarker(0, 1)),
			"Message23":   markers.NewMarkers(markers.NewMarker(0, 1)),
			"Message24":   markers.NewMarkers(markers.NewMarker(0, 1)),
		})
		checkMessageMetadataDiffBranchIDs(t, testFramework, map[string][]branchdag.BranchIDs{
			"Message1":    {branchdag.NewBranchIDs(), branchdag.NewBranchIDs()},
			"Message2":    {testFramework.BranchIDs("B"), branchdag.NewBranchIDs()},
			"Message3":    {testFramework.BranchIDs("C"), branchdag.NewBranchIDs()},
			"Message4":    {branchdag.NewBranchIDs(), branchdag.NewBranchIDs()},
			"Message5":    {testFramework.BranchIDs("D"), branchdag.NewBranchIDs()},
			"Message6":    {branchdag.NewBranchIDs(), branchdag.NewBranchIDs()},
			"Message7":    {branchdag.NewBranchIDs(), branchdag.NewBranchIDs()},
			"Message8":    {branchdag.NewBranchIDs(), branchdag.NewBranchIDs()},
			"Message9":    {testFramework.BranchIDs("A", "C"), testFramework.BranchIDs("B")},
			"Message10":   {branchdag.NewBranchIDs(), branchdag.NewBranchIDs()},
			"Message11":   {branchdag.NewBranchIDs(), branchdag.NewBranchIDs()},
			"Message11.5": {testFramework.BranchIDs("A", "C"), testFramework.BranchIDs("B")},
			"Message12":   {testFramework.BranchIDs("E"), branchdag.NewBranchIDs()},
			"Message13":   {testFramework.BranchIDs("E"), testFramework.BranchIDs("D")},
			"Message13.1": {testFramework.BranchIDs("E"), testFramework.BranchIDs("D")},
			"Message14":   {testFramework.BranchIDs("E"), testFramework.BranchIDs("D")},
			"Message15":   {branchdag.NewBranchIDs(), testFramework.BranchIDs("B", "D")},
			"Message16":   {testFramework.BranchIDs("Z"), branchdag.NewBranchIDs()},
			"Message17":   {testFramework.BranchIDs("E", "Y"), branchdag.NewBranchIDs()},
			"Message18":   {testFramework.BranchIDs("Y", "A", "E"), branchdag.NewBranchIDs()},
			"Message19":   {testFramework.BranchIDs("E", "Y"), branchdag.NewBranchIDs()},
			"Message20":   {testFramework.BranchIDs("Y", "E", "B"), branchdag.NewBranchIDs()},
			"Message21":   {testFramework.BranchIDs("B"), branchdag.NewBranchIDs()},
			"Message22":   {testFramework.BranchIDs(), testFramework.BranchIDs("A")},
			"Message23":   {testFramework.BranchIDs("B"), testFramework.BranchIDs("A")},
			"Message24":   {testFramework.BranchIDs("B", "D"), testFramework.BranchIDs("A")},
		})
		checkBranchIDs(t, testFramework, map[string]branchdag.BranchIDs{
			"Message1":    testFramework.BranchIDs("A"),
			"Message2":    testFramework.BranchIDs("B"),
			"Message3":    testFramework.BranchIDs("C"),
			"Message4":    branchdag.NewBranchIDs(branchdag.MasterBranchID),
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
			"Message15":   branchdag.NewBranchIDs(branchdag.MasterBranchID),
			"Message16":   testFramework.BranchIDs("Z", "A"),
			"Message17":   testFramework.BranchIDs("Y", "A", "E"),
			"Message18":   testFramework.BranchIDs("Y", "A", "E", "B", "D"),
			"Message19":   testFramework.BranchIDs("Y", "A", "E"),
			"Message20":   testFramework.BranchIDs("Y", "A", "E", "B"),
			"Message21":   testFramework.BranchIDs("A", "B"),
			"Message22":   branchdag.NewBranchIDs(branchdag.MasterBranchID),
			"Message23":   testFramework.BranchIDs("B"),
			"Message24":   testFramework.BranchIDs("B", "D"),
		})
	}

	// ISSUE Message25
	{
		testFramework.CreateMessage("Message25", WithStrongParents("Message22"), WithWeakParents("Message5"))

		testFramework.IssueMessages("Message25").WaitUntilAllTasksProcessed()

		checkMarkers(t, testFramework, map[string]*markers.Markers{
			"Message1":    markers.NewMarkers(markers.NewMarker(0, 1)),
			"Message2":    markers.NewMarkers(markers.NewMarker(0, 0)),
			"Message3":    markers.NewMarkers(markers.NewMarker(0, 0)),
			"Message4":    markers.NewMarkers(markers.NewMarker(0, 0)),
			"Message5":    markers.NewMarkers(markers.NewMarker(0, 0)),
			"Message6":    markers.NewMarkers(markers.NewMarker(0, 2)),
			"Message7":    markers.NewMarkers(markers.NewMarker(0, 3)),
			"Message8":    markers.NewMarkers(markers.NewMarker(0, 4)),
			"Message9":    markers.NewMarkers(markers.NewMarker(0, 3)),
			"Message10":   markers.NewMarkers(markers.NewMarker(0, 3)),
			"Message11":   markers.NewMarkers(markers.NewMarker(0, 5)),
			"Message11.5": markers.NewMarkers(markers.NewMarker(0, 3)),
			"Message12":   markers.NewMarkers(markers.NewMarker(0, 1)),
			"Message13":   markers.NewMarkers(markers.NewMarker(0, 3)),
			"Message13.1": markers.NewMarkers(markers.NewMarker(0, 3)),
			"Message14":   markers.NewMarkers(markers.NewMarker(0, 3)),
			"Message15":   markers.NewMarkers(markers.NewMarker(0, 3)),
			"Message16":   markers.NewMarkers(markers.NewMarker(0, 1)),
			"Message17":   markers.NewMarkers(markers.NewMarker(0, 1)),
			"Message18":   markers.NewMarkers(markers.NewMarker(0, 3)),
			"Message19":   markers.NewMarkers(markers.NewMarker(0, 1)),
			"Message20":   markers.NewMarkers(markers.NewMarker(0, 1)),
			"Message21":   markers.NewMarkers(markers.NewMarker(0, 1)),
			"Message22":   markers.NewMarkers(markers.NewMarker(0, 1)),
			"Message23":   markers.NewMarkers(markers.NewMarker(0, 1)),
			"Message24":   markers.NewMarkers(markers.NewMarker(0, 1)),
			"Message25":   markers.NewMarkers(markers.NewMarker(0, 1)),
		})
		checkMessageMetadataDiffBranchIDs(t, testFramework, map[string][]branchdag.BranchIDs{
			"Message1":    {branchdag.NewBranchIDs(), branchdag.NewBranchIDs()},
			"Message2":    {testFramework.BranchIDs("B"), branchdag.NewBranchIDs()},
			"Message3":    {testFramework.BranchIDs("C"), branchdag.NewBranchIDs()},
			"Message4":    {branchdag.NewBranchIDs(), branchdag.NewBranchIDs()},
			"Message5":    {testFramework.BranchIDs("D"), branchdag.NewBranchIDs()},
			"Message6":    {branchdag.NewBranchIDs(), branchdag.NewBranchIDs()},
			"Message7":    {branchdag.NewBranchIDs(), branchdag.NewBranchIDs()},
			"Message8":    {branchdag.NewBranchIDs(), branchdag.NewBranchIDs()},
			"Message9":    {testFramework.BranchIDs("A", "C"), testFramework.BranchIDs("B")},
			"Message10":   {branchdag.NewBranchIDs(), branchdag.NewBranchIDs()},
			"Message11":   {branchdag.NewBranchIDs(), branchdag.NewBranchIDs()},
			"Message11.5": {testFramework.BranchIDs("A", "C"), testFramework.BranchIDs("B")},
			"Message12":   {testFramework.BranchIDs("E"), branchdag.NewBranchIDs()},
			"Message13":   {testFramework.BranchIDs("E"), testFramework.BranchIDs("D")},
			"Message13.1": {testFramework.BranchIDs("E"), testFramework.BranchIDs("D")},
			"Message14":   {testFramework.BranchIDs("E"), testFramework.BranchIDs("D")},
			"Message15":   {branchdag.NewBranchIDs(), testFramework.BranchIDs("B", "D")},
			"Message16":   {testFramework.BranchIDs("Z"), branchdag.NewBranchIDs()},
			"Message17":   {testFramework.BranchIDs("E", "Y"), branchdag.NewBranchIDs()},
			"Message18":   {testFramework.BranchIDs("Y", "A", "E"), branchdag.NewBranchIDs()},
			"Message19":   {testFramework.BranchIDs("E", "Y"), branchdag.NewBranchIDs()},
			"Message20":   {testFramework.BranchIDs("Y", "E", "B"), branchdag.NewBranchIDs()},
			"Message21":   {testFramework.BranchIDs("B"), branchdag.NewBranchIDs()},
			"Message22":   {testFramework.BranchIDs(), testFramework.BranchIDs("A")},
			"Message23":   {testFramework.BranchIDs("B"), testFramework.BranchIDs("A")},
			"Message24":   {testFramework.BranchIDs("B", "D"), testFramework.BranchIDs("A")},
			"Message25":   {testFramework.BranchIDs("D"), testFramework.BranchIDs("A")},
		})
		checkBranchIDs(t, testFramework, map[string]branchdag.BranchIDs{
			"Message1":    testFramework.BranchIDs("A"),
			"Message2":    testFramework.BranchIDs("B"),
			"Message3":    testFramework.BranchIDs("C"),
			"Message4":    branchdag.NewBranchIDs(branchdag.MasterBranchID),
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
			"Message15":   branchdag.NewBranchIDs(branchdag.MasterBranchID),
			"Message16":   testFramework.BranchIDs("Z", "A"),
			"Message17":   testFramework.BranchIDs("Y", "A", "E"),
			"Message18":   testFramework.BranchIDs("Y", "A", "E", "B", "D"),
			"Message19":   testFramework.BranchIDs("Y", "A", "E"),
			"Message20":   testFramework.BranchIDs("Y", "A", "E", "B"),
			"Message21":   testFramework.BranchIDs("A", "B"),
			"Message22":   branchdag.NewBranchIDs(branchdag.MasterBranchID),
			"Message23":   testFramework.BranchIDs("B"),
			"Message24":   testFramework.BranchIDs("B", "D"),
			"Message25":   testFramework.BranchIDs("D"),
		})
	}

	// ISSUE Message26
	{
		msg := testFramework.CreateMessage("Message26", WithStrongParents("Message19"))

		testFramework.IssueMessages("Message26").WaitUntilAllTasksProcessed()

		tangle.Storage.MessageMetadata(msg.ID()).Consume(func(messageMetadata *MessageMetadata) {
			assert.False(t, messageMetadata.subjectivelyInvalid)
		})

		checkMarkers(t, testFramework, map[string]*markers.Markers{
			"Message1":    markers.NewMarkers(markers.NewMarker(0, 1)),
			"Message2":    markers.NewMarkers(markers.NewMarker(0, 0)),
			"Message3":    markers.NewMarkers(markers.NewMarker(0, 0)),
			"Message4":    markers.NewMarkers(markers.NewMarker(0, 0)),
			"Message5":    markers.NewMarkers(markers.NewMarker(0, 0)),
			"Message6":    markers.NewMarkers(markers.NewMarker(0, 2)),
			"Message7":    markers.NewMarkers(markers.NewMarker(0, 3)),
			"Message8":    markers.NewMarkers(markers.NewMarker(0, 4)),
			"Message9":    markers.NewMarkers(markers.NewMarker(0, 3)),
			"Message10":   markers.NewMarkers(markers.NewMarker(0, 3)),
			"Message11":   markers.NewMarkers(markers.NewMarker(0, 5)),
			"Message11.5": markers.NewMarkers(markers.NewMarker(0, 3)),
			"Message12":   markers.NewMarkers(markers.NewMarker(0, 1)),
			"Message13":   markers.NewMarkers(markers.NewMarker(0, 3)),
			"Message13.1": markers.NewMarkers(markers.NewMarker(0, 3)),
			"Message14":   markers.NewMarkers(markers.NewMarker(0, 3)),
			"Message15":   markers.NewMarkers(markers.NewMarker(0, 3)),
			"Message16":   markers.NewMarkers(markers.NewMarker(0, 1)),
			"Message17":   markers.NewMarkers(markers.NewMarker(0, 1)),
			"Message18":   markers.NewMarkers(markers.NewMarker(0, 3)),
			"Message19":   markers.NewMarkers(markers.NewMarker(0, 1)),
			"Message20":   markers.NewMarkers(markers.NewMarker(0, 1)),
			"Message21":   markers.NewMarkers(markers.NewMarker(0, 1)),
			"Message22":   markers.NewMarkers(markers.NewMarker(0, 1)),
			"Message23":   markers.NewMarkers(markers.NewMarker(0, 1)),
			"Message24":   markers.NewMarkers(markers.NewMarker(0, 1)),
			"Message25":   markers.NewMarkers(markers.NewMarker(0, 1)),
			"Message26":   markers.NewMarkers(markers.NewMarker(0, 1)),
		})
		checkMessageMetadataDiffBranchIDs(t, testFramework, map[string][]branchdag.BranchIDs{
			"Message1":    {branchdag.NewBranchIDs(), branchdag.NewBranchIDs()},
			"Message2":    {testFramework.BranchIDs("B"), branchdag.NewBranchIDs()},
			"Message3":    {testFramework.BranchIDs("C"), branchdag.NewBranchIDs()},
			"Message4":    {branchdag.NewBranchIDs(), branchdag.NewBranchIDs()},
			"Message5":    {testFramework.BranchIDs("D"), branchdag.NewBranchIDs()},
			"Message6":    {branchdag.NewBranchIDs(), branchdag.NewBranchIDs()},
			"Message7":    {branchdag.NewBranchIDs(), branchdag.NewBranchIDs()},
			"Message8":    {branchdag.NewBranchIDs(), branchdag.NewBranchIDs()},
			"Message9":    {testFramework.BranchIDs("A", "C"), testFramework.BranchIDs("B")},
			"Message10":   {branchdag.NewBranchIDs(), branchdag.NewBranchIDs()},
			"Message11":   {branchdag.NewBranchIDs(), branchdag.NewBranchIDs()},
			"Message11.5": {testFramework.BranchIDs("A", "C"), testFramework.BranchIDs("B")},
			"Message12":   {testFramework.BranchIDs("E"), branchdag.NewBranchIDs()},
			"Message13":   {testFramework.BranchIDs("E"), testFramework.BranchIDs("D")},
			"Message13.1": {testFramework.BranchIDs("E"), testFramework.BranchIDs("D")},
			"Message14":   {testFramework.BranchIDs("E"), testFramework.BranchIDs("D")},
			"Message15":   {branchdag.NewBranchIDs(), testFramework.BranchIDs("B", "D")},
			"Message16":   {testFramework.BranchIDs("Z"), branchdag.NewBranchIDs()},
			"Message17":   {testFramework.BranchIDs("E", "Y"), branchdag.NewBranchIDs()},
			"Message18":   {testFramework.BranchIDs("Y", "A", "E"), branchdag.NewBranchIDs()},
			"Message19":   {testFramework.BranchIDs("E", "Y"), branchdag.NewBranchIDs()},
			"Message20":   {testFramework.BranchIDs("Y", "E", "B"), branchdag.NewBranchIDs()},
			"Message21":   {testFramework.BranchIDs("B"), branchdag.NewBranchIDs()},
			"Message22":   {testFramework.BranchIDs(), testFramework.BranchIDs("A")},
			"Message23":   {testFramework.BranchIDs("B"), testFramework.BranchIDs("A")},
			"Message24":   {testFramework.BranchIDs("B", "D"), testFramework.BranchIDs("A")},
			"Message25":   {testFramework.BranchIDs("D"), testFramework.BranchIDs("A")},
			"Message26":   {testFramework.BranchIDs("E", "Y"), branchdag.NewBranchIDs()},
		})
		checkBranchIDs(t, testFramework, map[string]branchdag.BranchIDs{
			"Message1":    testFramework.BranchIDs("A"),
			"Message2":    testFramework.BranchIDs("B"),
			"Message3":    testFramework.BranchIDs("C"),
			"Message4":    branchdag.NewBranchIDs(branchdag.MasterBranchID),
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
			"Message15":   branchdag.NewBranchIDs(branchdag.MasterBranchID),
			"Message16":   testFramework.BranchIDs("Z", "A"),
			"Message17":   testFramework.BranchIDs("Y", "A", "E"),
			"Message18":   testFramework.BranchIDs("Y", "A", "E", "B", "D"),
			"Message19":   testFramework.BranchIDs("Y", "A", "E"),
			"Message20":   testFramework.BranchIDs("Y", "A", "E", "B"),
			"Message21":   testFramework.BranchIDs("A", "B"),
			"Message22":   branchdag.NewBranchIDs(branchdag.MasterBranchID),
			"Message23":   testFramework.BranchIDs("B"),
			"Message24":   testFramework.BranchIDs("B", "D"),
			"Message25":   testFramework.BranchIDs("D"),
			"Message26":   testFramework.BranchIDs("Y", "A", "E"),
		})
	}

	// ISSUE Message27
	{
		msg := testFramework.CreateMessage("Message27", WithStrongParents("Message19"))

		// We confirm E, thus we should NOT inherit it when attaching again to Message19.
		testFramework.tangle.Ledger.BranchDAG.SetBranchConfirmed(testFramework.BranchID("E"))
		testFramework.tangle.Ledger.BranchDAG.Storage.CachedBranch(testFramework.BranchID("E")).Consume(func(branch *branchdag.Branch) {
			assert.Equal(t, branch.InclusionState(), branchdag.Confirmed)
		})

		testFramework.tangle.Ledger.BranchDAG.Storage.CachedBranch(testFramework.BranchID("D")).Consume(func(branch *branchdag.Branch) {
			assert.Equal(t, branch.InclusionState(), branchdag.Rejected)
		})

		testFramework.IssueMessages("Message27").WaitUntilAllTasksProcessed()

		tangle.Storage.MessageMetadata(msg.ID()).Consume(func(messageMetadata *MessageMetadata) {
			assert.False(t, messageMetadata.IsSubjectivelyInvalid())
		})

		checkMarkers(t, testFramework, map[string]*markers.Markers{
			"Message1":    markers.NewMarkers(markers.NewMarker(0, 1)),
			"Message2":    markers.NewMarkers(markers.NewMarker(0, 0)),
			"Message3":    markers.NewMarkers(markers.NewMarker(0, 0)),
			"Message4":    markers.NewMarkers(markers.NewMarker(0, 0)),
			"Message5":    markers.NewMarkers(markers.NewMarker(0, 0)),
			"Message6":    markers.NewMarkers(markers.NewMarker(0, 2)),
			"Message7":    markers.NewMarkers(markers.NewMarker(0, 3)),
			"Message8":    markers.NewMarkers(markers.NewMarker(0, 4)),
			"Message9":    markers.NewMarkers(markers.NewMarker(0, 3)),
			"Message10":   markers.NewMarkers(markers.NewMarker(0, 3)),
			"Message11":   markers.NewMarkers(markers.NewMarker(0, 5)),
			"Message11.5": markers.NewMarkers(markers.NewMarker(0, 3)),
			"Message12":   markers.NewMarkers(markers.NewMarker(0, 1)),
			"Message13":   markers.NewMarkers(markers.NewMarker(0, 3)),
			"Message13.1": markers.NewMarkers(markers.NewMarker(0, 3)),
			"Message14":   markers.NewMarkers(markers.NewMarker(0, 3)),
			"Message15":   markers.NewMarkers(markers.NewMarker(0, 3)),
			"Message16":   markers.NewMarkers(markers.NewMarker(0, 1)),
			"Message17":   markers.NewMarkers(markers.NewMarker(0, 1)),
			"Message18":   markers.NewMarkers(markers.NewMarker(0, 3)),
			"Message19":   markers.NewMarkers(markers.NewMarker(0, 1)),
			"Message20":   markers.NewMarkers(markers.NewMarker(0, 1)),
			"Message21":   markers.NewMarkers(markers.NewMarker(0, 1)),
			"Message22":   markers.NewMarkers(markers.NewMarker(0, 1)),
			"Message23":   markers.NewMarkers(markers.NewMarker(0, 1)),
			"Message24":   markers.NewMarkers(markers.NewMarker(0, 1)),
			"Message25":   markers.NewMarkers(markers.NewMarker(0, 1)),
			"Message26":   markers.NewMarkers(markers.NewMarker(0, 1)),
			"Message27":   markers.NewMarkers(markers.NewMarker(0, 1)),
		})
		checkMessageMetadataDiffBranchIDs(t, testFramework, map[string][]branchdag.BranchIDs{
			"Message1":    {branchdag.NewBranchIDs(), branchdag.NewBranchIDs()},
			"Message2":    {testFramework.BranchIDs("B"), branchdag.NewBranchIDs()},
			"Message3":    {testFramework.BranchIDs("C"), branchdag.NewBranchIDs()},
			"Message4":    {branchdag.NewBranchIDs(), branchdag.NewBranchIDs()},
			"Message5":    {testFramework.BranchIDs("D"), branchdag.NewBranchIDs()},
			"Message6":    {branchdag.NewBranchIDs(), branchdag.NewBranchIDs()},
			"Message7":    {branchdag.NewBranchIDs(), branchdag.NewBranchIDs()},
			"Message8":    {branchdag.NewBranchIDs(), branchdag.NewBranchIDs()},
			"Message9":    {testFramework.BranchIDs("A", "C"), testFramework.BranchIDs("B")},
			"Message10":   {branchdag.NewBranchIDs(), branchdag.NewBranchIDs()},
			"Message11":   {branchdag.NewBranchIDs(), branchdag.NewBranchIDs()},
			"Message11.5": {testFramework.BranchIDs("A", "C"), testFramework.BranchIDs("B")},
			"Message12":   {testFramework.BranchIDs("E"), branchdag.NewBranchIDs()},
			"Message13":   {testFramework.BranchIDs("E"), testFramework.BranchIDs("D")},
			"Message13.1": {testFramework.BranchIDs("E"), testFramework.BranchIDs("D")},
			"Message14":   {testFramework.BranchIDs("E"), testFramework.BranchIDs("D")},
			"Message15":   {branchdag.NewBranchIDs(), testFramework.BranchIDs("B", "D")},
			"Message16":   {testFramework.BranchIDs("Z"), branchdag.NewBranchIDs()},
			"Message17":   {testFramework.BranchIDs("E", "Y"), branchdag.NewBranchIDs()},
			"Message18":   {testFramework.BranchIDs("Y", "A", "E"), branchdag.NewBranchIDs()},
			"Message19":   {testFramework.BranchIDs("E", "Y"), branchdag.NewBranchIDs()},
			"Message20":   {testFramework.BranchIDs("Y", "E", "B"), branchdag.NewBranchIDs()},
			"Message21":   {testFramework.BranchIDs("B"), branchdag.NewBranchIDs()},
			"Message22":   {testFramework.BranchIDs(), testFramework.BranchIDs("A")},
			"Message23":   {testFramework.BranchIDs("B"), testFramework.BranchIDs("A")},
			"Message24":   {testFramework.BranchIDs("B", "D"), testFramework.BranchIDs("A")},
			"Message25":   {testFramework.BranchIDs("D"), testFramework.BranchIDs("A")},
			"Message26":   {testFramework.BranchIDs("E", "Y"), branchdag.NewBranchIDs()},
			"Message27":   {testFramework.BranchIDs("Y"), branchdag.NewBranchIDs()},
		})
		checkBranchIDs(t, testFramework, map[string]branchdag.BranchIDs{
			"Message1":    testFramework.BranchIDs("A"),
			"Message2":    testFramework.BranchIDs("B"),
			"Message3":    testFramework.BranchIDs("C"),
			"Message4":    branchdag.NewBranchIDs(branchdag.MasterBranchID),
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
			"Message15":   branchdag.NewBranchIDs(branchdag.MasterBranchID),
			"Message16":   testFramework.BranchIDs("Z", "A"),
			"Message17":   testFramework.BranchIDs("Y", "A", "E"),
			"Message18":   testFramework.BranchIDs("Y", "A", "B", "D", "E"),
			"Message19":   testFramework.BranchIDs("Y", "A", "E"),
			"Message20":   testFramework.BranchIDs("Y", "A", "B", "E"),
			"Message21":   testFramework.BranchIDs("A", "B"),
			"Message22":   branchdag.NewBranchIDs(branchdag.MasterBranchID),
			"Message23":   testFramework.BranchIDs("B"),
			"Message24":   testFramework.BranchIDs("B", "D"),
			"Message25":   testFramework.BranchIDs("D"),
			"Message26":   testFramework.BranchIDs("Y", "A", "E"),
			"Message27":   testFramework.BranchIDs("Y", "A"),
		})
	}

	// ISSUE Message28
	{
		msg := testFramework.CreateMessage("Message28", WithStrongParents("Message13.1"))

		testFramework.IssueMessages("Message28").WaitUntilAllTasksProcessed()

		tangle.Storage.MessageMetadata(msg.ID()).Consume(func(messageMetadata *MessageMetadata) {
			assert.False(t, messageMetadata.subjectivelyInvalid)
		})

		checkMarkers(t, testFramework, map[string]*markers.Markers{
			"Message1":    markers.NewMarkers(markers.NewMarker(0, 1)),
			"Message2":    markers.NewMarkers(markers.NewMarker(0, 0)),
			"Message3":    markers.NewMarkers(markers.NewMarker(0, 0)),
			"Message4":    markers.NewMarkers(markers.NewMarker(0, 0)),
			"Message5":    markers.NewMarkers(markers.NewMarker(0, 0)),
			"Message6":    markers.NewMarkers(markers.NewMarker(0, 2)),
			"Message7":    markers.NewMarkers(markers.NewMarker(0, 3)),
			"Message8":    markers.NewMarkers(markers.NewMarker(0, 4)),
			"Message9":    markers.NewMarkers(markers.NewMarker(0, 3)),
			"Message10":   markers.NewMarkers(markers.NewMarker(0, 3)),
			"Message11":   markers.NewMarkers(markers.NewMarker(0, 5)),
			"Message11.5": markers.NewMarkers(markers.NewMarker(0, 3)),
			"Message12":   markers.NewMarkers(markers.NewMarker(0, 1)),
			"Message13":   markers.NewMarkers(markers.NewMarker(0, 3)),
			"Message13.1": markers.NewMarkers(markers.NewMarker(0, 3)),
			"Message14":   markers.NewMarkers(markers.NewMarker(0, 3)),
			"Message15":   markers.NewMarkers(markers.NewMarker(0, 3)),
			"Message16":   markers.NewMarkers(markers.NewMarker(0, 1)),
			"Message17":   markers.NewMarkers(markers.NewMarker(0, 1)),
			"Message18":   markers.NewMarkers(markers.NewMarker(0, 3)),
			"Message19":   markers.NewMarkers(markers.NewMarker(0, 1)),
			"Message20":   markers.NewMarkers(markers.NewMarker(0, 1)),
			"Message21":   markers.NewMarkers(markers.NewMarker(0, 1)),
			"Message22":   markers.NewMarkers(markers.NewMarker(0, 1)),
			"Message23":   markers.NewMarkers(markers.NewMarker(0, 1)),
			"Message24":   markers.NewMarkers(markers.NewMarker(0, 1)),
			"Message25":   markers.NewMarkers(markers.NewMarker(0, 1)),
			"Message26":   markers.NewMarkers(markers.NewMarker(0, 1)),
			"Message27":   markers.NewMarkers(markers.NewMarker(0, 1)),
			"Message28":   markers.NewMarkers(markers.NewMarker(0, 3)),
		})
		checkMessageMetadataDiffBranchIDs(t, testFramework, map[string][]branchdag.BranchIDs{
			"Message1":    {branchdag.NewBranchIDs(), branchdag.NewBranchIDs()},
			"Message2":    {testFramework.BranchIDs("B"), branchdag.NewBranchIDs()},
			"Message3":    {testFramework.BranchIDs("C"), branchdag.NewBranchIDs()},
			"Message4":    {branchdag.NewBranchIDs(), branchdag.NewBranchIDs()},
			"Message5":    {testFramework.BranchIDs("D"), branchdag.NewBranchIDs()},
			"Message6":    {branchdag.NewBranchIDs(), branchdag.NewBranchIDs()},
			"Message7":    {branchdag.NewBranchIDs(), branchdag.NewBranchIDs()},
			"Message8":    {branchdag.NewBranchIDs(), branchdag.NewBranchIDs()},
			"Message9":    {testFramework.BranchIDs("A", "C"), testFramework.BranchIDs("B")},
			"Message10":   {branchdag.NewBranchIDs(), branchdag.NewBranchIDs()},
			"Message11":   {branchdag.NewBranchIDs(), branchdag.NewBranchIDs()},
			"Message11.5": {testFramework.BranchIDs("A", "C"), testFramework.BranchIDs("B")},
			"Message12":   {testFramework.BranchIDs("E"), branchdag.NewBranchIDs()},
			"Message13":   {testFramework.BranchIDs("E"), testFramework.BranchIDs("D")},
			"Message13.1": {testFramework.BranchIDs("E"), testFramework.BranchIDs("D")},
			"Message14":   {testFramework.BranchIDs("E"), testFramework.BranchIDs("D")},
			"Message15":   {branchdag.NewBranchIDs(), testFramework.BranchIDs("B", "D")},
			"Message16":   {testFramework.BranchIDs("Z"), branchdag.NewBranchIDs()},
			"Message17":   {testFramework.BranchIDs("E", "Y"), branchdag.NewBranchIDs()},
			"Message18":   {testFramework.BranchIDs("Y", "A", "E"), branchdag.NewBranchIDs()},
			"Message19":   {testFramework.BranchIDs("E", "Y"), branchdag.NewBranchIDs()},
			"Message20":   {testFramework.BranchIDs("Y", "E", "B"), branchdag.NewBranchIDs()},
			"Message21":   {testFramework.BranchIDs("B"), branchdag.NewBranchIDs()},
			"Message22":   {testFramework.BranchIDs(), testFramework.BranchIDs("A")},
			"Message23":   {testFramework.BranchIDs("B"), testFramework.BranchIDs("A")},
			"Message24":   {testFramework.BranchIDs("B", "D"), testFramework.BranchIDs("A")},
			"Message25":   {testFramework.BranchIDs("D"), testFramework.BranchIDs("A")},
			"Message26":   {testFramework.BranchIDs("E", "Y"), branchdag.NewBranchIDs()},
			"Message27":   {testFramework.BranchIDs("Y"), branchdag.NewBranchIDs()},
			"Message28":   {branchdag.NewBranchIDs(), testFramework.BranchIDs("D")},
		})
		checkBranchIDs(t, testFramework, map[string]branchdag.BranchIDs{
			"Message1":    testFramework.BranchIDs("A"),
			"Message2":    testFramework.BranchIDs("B"),
			"Message3":    testFramework.BranchIDs("C"),
			"Message4":    branchdag.NewBranchIDs(branchdag.MasterBranchID),
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
			"Message15":   branchdag.NewBranchIDs(branchdag.MasterBranchID),
			"Message16":   testFramework.BranchIDs("Z", "A"),
			"Message17":   testFramework.BranchIDs("Y", "A", "E"),
			"Message18":   testFramework.BranchIDs("Y", "A", "B", "D", "E"),
			"Message19":   testFramework.BranchIDs("Y", "A", "E"),
			"Message20":   testFramework.BranchIDs("Y", "A", "B", "E"),
			"Message21":   testFramework.BranchIDs("A", "B"),
			"Message22":   branchdag.NewBranchIDs(branchdag.MasterBranchID),
			"Message23":   testFramework.BranchIDs("B"),
			"Message24":   testFramework.BranchIDs("B", "D"),
			"Message25":   testFramework.BranchIDs("D"),
			"Message26":   testFramework.BranchIDs("Y", "A", "E"),
			"Message27":   testFramework.BranchIDs("Y", "A"),
			"Message28":   testFramework.BranchIDs("B"),
		})
	}

	// ISSUE Message29
	{

		testFramework.CreateMessage("Message29", WithStrongParents("Message5"), WithInputs("D"), WithOutput("H", 500))
		testFramework.IssueMessages("Message29").WaitUntilAllTasksProcessed()

		checkMarkers(t, testFramework, map[string]*markers.Markers{
			"Message1":    markers.NewMarkers(markers.NewMarker(0, 1)),
			"Message2":    markers.NewMarkers(markers.NewMarker(0, 0)),
			"Message3":    markers.NewMarkers(markers.NewMarker(0, 0)),
			"Message4":    markers.NewMarkers(markers.NewMarker(0, 0)),
			"Message5":    markers.NewMarkers(markers.NewMarker(0, 0)),
			"Message6":    markers.NewMarkers(markers.NewMarker(0, 2)),
			"Message7":    markers.NewMarkers(markers.NewMarker(0, 3)),
			"Message8":    markers.NewMarkers(markers.NewMarker(0, 4)),
			"Message9":    markers.NewMarkers(markers.NewMarker(0, 3)),
			"Message10":   markers.NewMarkers(markers.NewMarker(0, 3)),
			"Message11":   markers.NewMarkers(markers.NewMarker(0, 5)),
			"Message11.5": markers.NewMarkers(markers.NewMarker(0, 3)),
			"Message12":   markers.NewMarkers(markers.NewMarker(0, 1)),
			"Message13":   markers.NewMarkers(markers.NewMarker(0, 3)),
			"Message13.1": markers.NewMarkers(markers.NewMarker(0, 3)),
			"Message14":   markers.NewMarkers(markers.NewMarker(0, 3)),
			"Message15":   markers.NewMarkers(markers.NewMarker(0, 3)),
			"Message16":   markers.NewMarkers(markers.NewMarker(0, 1)),
			"Message17":   markers.NewMarkers(markers.NewMarker(0, 1)),
			"Message18":   markers.NewMarkers(markers.NewMarker(0, 3)),
			"Message19":   markers.NewMarkers(markers.NewMarker(0, 1)),
			"Message20":   markers.NewMarkers(markers.NewMarker(0, 1)),
			"Message21":   markers.NewMarkers(markers.NewMarker(0, 1)),
			"Message22":   markers.NewMarkers(markers.NewMarker(0, 1)),
			"Message23":   markers.NewMarkers(markers.NewMarker(0, 1)),
			"Message24":   markers.NewMarkers(markers.NewMarker(0, 1)),
			"Message25":   markers.NewMarkers(markers.NewMarker(0, 1)),
			"Message26":   markers.NewMarkers(markers.NewMarker(0, 1)),
			"Message27":   markers.NewMarkers(markers.NewMarker(0, 1)),
			"Message28":   markers.NewMarkers(markers.NewMarker(0, 3)),
			"Message29":   markers.NewMarkers(markers.NewMarker(0, 0)),
		})
		checkMessageMetadataDiffBranchIDs(t, testFramework, map[string][]branchdag.BranchIDs{
			"Message1":    {branchdag.NewBranchIDs(), branchdag.NewBranchIDs()},
			"Message2":    {testFramework.BranchIDs("B"), branchdag.NewBranchIDs()},
			"Message3":    {testFramework.BranchIDs("C"), branchdag.NewBranchIDs()},
			"Message4":    {branchdag.NewBranchIDs(), branchdag.NewBranchIDs()},
			"Message5":    {testFramework.BranchIDs("D"), branchdag.NewBranchIDs()},
			"Message6":    {branchdag.NewBranchIDs(), branchdag.NewBranchIDs()},
			"Message7":    {branchdag.NewBranchIDs(), branchdag.NewBranchIDs()},
			"Message8":    {branchdag.NewBranchIDs(), branchdag.NewBranchIDs()},
			"Message9":    {testFramework.BranchIDs("A", "C"), testFramework.BranchIDs("B")},
			"Message10":   {branchdag.NewBranchIDs(), branchdag.NewBranchIDs()},
			"Message11":   {branchdag.NewBranchIDs(), branchdag.NewBranchIDs()},
			"Message11.5": {testFramework.BranchIDs("A", "C"), testFramework.BranchIDs("B")},
			"Message12":   {testFramework.BranchIDs("E"), branchdag.NewBranchIDs()},
			"Message13":   {testFramework.BranchIDs("E"), testFramework.BranchIDs("D")},
			"Message13.1": {testFramework.BranchIDs("E"), testFramework.BranchIDs("D")},
			"Message14":   {testFramework.BranchIDs("E"), testFramework.BranchIDs("D")},
			"Message15":   {branchdag.NewBranchIDs(), testFramework.BranchIDs("B", "D")},
			"Message16":   {testFramework.BranchIDs("Z"), branchdag.NewBranchIDs()},
			"Message17":   {testFramework.BranchIDs("E", "Y"), branchdag.NewBranchIDs()},
			"Message18":   {testFramework.BranchIDs("Y", "A", "E"), branchdag.NewBranchIDs()},
			"Message19":   {testFramework.BranchIDs("E", "Y"), branchdag.NewBranchIDs()},
			"Message20":   {testFramework.BranchIDs("Y", "E", "B"), branchdag.NewBranchIDs()},
			"Message21":   {testFramework.BranchIDs("B"), branchdag.NewBranchIDs()},
			"Message22":   {testFramework.BranchIDs(), testFramework.BranchIDs("A")},
			"Message23":   {testFramework.BranchIDs("B"), testFramework.BranchIDs("A")},
			"Message24":   {testFramework.BranchIDs("B", "D"), testFramework.BranchIDs("A")},
			"Message25":   {testFramework.BranchIDs("D"), testFramework.BranchIDs("A")},
			"Message26":   {testFramework.BranchIDs("E", "Y"), branchdag.NewBranchIDs()},
			"Message27":   {testFramework.BranchIDs("Y"), branchdag.NewBranchIDs()},
			"Message28":   {branchdag.NewBranchIDs(), testFramework.BranchIDs("D")},
			"Message29":   {testFramework.BranchIDs("D"), branchdag.NewBranchIDs()},
		})
		checkBranchIDs(t, testFramework, map[string]branchdag.BranchIDs{
			"Message1":    testFramework.BranchIDs("A"),
			"Message2":    testFramework.BranchIDs("B"),
			"Message3":    testFramework.BranchIDs("C"),
			"Message4":    branchdag.NewBranchIDs(branchdag.MasterBranchID),
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
			"Message15":   branchdag.NewBranchIDs(branchdag.MasterBranchID),
			"Message16":   testFramework.BranchIDs("Z", "A"),
			"Message17":   testFramework.BranchIDs("Y", "A", "E"),
			"Message18":   testFramework.BranchIDs("Y", "A", "B", "D", "E"),
			"Message19":   testFramework.BranchIDs("Y", "A", "E"),
			"Message20":   testFramework.BranchIDs("Y", "A", "B", "E"),
			"Message21":   testFramework.BranchIDs("A", "B"),
			"Message22":   branchdag.NewBranchIDs(branchdag.MasterBranchID),
			"Message23":   testFramework.BranchIDs("B"),
			"Message24":   testFramework.BranchIDs("B", "D"),
			"Message25":   testFramework.BranchIDs("D"),
			"Message26":   testFramework.BranchIDs("Y", "A", "E"),
			"Message27":   testFramework.BranchIDs("Y", "A"),
			"Message28":   testFramework.BranchIDs("B"),
			"Message29":   testFramework.BranchIDs("D"),
		})
	}

	// ISSUE Message30
	{

		testFramework.CreateMessage("Message30", WithStrongParents("Message5"), WithInputs("D"), WithOutput("I", 500))

		testFramework.RegisterBranchID("H", "Message29")
		testFramework.RegisterBranchID("I", "Message30")

		testFramework.tangle.Ledger.BranchDAG.Storage.CachedBranch(testFramework.BranchID("H")).Consume(func(branch *branchdag.Branch) {
			assert.Equal(t, branch.InclusionState(), branchdag.Rejected)
		})

		testFramework.tangle.Ledger.BranchDAG.Storage.CachedBranch(testFramework.BranchID("I")).Consume(func(branch *branchdag.Branch) {
			assert.Equal(t, branch.InclusionState(), branchdag.Rejected)
		})

		testFramework.IssueMessages("Message30").WaitUntilAllTasksProcessed()

		checkMarkers(t, testFramework, map[string]*markers.Markers{
			"Message1":    markers.NewMarkers(markers.NewMarker(0, 1)),
			"Message2":    markers.NewMarkers(markers.NewMarker(0, 0)),
			"Message3":    markers.NewMarkers(markers.NewMarker(0, 0)),
			"Message4":    markers.NewMarkers(markers.NewMarker(0, 0)),
			"Message5":    markers.NewMarkers(markers.NewMarker(0, 0)),
			"Message6":    markers.NewMarkers(markers.NewMarker(0, 2)),
			"Message7":    markers.NewMarkers(markers.NewMarker(0, 3)),
			"Message8":    markers.NewMarkers(markers.NewMarker(0, 4)),
			"Message9":    markers.NewMarkers(markers.NewMarker(0, 3)),
			"Message10":   markers.NewMarkers(markers.NewMarker(0, 3)),
			"Message11":   markers.NewMarkers(markers.NewMarker(0, 5)),
			"Message11.5": markers.NewMarkers(markers.NewMarker(0, 3)),
			"Message12":   markers.NewMarkers(markers.NewMarker(0, 1)),
			"Message13":   markers.NewMarkers(markers.NewMarker(0, 3)),
			"Message13.1": markers.NewMarkers(markers.NewMarker(0, 3)),
			"Message14":   markers.NewMarkers(markers.NewMarker(0, 3)),
			"Message15":   markers.NewMarkers(markers.NewMarker(0, 3)),
			"Message16":   markers.NewMarkers(markers.NewMarker(0, 1)),
			"Message17":   markers.NewMarkers(markers.NewMarker(0, 1)),
			"Message18":   markers.NewMarkers(markers.NewMarker(0, 3)),
			"Message19":   markers.NewMarkers(markers.NewMarker(0, 1)),
			"Message20":   markers.NewMarkers(markers.NewMarker(0, 1)),
			"Message21":   markers.NewMarkers(markers.NewMarker(0, 1)),
			"Message22":   markers.NewMarkers(markers.NewMarker(0, 1)),
			"Message23":   markers.NewMarkers(markers.NewMarker(0, 1)),
			"Message24":   markers.NewMarkers(markers.NewMarker(0, 1)),
			"Message25":   markers.NewMarkers(markers.NewMarker(0, 1)),
			"Message26":   markers.NewMarkers(markers.NewMarker(0, 1)),
			"Message27":   markers.NewMarkers(markers.NewMarker(0, 1)),
			"Message28":   markers.NewMarkers(markers.NewMarker(0, 3)),
			"Message29":   markers.NewMarkers(markers.NewMarker(0, 0)),
			"Message30":   markers.NewMarkers(markers.NewMarker(0, 0)),
		})
		checkMessageMetadataDiffBranchIDs(t, testFramework, map[string][]branchdag.BranchIDs{
			"Message1":    {branchdag.NewBranchIDs(), branchdag.NewBranchIDs()},
			"Message2":    {testFramework.BranchIDs("B"), branchdag.NewBranchIDs()},
			"Message3":    {testFramework.BranchIDs("C"), branchdag.NewBranchIDs()},
			"Message4":    {branchdag.NewBranchIDs(), branchdag.NewBranchIDs()},
			"Message5":    {testFramework.BranchIDs("D"), branchdag.NewBranchIDs()},
			"Message6":    {branchdag.NewBranchIDs(), branchdag.NewBranchIDs()},
			"Message7":    {branchdag.NewBranchIDs(), branchdag.NewBranchIDs()},
			"Message8":    {branchdag.NewBranchIDs(), branchdag.NewBranchIDs()},
			"Message9":    {testFramework.BranchIDs("A", "C"), testFramework.BranchIDs("B")},
			"Message10":   {branchdag.NewBranchIDs(), branchdag.NewBranchIDs()},
			"Message11":   {branchdag.NewBranchIDs(), branchdag.NewBranchIDs()},
			"Message11.5": {testFramework.BranchIDs("A", "C"), testFramework.BranchIDs("B")},
			"Message12":   {testFramework.BranchIDs("E"), branchdag.NewBranchIDs()},
			"Message13":   {testFramework.BranchIDs("E"), testFramework.BranchIDs("D")},
			"Message13.1": {testFramework.BranchIDs("E"), testFramework.BranchIDs("D")},
			"Message14":   {testFramework.BranchIDs("E"), testFramework.BranchIDs("D")},
			"Message15":   {branchdag.NewBranchIDs(), testFramework.BranchIDs("B", "D")},
			"Message16":   {testFramework.BranchIDs("Z"), branchdag.NewBranchIDs()},
			"Message17":   {testFramework.BranchIDs("E", "Y"), branchdag.NewBranchIDs()},
			"Message18":   {testFramework.BranchIDs("Y", "A", "E"), branchdag.NewBranchIDs()},
			"Message19":   {testFramework.BranchIDs("E", "Y"), branchdag.NewBranchIDs()},
			"Message20":   {testFramework.BranchIDs("Y", "E", "B"), branchdag.NewBranchIDs()},
			"Message21":   {testFramework.BranchIDs("B"), branchdag.NewBranchIDs()},
			"Message22":   {testFramework.BranchIDs(), testFramework.BranchIDs("A")},
			"Message23":   {testFramework.BranchIDs("B"), testFramework.BranchIDs("A")},
			"Message24":   {testFramework.BranchIDs("B", "D"), testFramework.BranchIDs("A")},
			"Message25":   {testFramework.BranchIDs("D"), testFramework.BranchIDs("A")},
			"Message26":   {testFramework.BranchIDs("E", "Y"), branchdag.NewBranchIDs()},
			"Message27":   {testFramework.BranchIDs("Y"), branchdag.NewBranchIDs()},
			"Message28":   {branchdag.NewBranchIDs(), testFramework.BranchIDs("D")},
			"Message29":   {testFramework.BranchIDs("H"), branchdag.NewBranchIDs()},
			"Message30":   {testFramework.BranchIDs("D", "I"), branchdag.NewBranchIDs()},
		})
		checkBranchIDs(t, testFramework, map[string]branchdag.BranchIDs{
			"Message1":    testFramework.BranchIDs("A"),
			"Message2":    testFramework.BranchIDs("B"),
			"Message3":    testFramework.BranchIDs("C"),
			"Message4":    branchdag.NewBranchIDs(branchdag.MasterBranchID),
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
			"Message15":   branchdag.NewBranchIDs(branchdag.MasterBranchID),
			"Message16":   testFramework.BranchIDs("Z", "A"),
			"Message17":   testFramework.BranchIDs("Y", "A", "E"),
			"Message18":   testFramework.BranchIDs("Y", "A", "B", "D", "E"),
			"Message19":   testFramework.BranchIDs("Y", "A", "E"),
			"Message20":   testFramework.BranchIDs("Y", "A", "B", "E"),
			"Message21":   testFramework.BranchIDs("A", "B"),
			"Message22":   branchdag.NewBranchIDs(branchdag.MasterBranchID),
			"Message23":   testFramework.BranchIDs("B"),
			"Message24":   testFramework.BranchIDs("B", "D"),
			"Message25":   testFramework.BranchIDs("D"),
			"Message26":   testFramework.BranchIDs("Y", "A", "E"),
			"Message27":   testFramework.BranchIDs("Y", "A"),
			"Message28":   testFramework.BranchIDs("B"),
			"Message29":   testFramework.BranchIDs("H"),
			"Message30":   testFramework.BranchIDs("D", "I"),
		})
	}

	// ISSUE Message31
	{

		testFramework.CreateMessage("Message31", WithStrongParents("Message5"), WithShallowLikeParents("Message1", "Message3"), WithInputs("L2"), WithOutput("M", 500))

		testFramework.IssueMessages("Message31").WaitUntilAllTasksProcessed()

		checkMarkers(t, testFramework, map[string]*markers.Markers{
			"Message1":    markers.NewMarkers(markers.NewMarker(0, 1)),
			"Message2":    markers.NewMarkers(markers.NewMarker(0, 0)),
			"Message3":    markers.NewMarkers(markers.NewMarker(0, 0)),
			"Message4":    markers.NewMarkers(markers.NewMarker(0, 0)),
			"Message5":    markers.NewMarkers(markers.NewMarker(0, 0)),
			"Message6":    markers.NewMarkers(markers.NewMarker(0, 2)),
			"Message7":    markers.NewMarkers(markers.NewMarker(0, 3)),
			"Message8":    markers.NewMarkers(markers.NewMarker(0, 4)),
			"Message9":    markers.NewMarkers(markers.NewMarker(0, 3)),
			"Message10":   markers.NewMarkers(markers.NewMarker(0, 3)),
			"Message11":   markers.NewMarkers(markers.NewMarker(0, 5)),
			"Message11.5": markers.NewMarkers(markers.NewMarker(0, 3)),
			"Message12":   markers.NewMarkers(markers.NewMarker(0, 1)),
			"Message13":   markers.NewMarkers(markers.NewMarker(0, 3)),
			"Message13.1": markers.NewMarkers(markers.NewMarker(0, 3)),
			"Message14":   markers.NewMarkers(markers.NewMarker(0, 3)),
			"Message15":   markers.NewMarkers(markers.NewMarker(0, 3)),
			"Message16":   markers.NewMarkers(markers.NewMarker(0, 1)),
			"Message17":   markers.NewMarkers(markers.NewMarker(0, 1)),
			"Message18":   markers.NewMarkers(markers.NewMarker(0, 3)),
			"Message19":   markers.NewMarkers(markers.NewMarker(0, 1)),
			"Message20":   markers.NewMarkers(markers.NewMarker(0, 1)),
			"Message21":   markers.NewMarkers(markers.NewMarker(0, 1)),
			"Message22":   markers.NewMarkers(markers.NewMarker(0, 1)),
			"Message23":   markers.NewMarkers(markers.NewMarker(0, 1)),
			"Message24":   markers.NewMarkers(markers.NewMarker(0, 1)),
			"Message25":   markers.NewMarkers(markers.NewMarker(0, 1)),
			"Message26":   markers.NewMarkers(markers.NewMarker(0, 1)),
			"Message27":   markers.NewMarkers(markers.NewMarker(0, 1)),
			"Message28":   markers.NewMarkers(markers.NewMarker(0, 3)),
			"Message29":   markers.NewMarkers(markers.NewMarker(0, 0)),
			"Message30":   markers.NewMarkers(markers.NewMarker(0, 0)),
			"Message31":   markers.NewMarkers(markers.NewMarker(0, 0)),
		})
		checkMessageMetadataDiffBranchIDs(t, testFramework, map[string][]branchdag.BranchIDs{
			"Message1":    {branchdag.NewBranchIDs(), branchdag.NewBranchIDs()},
			"Message2":    {testFramework.BranchIDs("B"), branchdag.NewBranchIDs()},
			"Message3":    {testFramework.BranchIDs("C"), branchdag.NewBranchIDs()},
			"Message4":    {branchdag.NewBranchIDs(), branchdag.NewBranchIDs()},
			"Message5":    {testFramework.BranchIDs("D"), branchdag.NewBranchIDs()},
			"Message6":    {branchdag.NewBranchIDs(), branchdag.NewBranchIDs()},
			"Message7":    {branchdag.NewBranchIDs(), branchdag.NewBranchIDs()},
			"Message8":    {branchdag.NewBranchIDs(), branchdag.NewBranchIDs()},
			"Message9":    {testFramework.BranchIDs("A", "C"), testFramework.BranchIDs("B")},
			"Message10":   {branchdag.NewBranchIDs(), branchdag.NewBranchIDs()},
			"Message11":   {branchdag.NewBranchIDs(), branchdag.NewBranchIDs()},
			"Message11.5": {testFramework.BranchIDs("A", "C"), testFramework.BranchIDs("B")},
			"Message12":   {testFramework.BranchIDs("E"), branchdag.NewBranchIDs()},
			"Message13":   {testFramework.BranchIDs("E"), testFramework.BranchIDs("D")},
			"Message13.1": {testFramework.BranchIDs("E"), testFramework.BranchIDs("D")},
			"Message14":   {testFramework.BranchIDs("E"), testFramework.BranchIDs("D")},
			"Message15":   {branchdag.NewBranchIDs(), testFramework.BranchIDs("B", "D")},
			"Message16":   {testFramework.BranchIDs("Z"), branchdag.NewBranchIDs()},
			"Message17":   {testFramework.BranchIDs("E", "Y"), branchdag.NewBranchIDs()},
			"Message18":   {testFramework.BranchIDs("Y", "A", "E"), branchdag.NewBranchIDs()},
			"Message19":   {testFramework.BranchIDs("E", "Y"), branchdag.NewBranchIDs()},
			"Message20":   {testFramework.BranchIDs("Y", "E", "B"), branchdag.NewBranchIDs()},
			"Message21":   {testFramework.BranchIDs("B"), branchdag.NewBranchIDs()},
			"Message22":   {testFramework.BranchIDs(), testFramework.BranchIDs("A")},
			"Message23":   {testFramework.BranchIDs("B"), testFramework.BranchIDs("A")},
			"Message24":   {testFramework.BranchIDs("B", "D"), testFramework.BranchIDs("A")},
			"Message25":   {testFramework.BranchIDs("D"), testFramework.BranchIDs("A")},
			"Message26":   {testFramework.BranchIDs("E", "Y"), branchdag.NewBranchIDs()},
			"Message27":   {testFramework.BranchIDs("Y"), branchdag.NewBranchIDs()},
			"Message28":   {branchdag.NewBranchIDs(), testFramework.BranchIDs("D")},
			"Message29":   {testFramework.BranchIDs("H"), branchdag.NewBranchIDs()},
			"Message30":   {testFramework.BranchIDs("D", "I"), branchdag.NewBranchIDs()},
			"Message31":   {testFramework.BranchIDs("A", "C", "D"), branchdag.NewBranchIDs()},
		})
		checkBranchIDs(t, testFramework, map[string]branchdag.BranchIDs{
			"Message1":    testFramework.BranchIDs("A"),
			"Message2":    testFramework.BranchIDs("B"),
			"Message3":    testFramework.BranchIDs("C"),
			"Message4":    branchdag.NewBranchIDs(branchdag.MasterBranchID),
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
			"Message15":   branchdag.NewBranchIDs(branchdag.MasterBranchID),
			"Message16":   testFramework.BranchIDs("Z", "A"),
			"Message17":   testFramework.BranchIDs("Y", "A", "E"),
			"Message18":   testFramework.BranchIDs("Y", "A", "B", "D", "E"),
			"Message19":   testFramework.BranchIDs("Y", "A", "E"),
			"Message20":   testFramework.BranchIDs("Y", "A", "B", "E"),
			"Message21":   testFramework.BranchIDs("A", "B"),
			"Message22":   branchdag.NewBranchIDs(branchdag.MasterBranchID),
			"Message23":   testFramework.BranchIDs("B"),
			"Message24":   testFramework.BranchIDs("B", "D"),
			"Message25":   testFramework.BranchIDs("D"),
			"Message26":   testFramework.BranchIDs("Y", "A", "E"),
			"Message27":   testFramework.BranchIDs("Y", "A"),
			"Message28":   testFramework.BranchIDs("B"),
			"Message29":   testFramework.BranchIDs("H"),
			"Message30":   testFramework.BranchIDs("D", "I"),
			"Message31":   testFramework.BranchIDs("A", "C", "D"),
		})
	}

	// ISSUE Message32
	{
		testFramework.CreateMessage("Message32", WithStrongParents("Message31"), WithShallowLikeParents("Message16"))

		testFramework.IssueMessages("Message32").WaitUntilAllTasksProcessed()

		checkMarkers(t, testFramework, map[string]*markers.Markers{
			"Message1":    markers.NewMarkers(markers.NewMarker(0, 1)),
			"Message2":    markers.NewMarkers(markers.NewMarker(0, 0)),
			"Message3":    markers.NewMarkers(markers.NewMarker(0, 0)),
			"Message4":    markers.NewMarkers(markers.NewMarker(0, 0)),
			"Message5":    markers.NewMarkers(markers.NewMarker(0, 0)),
			"Message6":    markers.NewMarkers(markers.NewMarker(0, 2)),
			"Message7":    markers.NewMarkers(markers.NewMarker(0, 3)),
			"Message8":    markers.NewMarkers(markers.NewMarker(0, 4)),
			"Message9":    markers.NewMarkers(markers.NewMarker(0, 3)),
			"Message10":   markers.NewMarkers(markers.NewMarker(0, 3)),
			"Message11":   markers.NewMarkers(markers.NewMarker(0, 5)),
			"Message11.5": markers.NewMarkers(markers.NewMarker(0, 3)),
			"Message12":   markers.NewMarkers(markers.NewMarker(0, 1)),
			"Message13":   markers.NewMarkers(markers.NewMarker(0, 3)),
			"Message13.1": markers.NewMarkers(markers.NewMarker(0, 3)),
			"Message14":   markers.NewMarkers(markers.NewMarker(0, 3)),
			"Message15":   markers.NewMarkers(markers.NewMarker(0, 3)),
			"Message16":   markers.NewMarkers(markers.NewMarker(0, 1)),
			"Message17":   markers.NewMarkers(markers.NewMarker(0, 1)),
			"Message18":   markers.NewMarkers(markers.NewMarker(0, 3)),
			"Message19":   markers.NewMarkers(markers.NewMarker(0, 1)),
			"Message20":   markers.NewMarkers(markers.NewMarker(0, 1)),
			"Message21":   markers.NewMarkers(markers.NewMarker(0, 1)),
			"Message22":   markers.NewMarkers(markers.NewMarker(0, 1)),
			"Message23":   markers.NewMarkers(markers.NewMarker(0, 1)),
			"Message24":   markers.NewMarkers(markers.NewMarker(0, 1)),
			"Message25":   markers.NewMarkers(markers.NewMarker(0, 1)),
			"Message26":   markers.NewMarkers(markers.NewMarker(0, 1)),
			"Message27":   markers.NewMarkers(markers.NewMarker(0, 1)),
			"Message28":   markers.NewMarkers(markers.NewMarker(0, 3)),
			"Message29":   markers.NewMarkers(markers.NewMarker(0, 0)),
			"Message30":   markers.NewMarkers(markers.NewMarker(0, 0)),
			"Message31":   markers.NewMarkers(markers.NewMarker(0, 0)),
			"Message32":   markers.NewMarkers(markers.NewMarker(0, 0)),
		})
		checkMessageMetadataDiffBranchIDs(t, testFramework, map[string][]branchdag.BranchIDs{
			"Message1":    {branchdag.NewBranchIDs(), branchdag.NewBranchIDs()},
			"Message2":    {testFramework.BranchIDs("B"), branchdag.NewBranchIDs()},
			"Message3":    {testFramework.BranchIDs("C"), branchdag.NewBranchIDs()},
			"Message4":    {branchdag.NewBranchIDs(), branchdag.NewBranchIDs()},
			"Message5":    {testFramework.BranchIDs("D"), branchdag.NewBranchIDs()},
			"Message6":    {branchdag.NewBranchIDs(), branchdag.NewBranchIDs()},
			"Message7":    {branchdag.NewBranchIDs(), branchdag.NewBranchIDs()},
			"Message8":    {branchdag.NewBranchIDs(), branchdag.NewBranchIDs()},
			"Message9":    {testFramework.BranchIDs("A", "C"), testFramework.BranchIDs("B")},
			"Message10":   {branchdag.NewBranchIDs(), branchdag.NewBranchIDs()},
			"Message11":   {branchdag.NewBranchIDs(), branchdag.NewBranchIDs()},
			"Message11.5": {testFramework.BranchIDs("A", "C"), testFramework.BranchIDs("B")},
			"Message12":   {testFramework.BranchIDs("E"), branchdag.NewBranchIDs()},
			"Message13":   {testFramework.BranchIDs("E"), testFramework.BranchIDs("D")},
			"Message13.1": {testFramework.BranchIDs("E"), testFramework.BranchIDs("D")},
			"Message14":   {testFramework.BranchIDs("E"), testFramework.BranchIDs("D")},
			"Message15":   {branchdag.NewBranchIDs(), testFramework.BranchIDs("B", "D")},
			"Message16":   {testFramework.BranchIDs("Z"), branchdag.NewBranchIDs()},
			"Message17":   {testFramework.BranchIDs("E", "Y"), branchdag.NewBranchIDs()},
			"Message18":   {testFramework.BranchIDs("Y", "A", "E"), branchdag.NewBranchIDs()},
			"Message19":   {testFramework.BranchIDs("E", "Y"), branchdag.NewBranchIDs()},
			"Message20":   {testFramework.BranchIDs("Y", "E", "B"), branchdag.NewBranchIDs()},
			"Message21":   {testFramework.BranchIDs("B"), branchdag.NewBranchIDs()},
			"Message22":   {testFramework.BranchIDs(), testFramework.BranchIDs("A")},
			"Message23":   {testFramework.BranchIDs("B"), testFramework.BranchIDs("A")},
			"Message24":   {testFramework.BranchIDs("B", "D"), testFramework.BranchIDs("A")},
			"Message25":   {testFramework.BranchIDs("D"), testFramework.BranchIDs("A")},
			"Message26":   {testFramework.BranchIDs("E", "Y"), branchdag.NewBranchIDs()},
			"Message27":   {testFramework.BranchIDs("Y"), branchdag.NewBranchIDs()},
			"Message28":   {branchdag.NewBranchIDs(), testFramework.BranchIDs("D")},
			"Message29":   {testFramework.BranchIDs("H"), branchdag.NewBranchIDs()},
			"Message30":   {testFramework.BranchIDs("D", "I"), branchdag.NewBranchIDs()},
			"Message31":   {testFramework.BranchIDs("A", "C", "D"), branchdag.NewBranchIDs()},
			"Message32":   {testFramework.BranchIDs("A", "C", "D", "Z"), branchdag.NewBranchIDs()},
		})
		checkBranchIDs(t, testFramework, map[string]branchdag.BranchIDs{
			"Message1":    testFramework.BranchIDs("A"),
			"Message2":    testFramework.BranchIDs("B"),
			"Message3":    testFramework.BranchIDs("C"),
			"Message4":    branchdag.NewBranchIDs(branchdag.MasterBranchID),
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
			"Message15":   branchdag.NewBranchIDs(branchdag.MasterBranchID),
			"Message16":   testFramework.BranchIDs("Z", "A"),
			"Message17":   testFramework.BranchIDs("Y", "A", "E"),
			"Message18":   testFramework.BranchIDs("Y", "A", "B", "D", "E"),
			"Message19":   testFramework.BranchIDs("Y", "A", "E"),
			"Message20":   testFramework.BranchIDs("Y", "A", "B", "E"),
			"Message21":   testFramework.BranchIDs("A", "B"),
			"Message22":   branchdag.NewBranchIDs(branchdag.MasterBranchID),
			"Message23":   testFramework.BranchIDs("B"),
			"Message24":   testFramework.BranchIDs("B", "D"),
			"Message25":   testFramework.BranchIDs("D"),
			"Message26":   testFramework.BranchIDs("Y", "A", "E"),
			"Message27":   testFramework.BranchIDs("Y", "A"),
			"Message28":   testFramework.BranchIDs("B"),
			"Message29":   testFramework.BranchIDs("H"),
			"Message30":   testFramework.BranchIDs("D", "I"),
			"Message31":   testFramework.BranchIDs("A", "C", "D"),
			"Message32":   testFramework.BranchIDs("A", "C", "D", "Z"),
		})
	}

	// ISSUE Message33
	{
		testFramework.CreateMessage("Message33", WithStrongParents("Message31"), WithShallowLikeParents("Message16"))

		testFramework.IssueMessages("Message33").WaitUntilAllTasksProcessed()

		checkMarkers(t, testFramework, map[string]*markers.Markers{
			"Message1":    markers.NewMarkers(markers.NewMarker(0, 1)),
			"Message2":    markers.NewMarkers(markers.NewMarker(0, 0)),
			"Message3":    markers.NewMarkers(markers.NewMarker(0, 0)),
			"Message4":    markers.NewMarkers(markers.NewMarker(0, 0)),
			"Message5":    markers.NewMarkers(markers.NewMarker(0, 0)),
			"Message6":    markers.NewMarkers(markers.NewMarker(0, 2)),
			"Message7":    markers.NewMarkers(markers.NewMarker(0, 3)),
			"Message8":    markers.NewMarkers(markers.NewMarker(0, 4)),
			"Message9":    markers.NewMarkers(markers.NewMarker(0, 3)),
			"Message10":   markers.NewMarkers(markers.NewMarker(0, 3)),
			"Message11":   markers.NewMarkers(markers.NewMarker(0, 5)),
			"Message11.5": markers.NewMarkers(markers.NewMarker(0, 3)),
			"Message12":   markers.NewMarkers(markers.NewMarker(0, 1)),
			"Message13":   markers.NewMarkers(markers.NewMarker(0, 3)),
			"Message13.1": markers.NewMarkers(markers.NewMarker(0, 3)),
			"Message14":   markers.NewMarkers(markers.NewMarker(0, 3)),
			"Message15":   markers.NewMarkers(markers.NewMarker(0, 3)),
			"Message16":   markers.NewMarkers(markers.NewMarker(0, 1)),
			"Message17":   markers.NewMarkers(markers.NewMarker(0, 1)),
			"Message18":   markers.NewMarkers(markers.NewMarker(0, 3)),
			"Message19":   markers.NewMarkers(markers.NewMarker(0, 1)),
			"Message20":   markers.NewMarkers(markers.NewMarker(0, 1)),
			"Message21":   markers.NewMarkers(markers.NewMarker(0, 1)),
			"Message22":   markers.NewMarkers(markers.NewMarker(0, 1)),
			"Message23":   markers.NewMarkers(markers.NewMarker(0, 1)),
			"Message24":   markers.NewMarkers(markers.NewMarker(0, 1)),
			"Message25":   markers.NewMarkers(markers.NewMarker(0, 1)),
			"Message26":   markers.NewMarkers(markers.NewMarker(0, 1)),
			"Message27":   markers.NewMarkers(markers.NewMarker(0, 1)),
			"Message28":   markers.NewMarkers(markers.NewMarker(0, 3)),
			"Message29":   markers.NewMarkers(markers.NewMarker(0, 0)),
			"Message30":   markers.NewMarkers(markers.NewMarker(0, 0)),
			"Message31":   markers.NewMarkers(markers.NewMarker(0, 0)),
			"Message32":   markers.NewMarkers(markers.NewMarker(0, 0)),
			"Message33":   markers.NewMarkers(markers.NewMarker(0, 0)),
		})
		checkMessageMetadataDiffBranchIDs(t, testFramework, map[string][]branchdag.BranchIDs{
			"Message1":    {branchdag.NewBranchIDs(), branchdag.NewBranchIDs()},
			"Message2":    {testFramework.BranchIDs("B"), branchdag.NewBranchIDs()},
			"Message3":    {testFramework.BranchIDs("C"), branchdag.NewBranchIDs()},
			"Message4":    {branchdag.NewBranchIDs(), branchdag.NewBranchIDs()},
			"Message5":    {testFramework.BranchIDs("D"), branchdag.NewBranchIDs()},
			"Message6":    {branchdag.NewBranchIDs(), branchdag.NewBranchIDs()},
			"Message7":    {branchdag.NewBranchIDs(), branchdag.NewBranchIDs()},
			"Message8":    {branchdag.NewBranchIDs(), branchdag.NewBranchIDs()},
			"Message9":    {testFramework.BranchIDs("A", "C"), testFramework.BranchIDs("B")},
			"Message10":   {branchdag.NewBranchIDs(), branchdag.NewBranchIDs()},
			"Message11":   {branchdag.NewBranchIDs(), branchdag.NewBranchIDs()},
			"Message11.5": {testFramework.BranchIDs("A", "C"), testFramework.BranchIDs("B")},
			"Message12":   {testFramework.BranchIDs("E"), branchdag.NewBranchIDs()},
			"Message13":   {testFramework.BranchIDs("E"), testFramework.BranchIDs("D")},
			"Message13.1": {testFramework.BranchIDs("E"), testFramework.BranchIDs("D")},
			"Message14":   {testFramework.BranchIDs("E"), testFramework.BranchIDs("D")},
			"Message15":   {branchdag.NewBranchIDs(), testFramework.BranchIDs("B", "D")},
			"Message16":   {testFramework.BranchIDs("Z"), branchdag.NewBranchIDs()},
			"Message17":   {testFramework.BranchIDs("E", "Y"), branchdag.NewBranchIDs()},
			"Message18":   {testFramework.BranchIDs("Y", "A", "E"), branchdag.NewBranchIDs()},
			"Message19":   {testFramework.BranchIDs("E", "Y"), branchdag.NewBranchIDs()},
			"Message20":   {testFramework.BranchIDs("Y", "E", "B"), branchdag.NewBranchIDs()},
			"Message21":   {testFramework.BranchIDs("B"), branchdag.NewBranchIDs()},
			"Message22":   {testFramework.BranchIDs(), testFramework.BranchIDs("A")},
			"Message23":   {testFramework.BranchIDs("B"), testFramework.BranchIDs("A")},
			"Message24":   {testFramework.BranchIDs("B", "D"), testFramework.BranchIDs("A")},
			"Message25":   {testFramework.BranchIDs("D"), testFramework.BranchIDs("A")},
			"Message26":   {testFramework.BranchIDs("E", "Y"), branchdag.NewBranchIDs()},
			"Message27":   {testFramework.BranchIDs("Y"), branchdag.NewBranchIDs()},
			"Message28":   {branchdag.NewBranchIDs(), testFramework.BranchIDs("D")},
			"Message29":   {testFramework.BranchIDs("H"), branchdag.NewBranchIDs()},
			"Message30":   {testFramework.BranchIDs("D", "I"), branchdag.NewBranchIDs()},
			"Message31":   {testFramework.BranchIDs("A", "C", "D"), branchdag.NewBranchIDs()},
			"Message32":   {testFramework.BranchIDs("A", "C", "D", "Z"), branchdag.NewBranchIDs()},
			"Message33":   {testFramework.BranchIDs("A", "C", "D", "Z"), branchdag.NewBranchIDs()},
		})
		checkBranchIDs(t, testFramework, map[string]branchdag.BranchIDs{
			"Message1":    testFramework.BranchIDs("A"),
			"Message2":    testFramework.BranchIDs("B"),
			"Message3":    testFramework.BranchIDs("C"),
			"Message4":    branchdag.NewBranchIDs(branchdag.MasterBranchID),
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
			"Message15":   branchdag.NewBranchIDs(branchdag.MasterBranchID),
			"Message16":   testFramework.BranchIDs("Z", "A"),
			"Message17":   testFramework.BranchIDs("Y", "A", "E"),
			"Message18":   testFramework.BranchIDs("Y", "A", "B", "D", "E"),
			"Message19":   testFramework.BranchIDs("Y", "A", "E"),
			"Message20":   testFramework.BranchIDs("Y", "A", "B", "E"),
			"Message21":   testFramework.BranchIDs("A", "B"),
			"Message22":   branchdag.NewBranchIDs(branchdag.MasterBranchID),
			"Message23":   testFramework.BranchIDs("B"),
			"Message24":   testFramework.BranchIDs("B", "D"),
			"Message25":   testFramework.BranchIDs("D"),
			"Message26":   testFramework.BranchIDs("Y", "A", "E"),
			"Message27":   testFramework.BranchIDs("Y", "A"),
			"Message28":   testFramework.BranchIDs("B"),
			"Message29":   testFramework.BranchIDs("H"),
			"Message30":   testFramework.BranchIDs("D", "I"),
			"Message31":   testFramework.BranchIDs("A", "C", "D"),
			"Message32":   testFramework.BranchIDs("A", "C", "D", "Z"),
			"Message33":   testFramework.BranchIDs("A", "C", "D", "Z"),
		})
	}

	// ISSUE Message34
	{
		msg := testFramework.CreateMessage("Message34", WithStrongParents("Message31"), WithInputs("L2"), WithOutput("N", 500))

		tangle.Storage.MessageMetadata(msg.ID()).Consume(func(messageMetadata *MessageMetadata) {
			assert.True(t, messageMetadata.subjectivelyInvalid)
		})

		testFramework.RegisterBranchID("M", "Message31")
		testFramework.RegisterBranchID("N", "Message34")

		testFramework.IssueMessages("Message34").WaitUntilAllTasksProcessed()

		checkMarkers(t, testFramework, map[string]*markers.Markers{
			"Message1":    markers.NewMarkers(markers.NewMarker(0, 1)),
			"Message2":    markers.NewMarkers(markers.NewMarker(0, 0)),
			"Message3":    markers.NewMarkers(markers.NewMarker(0, 0)),
			"Message4":    markers.NewMarkers(markers.NewMarker(0, 0)),
			"Message5":    markers.NewMarkers(markers.NewMarker(0, 0)),
			"Message6":    markers.NewMarkers(markers.NewMarker(0, 2)),
			"Message7":    markers.NewMarkers(markers.NewMarker(0, 3)),
			"Message8":    markers.NewMarkers(markers.NewMarker(0, 4)),
			"Message9":    markers.NewMarkers(markers.NewMarker(0, 3)),
			"Message10":   markers.NewMarkers(markers.NewMarker(0, 3)),
			"Message11":   markers.NewMarkers(markers.NewMarker(0, 5)),
			"Message11.5": markers.NewMarkers(markers.NewMarker(0, 3)),
			"Message12":   markers.NewMarkers(markers.NewMarker(0, 1)),
			"Message13":   markers.NewMarkers(markers.NewMarker(0, 3)),
			"Message13.1": markers.NewMarkers(markers.NewMarker(0, 3)),
			"Message14":   markers.NewMarkers(markers.NewMarker(0, 3)),
			"Message15":   markers.NewMarkers(markers.NewMarker(0, 3)),
			"Message16":   markers.NewMarkers(markers.NewMarker(0, 1)),
			"Message17":   markers.NewMarkers(markers.NewMarker(0, 1)),
			"Message18":   markers.NewMarkers(markers.NewMarker(0, 3)),
			"Message19":   markers.NewMarkers(markers.NewMarker(0, 1)),
			"Message20":   markers.NewMarkers(markers.NewMarker(0, 1)),
			"Message21":   markers.NewMarkers(markers.NewMarker(0, 1)),
			"Message22":   markers.NewMarkers(markers.NewMarker(0, 1)),
			"Message23":   markers.NewMarkers(markers.NewMarker(0, 1)),
			"Message24":   markers.NewMarkers(markers.NewMarker(0, 1)),
			"Message25":   markers.NewMarkers(markers.NewMarker(0, 1)),
			"Message26":   markers.NewMarkers(markers.NewMarker(0, 1)),
			"Message27":   markers.NewMarkers(markers.NewMarker(0, 1)),
			"Message28":   markers.NewMarkers(markers.NewMarker(0, 3)),
			"Message29":   markers.NewMarkers(markers.NewMarker(0, 0)),
			"Message30":   markers.NewMarkers(markers.NewMarker(0, 0)),
			"Message31":   markers.NewMarkers(markers.NewMarker(0, 0)),
			"Message32":   markers.NewMarkers(markers.NewMarker(0, 0)),
			"Message33":   markers.NewMarkers(markers.NewMarker(0, 0)),
			"Message34":   markers.NewMarkers(markers.NewMarker(0, 0)),
		})
		checkMessageMetadataDiffBranchIDs(t, testFramework, map[string][]branchdag.BranchIDs{
			"Message1":    {branchdag.NewBranchIDs(), branchdag.NewBranchIDs()},
			"Message2":    {testFramework.BranchIDs("B"), branchdag.NewBranchIDs()},
			"Message3":    {testFramework.BranchIDs("C"), branchdag.NewBranchIDs()},
			"Message4":    {branchdag.NewBranchIDs(), branchdag.NewBranchIDs()},
			"Message5":    {testFramework.BranchIDs("D"), branchdag.NewBranchIDs()},
			"Message6":    {branchdag.NewBranchIDs(), branchdag.NewBranchIDs()},
			"Message7":    {branchdag.NewBranchIDs(), branchdag.NewBranchIDs()},
			"Message8":    {branchdag.NewBranchIDs(), branchdag.NewBranchIDs()},
			"Message9":    {testFramework.BranchIDs("A", "C"), testFramework.BranchIDs("B")},
			"Message10":   {branchdag.NewBranchIDs(), branchdag.NewBranchIDs()},
			"Message11":   {branchdag.NewBranchIDs(), branchdag.NewBranchIDs()},
			"Message11.5": {testFramework.BranchIDs("A", "C"), testFramework.BranchIDs("B")},
			"Message12":   {testFramework.BranchIDs("E"), branchdag.NewBranchIDs()},
			"Message13":   {testFramework.BranchIDs("E"), testFramework.BranchIDs("D")},
			"Message13.1": {testFramework.BranchIDs("E"), testFramework.BranchIDs("D")},
			"Message14":   {testFramework.BranchIDs("E"), testFramework.BranchIDs("D")},
			"Message15":   {branchdag.NewBranchIDs(), testFramework.BranchIDs("B", "D")},
			"Message16":   {testFramework.BranchIDs("Z"), branchdag.NewBranchIDs()},
			"Message17":   {testFramework.BranchIDs("E", "Y"), branchdag.NewBranchIDs()},
			"Message18":   {testFramework.BranchIDs("Y", "A", "E"), branchdag.NewBranchIDs()},
			"Message19":   {testFramework.BranchIDs("E", "Y"), branchdag.NewBranchIDs()},
			"Message20":   {testFramework.BranchIDs("Y", "E", "B"), branchdag.NewBranchIDs()},
			"Message21":   {testFramework.BranchIDs("B"), branchdag.NewBranchIDs()},
			"Message22":   {testFramework.BranchIDs(), testFramework.BranchIDs("A")},
			"Message23":   {testFramework.BranchIDs("B"), testFramework.BranchIDs("A")},
			"Message24":   {testFramework.BranchIDs("B", "D"), testFramework.BranchIDs("A")},
			"Message25":   {testFramework.BranchIDs("D"), testFramework.BranchIDs("A")},
			"Message26":   {testFramework.BranchIDs("E", "Y"), branchdag.NewBranchIDs()},
			"Message27":   {testFramework.BranchIDs("Y"), branchdag.NewBranchIDs()},
			"Message28":   {branchdag.NewBranchIDs(), testFramework.BranchIDs("D")},
			"Message29":   {testFramework.BranchIDs("H"), branchdag.NewBranchIDs()},
			"Message30":   {testFramework.BranchIDs("D", "I"), branchdag.NewBranchIDs()},
			"Message31":   {testFramework.BranchIDs("A", "C", "D", "M"), branchdag.NewBranchIDs()},
			"Message32":   {testFramework.BranchIDs("A", "C", "D", "M", "Z"), branchdag.NewBranchIDs()},
			"Message33":   {testFramework.BranchIDs("A", "C", "D", "M", "Z"), branchdag.NewBranchIDs()},
			"Message34":   {testFramework.BranchIDs("A", "C", "D", "M", "N"), branchdag.NewBranchIDs()},
		})
		checkBranchIDs(t, testFramework, map[string]branchdag.BranchIDs{
			"Message1":    testFramework.BranchIDs("A"),
			"Message2":    testFramework.BranchIDs("B"),
			"Message3":    testFramework.BranchIDs("C"),
			"Message4":    branchdag.NewBranchIDs(branchdag.MasterBranchID),
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
			"Message15":   branchdag.NewBranchIDs(branchdag.MasterBranchID),
			"Message16":   testFramework.BranchIDs("Z", "A"),
			"Message17":   testFramework.BranchIDs("Y", "A", "E"),
			"Message18":   testFramework.BranchIDs("Y", "A", "B", "D", "E"),
			"Message19":   testFramework.BranchIDs("Y", "A", "E"),
			"Message20":   testFramework.BranchIDs("Y", "A", "B", "E"),
			"Message21":   testFramework.BranchIDs("A", "B"),
			"Message22":   branchdag.NewBranchIDs(branchdag.MasterBranchID),
			"Message23":   testFramework.BranchIDs("B"),
			"Message24":   testFramework.BranchIDs("B", "D"),
			"Message25":   testFramework.BranchIDs("D"),
			"Message26":   testFramework.BranchIDs("Y", "A", "E"),
			"Message27":   testFramework.BranchIDs("Y", "A"),
			"Message28":   testFramework.BranchIDs("B"),
			"Message29":   testFramework.BranchIDs("H"),
			"Message30":   testFramework.BranchIDs("D", "I"),
			"Message31":   testFramework.BranchIDs("A", "C", "D", "M"),
			"Message32":   testFramework.BranchIDs("A", "C", "D", "M", "Z"),
			"Message33":   testFramework.BranchIDs("A", "C", "D", "M", "Z"),
			"Message34":   testFramework.BranchIDs("A", "C", "D", "M", "N"),
		})
	}
}

func TestBookerMarkerMappingContinue(t *testing.T) {
	tg := NewTestTangle()
	defer tg.Shutdown()

	testFramework := NewMessageTestFramework(
		tg,
		WithGenesisOutput("A", 500),
		WithGenesisOutput("B", 500),
	)

	tg.Setup()

	// ISSUE Message1
	{
		testFramework.CreateMessage("Message1", WithStrongParents("Genesis"))
		testFramework.IssueMessages("Message1").WaitUntilAllTasksProcessed()

		checkMarkers(t, testFramework, map[string]*markers.Markers{
			"Message1": markers.NewMarkers(markers.NewMarker(0, 1)),
		})
		checkMessageMetadataDiffBranchIDs(t, testFramework, map[string][]branchdag.BranchIDs{
			"Message1": {branchdag.NewBranchIDs(), branchdag.NewBranchIDs()},
		})
		checkBranchIDs(t, testFramework, map[string]branchdag.BranchIDs{
			"Message1": branchdag.NewBranchIDs(branchdag.MasterBranchID),
		})
	}

	// ISSUE Message2
	{
		testFramework.CreateMessage("Message2", WithStrongParents("Message1"), WithInputs("A"), WithOutput("blue", 500))
		testFramework.IssueMessages("Message2").WaitUntilAllTasksProcessed()

		checkMarkers(t, testFramework, map[string]*markers.Markers{
			"Message1": markers.NewMarkers(markers.NewMarker(0, 1)),
			"Message2": markers.NewMarkers(markers.NewMarker(0, 2)),
		})
		checkMessageMetadataDiffBranchIDs(t, testFramework, map[string][]branchdag.BranchIDs{
			"Message1": {branchdag.NewBranchIDs(), branchdag.NewBranchIDs()},
			"Message2": {branchdag.NewBranchIDs(), branchdag.NewBranchIDs()},
		})
		checkBranchIDs(t, testFramework, map[string]branchdag.BranchIDs{
			"Message1": branchdag.NewBranchIDs(branchdag.MasterBranchID),
			"Message2": branchdag.NewBranchIDs(branchdag.MasterBranchID),
		})
	}

	// ISSUE Message3
	{
		testFramework.CreateMessage("Message3", WithStrongParents("Message2"))

		testFramework.IssueMessages("Message3").WaitUntilAllTasksProcessed()

		checkMarkers(t, testFramework, map[string]*markers.Markers{
			"Message1": markers.NewMarkers(markers.NewMarker(0, 1)),
			"Message2": markers.NewMarkers(markers.NewMarker(0, 2)),
			"Message3": markers.NewMarkers(markers.NewMarker(0, 3)),
		})
		checkMessageMetadataDiffBranchIDs(t, testFramework, map[string][]branchdag.BranchIDs{
			"Message1": {branchdag.NewBranchIDs(), branchdag.NewBranchIDs()},
			"Message2": {branchdag.NewBranchIDs(), branchdag.NewBranchIDs()},
			"Message3": {branchdag.NewBranchIDs(), branchdag.NewBranchIDs()},
		})
		checkBranchIDs(t, testFramework, map[string]branchdag.BranchIDs{
			"Message1": branchdag.NewBranchIDs(branchdag.MasterBranchID),
			"Message2": branchdag.NewBranchIDs(branchdag.MasterBranchID),
			"Message3": branchdag.NewBranchIDs(branchdag.MasterBranchID),
		})
	}

	// ISSUE Message4
	{
		testFramework.CreateMessage("Message4", WithStrongParents("Message1"), WithInputs("A"), WithOutput("red", 500))

		testFramework.RegisterBranchID("blue", "Message2")
		testFramework.RegisterBranchID("red", "Message4")

		testFramework.IssueMessages("Message4").WaitUntilAllTasksProcessed()

		checkMarkers(t, testFramework, map[string]*markers.Markers{
			"Message1": markers.NewMarkers(markers.NewMarker(0, 1)),
			"Message2": markers.NewMarkers(markers.NewMarker(0, 2)),
			"Message3": markers.NewMarkers(markers.NewMarker(0, 3)),
			"Message4": markers.NewMarkers(markers.NewMarker(0, 1)),
		})
		checkMessageMetadataDiffBranchIDs(t, testFramework, map[string][]branchdag.BranchIDs{
			"Message1": {branchdag.NewBranchIDs(), branchdag.NewBranchIDs()},
			"Message2": {branchdag.NewBranchIDs(), branchdag.NewBranchIDs()},
			"Message3": {branchdag.NewBranchIDs(), branchdag.NewBranchIDs()},
			"Message4": {testFramework.BranchIDs("red"), branchdag.NewBranchIDs()},
		})
		checkBranchIDs(t, testFramework, map[string]branchdag.BranchIDs{
			"Message1": branchdag.NewBranchIDs(branchdag.MasterBranchID),
			"Message2": testFramework.BranchIDs("blue"),
			"Message3": testFramework.BranchIDs("blue"),
			"Message4": testFramework.BranchIDs("red"),
		})
	}

	// ISSUE Message5
	{

		testFramework.CreateMessage("Message5", WithStrongParents("Message3"))

		testFramework.IssueMessages("Message5").WaitUntilAllTasksProcessed()

		checkMarkers(t, testFramework, map[string]*markers.Markers{
			"Message1": markers.NewMarkers(markers.NewMarker(0, 1)),
			"Message2": markers.NewMarkers(markers.NewMarker(0, 2)),
			"Message3": markers.NewMarkers(markers.NewMarker(0, 3)),
			"Message4": markers.NewMarkers(markers.NewMarker(0, 1)),
			"Message5": markers.NewMarkers(markers.NewMarker(0, 4)),
		})
		checkMessageMetadataDiffBranchIDs(t, testFramework, map[string][]branchdag.BranchIDs{
			"Message1": {branchdag.NewBranchIDs(), branchdag.NewBranchIDs()},
			"Message2": {branchdag.NewBranchIDs(), branchdag.NewBranchIDs()},
			"Message3": {branchdag.NewBranchIDs(), branchdag.NewBranchIDs()},
			"Message4": {testFramework.BranchIDs("red"), branchdag.NewBranchIDs()},
			"Message5": {branchdag.NewBranchIDs(), branchdag.NewBranchIDs()},
		})
		checkBranchIDs(t, testFramework, map[string]branchdag.BranchIDs{
			"Message1": branchdag.NewBranchIDs(branchdag.MasterBranchID),
			"Message2": testFramework.BranchIDs("blue"),
			"Message3": testFramework.BranchIDs("blue"),
			"Message4": testFramework.BranchIDs("red"),
			"Message5": testFramework.BranchIDs("blue"),
		})
	}

	// ISSUE Message6
	{

		tg.Ledger.BranchDAG.SetBranchConfirmed(testFramework.BranchID("red"))

		testFramework.CreateMessage("Message6", WithStrongParents("Message4"))

		testFramework.IssueMessages("Message6").WaitUntilAllTasksProcessed()

		checkMarkers(t, testFramework, map[string]*markers.Markers{
			"Message1": markers.NewMarkers(markers.NewMarker(0, 1)),
			"Message2": markers.NewMarkers(markers.NewMarker(0, 2)),
			"Message3": markers.NewMarkers(markers.NewMarker(0, 3)),
			"Message4": markers.NewMarkers(markers.NewMarker(0, 1)),
			"Message5": markers.NewMarkers(markers.NewMarker(0, 4)),
			"Message6": markers.NewMarkers(markers.NewMarker(0, 1)),
		})
		checkMessageMetadataDiffBranchIDs(t, testFramework, map[string][]branchdag.BranchIDs{
			"Message1": {branchdag.NewBranchIDs(), branchdag.NewBranchIDs()},
			"Message2": {branchdag.NewBranchIDs(), branchdag.NewBranchIDs()},
			"Message3": {branchdag.NewBranchIDs(), branchdag.NewBranchIDs()},
			"Message4": {testFramework.BranchIDs("red"), branchdag.NewBranchIDs()},
			"Message5": {branchdag.NewBranchIDs(), branchdag.NewBranchIDs()},
			"Message6": {branchdag.NewBranchIDs(), branchdag.NewBranchIDs()},
		})
		checkBranchIDs(t, testFramework, map[string]branchdag.BranchIDs{
			"Message1": branchdag.NewBranchIDs(branchdag.MasterBranchID),
			"Message2": testFramework.BranchIDs("blue"),
			"Message3": testFramework.BranchIDs("blue"),
			"Message4": testFramework.BranchIDs("red"),
			"Message5": testFramework.BranchIDs("blue"),
			"Message6": branchdag.NewBranchIDs(branchdag.MasterBranchID),
		})
	}

	// ISSUE Message7
	{

		testFramework.CreateMessage("Message7", WithStrongParents("Message6"))

		testFramework.IssueMessages("Message7").WaitUntilAllTasksProcessed()

		checkMarkers(t, testFramework, map[string]*markers.Markers{
			"Message1": markers.NewMarkers(markers.NewMarker(0, 1)),
			"Message2": markers.NewMarkers(markers.NewMarker(0, 2)),
			"Message3": markers.NewMarkers(markers.NewMarker(0, 3)),
			"Message4": markers.NewMarkers(markers.NewMarker(0, 1)),
			"Message5": markers.NewMarkers(markers.NewMarker(0, 4)),
			"Message6": markers.NewMarkers(markers.NewMarker(0, 1)),
			"Message7": markers.NewMarkers(markers.NewMarker(0, 1)),
		})
		checkMessageMetadataDiffBranchIDs(t, testFramework, map[string][]branchdag.BranchIDs{
			"Message1": {branchdag.NewBranchIDs(), branchdag.NewBranchIDs()},
			"Message2": {branchdag.NewBranchIDs(), branchdag.NewBranchIDs()},
			"Message3": {branchdag.NewBranchIDs(), branchdag.NewBranchIDs()},
			"Message4": {testFramework.BranchIDs("red"), branchdag.NewBranchIDs()},
			"Message5": {branchdag.NewBranchIDs(), branchdag.NewBranchIDs()},
			"Message6": {branchdag.NewBranchIDs(), branchdag.NewBranchIDs()},
			"Message7": {branchdag.NewBranchIDs(), branchdag.NewBranchIDs()},
		})
		checkBranchIDs(t, testFramework, map[string]branchdag.BranchIDs{
			"Message1": branchdag.NewBranchIDs(branchdag.MasterBranchID),
			"Message2": testFramework.BranchIDs("blue"),
			"Message3": testFramework.BranchIDs("blue"),
			"Message4": testFramework.BranchIDs("red"),
			"Message5": testFramework.BranchIDs("blue"),
			"Message6": branchdag.NewBranchIDs(branchdag.MasterBranchID),
			"Message7": branchdag.NewBranchIDs(branchdag.MasterBranchID),
		})
	}
}

func TestObjectiveInvalidity(t *testing.T) {
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
		testFramework.IssueMessages("Message1").WaitUntilAllTasksProcessed()

		checkMarkers(t, testFramework, map[string]*markers.Markers{
			"Message1": markers.NewMarkers(markers.NewMarker(0, 1)),
		})
		checkMessageMetadataDiffBranchIDs(t, testFramework, map[string][]branchdag.BranchIDs{
			"Message1": {branchdag.NewBranchIDs(), branchdag.NewBranchIDs()},
		})
		checkBranchIDs(t, testFramework, map[string]branchdag.BranchIDs{
			"Message1": branchdag.NewBranchIDs(branchdag.MasterBranchID),
		})
	}

	// ISSUE Message2
	{
		testFramework.CreateMessage("Message2", WithStrongParents("Genesis"), WithInputs("A", "B"), WithOutput("E", 1000))

		testFramework.RegisterBranchID("A", "Message1")
		testFramework.RegisterBranchID("B", "Message2")

		testFramework.IssueMessages("Message2").WaitUntilAllTasksProcessed()

		checkMarkers(t, testFramework, map[string]*markers.Markers{
			"Message1": markers.NewMarkers(markers.NewMarker(0, 1)),
			"Message2": markers.NewMarkers(markers.NewMarker(0, 0)),
		})
		checkMessageMetadataDiffBranchIDs(t, testFramework, map[string][]branchdag.BranchIDs{
			"Message1": {branchdag.NewBranchIDs(), branchdag.NewBranchIDs()},
			"Message2": {testFramework.BranchIDs("B"), branchdag.NewBranchIDs()},
		})
		checkBranchIDs(t, testFramework, map[string]branchdag.BranchIDs{
			"Message1": testFramework.BranchIDs("A"),
			"Message2": testFramework.BranchIDs("B"),
		})
	}

	// ISSUE Message3
	{
		testFramework.CreateMessage("Message3", WithStrongParents("Genesis"), WithInputs("B"), WithOutput("F", 500))

		testFramework.RegisterBranchID("C", "Message3")

		testFramework.IssueMessages("Message3").WaitUntilAllTasksProcessed()

		checkMarkers(t, testFramework, map[string]*markers.Markers{
			"Message1": markers.NewMarkers(markers.NewMarker(0, 1)),
			"Message2": markers.NewMarkers(markers.NewMarker(0, 0)),
			"Message3": markers.NewMarkers(markers.NewMarker(0, 0)),
		})
		checkMessageMetadataDiffBranchIDs(t, testFramework, map[string][]branchdag.BranchIDs{
			"Message1": {branchdag.NewBranchIDs(), branchdag.NewBranchIDs()},
			"Message2": {testFramework.BranchIDs("B"), branchdag.NewBranchIDs()},
			"Message3": {testFramework.BranchIDs("C"), branchdag.NewBranchIDs()},
		})
		checkBranchIDs(t, testFramework, map[string]branchdag.BranchIDs{
			"Message1": testFramework.BranchIDs("A"),
			"Message2": testFramework.BranchIDs("B"),
			"Message3": testFramework.BranchIDs("C"),
		})
	}

	// ISSUE Message4
	{
		testFramework.CreateMessage("Message4", WithStrongParents("Message3"))
		testFramework.IssueMessages("Message4").WaitUntilAllTasksProcessed()

		checkMarkers(t, testFramework, map[string]*markers.Markers{
			"Message1": markers.NewMarkers(markers.NewMarker(0, 1)),
			"Message2": markers.NewMarkers(markers.NewMarker(0, 0)),
			"Message3": markers.NewMarkers(markers.NewMarker(0, 0)),
			"Message4": markers.NewMarkers(markers.NewMarker(0, 0)),
		})
		checkMessageMetadataDiffBranchIDs(t, testFramework, map[string][]branchdag.BranchIDs{
			"Message1": {branchdag.NewBranchIDs(), branchdag.NewBranchIDs()},
			"Message2": {testFramework.BranchIDs("B"), branchdag.NewBranchIDs()},
			"Message3": {testFramework.BranchIDs("C"), branchdag.NewBranchIDs()},
			"Message4": {testFramework.BranchIDs("C"), branchdag.NewBranchIDs()},
		})
		checkBranchIDs(t, testFramework, map[string]branchdag.BranchIDs{
			"Message1": testFramework.BranchIDs("A"),
			"Message2": testFramework.BranchIDs("B"),
			"Message3": testFramework.BranchIDs("C"),
			"Message4": testFramework.BranchIDs("C"),
		})
	}

	// ISSUE Message5
	{
		msg := testFramework.CreateMessage("Message5", WithStrongParents("Message4"), WithShallowDislikeParents("Message4"))
		testFramework.IssueMessages("Message5").WaitUntilAllTasksProcessed()

		tangle.Storage.MessageMetadata(msg.ID()).Consume(func(messageMetadata *MessageMetadata) {
			assert.True(t, messageMetadata.IsObjectivelyInvalid())
		})

		checkMarkers(t, testFramework, map[string]*markers.Markers{
			"Message1": markers.NewMarkers(markers.NewMarker(0, 1)),
			"Message2": markers.NewMarkers(markers.NewMarker(0, 0)),
			"Message3": markers.NewMarkers(markers.NewMarker(0, 0)),
			"Message4": markers.NewMarkers(markers.NewMarker(0, 0)),
		})
		checkMessageMetadataDiffBranchIDs(t, testFramework, map[string][]branchdag.BranchIDs{
			"Message1": {branchdag.NewBranchIDs(), branchdag.NewBranchIDs()},
			"Message2": {testFramework.BranchIDs("B"), branchdag.NewBranchIDs()},
			"Message3": {testFramework.BranchIDs("C"), branchdag.NewBranchIDs()},
			"Message4": {testFramework.BranchIDs("C"), branchdag.NewBranchIDs()},
		})
		checkBranchIDs(t, testFramework, map[string]branchdag.BranchIDs{
			"Message1": testFramework.BranchIDs("A"),
			"Message2": testFramework.BranchIDs("B"),
			"Message3": testFramework.BranchIDs("C"),
			"Message4": testFramework.BranchIDs("C"),
		})
	}
}

func TestFutureConeDislike(t *testing.T) {
	tangle := NewTestTangle(WithBranchDAGOptions(branchdag.WithMergeToMaster(false)))
	defer tangle.Shutdown()

	testFramework := NewMessageTestFramework(
		tangle,
		WithGenesisOutput("G", 1),
	)

	tangle.Setup()

	testFramework.CreateMessage("Message1", WithStrongParents("Genesis"), WithInputs("G"), WithOutput("A", 1))
	testFramework.CreateMessage("Message1*", WithStrongParents("Genesis"), WithInputs("G"), WithOutput("A*", 1))
	testFramework.CreateMessage("Message2", WithStrongParents("Message1"), WithInputs("A"), WithOutput("B", 1))
	testFramework.CreateMessage("Message2*", WithStrongParents("Message1"), WithInputs("A"), WithOutput("B*", 1))
	testFramework.CreateMessage("Message3", WithStrongParents("Message2"), WithShallowDislikeParents("Message1*"))
	testFramework.CreateMessage("Message4", WithStrongParents("Message2"), WithShallowDislikeParents("Message1*"))

	testFramework.RegisterBranchID("A", "Message1")
	testFramework.RegisterBranchID("A*", "Message1*")
	testFramework.RegisterBranchID("B", "Message2")
	testFramework.RegisterBranchID("B*", "Message2*")

	{
		testFramework.IssueMessages("Message1", "Message1*", "Message2", "Message3").WaitUntilAllTasksProcessed()

		checkBranchIDs(t, testFramework, map[string]branchdag.BranchIDs{
			"Message1":  testFramework.BranchIDs("A"),
			"Message1*": testFramework.BranchIDs("A*"),
			"Message2":  testFramework.BranchIDs("A"),
			"Message3":  branchdag.NewBranchIDs(branchdag.MasterBranchID),
		})
	}

	{
		testFramework.IssueMessages("Message2*").WaitUntilAllTasksProcessed()

		checkBranchIDs(t, testFramework, map[string]branchdag.BranchIDs{
			"Message1":  testFramework.BranchIDs("A"),
			"Message1*": testFramework.BranchIDs("A*"),
			"Message2":  testFramework.BranchIDs("B"),
			"Message2*": testFramework.BranchIDs("A", "B*"),
			"Message3":  branchdag.NewBranchIDs(branchdag.MasterBranchID),
		})
	}

	{
		testFramework.IssueMessages("Message4").WaitUntilAllTasksProcessed()

		checkBranchIDs(t, testFramework, map[string]branchdag.BranchIDs{
			"Message1":  testFramework.BranchIDs("A"),
			"Message1*": testFramework.BranchIDs("A*"),
			"Message2":  testFramework.BranchIDs("B"),
			"Message2*": testFramework.BranchIDs("A", "B*"),
			"Message3":  branchdag.NewBranchIDs(branchdag.MasterBranchID),
			"Message4":  branchdag.NewBranchIDs(branchdag.MasterBranchID),
		})
	}
}

func TestMultiThreadedBookingAndForking(t *testing.T) {
	const layersNum = 127
	const widthSize = 8 // since we reference all messages in the layer below, this is limited by the max parents

	tangle := NewTestTangle(WithBranchDAGOptions(branchdag.WithMergeToMaster(false)))
	defer tangle.Shutdown()

	// tangle.Booker.Events.MessageBooked.Attach(event.NewClosure(func(event *MessageBookedEvent) {
	// 	fmt.Println("Booked", event.MessageID)
	// }))

	testFramework := NewMessageTestFramework(
		tangle,
		WithGenesisOutput("G", layersNum),
	)

	tangle.Setup()

	// Create base-layer outputs to double-spend
	genesisMessageOptions := []MessageOption{
		WithStrongParents("Genesis"),
		WithInputs("G"),
	}
	for layer := 0; layer < layersNum; layer++ {
		genesisMessageOptions = append(genesisMessageOptions, WithOutput(fmt.Sprintf("G.%d", layer), 1))
	}

	testFramework.CreateMessage("Message.G", genesisMessageOptions...)
	testFramework.IssueMessages("Message.G").WaitUntilAllTasksProcessed()

	msgs := make([]string, 0)

	for layer := 0; layer < layersNum; layer++ {
		for width := 0; width < widthSize; width++ {
			msgName := fmt.Sprintf("Message.%d.%d", layer, width)
			strongParents := make([]string, 0)
			if layer == 0 {
				strongParents = append(strongParents, "Message.G")
			} else {
				for innerWidth := 0; innerWidth < widthSize; innerWidth++ {
					strongParents = append(strongParents, fmt.Sprintf("Message.%d.%d", layer-1, innerWidth))
				}
			}

			var input string
			var output string
			var conflict string

			// We fork on the first two messages for each layer
			if width < 2 {
				input = fmt.Sprintf("G.%d", layer)
				output = fmt.Sprintf("O.%d.%d", layer, width)
				conflict = fmt.Sprintf("C.%d.%d", layer, width)
			}

			if input != "" {
				testFramework.CreateMessage(msgName, WithStrongParents(strongParents...), WithInputs(input), WithOutput(output, 1))
				testFramework.RegisterBranchID(conflict, msgName)
				testFramework.RegisterTransactionID(conflict, msgName)
			} else {
				testFramework.CreateMessage(msgName, WithStrongParents(strongParents...))
			}

			msgs = append(msgs, msgName)
		}
	}

	rand.Seed(time.Now().UnixNano())

	var wg sync.WaitGroup

	for i := 0; i < len(msgs); i++ {
		wg.Add(1)
		go func(i int) {
			// time.Sleep(time.Duration(int(50 * 1000000 * rand.Float32())))
			time.Sleep(time.Duration(int(50 * 1000 * rand.Float32())))
			testFramework.IssueMessages(msgs[i])
			wg.Done()
		}(i)
	}

	wg.Wait()
	testFramework.WaitUntilAllTasksProcessed()

	expectedBranches := make(map[string]branchdag.BranchIDs)
	for layer := 0; layer < layersNum; layer++ {
		for width := 0; width < widthSize; width++ {
			msgName := fmt.Sprintf("Message.%d.%d", layer, width)
			branches := make([]string, 0)

			// Add conflicts of the current layer
			if width < 2 {
				branches = append(branches, fmt.Sprintf("C.%d.%d", layer, width))
			}

			for innerLayer := layer - 1; innerLayer >= 0; innerLayer-- {
				branches = append(branches, fmt.Sprintf("C.%d.%d", innerLayer, 0))
				branches = append(branches, fmt.Sprintf("C.%d.%d", innerLayer, 1))
			}

			if layer == 0 && width >= 2 {
				expectedBranches[msgName] = branchdag.NewBranchIDs(branchdag.MasterBranchID)
				continue
			}

			expectedBranches[msgName] = testFramework.BranchIDs(branches...)
		}
	}

	checkBranchIDs(t, testFramework, expectedBranches)
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
				// Messages attaching to Genesis can have 0,0 as a PastMarker, so do not check Markers -> Message.
				if currentMarker.SequenceID() == 0 && currentMarker.Index() == 0 {
					return
				}
				assert.True(t, messageMetadata.StructureDetails().IsPastMarker && *messageMetadata.StructureDetails().PastMarkers.Marker() == *currentMarker, "%s was mapped to wrong %s", currentMarker, messageMetadata.ID())
			}), "failed to load Message with %s", mappedMessageIDOfMarker)
		}
	}
}

func checkBranchIDs(t *testing.T, testFramework *MessageTestFramework, expectedBranchIDs map[string]branchdag.BranchIDs) {
	for messageID, messageExpectedBranchIDs := range expectedBranchIDs {
		// fmt.Println(">>", messageID)

		// messageMetadata := testFramework.MessageMetadata(messageID)
		// fmt.Println("Add:", messageMetadata.addedBranchIDs, "Sub:", messageMetadata.subtractedBranchIDs)

		retrievedBranchIDs, errRetrieve := testFramework.tangle.Booker.MessageBranchIDs(testFramework.Message(messageID).ID())
		assert.NoError(t, errRetrieve)

		// assert.True(t, messageExpectedBranchIDs.Equal(retrievedBranchIDs), "BranchID of %s should be %s but is %s", messageID, messageExpectedBranchIDs, retrievedBranchIDs)
		missing := messageExpectedBranchIDs.Clone()
		missing.DeleteAll(retrievedBranchIDs)
		assert.True(t, messageExpectedBranchIDs.Equal(retrievedBranchIDs), "BranchID of %s misses %s", messageID, missing)
	}
}

func checkMessageMetadataDiffBranchIDs(t *testing.T, testFramework *MessageTestFramework, expectedDiffBranchIDs map[string][]branchdag.BranchIDs) {
	for messageID, expectedDiffBranchID := range expectedDiffBranchIDs {
		assert.True(t, testFramework.tangle.Storage.MessageMetadata(testFramework.Message(messageID).ID()).Consume(func(messageMetadata *MessageMetadata) {
			assert.True(t, expectedDiffBranchID[0].Equal(messageMetadata.AddedBranchIDs()), "AddBranchIDs of %s should be %s but is %s in the Metadata", messageID, expectedDiffBranchID[0], messageMetadata.AddedBranchIDs())
		}))
		assert.True(t, testFramework.tangle.Storage.MessageMetadata(testFramework.Message(messageID).ID()).Consume(func(messageMetadata *MessageMetadata) {
			assert.True(t, expectedDiffBranchID[1].Equal(messageMetadata.SubtractedBranchIDs()), "SubtractedBranchIDs of %s should be %s but is %s in the Metadata", messageID, expectedDiffBranchID[1], messageMetadata.SubtractedBranchIDs())
		}))
	}
}
