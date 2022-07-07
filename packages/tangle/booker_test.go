//nolint:dupl,whitespace
package tangle

import (
	"fmt"
	"math/rand"
	"sync"
	"testing"
	"time"

	"github.com/iotaledger/hive.go/debug"
	"github.com/iotaledger/hive.go/generics/lo"
	"github.com/iotaledger/hive.go/generics/set"
	"github.com/stretchr/testify/assert"

	"github.com/iotaledger/goshimmer/packages/conflictdag"
	"github.com/iotaledger/goshimmer/packages/ledger/utxo"
	"github.com/iotaledger/goshimmer/packages/markers"
)

func TestScenario_1(t *testing.T) {
	debug.SetEnabled(true)

	tangle := NewTestTangle(WithConflictDAGOptions(conflictdag.WithMergeToMaster(false)))
	defer tangle.Shutdown()

	testFramework := NewBlockTestFramework(
		tangle,
		WithGenesisOutput("G", 3),
	)

	tangle.Setup()

	testFramework.CreateBlock("Block1", WithStrongParents("Genesis"), WithInputs("G"), WithOutput("A", 1), WithOutput("B", 1), WithOutput("C", 1))
	testFramework.CreateBlock("Block2", WithStrongParents("Genesis", "Block1"), WithInputs("B", "C"), WithOutput("E", 2))
	testFramework.CreateBlock("Block3", WithStrongParents("Block1", "Block2"), WithReattachment("Block2"))
	testFramework.CreateBlock("Block4", WithStrongParents("Genesis", "Block1"), WithInputs("A"), WithOutput("D", 1))
	testFramework.CreateBlock("Block5", WithStrongParents("Block1", "Block2"), WithInputs("A"), WithOutput("F", 1))
	testFramework.CreateBlock("Block6", WithStrongParents("Block2", "Block5"), WithInputs("E", "F"), WithOutput("H", 3))
	testFramework.CreateBlock("Block7", WithStrongParents("Block4", "Block5"), WithReattachment("Block2"))
	testFramework.CreateBlock("Block8", WithStrongParents("Block4", "Block5"), WithInputs("F", "D"), WithOutput("I", 2))
	testFramework.CreateBlock("Block9", WithStrongParents("Block4", "Block6"), WithInputs("H"), WithOutput("J", 3))

	testFramework.RegisterBranchID("Branch4", "Block4")
	testFramework.RegisterBranchID("Branch5", "Block5")

	testFramework.RegisterBranchID("Branch6", "Block6")
	testFramework.RegisterBranchID("Branch8", "Block8")

	testFramework.IssueBlocks("Block1", "Block2", "Block3", "Block4", "Block5", "Block6", "Block7", "Block8", "Block9").WaitUntilAllTasksProcessed()
	// Block8 combines conflicting branches on UTXO level
	for _, blockAlias := range []string{"Block7", "Block8", "Block9"} {
		assert.Truef(t, testFramework.BlockMetadata(blockAlias).IsSubjectivelyInvalid(), "%s not subjectively invalid", blockAlias)
	}

	for _, alias := range []string{"Block1", "Block2", "Block3", "Block4", "Block5", "Block6", "Block7", "Block8", "Block9"} {
		fmt.Println(alias, lo.PanicOnErr(tangle.Booker.BlockBranchIDs(testFramework.Block(alias).ID())))
		tangle.Storage.BlockMetadata(testFramework.Block(alias).ID()).Consume(func(blockMetadata *BlockMetadata) {
			fmt.Println(alias, "added", blockMetadata.AddedBranchIDs(), "subtracted", blockMetadata.SubtractedBranchIDs())
			fmt.Println(alias, "all", blockMetadata.StructureDetails())
			meta := testFramework.TransactionMetadata(alias)
			if meta != nil {
				fmt.Println("UTXO", meta.BranchIDs())
			}
			fmt.Println("-----------------------------------------------------")
		})
	}

	checkBranchIDs(t, testFramework, map[string]*set.AdvancedSet[utxo.TransactionID]{
		"Block1": utxo.NewTransactionIDs(),
		"Block3": utxo.NewTransactionIDs(),
		"Block2": utxo.NewTransactionIDs(),
		"Block4": testFramework.BranchIDs("Branch4"),
		"Block5": testFramework.BranchIDs("Branch5"),
		"Block6": testFramework.BranchIDs("Branch5", "Branch6"),
		"Block7": testFramework.BranchIDs("Branch4", "Branch5"),
		"Block8": testFramework.BranchIDs("Branch4", "Branch5", "Branch8"),
		"Block9": testFramework.BranchIDs("Branch4", "Branch5", "Branch6"),
	})
}

func TestScenario_2(t *testing.T) {
	tangle := NewTestTangle(WithConflictDAGOptions(conflictdag.WithMergeToMaster(false)))
	defer tangle.Shutdown()

	testFramework := NewBlockTestFramework(
		tangle,
		WithGenesisOutput("G", 3),
	)

	tangle.Setup()

	testFramework.CreateBlock("Block1", WithStrongParents("Genesis"), WithInputs("G"), WithOutput("A", 1), WithOutput("B", 1), WithOutput("C", 1))
	testFramework.CreateBlock("Block2", WithStrongParents("Genesis", "Block1"), WithInputs("B", "C"), WithOutput("E", 2))
	testFramework.CreateBlock("Block3", WithStrongParents("Block1", "Block2"), WithReattachment("Block2"))
	testFramework.CreateBlock("Block4", WithStrongParents("Genesis", "Block1"), WithInputs("A"), WithOutput("D", 1))
	testFramework.CreateBlock("Block5", WithStrongParents("Block1", "Block2"), WithInputs("A"), WithOutput("F", 1))
	testFramework.CreateBlock("Block6", WithStrongParents("Block2", "Block5"), WithInputs("E", "F"), WithOutput("L", 3))
	testFramework.CreateBlock("Block7", WithStrongParents("Block1", "Block4"), WithInputs("C"), WithOutput("H", 1))
	testFramework.CreateBlock("Block8", WithStrongParents("Block4", "Block7"), WithInputs("H", "D"), WithOutput("I", 2))
	testFramework.CreateBlock("Block9", WithStrongParents("Block4", "Block7"), WithInputs("B"), WithOutput("J", 1))

	testFramework.RegisterBranchID("green", "Block5")
	testFramework.RegisterBranchID("red", "Block4")
	testFramework.RegisterBranchID("yellow", "Block2")
	testFramework.RegisterBranchID("black", "Block7")
	testFramework.RegisterBranchID("blue", "Block9")

	testFramework.IssueBlocks("Block1").WaitUntilAllTasksProcessed()
	testFramework.IssueBlocks("Block2").WaitUntilAllTasksProcessed()
	testFramework.IssueBlocks("Block3", "Block4").WaitUntilAllTasksProcessed()
	testFramework.IssueBlocks("Block5").WaitUntilAllTasksProcessed()
	testFramework.IssueBlocks("Block6").WaitUntilAllTasksProcessed()
	testFramework.IssueBlocks("Block7").WaitUntilAllTasksProcessed()
	testFramework.IssueBlocks("Block8").WaitUntilAllTasksProcessed()
	testFramework.IssueBlocks("Block9").WaitUntilAllTasksProcessed()

	checkBranchIDs(t, testFramework, map[string]*set.AdvancedSet[utxo.TransactionID]{
		"Block1": set.NewAdvancedSet[utxo.TransactionID](),
		"Block2": testFramework.BranchIDs("yellow"),
		"Block3": testFramework.BranchIDs("yellow"),
		"Block4": testFramework.BranchIDs("red"),
		"Block5": testFramework.BranchIDs("yellow", "green"),
		"Block6": testFramework.BranchIDs("yellow", "green"),
		"Block7": testFramework.BranchIDs("red", "black"),
		"Block8": testFramework.BranchIDs("red", "black"),
		"Block9": testFramework.BranchIDs("red", "black", "blue"),
	})

	checkMarkers(t, testFramework, map[string]*markers.Markers{
		"Block1": markers.NewMarkers(markers.NewMarker(0, 1)),
		"Block2": markers.NewMarkers(markers.NewMarker(0, 2)),
		"Block3": markers.NewMarkers(markers.NewMarker(0, 3)),
		"Block4": markers.NewMarkers(markers.NewMarker(0, 1)),
		"Block5": markers.NewMarkers(markers.NewMarker(0, 2)),
		"Block6": markers.NewMarkers(markers.NewMarker(0, 2)),
		"Block7": markers.NewMarkers(markers.NewMarker(0, 1)),
		"Block8": markers.NewMarkers(markers.NewMarker(0, 1)),
		"Block9": markers.NewMarkers(markers.NewMarker(0, 1)),
	})
}

func TestScenario_3(t *testing.T) {
	tangle := NewTestTangle()
	defer tangle.Shutdown()

	testFramework := NewBlockTestFramework(
		tangle,
		WithGenesisOutput("G", 3),
	)

	tangle.Setup()

	testFramework.CreateBlock("Block1", WithStrongParents("Genesis"), WithInputs("G"), WithOutput("A", 1), WithOutput("B", 1), WithOutput("C", 1))
	testFramework.CreateBlock("Block2", WithStrongParents("Genesis", "Block1"), WithInputs("B", "C"), WithOutput("E", 2))
	testFramework.CreateBlock("Block3", WithStrongParents("Block1", "Block2"), WithReattachment("Block2"))
	testFramework.CreateBlock("Block4", WithStrongParents("Genesis", "Block1"), WithInputs("A"), WithOutput("D", 1))
	testFramework.CreateBlock("Block5", WithStrongParents("Block1"), WithWeakParents("Block2"), WithInputs("A"), WithOutput("F", 1))
	testFramework.CreateBlock("Block6", WithStrongParents("Block2", "Block5"), WithInputs("E", "F"), WithOutput("L", 3))
	testFramework.CreateBlock("Block7", WithStrongParents("Block1", "Block4"), WithInputs("C"), WithOutput("H", 1))
	testFramework.CreateBlock("Block8", WithStrongParents("Block4", "Block7"), WithInputs("H", "D"), WithOutput("I", 2))
	testFramework.CreateBlock("Block9", WithStrongParents("Block4", "Block7"), WithInputs("B"), WithOutput("J", 1))

	testFramework.IssueBlocks("Block1").WaitUntilAllTasksProcessed()
	testFramework.IssueBlocks("Block2").WaitUntilAllTasksProcessed()
	testFramework.IssueBlocks("Block3", "Block4").WaitUntilAllTasksProcessed()
	testFramework.IssueBlocks("Block5").WaitUntilAllTasksProcessed()
	testFramework.IssueBlocks("Block6").WaitUntilAllTasksProcessed()
	testFramework.IssueBlocks("Block7").WaitUntilAllTasksProcessed()
	testFramework.IssueBlocks("Block8").WaitUntilAllTasksProcessed()
	testFramework.IssueBlocks("Block9").WaitUntilAllTasksProcessed()

	testFramework.RegisterBranchID("purple", "Block2")
	testFramework.RegisterBranchID("red", "Block4")
	testFramework.RegisterBranchID("yellow", "Block5")
	testFramework.RegisterBranchID("orange", "Block7")
	testFramework.RegisterBranchID("blue", "Block9")
	testFramework.RegisterBranchID("blue", "Block9")

	checkBranchIDs(t, testFramework, map[string]*set.AdvancedSet[utxo.TransactionID]{
		"Block1": set.NewAdvancedSet[utxo.TransactionID](),
		"Block2": testFramework.BranchIDs("purple"),
		"Block3": testFramework.BranchIDs("purple"),
		"Block4": testFramework.BranchIDs("red"),
		"Block5": testFramework.BranchIDs("yellow"),
		"Block6": testFramework.BranchIDs("yellow", "purple"),
		"Block7": testFramework.BranchIDs("red", "orange"),
		"Block8": testFramework.BranchIDs("red", "orange"),
		"Block9": testFramework.BranchIDs("red", "orange", "blue"),
	})
}

func TestBookerMarkerGap(t *testing.T) {
	tangle := NewTestTangle()
	defer tangle.Shutdown()

	testFramework := NewBlockTestFramework(
		tangle,
		WithGenesisOutput("A", 500),
		WithGenesisOutput("B", 500),
		WithGenesisOutput("C", 500),
		WithGenesisOutput("L", 500),
	)

	tangle.Setup()

	// ISSUE Block1
	{
		testFramework.CreateBlock("Block1", WithStrongParents("Genesis"), WithInputs("A"), WithOutput("G", 500))
		testFramework.IssueBlocks("Block1").WaitUntilAllTasksProcessed()

		checkMarkers(t, testFramework, map[string]*markers.Markers{
			"Block1": markers.NewMarkers(markers.NewMarker(0, 1)),
		})
		checkBlockMetadataDiffBranchIDs(t, testFramework, map[string][]*set.AdvancedSet[utxo.TransactionID]{
			"Block1": {set.NewAdvancedSet[utxo.TransactionID](), set.NewAdvancedSet[utxo.TransactionID]()},
		})
		checkBranchIDs(t, testFramework, map[string]*set.AdvancedSet[utxo.TransactionID]{
			"Block1": set.NewAdvancedSet[utxo.TransactionID](),
		})
	}

	// ISSUE Block1.5
	{
		testFramework.CreateBlock("Block1.5", WithStrongParents("Block1"))
		testFramework.IssueBlocks("Block1.5").WaitUntilAllTasksProcessed()

		checkMarkers(t, testFramework, map[string]*markers.Markers{
			"Block1":   markers.NewMarkers(markers.NewMarker(0, 1)),
			"Block1.5": markers.NewMarkers(markers.NewMarker(0, 2)),
		})
		checkBlockMetadataDiffBranchIDs(t, testFramework, map[string][]*set.AdvancedSet[utxo.TransactionID]{
			"Block1":   {set.NewAdvancedSet[utxo.TransactionID](), set.NewAdvancedSet[utxo.TransactionID]()},
			"Block1.5": {set.NewAdvancedSet[utxo.TransactionID](), set.NewAdvancedSet[utxo.TransactionID]()},
		})
		checkBranchIDs(t, testFramework, map[string]*set.AdvancedSet[utxo.TransactionID]{
			"Block1":   set.NewAdvancedSet[utxo.TransactionID](),
			"Block1.5": set.NewAdvancedSet[utxo.TransactionID](),
		})
	}

	// ISSUE Block2
	{
		testFramework.CreateBlock("Block2", WithStrongParents("Block1.5"), WithInputs("B"), WithOutput("E", 500))

		testFramework.IssueBlocks("Block2").WaitUntilAllTasksProcessed()

		checkMarkers(t, testFramework, map[string]*markers.Markers{
			"Block1":   markers.NewMarkers(markers.NewMarker(0, 1)),
			"Block1.5": markers.NewMarkers(markers.NewMarker(0, 2)),
			"Block2":   markers.NewMarkers(markers.NewMarker(0, 3)),
		})
		checkBlockMetadataDiffBranchIDs(t, testFramework, map[string][]*set.AdvancedSet[utxo.TransactionID]{
			"Block1":   {set.NewAdvancedSet[utxo.TransactionID](), set.NewAdvancedSet[utxo.TransactionID]()},
			"Block1.5": {set.NewAdvancedSet[utxo.TransactionID](), set.NewAdvancedSet[utxo.TransactionID]()},
			"Block2":   {set.NewAdvancedSet[utxo.TransactionID](), set.NewAdvancedSet[utxo.TransactionID]()},
		})
		checkBranchIDs(t, testFramework, map[string]*set.AdvancedSet[utxo.TransactionID]{
			"Block1":   set.NewAdvancedSet[utxo.TransactionID](),
			"Block1.5": set.NewAdvancedSet[utxo.TransactionID](),
			"Block2":   set.NewAdvancedSet[utxo.TransactionID](),
		})
	}
	// ISSUE Block3
	{
		testFramework.CreateBlock("Block3", WithStrongParents("Genesis"), WithInputs("B"), WithOutput("F", 500))

		testFramework.RegisterBranchID("Block2", "Block2")
		testFramework.RegisterBranchID("Block3", "Block3")

		testFramework.IssueBlocks("Block3").WaitUntilAllTasksProcessed()

		checkMarkers(t, testFramework, map[string]*markers.Markers{
			"Block1":   markers.NewMarkers(markers.NewMarker(0, 1)),
			"Block1.5": markers.NewMarkers(markers.NewMarker(0, 2)),
			"Block2":   markers.NewMarkers(markers.NewMarker(0, 3)),
			"Block3":   markers.NewMarkers(markers.NewMarker(0, 0)),
		})
		checkBlockMetadataDiffBranchIDs(t, testFramework, map[string][]*set.AdvancedSet[utxo.TransactionID]{
			"Block1":   {set.NewAdvancedSet[utxo.TransactionID](), set.NewAdvancedSet[utxo.TransactionID]()},
			"Block1.5": {set.NewAdvancedSet[utxo.TransactionID](), set.NewAdvancedSet[utxo.TransactionID]()},
			"Block2":   {set.NewAdvancedSet[utxo.TransactionID](), set.NewAdvancedSet[utxo.TransactionID]()},
			"Block3":   {testFramework.BranchIDs("Block3"), set.NewAdvancedSet[utxo.TransactionID]()},
		})
		checkBranchIDs(t, testFramework, map[string]*set.AdvancedSet[utxo.TransactionID]{
			"Block1":   set.NewAdvancedSet[utxo.TransactionID](),
			"Block1.5": set.NewAdvancedSet[utxo.TransactionID](),
			"Block2":   testFramework.BranchIDs("Block2"),
			"Block3":   testFramework.BranchIDs("Block3"),
		})
	}

	// ISSUE Block4
	{
		testFramework.CreateBlock("Block4", WithStrongParents("Block3"), WithInputs("A"), WithOutput("X", 500))

		testFramework.RegisterBranchID("Block1", "Block1")
		testFramework.RegisterBranchID("Block4", "Block4")

		testFramework.IssueBlocks("Block4").WaitUntilAllTasksProcessed()

		checkMarkers(t, testFramework, map[string]*markers.Markers{
			"Block1":   markers.NewMarkers(markers.NewMarker(0, 1)),
			"Block1.5": markers.NewMarkers(markers.NewMarker(0, 2)),
			"Block2":   markers.NewMarkers(markers.NewMarker(0, 3)),
			"Block3":   markers.NewMarkers(markers.NewMarker(0, 0)),
			"Block4":   markers.NewMarkers(markers.NewMarker(0, 0)),
		})
		checkBlockMetadataDiffBranchIDs(t, testFramework, map[string][]*set.AdvancedSet[utxo.TransactionID]{
			"Block1":   {set.NewAdvancedSet[utxo.TransactionID](), set.NewAdvancedSet[utxo.TransactionID]()},
			"Block1.5": {set.NewAdvancedSet[utxo.TransactionID](), set.NewAdvancedSet[utxo.TransactionID]()},
			"Block2":   {set.NewAdvancedSet[utxo.TransactionID](), set.NewAdvancedSet[utxo.TransactionID]()},
			"Block3":   {testFramework.BranchIDs("Block3"), set.NewAdvancedSet[utxo.TransactionID]()},
			"Block4":   {testFramework.BranchIDs("Block3", "Block4"), set.NewAdvancedSet[utxo.TransactionID]()},
		})
		checkBranchIDs(t, testFramework, map[string]*set.AdvancedSet[utxo.TransactionID]{
			"Block1":   testFramework.BranchIDs("Block1"),
			"Block1.5": testFramework.BranchIDs("Block1"),
			"Block2":   testFramework.BranchIDs("Block1", "Block2"),
			"Block3":   testFramework.BranchIDs("Block3"),
			"Block4":   testFramework.BranchIDs("Block3", "Block4"),
		})
	}
}

func TestBookerMarkerGap2(t *testing.T) {
	tangle := NewTestTangle()
	defer tangle.Shutdown()

	testFramework := NewBlockTestFramework(
		tangle,
		WithGenesisOutput("Genesis1", 500),
		WithGenesisOutput("Genesis2", 500),
		WithGenesisOutput("Genesis3", 500),
	)

	tangle.Setup()

	// ISSUE Block1
	{
		testFramework.CreateBlock("Block1", WithStrongParents("Genesis"), WithInputs("Genesis1"), WithOutput("Block1", 500))
		testFramework.IssueBlocks("Block1").WaitUntilAllTasksProcessed()

		checkMarkers(t, testFramework, map[string]*markers.Markers{
			"Block1": markers.NewMarkers(markers.NewMarker(0, 1)),
		})
		checkBlockMetadataDiffBranchIDs(t, testFramework, map[string][]*set.AdvancedSet[utxo.TransactionID]{
			"Block1": {set.NewAdvancedSet[utxo.TransactionID](), set.NewAdvancedSet[utxo.TransactionID]()},
		})
		checkBranchIDs(t, testFramework, map[string]*set.AdvancedSet[utxo.TransactionID]{
			"Block1": set.NewAdvancedSet[utxo.TransactionID](),
		})
	}

	// ISSUE Block2
	{
		testFramework.CreateBlock("Block2", WithStrongParents("Genesis"), WithInputs("Genesis1"), WithOutput("Block2", 500))
		testFramework.IssueBlocks("Block2").WaitUntilAllTasksProcessed()

		testFramework.RegisterBranchID("Block1", "Block1")
		testFramework.RegisterBranchID("Block2", "Block2")

		checkMarkers(t, testFramework, map[string]*markers.Markers{
			"Block1": markers.NewMarkers(markers.NewMarker(0, 1)),
			"Block2": markers.NewMarkers(markers.NewMarker(0, 0)),
		})
		checkBlockMetadataDiffBranchIDs(t, testFramework, map[string][]*set.AdvancedSet[utxo.TransactionID]{
			"Block1": {set.NewAdvancedSet[utxo.TransactionID](), set.NewAdvancedSet[utxo.TransactionID]()},
			"Block2": {testFramework.BranchIDs("Block2"), set.NewAdvancedSet[utxo.TransactionID]()},
		})
		checkBranchIDs(t, testFramework, map[string]*set.AdvancedSet[utxo.TransactionID]{
			"Block1": testFramework.BranchIDs("Block1"),
			"Block2": testFramework.BranchIDs("Block2"),
		})
	}

	// ISSUE Block3
	{
		testFramework.CreateBlock("Block3", WithStrongParents("Genesis"), WithInputs("Genesis2"), WithOutput("Block3", 500))
		testFramework.IssueBlocks("Block3").WaitUntilAllTasksProcessed()

		checkMarkers(t, testFramework, map[string]*markers.Markers{
			"Block1": markers.NewMarkers(markers.NewMarker(0, 1)),
			"Block2": markers.NewMarkers(markers.NewMarker(0, 0)),
			"Block3": markers.NewMarkers(markers.NewMarker(0, 0)),
		})
		checkBlockMetadataDiffBranchIDs(t, testFramework, map[string][]*set.AdvancedSet[utxo.TransactionID]{
			"Block1": {set.NewAdvancedSet[utxo.TransactionID](), set.NewAdvancedSet[utxo.TransactionID]()},
			"Block2": {testFramework.BranchIDs("Block2"), set.NewAdvancedSet[utxo.TransactionID]()},
			"Block3": {set.NewAdvancedSet[utxo.TransactionID](), set.NewAdvancedSet[utxo.TransactionID]()},
		})
		checkBranchIDs(t, testFramework, map[string]*set.AdvancedSet[utxo.TransactionID]{
			"Block1": testFramework.BranchIDs("Block1"),
			"Block2": testFramework.BranchIDs("Block2"),
			"Block3": set.NewAdvancedSet[utxo.TransactionID](),
		})
	}

	// ISSUE Block4
	{
		testFramework.CreateBlock("Block4", WithStrongParents("Genesis"), WithInputs("Genesis2"), WithOutput("Block4", 500))
		testFramework.IssueBlocks("Block4").WaitUntilAllTasksProcessed()

		testFramework.RegisterBranchID("Block3", "Block3")
		testFramework.RegisterBranchID("Block4", "Block4")

		checkMarkers(t, testFramework, map[string]*markers.Markers{
			"Block1": markers.NewMarkers(markers.NewMarker(0, 1)),
			"Block2": markers.NewMarkers(markers.NewMarker(0, 0)),
			"Block3": markers.NewMarkers(markers.NewMarker(0, 0)),
			"Block4": markers.NewMarkers(markers.NewMarker(0, 0)),
		})
		checkBlockMetadataDiffBranchIDs(t, testFramework, map[string][]*set.AdvancedSet[utxo.TransactionID]{
			"Block1": {set.NewAdvancedSet[utxo.TransactionID](), set.NewAdvancedSet[utxo.TransactionID]()},
			"Block2": {testFramework.BranchIDs("Block2"), set.NewAdvancedSet[utxo.TransactionID]()},
			"Block3": {testFramework.BranchIDs("Block3"), set.NewAdvancedSet[utxo.TransactionID]()},
			"Block4": {testFramework.BranchIDs("Block4"), set.NewAdvancedSet[utxo.TransactionID]()},
		})
		checkBranchIDs(t, testFramework, map[string]*set.AdvancedSet[utxo.TransactionID]{
			"Block1": testFramework.BranchIDs("Block1"),
			"Block2": testFramework.BranchIDs("Block2"),
			"Block3": testFramework.BranchIDs("Block3"),
			"Block4": testFramework.BranchIDs("Block4"),
		})
	}

	// ISSUE Block5
	{
		testFramework.CreateBlock("Block5", WithStrongParents("Block1"), WithInputs("Genesis3"), WithOutput("Block5", 500))
		testFramework.IssueBlocks("Block5").WaitUntilAllTasksProcessed()

		checkMarkers(t, testFramework, map[string]*markers.Markers{
			"Block1": markers.NewMarkers(markers.NewMarker(0, 1)),
			"Block2": markers.NewMarkers(markers.NewMarker(0, 0)),
			"Block3": markers.NewMarkers(markers.NewMarker(0, 0)),
			"Block4": markers.NewMarkers(markers.NewMarker(0, 0)),
			"Block5": markers.NewMarkers(markers.NewMarker(0, 2)),
		})
		checkBlockMetadataDiffBranchIDs(t, testFramework, map[string][]*set.AdvancedSet[utxo.TransactionID]{
			"Block1": {set.NewAdvancedSet[utxo.TransactionID](), set.NewAdvancedSet[utxo.TransactionID]()},
			"Block2": {testFramework.BranchIDs("Block2"), set.NewAdvancedSet[utxo.TransactionID]()},
			"Block3": {testFramework.BranchIDs("Block3"), set.NewAdvancedSet[utxo.TransactionID]()},
			"Block4": {testFramework.BranchIDs("Block4"), set.NewAdvancedSet[utxo.TransactionID]()},
			"Block5": {set.NewAdvancedSet[utxo.TransactionID](), set.NewAdvancedSet[utxo.TransactionID]()},
		})
		checkBranchIDs(t, testFramework, map[string]*set.AdvancedSet[utxo.TransactionID]{
			"Block1": testFramework.BranchIDs("Block1"),
			"Block2": testFramework.BranchIDs("Block2"),
			"Block3": testFramework.BranchIDs("Block3"),
			"Block4": testFramework.BranchIDs("Block4"),
			"Block5": testFramework.BranchIDs("Block1"),
		})
	}

	// ISSUE Block6
	{
		testFramework.CreateBlock("Block6", WithStrongParents("Block1", "Block3"))
		testFramework.IssueBlocks("Block6").WaitUntilAllTasksProcessed()

		checkMarkers(t, testFramework, map[string]*markers.Markers{
			"Block1": markers.NewMarkers(markers.NewMarker(0, 1)),
			"Block2": markers.NewMarkers(markers.NewMarker(0, 0)),
			"Block3": markers.NewMarkers(markers.NewMarker(0, 0)),
			"Block4": markers.NewMarkers(markers.NewMarker(0, 0)),
			"Block5": markers.NewMarkers(markers.NewMarker(0, 2)),
			"Block6": markers.NewMarkers(markers.NewMarker(0, 1)),
		})
		checkBlockMetadataDiffBranchIDs(t, testFramework, map[string][]*set.AdvancedSet[utxo.TransactionID]{
			"Block1": {set.NewAdvancedSet[utxo.TransactionID](), set.NewAdvancedSet[utxo.TransactionID]()},
			"Block2": {testFramework.BranchIDs("Block2"), set.NewAdvancedSet[utxo.TransactionID]()},
			"Block3": {testFramework.BranchIDs("Block3"), set.NewAdvancedSet[utxo.TransactionID]()},
			"Block4": {testFramework.BranchIDs("Block4"), set.NewAdvancedSet[utxo.TransactionID]()},
			"Block5": {set.NewAdvancedSet[utxo.TransactionID](), set.NewAdvancedSet[utxo.TransactionID]()},
			"Block6": {testFramework.BranchIDs("Block3"), set.NewAdvancedSet[utxo.TransactionID]()},
		})
		checkBranchIDs(t, testFramework, map[string]*set.AdvancedSet[utxo.TransactionID]{
			"Block1": testFramework.BranchIDs("Block1"),
			"Block2": testFramework.BranchIDs("Block2"),
			"Block3": testFramework.BranchIDs("Block3"),
			"Block4": testFramework.BranchIDs("Block4"),
			"Block5": testFramework.BranchIDs("Block1"),
			"Block6": testFramework.BranchIDs("Block1", "Block3"),
		})
	}

	// ISSUE Block7
	{
		testFramework.CreateBlock("Block7", WithStrongParents("Block3", "Block5"))
		testFramework.IssueBlocks("Block7").WaitUntilAllTasksProcessed()

		checkMarkers(t, testFramework, map[string]*markers.Markers{
			"Block1": markers.NewMarkers(markers.NewMarker(0, 1)),
			"Block2": markers.NewMarkers(markers.NewMarker(0, 0)),
			"Block3": markers.NewMarkers(markers.NewMarker(0, 0)),
			"Block4": markers.NewMarkers(markers.NewMarker(0, 0)),
			"Block5": markers.NewMarkers(markers.NewMarker(0, 2)),
			"Block6": markers.NewMarkers(markers.NewMarker(0, 1)),
			"Block7": markers.NewMarkers(markers.NewMarker(0, 3)),
		})
		checkBlockMetadataDiffBranchIDs(t, testFramework, map[string][]*set.AdvancedSet[utxo.TransactionID]{
			"Block1": {set.NewAdvancedSet[utxo.TransactionID](), set.NewAdvancedSet[utxo.TransactionID]()},
			"Block2": {testFramework.BranchIDs("Block2"), set.NewAdvancedSet[utxo.TransactionID]()},
			"Block3": {testFramework.BranchIDs("Block3"), set.NewAdvancedSet[utxo.TransactionID]()},
			"Block4": {testFramework.BranchIDs("Block4"), set.NewAdvancedSet[utxo.TransactionID]()},
			"Block5": {set.NewAdvancedSet[utxo.TransactionID](), set.NewAdvancedSet[utxo.TransactionID]()},
			"Block6": {testFramework.BranchIDs("Block3"), set.NewAdvancedSet[utxo.TransactionID]()},
			"Block7": {set.NewAdvancedSet[utxo.TransactionID](), set.NewAdvancedSet[utxo.TransactionID]()},
		})
		checkBranchIDs(t, testFramework, map[string]*set.AdvancedSet[utxo.TransactionID]{
			"Block1": testFramework.BranchIDs("Block1"),
			"Block2": testFramework.BranchIDs("Block2"),
			"Block3": testFramework.BranchIDs("Block3"),
			"Block4": testFramework.BranchIDs("Block4"),
			"Block5": testFramework.BranchIDs("Block1"),
			"Block6": testFramework.BranchIDs("Block1", "Block3"),
			"Block7": testFramework.BranchIDs("Block1", "Block3"),
		})
	}

	// ISSUE Block8
	{
		testFramework.CreateBlock("Block8", WithStrongParents("Genesis"), WithInputs("Genesis3"), WithOutput("Block8", 500))

		testFramework.RegisterBranchID("Block5", "Block5")
		testFramework.RegisterBranchID("Block8", "Block8")

		testFramework.IssueBlocks("Block8").WaitUntilAllTasksProcessed()

		checkMarkers(t, testFramework, map[string]*markers.Markers{
			"Block1": markers.NewMarkers(markers.NewMarker(0, 1)),
			"Block2": markers.NewMarkers(markers.NewMarker(0, 0)),
			"Block3": markers.NewMarkers(markers.NewMarker(0, 0)),
			"Block4": markers.NewMarkers(markers.NewMarker(0, 0)),
			"Block5": markers.NewMarkers(markers.NewMarker(0, 2)),
			"Block6": markers.NewMarkers(markers.NewMarker(0, 1)),
			"Block7": markers.NewMarkers(markers.NewMarker(0, 3)),
			"Block8": markers.NewMarkers(markers.NewMarker(0, 0)),
		})
		checkBlockMetadataDiffBranchIDs(t, testFramework, map[string][]*set.AdvancedSet[utxo.TransactionID]{
			"Block1": {set.NewAdvancedSet[utxo.TransactionID](), set.NewAdvancedSet[utxo.TransactionID]()},
			"Block2": {testFramework.BranchIDs("Block2"), set.NewAdvancedSet[utxo.TransactionID]()},
			"Block3": {testFramework.BranchIDs("Block3"), set.NewAdvancedSet[utxo.TransactionID]()},
			"Block4": {testFramework.BranchIDs("Block4"), set.NewAdvancedSet[utxo.TransactionID]()},
			"Block5": {set.NewAdvancedSet[utxo.TransactionID](), set.NewAdvancedSet[utxo.TransactionID]()},
			"Block6": {testFramework.BranchIDs("Block3"), set.NewAdvancedSet[utxo.TransactionID]()},
			"Block7": {set.NewAdvancedSet[utxo.TransactionID](), set.NewAdvancedSet[utxo.TransactionID]()},
			"Block8": {testFramework.BranchIDs("Block8"), set.NewAdvancedSet[utxo.TransactionID]()},
		})
		checkBranchIDs(t, testFramework, map[string]*set.AdvancedSet[utxo.TransactionID]{
			"Block1": testFramework.BranchIDs("Block1"),
			"Block2": testFramework.BranchIDs("Block2"),
			"Block3": testFramework.BranchIDs("Block3"),
			"Block4": testFramework.BranchIDs("Block4"),
			"Block5": testFramework.BranchIDs("Block1", "Block5"),
			"Block6": testFramework.BranchIDs("Block1", "Block3"),
			"Block7": testFramework.BranchIDs("Block1", "Block3", "Block5"),
			"Block8": testFramework.BranchIDs("Block8"),
		})
	}
}

func TestBookerIndividuallyMappedBlocksSameSequence(t *testing.T) {
	tangle := NewTestTangle()
	defer tangle.Shutdown()

	testFramework := NewBlockTestFramework(
		tangle,
		WithGenesisOutput("A", 500),
		WithGenesisOutput("B", 500),
	)

	tangle.Setup()

	// ISSUE A1
	{
		testFramework.CreateBlock("A1", WithStrongParents("Genesis"), WithInputs("A"), WithOutput("A1", 500))
		testFramework.IssueBlocks("A1").WaitUntilAllTasksProcessed()

		checkMarkers(t, testFramework, map[string]*markers.Markers{
			"A1": markers.NewMarkers(markers.NewMarker(0, 1)),
		})
		checkBlockMetadataDiffBranchIDs(t, testFramework, map[string][]*set.AdvancedSet[utxo.TransactionID]{
			"A1": {set.NewAdvancedSet[utxo.TransactionID](), set.NewAdvancedSet[utxo.TransactionID]()},
		})
		checkBranchIDs(t, testFramework, map[string]*set.AdvancedSet[utxo.TransactionID]{
			"A1": set.NewAdvancedSet[utxo.TransactionID](),
		})
	}

	// ISSUE A2
	{
		testFramework.CreateBlock("A2", WithStrongParents("A1"))
		testFramework.IssueBlocks("A2").WaitUntilAllTasksProcessed()

		checkMarkers(t, testFramework, map[string]*markers.Markers{
			"A1": markers.NewMarkers(markers.NewMarker(0, 1)),
			"A2": markers.NewMarkers(markers.NewMarker(0, 2)),
		})
		checkBlockMetadataDiffBranchIDs(t, testFramework, map[string][]*set.AdvancedSet[utxo.TransactionID]{
			"A1": {set.NewAdvancedSet[utxo.TransactionID](), set.NewAdvancedSet[utxo.TransactionID]()},
			"A2": {set.NewAdvancedSet[utxo.TransactionID](), set.NewAdvancedSet[utxo.TransactionID]()},
		})
		checkBranchIDs(t, testFramework, map[string]*set.AdvancedSet[utxo.TransactionID]{
			"A1": set.NewAdvancedSet[utxo.TransactionID](),
			"A2": set.NewAdvancedSet[utxo.TransactionID](),
		})
	}

	// ISSUE A3
	{
		testFramework.PreventNewMarkers(true).CreateBlock("A3", WithStrongParents("A2"), WithInputs("B"), WithOutput("B1", 500))
		testFramework.IssueBlocks("A3").WaitUntilAllTasksProcessed().PreventNewMarkers(false)

		checkMarkers(t, testFramework, map[string]*markers.Markers{
			"A1": markers.NewMarkers(markers.NewMarker(0, 1)),
			"A2": markers.NewMarkers(markers.NewMarker(0, 2)),
			"A3": markers.NewMarkers(markers.NewMarker(0, 2)),
		})
		checkBlockMetadataDiffBranchIDs(t, testFramework, map[string][]*set.AdvancedSet[utxo.TransactionID]{
			"A1": {set.NewAdvancedSet[utxo.TransactionID](), set.NewAdvancedSet[utxo.TransactionID]()},
			"A2": {set.NewAdvancedSet[utxo.TransactionID](), set.NewAdvancedSet[utxo.TransactionID]()},
			"A3": {set.NewAdvancedSet[utxo.TransactionID](), set.NewAdvancedSet[utxo.TransactionID]()},
		})
		checkBranchIDs(t, testFramework, map[string]*set.AdvancedSet[utxo.TransactionID]{
			"A1": set.NewAdvancedSet[utxo.TransactionID](),
			"A2": set.NewAdvancedSet[utxo.TransactionID](),
			"A3": set.NewAdvancedSet[utxo.TransactionID](),
		})
	}

	// ISSUE A4
	{
		testFramework.CreateBlock("A4", WithStrongParents("A3"))
		testFramework.IssueBlocks("A4").WaitUntilAllTasksProcessed()

		checkMarkers(t, testFramework, map[string]*markers.Markers{
			"A1": markers.NewMarkers(markers.NewMarker(0, 1)),
			"A2": markers.NewMarkers(markers.NewMarker(0, 2)),
			"A3": markers.NewMarkers(markers.NewMarker(0, 2)),
			"A4": markers.NewMarkers(markers.NewMarker(0, 3)),
		})
		checkBlockMetadataDiffBranchIDs(t, testFramework, map[string][]*set.AdvancedSet[utxo.TransactionID]{
			"A1": {set.NewAdvancedSet[utxo.TransactionID](), set.NewAdvancedSet[utxo.TransactionID]()},
			"A2": {set.NewAdvancedSet[utxo.TransactionID](), set.NewAdvancedSet[utxo.TransactionID]()},
			"A3": {set.NewAdvancedSet[utxo.TransactionID](), set.NewAdvancedSet[utxo.TransactionID]()},
			"A4": {set.NewAdvancedSet[utxo.TransactionID](), set.NewAdvancedSet[utxo.TransactionID]()},
		})
		checkBranchIDs(t, testFramework, map[string]*set.AdvancedSet[utxo.TransactionID]{
			"A1": set.NewAdvancedSet[utxo.TransactionID](),
			"A2": set.NewAdvancedSet[utxo.TransactionID](),
			"A3": set.NewAdvancedSet[utxo.TransactionID](),
			"A4": set.NewAdvancedSet[utxo.TransactionID](),
		})
	}

	// ISSUE A3*
	{
		testFramework.CreateBlock("A3*", WithStrongParents("Genesis"), WithInputs("B"), WithOutput("A3*", 500))

		testFramework.RegisterBranchID("A3", "A3")
		testFramework.RegisterBranchID("A3*", "A3*")

		testFramework.IssueBlocks("A3*").WaitUntilAllTasksProcessed()

		checkMarkers(t, testFramework, map[string]*markers.Markers{
			"A1":  markers.NewMarkers(markers.NewMarker(0, 1)),
			"A2":  markers.NewMarkers(markers.NewMarker(0, 2)),
			"A3":  markers.NewMarkers(markers.NewMarker(0, 2)),
			"A4":  markers.NewMarkers(markers.NewMarker(0, 3)),
			"A3*": markers.NewMarkers(markers.NewMarker(0, 0)),
		})
		checkBlockMetadataDiffBranchIDs(t, testFramework, map[string][]*set.AdvancedSet[utxo.TransactionID]{
			"A1":  {set.NewAdvancedSet[utxo.TransactionID](), set.NewAdvancedSet[utxo.TransactionID]()},
			"A2":  {set.NewAdvancedSet[utxo.TransactionID](), set.NewAdvancedSet[utxo.TransactionID]()},
			"A3":  {testFramework.BranchIDs("A3"), set.NewAdvancedSet[utxo.TransactionID]()},
			"A4":  {set.NewAdvancedSet[utxo.TransactionID](), set.NewAdvancedSet[utxo.TransactionID]()},
			"A3*": {testFramework.BranchIDs("A3*"), set.NewAdvancedSet[utxo.TransactionID]()},
		})
		checkBranchIDs(t, testFramework, map[string]*set.AdvancedSet[utxo.TransactionID]{
			"A1":  set.NewAdvancedSet[utxo.TransactionID](),
			"A2":  set.NewAdvancedSet[utxo.TransactionID](),
			"A3":  testFramework.BranchIDs("A3"),
			"A4":  testFramework.BranchIDs("A3"),
			"A3*": testFramework.BranchIDs("A3*"),
		})
	}

	// ISSUE A1*
	{
		testFramework.CreateBlock("A1*", WithStrongParents("Genesis"), WithInputs("A"), WithOutput("A1*", 500))

		testFramework.RegisterBranchID("A1", "A1")
		testFramework.RegisterBranchID("A1*", "A1*")

		testFramework.IssueBlocks("A1*").WaitUntilAllTasksProcessed()

		checkMarkers(t, testFramework, map[string]*markers.Markers{
			"A1":  markers.NewMarkers(markers.NewMarker(0, 1)),
			"A2":  markers.NewMarkers(markers.NewMarker(0, 2)),
			"A3":  markers.NewMarkers(markers.NewMarker(0, 2)),
			"A4":  markers.NewMarkers(markers.NewMarker(0, 3)),
			"A3*": markers.NewMarkers(markers.NewMarker(0, 0)),
			"A1*": markers.NewMarkers(markers.NewMarker(0, 0)),
		})
		checkBlockMetadataDiffBranchIDs(t, testFramework, map[string][]*set.AdvancedSet[utxo.TransactionID]{
			"A1":  {set.NewAdvancedSet[utxo.TransactionID](), set.NewAdvancedSet[utxo.TransactionID]()},
			"A2":  {set.NewAdvancedSet[utxo.TransactionID](), set.NewAdvancedSet[utxo.TransactionID]()},
			"A3":  {testFramework.BranchIDs("A1", "A3"), set.NewAdvancedSet[utxo.TransactionID]()},
			"A4":  {set.NewAdvancedSet[utxo.TransactionID](), set.NewAdvancedSet[utxo.TransactionID]()},
			"A3*": {testFramework.BranchIDs("A3*"), set.NewAdvancedSet[utxo.TransactionID]()},
			"A1*": {testFramework.BranchIDs("A1*"), set.NewAdvancedSet[utxo.TransactionID]()},
		})
		checkBranchIDs(t, testFramework, map[string]*set.AdvancedSet[utxo.TransactionID]{
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

	testFramework := NewBlockTestFramework(
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
		testFramework.CreateBlock("A1", WithStrongParents("Genesis"), WithInputs("A"), WithOutput("A1", 500))
		testFramework.IssueBlocks("A1").WaitUntilAllTasksProcessed()

		checkMarkers(t, testFramework, map[string]*markers.Markers{
			"A1": markers.NewMarkers(markers.NewMarker(0, 1)),
		})
		checkBlockMetadataDiffBranchIDs(t, testFramework, map[string][]*set.AdvancedSet[utxo.TransactionID]{
			"A1": {set.NewAdvancedSet[utxo.TransactionID](), set.NewAdvancedSet[utxo.TransactionID]()},
		})
		checkBranchIDs(t, testFramework, map[string]*set.AdvancedSet[utxo.TransactionID]{
			"A1": set.NewAdvancedSet[utxo.TransactionID](),
		})
	}

	// ISSUE B1
	{
		testFramework.CreateBlock("B1", WithStrongParents("Genesis"), WithInputs("A"), WithOutput("B1", 500))

		testFramework.RegisterBranchID("A", "A1")
		testFramework.RegisterBranchID("B", "B1")

		testFramework.IssueBlocks("B1").WaitUntilAllTasksProcessed()

		checkMarkers(t, testFramework, map[string]*markers.Markers{
			"A1": markers.NewMarkers(markers.NewMarker(0, 1)),
			"B1": markers.NewMarkers(markers.NewMarker(0, 0)),
		})
		checkBlockMetadataDiffBranchIDs(t, testFramework, map[string][]*set.AdvancedSet[utxo.TransactionID]{
			"A1": {set.NewAdvancedSet[utxo.TransactionID](), set.NewAdvancedSet[utxo.TransactionID]()},
			"B1": {testFramework.BranchIDs("B"), set.NewAdvancedSet[utxo.TransactionID]()},
		})
		checkBranchIDs(t, testFramework, map[string]*set.AdvancedSet[utxo.TransactionID]{
			"A1": testFramework.BranchIDs("A"),
			"B1": testFramework.BranchIDs("B"),
		})
	}

	// ISSUE C1
	{
		testFramework.CreateBlock("C1", WithStrongParents("Genesis"), WithInputs("C"), WithOutput("C1", 500))

		testFramework.RegisterBranchID("A", "A1")
		testFramework.RegisterBranchID("B", "B1")

		testFramework.IssueBlocks("C1").WaitUntilAllTasksProcessed()

		checkMarkers(t, testFramework, map[string]*markers.Markers{
			"A1": markers.NewMarkers(markers.NewMarker(0, 1)),
			"B1": markers.NewMarkers(markers.NewMarker(0, 0)),
			"C1": markers.NewMarkers(markers.NewMarker(0, 0)),
		})
		checkBlockMetadataDiffBranchIDs(t, testFramework, map[string][]*set.AdvancedSet[utxo.TransactionID]{
			"A1": {set.NewAdvancedSet[utxo.TransactionID](), set.NewAdvancedSet[utxo.TransactionID]()},
			"B1": {testFramework.BranchIDs("B"), set.NewAdvancedSet[utxo.TransactionID]()},
			"C1": {set.NewAdvancedSet[utxo.TransactionID](), set.NewAdvancedSet[utxo.TransactionID]()},
		})
		checkBranchIDs(t, testFramework, map[string]*set.AdvancedSet[utxo.TransactionID]{
			"A1": testFramework.BranchIDs("A"),
			"B1": testFramework.BranchIDs("B"),
			"C1": set.NewAdvancedSet[utxo.TransactionID](),
		})
	}

	// ISSUE D1
	{
		testFramework.CreateBlock("D1", WithStrongParents("Genesis"), WithInputs("C"), WithOutput("D1", 500))

		testFramework.RegisterBranchID("C", "C1")
		testFramework.RegisterBranchID("D", "D1")

		testFramework.IssueBlocks("D1").WaitUntilAllTasksProcessed()

		checkMarkers(t, testFramework, map[string]*markers.Markers{
			"A1": markers.NewMarkers(markers.NewMarker(0, 1)),
			"B1": markers.NewMarkers(markers.NewMarker(0, 0)),
			"C1": markers.NewMarkers(markers.NewMarker(0, 0)),
			"D1": markers.NewMarkers(markers.NewMarker(0, 0)),
		})
		checkBlockMetadataDiffBranchIDs(t, testFramework, map[string][]*set.AdvancedSet[utxo.TransactionID]{
			"A1": {set.NewAdvancedSet[utxo.TransactionID](), set.NewAdvancedSet[utxo.TransactionID]()},
			"B1": {testFramework.BranchIDs("B"), set.NewAdvancedSet[utxo.TransactionID]()},
			"C1": {testFramework.BranchIDs("C"), set.NewAdvancedSet[utxo.TransactionID]()},
			"D1": {testFramework.BranchIDs("D"), set.NewAdvancedSet[utxo.TransactionID]()},
		})
		checkBranchIDs(t, testFramework, map[string]*set.AdvancedSet[utxo.TransactionID]{
			"A1": testFramework.BranchIDs("A"),
			"B1": testFramework.BranchIDs("B"),
			"C1": testFramework.BranchIDs("C"),
			"D1": testFramework.BranchIDs("D"),
		})
	}

	// ISSUE A2
	{
		testFramework.CreateBlock("A2", WithStrongParents("A1"), WithInputs("A1"), WithOutput("A2", 500))

		testFramework.IssueBlocks("A2").WaitUntilAllTasksProcessed()

		checkMarkers(t, testFramework, map[string]*markers.Markers{
			"A1": markers.NewMarkers(markers.NewMarker(0, 1)),
			"B1": markers.NewMarkers(markers.NewMarker(0, 0)),
			"C1": markers.NewMarkers(markers.NewMarker(0, 0)),
			"D1": markers.NewMarkers(markers.NewMarker(0, 0)),
			"A2": markers.NewMarkers(markers.NewMarker(0, 2)),
		})
		checkBlockMetadataDiffBranchIDs(t, testFramework, map[string][]*set.AdvancedSet[utxo.TransactionID]{
			"A1": {set.NewAdvancedSet[utxo.TransactionID](), set.NewAdvancedSet[utxo.TransactionID]()},
			"B1": {testFramework.BranchIDs("B"), set.NewAdvancedSet[utxo.TransactionID]()},
			"C1": {testFramework.BranchIDs("C"), set.NewAdvancedSet[utxo.TransactionID]()},
			"D1": {testFramework.BranchIDs("D"), set.NewAdvancedSet[utxo.TransactionID]()},
			"A2": {set.NewAdvancedSet[utxo.TransactionID](), set.NewAdvancedSet[utxo.TransactionID]()},
		})
		checkBranchIDs(t, testFramework, map[string]*set.AdvancedSet[utxo.TransactionID]{
			"A1": testFramework.BranchIDs("A"),
			"B1": testFramework.BranchIDs("B"),
			"C1": testFramework.BranchIDs("C"),
			"D1": testFramework.BranchIDs("D"),
			"A2": testFramework.BranchIDs("A"),
		})
	}

	// ISSUE A3
	{
		testFramework.CreateBlock("A3", WithStrongParents("A2"))

		testFramework.IssueBlocks("A3").WaitUntilAllTasksProcessed()

		checkMarkers(t, testFramework, map[string]*markers.Markers{
			"A1": markers.NewMarkers(markers.NewMarker(0, 1)),
			"B1": markers.NewMarkers(markers.NewMarker(0, 0)),
			"C1": markers.NewMarkers(markers.NewMarker(0, 0)),
			"D1": markers.NewMarkers(markers.NewMarker(0, 0)),
			"A2": markers.NewMarkers(markers.NewMarker(0, 2)),
			"A3": markers.NewMarkers(markers.NewMarker(0, 3)),
		})
		checkBlockMetadataDiffBranchIDs(t, testFramework, map[string][]*set.AdvancedSet[utxo.TransactionID]{
			"A1": {set.NewAdvancedSet[utxo.TransactionID](), set.NewAdvancedSet[utxo.TransactionID]()},
			"B1": {testFramework.BranchIDs("B"), set.NewAdvancedSet[utxo.TransactionID]()},
			"C1": {testFramework.BranchIDs("C"), set.NewAdvancedSet[utxo.TransactionID]()},
			"D1": {testFramework.BranchIDs("D"), set.NewAdvancedSet[utxo.TransactionID]()},
			"A2": {set.NewAdvancedSet[utxo.TransactionID](), set.NewAdvancedSet[utxo.TransactionID]()},
			"A3": {set.NewAdvancedSet[utxo.TransactionID](), set.NewAdvancedSet[utxo.TransactionID]()},
		})
		checkBranchIDs(t, testFramework, map[string]*set.AdvancedSet[utxo.TransactionID]{
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
		testFramework.CreateBlock("A+C1", WithStrongParents("A3", "C1"))

		testFramework.IssueBlocks("A+C1").WaitUntilAllTasksProcessed()

		checkMarkers(t, testFramework, map[string]*markers.Markers{
			"A1":   markers.NewMarkers(markers.NewMarker(0, 1)),
			"B1":   markers.NewMarkers(markers.NewMarker(0, 0)),
			"C1":   markers.NewMarkers(markers.NewMarker(0, 0)),
			"D1":   markers.NewMarkers(markers.NewMarker(0, 0)),
			"A2":   markers.NewMarkers(markers.NewMarker(0, 2)),
			"A3":   markers.NewMarkers(markers.NewMarker(0, 3)),
			"A+C1": markers.NewMarkers(markers.NewMarker(0, 4)),
		})
		checkBlockMetadataDiffBranchIDs(t, testFramework, map[string][]*set.AdvancedSet[utxo.TransactionID]{
			"A1":   {set.NewAdvancedSet[utxo.TransactionID](), set.NewAdvancedSet[utxo.TransactionID]()},
			"B1":   {testFramework.BranchIDs("B"), set.NewAdvancedSet[utxo.TransactionID]()},
			"C1":   {testFramework.BranchIDs("C"), set.NewAdvancedSet[utxo.TransactionID]()},
			"D1":   {testFramework.BranchIDs("D"), set.NewAdvancedSet[utxo.TransactionID]()},
			"A2":   {set.NewAdvancedSet[utxo.TransactionID](), set.NewAdvancedSet[utxo.TransactionID]()},
			"A3":   {set.NewAdvancedSet[utxo.TransactionID](), set.NewAdvancedSet[utxo.TransactionID]()},
			"A+C1": {set.NewAdvancedSet[utxo.TransactionID](), set.NewAdvancedSet[utxo.TransactionID]()},
		})
		checkBranchIDs(t, testFramework, map[string]*set.AdvancedSet[utxo.TransactionID]{
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
		testFramework.CreateBlock("A+C2", WithStrongParents("A3", "C1"))

		testFramework.IssueBlocks("A+C2").WaitUntilAllTasksProcessed()

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
		checkBlockMetadataDiffBranchIDs(t, testFramework, map[string][]*set.AdvancedSet[utxo.TransactionID]{
			"A1":   {set.NewAdvancedSet[utxo.TransactionID](), set.NewAdvancedSet[utxo.TransactionID]()},
			"B1":   {testFramework.BranchIDs("B"), set.NewAdvancedSet[utxo.TransactionID]()},
			"C1":   {testFramework.BranchIDs("C"), set.NewAdvancedSet[utxo.TransactionID]()},
			"D1":   {testFramework.BranchIDs("D"), set.NewAdvancedSet[utxo.TransactionID]()},
			"A2":   {set.NewAdvancedSet[utxo.TransactionID](), set.NewAdvancedSet[utxo.TransactionID]()},
			"A3":   {set.NewAdvancedSet[utxo.TransactionID](), set.NewAdvancedSet[utxo.TransactionID]()},
			"A+C1": {set.NewAdvancedSet[utxo.TransactionID](), set.NewAdvancedSet[utxo.TransactionID]()},
			"A+C2": {testFramework.BranchIDs("C"), set.NewAdvancedSet[utxo.TransactionID]()},
		})
		checkBranchIDs(t, testFramework, map[string]*set.AdvancedSet[utxo.TransactionID]{
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
		testFramework.CreateBlock("A2*", WithStrongParents("A1"), WithInputs("A1"), WithOutput("A2*", 500))

		testFramework.RegisterBranchID("A2", "A2")
		testFramework.RegisterBranchID("A2*", "A2*")

		testFramework.IssueBlocks("A2*").WaitUntilAllTasksProcessed()

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
		checkBlockMetadataDiffBranchIDs(t, testFramework, map[string][]*set.AdvancedSet[utxo.TransactionID]{
			"A1":   {set.NewAdvancedSet[utxo.TransactionID](), set.NewAdvancedSet[utxo.TransactionID]()},
			"B1":   {testFramework.BranchIDs("B"), set.NewAdvancedSet[utxo.TransactionID]()},
			"C1":   {testFramework.BranchIDs("C"), set.NewAdvancedSet[utxo.TransactionID]()},
			"D1":   {testFramework.BranchIDs("D"), set.NewAdvancedSet[utxo.TransactionID]()},
			"A2":   {set.NewAdvancedSet[utxo.TransactionID](), set.NewAdvancedSet[utxo.TransactionID]()},
			"A3":   {set.NewAdvancedSet[utxo.TransactionID](), set.NewAdvancedSet[utxo.TransactionID]()},
			"A+C1": {set.NewAdvancedSet[utxo.TransactionID](), set.NewAdvancedSet[utxo.TransactionID]()},
			"A+C2": {testFramework.BranchIDs("A2", "C"), set.NewAdvancedSet[utxo.TransactionID]()},
			"A2*":  {testFramework.BranchIDs("A2*"), set.NewAdvancedSet[utxo.TransactionID]()},
		})
		checkBranchIDs(t, testFramework, map[string]*set.AdvancedSet[utxo.TransactionID]{
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

func TestBookerMarkerMappingContinue(t *testing.T) {
	tg := NewTestTangle()
	defer tg.Shutdown()

	testFramework := NewBlockTestFramework(
		tg,
		WithGenesisOutput("A", 500),
		WithGenesisOutput("B", 500),
	)

	tg.Setup()

	// ISSUE Block1
	{
		testFramework.CreateBlock("Block1", WithStrongParents("Genesis"))
		testFramework.IssueBlocks("Block1").WaitUntilAllTasksProcessed()

		checkMarkers(t, testFramework, map[string]*markers.Markers{
			"Block1": markers.NewMarkers(markers.NewMarker(0, 1)),
		})
		checkBlockMetadataDiffBranchIDs(t, testFramework, map[string][]*set.AdvancedSet[utxo.TransactionID]{
			"Block1": {set.NewAdvancedSet[utxo.TransactionID](), set.NewAdvancedSet[utxo.TransactionID]()},
		})
		checkBranchIDs(t, testFramework, map[string]*set.AdvancedSet[utxo.TransactionID]{
			"Block1": set.NewAdvancedSet[utxo.TransactionID](),
		})
	}

	// ISSUE Block2
	{
		testFramework.CreateBlock("Block2", WithStrongParents("Block1"), WithInputs("A"), WithOutput("blue", 500))
		testFramework.IssueBlocks("Block2").WaitUntilAllTasksProcessed()

		checkMarkers(t, testFramework, map[string]*markers.Markers{
			"Block1": markers.NewMarkers(markers.NewMarker(0, 1)),
			"Block2": markers.NewMarkers(markers.NewMarker(0, 2)),
		})
		checkBlockMetadataDiffBranchIDs(t, testFramework, map[string][]*set.AdvancedSet[utxo.TransactionID]{
			"Block1": {set.NewAdvancedSet[utxo.TransactionID](), set.NewAdvancedSet[utxo.TransactionID]()},
			"Block2": {set.NewAdvancedSet[utxo.TransactionID](), set.NewAdvancedSet[utxo.TransactionID]()},
		})
		checkBranchIDs(t, testFramework, map[string]*set.AdvancedSet[utxo.TransactionID]{
			"Block1": set.NewAdvancedSet[utxo.TransactionID](),
			"Block2": set.NewAdvancedSet[utxo.TransactionID](),
		})
	}

	// ISSUE Block3
	{
		testFramework.CreateBlock("Block3", WithStrongParents("Block2"))

		testFramework.IssueBlocks("Block3").WaitUntilAllTasksProcessed()

		checkMarkers(t, testFramework, map[string]*markers.Markers{
			"Block1": markers.NewMarkers(markers.NewMarker(0, 1)),
			"Block2": markers.NewMarkers(markers.NewMarker(0, 2)),
			"Block3": markers.NewMarkers(markers.NewMarker(0, 3)),
		})
		checkBlockMetadataDiffBranchIDs(t, testFramework, map[string][]*set.AdvancedSet[utxo.TransactionID]{
			"Block1": {set.NewAdvancedSet[utxo.TransactionID](), set.NewAdvancedSet[utxo.TransactionID]()},
			"Block2": {set.NewAdvancedSet[utxo.TransactionID](), set.NewAdvancedSet[utxo.TransactionID]()},
			"Block3": {set.NewAdvancedSet[utxo.TransactionID](), set.NewAdvancedSet[utxo.TransactionID]()},
		})
		checkBranchIDs(t, testFramework, map[string]*set.AdvancedSet[utxo.TransactionID]{
			"Block1": set.NewAdvancedSet[utxo.TransactionID](),
			"Block2": set.NewAdvancedSet[utxo.TransactionID](),
			"Block3": set.NewAdvancedSet[utxo.TransactionID](),
		})
	}

	// ISSUE Block4
	{
		testFramework.CreateBlock("Block4", WithStrongParents("Block1"), WithInputs("A"), WithOutput("red", 500))

		testFramework.RegisterBranchID("blue", "Block2")
		testFramework.RegisterBranchID("red", "Block4")

		testFramework.IssueBlocks("Block4").WaitUntilAllTasksProcessed()

		checkMarkers(t, testFramework, map[string]*markers.Markers{
			"Block1": markers.NewMarkers(markers.NewMarker(0, 1)),
			"Block2": markers.NewMarkers(markers.NewMarker(0, 2)),
			"Block3": markers.NewMarkers(markers.NewMarker(0, 3)),
			"Block4": markers.NewMarkers(markers.NewMarker(0, 1)),
		})
		checkBlockMetadataDiffBranchIDs(t, testFramework, map[string][]*set.AdvancedSet[utxo.TransactionID]{
			"Block1": {set.NewAdvancedSet[utxo.TransactionID](), set.NewAdvancedSet[utxo.TransactionID]()},
			"Block2": {set.NewAdvancedSet[utxo.TransactionID](), set.NewAdvancedSet[utxo.TransactionID]()},
			"Block3": {set.NewAdvancedSet[utxo.TransactionID](), set.NewAdvancedSet[utxo.TransactionID]()},
			"Block4": {testFramework.BranchIDs("red"), set.NewAdvancedSet[utxo.TransactionID]()},
		})
		checkBranchIDs(t, testFramework, map[string]*set.AdvancedSet[utxo.TransactionID]{
			"Block1": set.NewAdvancedSet[utxo.TransactionID](),
			"Block2": testFramework.BranchIDs("blue"),
			"Block3": testFramework.BranchIDs("blue"),
			"Block4": testFramework.BranchIDs("red"),
		})
	}

	// ISSUE Block5
	{

		testFramework.CreateBlock("Block5", WithStrongParents("Block3"))

		testFramework.IssueBlocks("Block5").WaitUntilAllTasksProcessed()

		checkMarkers(t, testFramework, map[string]*markers.Markers{
			"Block1": markers.NewMarkers(markers.NewMarker(0, 1)),
			"Block2": markers.NewMarkers(markers.NewMarker(0, 2)),
			"Block3": markers.NewMarkers(markers.NewMarker(0, 3)),
			"Block4": markers.NewMarkers(markers.NewMarker(0, 1)),
			"Block5": markers.NewMarkers(markers.NewMarker(0, 4)),
		})
		checkBlockMetadataDiffBranchIDs(t, testFramework, map[string][]*set.AdvancedSet[utxo.TransactionID]{
			"Block1": {set.NewAdvancedSet[utxo.TransactionID](), set.NewAdvancedSet[utxo.TransactionID]()},
			"Block2": {set.NewAdvancedSet[utxo.TransactionID](), set.NewAdvancedSet[utxo.TransactionID]()},
			"Block3": {set.NewAdvancedSet[utxo.TransactionID](), set.NewAdvancedSet[utxo.TransactionID]()},
			"Block4": {testFramework.BranchIDs("red"), set.NewAdvancedSet[utxo.TransactionID]()},
			"Block5": {set.NewAdvancedSet[utxo.TransactionID](), set.NewAdvancedSet[utxo.TransactionID]()},
		})
		checkBranchIDs(t, testFramework, map[string]*set.AdvancedSet[utxo.TransactionID]{
			"Block1": set.NewAdvancedSet[utxo.TransactionID](),
			"Block2": testFramework.BranchIDs("blue"),
			"Block3": testFramework.BranchIDs("blue"),
			"Block4": testFramework.BranchIDs("red"),
			"Block5": testFramework.BranchIDs("blue"),
		})
	}

	// ISSUE Block6
	{

		tg.Ledger.ConflictDAG.SetBranchAccepted(testFramework.BranchID("red"))

		testFramework.CreateBlock("Block6", WithStrongParents("Block4"))

		testFramework.IssueBlocks("Block6").WaitUntilAllTasksProcessed()

		checkMarkers(t, testFramework, map[string]*markers.Markers{
			"Block1": markers.NewMarkers(markers.NewMarker(0, 1)),
			"Block2": markers.NewMarkers(markers.NewMarker(0, 2)),
			"Block3": markers.NewMarkers(markers.NewMarker(0, 3)),
			"Block4": markers.NewMarkers(markers.NewMarker(0, 1)),
			"Block5": markers.NewMarkers(markers.NewMarker(0, 4)),
			"Block6": markers.NewMarkers(markers.NewMarker(0, 1)),
		})
		checkBlockMetadataDiffBranchIDs(t, testFramework, map[string][]*set.AdvancedSet[utxo.TransactionID]{
			"Block1": {set.NewAdvancedSet[utxo.TransactionID](), set.NewAdvancedSet[utxo.TransactionID]()},
			"Block2": {set.NewAdvancedSet[utxo.TransactionID](), set.NewAdvancedSet[utxo.TransactionID]()},
			"Block3": {set.NewAdvancedSet[utxo.TransactionID](), set.NewAdvancedSet[utxo.TransactionID]()},
			"Block4": {testFramework.BranchIDs("red"), set.NewAdvancedSet[utxo.TransactionID]()},
			"Block5": {set.NewAdvancedSet[utxo.TransactionID](), set.NewAdvancedSet[utxo.TransactionID]()},
			"Block6": {set.NewAdvancedSet[utxo.TransactionID](), set.NewAdvancedSet[utxo.TransactionID]()},
		})
		checkBranchIDs(t, testFramework, map[string]*set.AdvancedSet[utxo.TransactionID]{
			"Block1": set.NewAdvancedSet[utxo.TransactionID](),
			"Block2": testFramework.BranchIDs("blue"),
			"Block3": testFramework.BranchIDs("blue"),
			"Block4": testFramework.BranchIDs("red"),
			"Block5": testFramework.BranchIDs("blue"),
			"Block6": set.NewAdvancedSet[utxo.TransactionID](),
		})
	}

	// ISSUE Block7
	{

		testFramework.CreateBlock("Block7", WithStrongParents("Block6"))

		testFramework.IssueBlocks("Block7").WaitUntilAllTasksProcessed()

		checkMarkers(t, testFramework, map[string]*markers.Markers{
			"Block1": markers.NewMarkers(markers.NewMarker(0, 1)),
			"Block2": markers.NewMarkers(markers.NewMarker(0, 2)),
			"Block3": markers.NewMarkers(markers.NewMarker(0, 3)),
			"Block4": markers.NewMarkers(markers.NewMarker(0, 1)),
			"Block5": markers.NewMarkers(markers.NewMarker(0, 4)),
			"Block6": markers.NewMarkers(markers.NewMarker(0, 1)),
			"Block7": markers.NewMarkers(markers.NewMarker(0, 1)),
		})
		checkBlockMetadataDiffBranchIDs(t, testFramework, map[string][]*set.AdvancedSet[utxo.TransactionID]{
			"Block1": {set.NewAdvancedSet[utxo.TransactionID](), set.NewAdvancedSet[utxo.TransactionID]()},
			"Block2": {set.NewAdvancedSet[utxo.TransactionID](), set.NewAdvancedSet[utxo.TransactionID]()},
			"Block3": {set.NewAdvancedSet[utxo.TransactionID](), set.NewAdvancedSet[utxo.TransactionID]()},
			"Block4": {testFramework.BranchIDs("red"), set.NewAdvancedSet[utxo.TransactionID]()},
			"Block5": {set.NewAdvancedSet[utxo.TransactionID](), set.NewAdvancedSet[utxo.TransactionID]()},
			"Block6": {set.NewAdvancedSet[utxo.TransactionID](), set.NewAdvancedSet[utxo.TransactionID]()},
			"Block7": {set.NewAdvancedSet[utxo.TransactionID](), set.NewAdvancedSet[utxo.TransactionID]()},
		})
		checkBranchIDs(t, testFramework, map[string]*set.AdvancedSet[utxo.TransactionID]{
			"Block1": set.NewAdvancedSet[utxo.TransactionID](),
			"Block2": testFramework.BranchIDs("blue"),
			"Block3": testFramework.BranchIDs("blue"),
			"Block4": testFramework.BranchIDs("red"),
			"Block5": testFramework.BranchIDs("blue"),
			"Block6": set.NewAdvancedSet[utxo.TransactionID](),
			"Block7": set.NewAdvancedSet[utxo.TransactionID](),
		})
	}
}

func TestObjectiveInvalidity(t *testing.T) {
	tangle := NewTestTangle()
	defer tangle.Shutdown()

	testFramework := NewBlockTestFramework(
		tangle,
		WithGenesisOutput("A", 500),
		WithGenesisOutput("B", 500),
		WithGenesisOutput("C", 500),
		WithGenesisOutput("L", 500),
	)

	tangle.Setup()

	// ISSUE Block1
	{
		testFramework.CreateBlock("Block1", WithStrongParents("Genesis"), WithInputs("A"), WithOutput("G", 500))
		testFramework.IssueBlocks("Block1").WaitUntilAllTasksProcessed()

		checkMarkers(t, testFramework, map[string]*markers.Markers{
			"Block1": markers.NewMarkers(markers.NewMarker(0, 1)),
		})
		checkBlockMetadataDiffBranchIDs(t, testFramework, map[string][]*set.AdvancedSet[utxo.TransactionID]{
			"Block1": {set.NewAdvancedSet[utxo.TransactionID](), set.NewAdvancedSet[utxo.TransactionID]()},
		})
		checkBranchIDs(t, testFramework, map[string]*set.AdvancedSet[utxo.TransactionID]{
			"Block1": set.NewAdvancedSet[utxo.TransactionID](),
		})
	}

	// ISSUE Block2
	{
		testFramework.CreateBlock("Block2", WithStrongParents("Genesis"), WithInputs("A", "B"), WithOutput("E", 1000))

		testFramework.RegisterBranchID("A", "Block1")
		testFramework.RegisterBranchID("B", "Block2")

		testFramework.IssueBlocks("Block2").WaitUntilAllTasksProcessed()

		checkMarkers(t, testFramework, map[string]*markers.Markers{
			"Block1": markers.NewMarkers(markers.NewMarker(0, 1)),
			"Block2": markers.NewMarkers(markers.NewMarker(0, 0)),
		})
		checkBlockMetadataDiffBranchIDs(t, testFramework, map[string][]*set.AdvancedSet[utxo.TransactionID]{
			"Block1": {set.NewAdvancedSet[utxo.TransactionID](), set.NewAdvancedSet[utxo.TransactionID]()},
			"Block2": {testFramework.BranchIDs("B"), set.NewAdvancedSet[utxo.TransactionID]()},
		})
		checkBranchIDs(t, testFramework, map[string]*set.AdvancedSet[utxo.TransactionID]{
			"Block1": testFramework.BranchIDs("A"),
			"Block2": testFramework.BranchIDs("B"),
		})
	}

	// ISSUE Block3
	{
		testFramework.CreateBlock("Block3", WithStrongParents("Genesis"), WithInputs("B"), WithOutput("F", 500))

		testFramework.RegisterBranchID("C", "Block3")

		testFramework.IssueBlocks("Block3").WaitUntilAllTasksProcessed()

		checkMarkers(t, testFramework, map[string]*markers.Markers{
			"Block1": markers.NewMarkers(markers.NewMarker(0, 1)),
			"Block2": markers.NewMarkers(markers.NewMarker(0, 0)),
			"Block3": markers.NewMarkers(markers.NewMarker(0, 0)),
		})
		checkBlockMetadataDiffBranchIDs(t, testFramework, map[string][]*set.AdvancedSet[utxo.TransactionID]{
			"Block1": {set.NewAdvancedSet[utxo.TransactionID](), set.NewAdvancedSet[utxo.TransactionID]()},
			"Block2": {testFramework.BranchIDs("B"), set.NewAdvancedSet[utxo.TransactionID]()},
			"Block3": {testFramework.BranchIDs("C"), set.NewAdvancedSet[utxo.TransactionID]()},
		})
		checkBranchIDs(t, testFramework, map[string]*set.AdvancedSet[utxo.TransactionID]{
			"Block1": testFramework.BranchIDs("A"),
			"Block2": testFramework.BranchIDs("B"),
			"Block3": testFramework.BranchIDs("C"),
		})
	}

	// ISSUE Block4
	{
		testFramework.CreateBlock("Block4", WithStrongParents("Block3"))
		testFramework.IssueBlocks("Block4").WaitUntilAllTasksProcessed()

		checkMarkers(t, testFramework, map[string]*markers.Markers{
			"Block1": markers.NewMarkers(markers.NewMarker(0, 1)),
			"Block2": markers.NewMarkers(markers.NewMarker(0, 0)),
			"Block3": markers.NewMarkers(markers.NewMarker(0, 0)),
			"Block4": markers.NewMarkers(markers.NewMarker(0, 0)),
		})
		checkBlockMetadataDiffBranchIDs(t, testFramework, map[string][]*set.AdvancedSet[utxo.TransactionID]{
			"Block1": {set.NewAdvancedSet[utxo.TransactionID](), set.NewAdvancedSet[utxo.TransactionID]()},
			"Block2": {testFramework.BranchIDs("B"), set.NewAdvancedSet[utxo.TransactionID]()},
			"Block3": {testFramework.BranchIDs("C"), set.NewAdvancedSet[utxo.TransactionID]()},
			"Block4": {testFramework.BranchIDs("C"), set.NewAdvancedSet[utxo.TransactionID]()},
		})
		checkBranchIDs(t, testFramework, map[string]*set.AdvancedSet[utxo.TransactionID]{
			"Block1": testFramework.BranchIDs("A"),
			"Block2": testFramework.BranchIDs("B"),
			"Block3": testFramework.BranchIDs("C"),
			"Block4": testFramework.BranchIDs("C"),
		})
	}

	// ISSUE Block5
	{
		blk := testFramework.CreateBlock("Block5", WithStrongParents("Block4"), WithShallowLikeParents("Block4"))
		testFramework.IssueBlocks("Block5").WaitUntilAllTasksProcessed()

		tangle.Storage.BlockMetadata(blk.ID()).Consume(func(blockMetadata *BlockMetadata) {
			assert.True(t, blockMetadata.IsObjectivelyInvalid())
		})

		checkMarkers(t, testFramework, map[string]*markers.Markers{
			"Block1": markers.NewMarkers(markers.NewMarker(0, 1)),
			"Block2": markers.NewMarkers(markers.NewMarker(0, 0)),
			"Block3": markers.NewMarkers(markers.NewMarker(0, 0)),
			"Block4": markers.NewMarkers(markers.NewMarker(0, 0)),
		})
		checkBlockMetadataDiffBranchIDs(t, testFramework, map[string][]*set.AdvancedSet[utxo.TransactionID]{
			"Block1": {set.NewAdvancedSet[utxo.TransactionID](), set.NewAdvancedSet[utxo.TransactionID]()},
			"Block2": {testFramework.BranchIDs("B"), set.NewAdvancedSet[utxo.TransactionID]()},
			"Block3": {testFramework.BranchIDs("C"), set.NewAdvancedSet[utxo.TransactionID]()},
			"Block4": {testFramework.BranchIDs("C"), set.NewAdvancedSet[utxo.TransactionID]()},
		})
		checkBranchIDs(t, testFramework, map[string]*set.AdvancedSet[utxo.TransactionID]{
			"Block1": testFramework.BranchIDs("A"),
			"Block2": testFramework.BranchIDs("B"),
			"Block3": testFramework.BranchIDs("C"),
			"Block4": testFramework.BranchIDs("C"),
		})
	}
}

func TestFutureConeDislike(t *testing.T) {
	tangle := NewTestTangle(WithConflictDAGOptions(conflictdag.WithMergeToMaster(false)))
	defer tangle.Shutdown()

	testFramework := NewBlockTestFramework(
		tangle,
		WithGenesisOutput("G", 1),
	)

	tangle.Setup()

	testFramework.CreateBlock("Block1", WithStrongParents("Genesis"), WithInputs("G"), WithOutput("A", 1))
	testFramework.CreateBlock("Block1*", WithStrongParents("Genesis"), WithInputs("G"), WithOutput("A*", 1))
	testFramework.CreateBlock("Block2", WithStrongParents("Block1"), WithInputs("A"), WithOutput("B", 1))
	testFramework.CreateBlock("Block2*", WithStrongParents("Block1"), WithInputs("A"), WithOutput("B*", 1))
	testFramework.CreateBlock("Block3", WithStrongParents("Block2"), WithShallowLikeParents("Block1*"))
	testFramework.CreateBlock("Block4", WithStrongParents("Block2"), WithShallowLikeParents("Block1*"))

	testFramework.RegisterBranchID("A", "Block1")
	testFramework.RegisterBranchID("A*", "Block1*")
	testFramework.RegisterBranchID("B", "Block2")
	testFramework.RegisterBranchID("B*", "Block2*")

	{
		testFramework.IssueBlocks("Block1").WaitUntilAllTasksProcessed()
		testFramework.IssueBlocks("Block1*").WaitUntilAllTasksProcessed()
		testFramework.IssueBlocks("Block2").WaitUntilAllTasksProcessed()
		testFramework.IssueBlocks("Block3").WaitUntilAllTasksProcessed()

		checkBranchIDs(t, testFramework, map[string]utxo.TransactionIDs{
			"Block1":  testFramework.BranchIDs("A"),
			"Block1*": testFramework.BranchIDs("A*"),
			"Block2":  testFramework.BranchIDs("A"),
			"Block3":  testFramework.BranchIDs("A*"),
		})
	}

	{
		testFramework.IssueBlocks("Block2*").WaitUntilAllTasksProcessed()

		checkBranchIDs(t, testFramework, map[string]utxo.TransactionIDs{
			"Block1":  testFramework.BranchIDs("A"),
			"Block1*": testFramework.BranchIDs("A*"),
			"Block2":  testFramework.BranchIDs("A", "B"),
			"Block2*": testFramework.BranchIDs("A", "B*"),
			"Block3":  testFramework.BranchIDs("A*"),
		})
	}

	{
		testFramework.IssueBlocks("Block4").WaitUntilAllTasksProcessed()

		checkBranchIDs(t, testFramework, map[string]utxo.TransactionIDs{
			"Block1":  testFramework.BranchIDs("A"),
			"Block1*": testFramework.BranchIDs("A*"),
			"Block2":  testFramework.BranchIDs("A", "B"),
			"Block2*": testFramework.BranchIDs("A", "B*"),
			"Block3":  testFramework.BranchIDs("A*"),
			"Block4":  testFramework.BranchIDs("A*"),
		})
	}
}

func TestMultiThreadedBookingAndForkingParallel(t *testing.T) {
	debug.SetEnabled(true)
	const layersNum = 127
	const widthSize = 8 // since we reference all blocks in the layer below, this is limited by the max parents

	tangle := NewTestTangle(WithConflictDAGOptions(conflictdag.WithMergeToMaster(false)))
	defer tangle.Shutdown()

	testFramework := NewBlockTestFramework(
		tangle,
		WithGenesisOutput("G", layersNum),
	)

	tangle.Setup()

	// Create base-layer outputs to double-spend
	genesisBlockOptions := []BlockOption{
		WithStrongParents("Genesis"),
		WithInputs("G"),
	}
	for layer := 0; layer < layersNum; layer++ {
		genesisBlockOptions = append(genesisBlockOptions, WithOutput(fmt.Sprintf("G.%d", layer), 1))
	}

	testFramework.CreateBlock("Block.G", genesisBlockOptions...)
	testFramework.IssueBlocks("Block.G").WaitUntilAllTasksProcessed()

	blks := make([]string, 0)

	for layer := 0; layer < layersNum; layer++ {
		for width := 0; width < widthSize; width++ {
			blkName := fmt.Sprintf("Block.%d.%d", layer, width)
			strongParents := make([]string, 0)
			if layer == 0 {
				strongParents = append(strongParents, "Block.G")
			} else {
				for innerWidth := 0; innerWidth < widthSize; innerWidth++ {
					strongParents = append(strongParents, fmt.Sprintf("Block.%d.%d", layer-1, innerWidth))
				}
			}

			var input string
			var output string
			var conflict string

			// We fork on the first two blocks for each layer
			if width < 2 {
				input = fmt.Sprintf("G.%d", layer)
				output = fmt.Sprintf("O.%d.%d", layer, width)
				conflict = fmt.Sprintf("C.%d.%d", layer, width)
			}

			if input != "" {
				testFramework.CreateBlock(blkName, WithStrongParents(strongParents...), WithInputs(input), WithOutput(output, 1))
				testFramework.RegisterBranchID(conflict, blkName)
				testFramework.RegisterTransactionID(conflict, blkName)
			} else {
				testFramework.CreateBlock(blkName, WithStrongParents(strongParents...))
			}

			blks = append(blks, blkName)
		}
	}

	rand.Seed(time.Now().UnixNano())

	var wg sync.WaitGroup

	for i := 0; i < len(blks); i++ {
		wg.Add(1)
		go func(i int) {
			time.Sleep(time.Duration(int(50 * 1000 * rand.Float32())))
			testFramework.IssueBlocks(blks[i])
			wg.Done()
		}(i)
	}

	wg.Wait()
	testFramework.WaitUntilAllTasksProcessed()

	expectedConflicts := make(map[string]utxo.TransactionIDs)
	for layer := 0; layer < layersNum; layer++ {
		for width := 0; width < widthSize; width++ {
			blkName := fmt.Sprintf("Block.%d.%d", layer, width)
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
				expectedConflicts[blkName] = utxo.NewTransactionIDs()
				continue
			}

			expectedConflicts[blkName] = testFramework.BranchIDs(branches...)
		}
	}

	checkBranchIDs(t, testFramework, expectedConflicts)
}

func TestMultiThreadedBookingAndForkingNested(t *testing.T) {
	const layersNum = 50
	const widthSize = 8 // since we reference all blocks in the layer below, this is limited by the max parents

	tangle := NewTestTangle(WithConflictDAGOptions(conflictdag.WithMergeToMaster(false)))
	defer tangle.Shutdown()

	testFramework := NewBlockTestFramework(
		tangle,
		WithGenesisOutput("G", widthSize),
	)

	tangle.Setup()

	// Create base-layer outputs to double-spend
	genesisBlockOptions := []BlockOption{
		WithStrongParents("Genesis"),
		WithInputs("G"),
	}
	for layer := 0; layer < widthSize; layer++ {
		genesisBlockOptions = append(genesisBlockOptions, WithOutput(fmt.Sprintf("G.%d", layer), 1))
	}
	testFramework.CreateBlock("Block.G", genesisBlockOptions...)
	testFramework.IssueBlocks("Block.G").WaitUntilAllTasksProcessed()

	blks := make([]string, 0)

	for layer := 0; layer < layersNum; layer++ {
		for width := 0; width < widthSize; width++ {
			blkName := fmt.Sprintf("Block.%d.%d", layer, width)
			strongParents := make([]string, 0)
			likeParents := make([]string, 0)
			if layer == 0 {
				strongParents = append(strongParents, "Block.G")
			} else {
				for innerWidth := 0; innerWidth < widthSize; innerWidth++ {
					strongParents = append(strongParents, fmt.Sprintf("Block.%d.%d", layer-1, innerWidth))
					// We only like the first conflict over the second, to fork it on the next layer.
					if innerWidth%2 == 0 {
						likeParents = append(likeParents, fmt.Sprintf("Block.%d.%d", layer-1, innerWidth))
					}
				}
			}

			var input string
			var output string
			var conflict string

			if layer == 0 {
				input = fmt.Sprintf("G.%d", width-width%2)
			} else {
				// We spend from the first of the couple forks
				input = fmt.Sprintf("O.%d.%d", layer-1, width-width%2)
			}
			output = fmt.Sprintf("O.%d.%d", layer, width)
			conflict = fmt.Sprintf("C.%d.%d", layer, width)

			testFramework.CreateBlock(blkName, WithStrongParents(strongParents...), WithShallowLikeParents(likeParents...), WithInputs(input), WithOutput(output, 1))
			testFramework.RegisterBranchID(conflict, blkName)
			testFramework.RegisterTransactionID(conflict, blkName)

			blks = append(blks, blkName)
		}
	}

	var wg sync.WaitGroup
	rand.Seed(time.Now().UnixNano())
	for i := 0; i < len(blks); i++ {
		wg.Add(1)
		go func(i int) {
			time.Sleep(time.Duration(int(50 * 1000 * rand.Float32())))
			testFramework.IssueBlocks(blks[i])
			wg.Done()
		}(i)
	}
	wg.Wait()
	testFramework.WaitUntilAllTasksProcessed()

	expectedConflicts := make(map[string]utxo.TransactionIDs)
	for layer := 0; layer < layersNum; layer++ {
		for width := 0; width < widthSize; width++ {
			blkName := fmt.Sprintf("Block.%d.%d", layer, width)
			branches := make([]string, 0)

			// Add conflict of the current layer
			branches = append(branches, fmt.Sprintf("C.%d.%d", layer, width))

			// Add conflicts of the previous layers
			for innerLayer := layer - 1; innerLayer >= 0; innerLayer-- {
				for innerWidth := 0; innerWidth < widthSize; innerWidth += 2 {
					branches = append(branches, fmt.Sprintf("C.%d.%d", innerLayer, innerWidth))
				}
			}

			expectedConflicts[blkName] = testFramework.BranchIDs(branches...)
		}
	}

	checkNormalizedBranchIDsContained(t, testFramework, expectedConflicts)
}

func checkMarkers(t *testing.T, testFramework *BlockTestFramework, expectedMarkers map[string]*markers.Markers) {
	for blockID, expectedMarkersOfBlock := range expectedMarkers {
		assert.True(t, testFramework.tangle.Storage.BlockMetadata(testFramework.Block(blockID).ID()).Consume(func(blockMetadata *BlockMetadata) {
			assert.True(t, expectedMarkersOfBlock.Equals(blockMetadata.StructureDetails().PastMarkers()), "Markers of %s are wrong.\n"+
				"Expected: %+v\nActual: %+v", blockID, expectedMarkersOfBlock, blockMetadata.StructureDetails().PastMarkers)
		}))

		// if we have only a single marker - check if the marker is mapped to this block (or its inherited past marker)
		if expectedMarkersOfBlock.Size() == 1 {
			currentMarker := expectedMarkersOfBlock.Marker()

			mappedBlockIDOfMarker := testFramework.tangle.Booker.MarkersManager.BlockID(currentMarker)
			currentBlockID := testFramework.Block(blockID).ID()

			if mappedBlockIDOfMarker == currentBlockID {
				continue
			}

			assert.True(t, testFramework.tangle.Storage.BlockMetadata(mappedBlockIDOfMarker).Consume(func(blockMetadata *BlockMetadata) {
				// Blocks attaching to Genesis can have 0,0 as a PastMarker, so do not check Markers -> Block.
				if currentMarker.SequenceID() == 0 && currentMarker.Index() == 0 {
					return
				}

				if assert.True(t, blockMetadata.StructureDetails().IsPastMarker(), "Block with %s should be PastMarker", blockMetadata.ID()) {
					assert.True(t, blockMetadata.StructureDetails().PastMarkers().Marker() == currentMarker, "PastMarker of %s is wrong.\n"+
						"Expected: %+v\nActual: %+v", blockMetadata.ID(), currentMarker, blockMetadata.StructureDetails().PastMarkers().Marker())
				}
			}), "failed to load Block with %s", mappedBlockIDOfMarker)
		}
	}
}

func checkBranchIDs(t *testing.T, testFramework *BlockTestFramework, expectedBranchIDs map[string]*set.AdvancedSet[utxo.TransactionID]) {
	for blockID, blockExpectedBranchIDs := range expectedBranchIDs {
		// blockMetadata := testFramework.BlockMetadata(blockID)
		// fmt.Println("Add:", blockMetadata.addedBranchIDs, "Sub:", blockMetadata.subtractedBranchIDs)

		retrievedBranchIDs, errRetrieve := testFramework.tangle.Booker.BlockBranchIDs(testFramework.Block(blockID).ID())
		assert.NoError(t, errRetrieve)

		assert.True(t, blockExpectedBranchIDs.Equal(retrievedBranchIDs), "BranchID of %s should be %s but is %s", blockID, blockExpectedBranchIDs, retrievedBranchIDs)
	}
}

func checkNormalizedBranchIDsContained(t *testing.T, testFramework *BlockTestFramework, expectedContainedBranchIDs map[string]*set.AdvancedSet[utxo.TransactionID]) {
	for blockID, blockExpectedBranchIDs := range expectedContainedBranchIDs {
		retrievedBranchIDs, errRetrieve := testFramework.tangle.Booker.BlockBranchIDs(testFramework.Block(blockID).ID())
		assert.NoError(t, errRetrieve)

		normalizedRetrievedBranchIDs := retrievedBranchIDs.Clone()
		for it := retrievedBranchIDs.Iterator(); it.HasNext(); {
			testFramework.tangle.Ledger.ConflictDAG.Storage.CachedConflict(it.Next()).Consume(func(b *conflictdag.Conflict[utxo.TransactionID, utxo.OutputID]) {
				normalizedRetrievedBranchIDs.DeleteAll(b.Parents())
			})
		}

		normalizedExpectedBranchIDs := blockExpectedBranchIDs.Clone()
		for it := blockExpectedBranchIDs.Iterator(); it.HasNext(); {
			testFramework.tangle.Ledger.ConflictDAG.Storage.CachedConflict(it.Next()).Consume(func(b *conflictdag.Conflict[utxo.TransactionID, utxo.OutputID]) {
				normalizedExpectedBranchIDs.DeleteAll(b.Parents())
			})
		}

		assert.True(t, normalizedExpectedBranchIDs.Intersect(normalizedRetrievedBranchIDs).Size() == normalizedExpectedBranchIDs.Size(), "BranchID of %s should be %s but is %s", blockID, normalizedExpectedBranchIDs, normalizedRetrievedBranchIDs)
	}
}

func checkBlockMetadataDiffBranchIDs(t *testing.T, testFramework *BlockTestFramework, expectedDiffBranchIDs map[string][]*set.AdvancedSet[utxo.TransactionID]) {
	for blockID, expectedDiffBranchID := range expectedDiffBranchIDs {
		assert.True(t, testFramework.tangle.Storage.BlockMetadata(testFramework.Block(blockID).ID()).Consume(func(blockMetadata *BlockMetadata) {
			assert.True(t, expectedDiffBranchID[0].Equal(blockMetadata.AddedBranchIDs()), "AddBranchIDs of %s should be %s but is %s in the Metadata", blockID, expectedDiffBranchID[0], blockMetadata.AddedBranchIDs())
		}))
		assert.True(t, testFramework.tangle.Storage.BlockMetadata(testFramework.Block(blockID).ID()).Consume(func(blockMetadata *BlockMetadata) {
			assert.True(t, expectedDiffBranchID[1].Equal(blockMetadata.SubtractedBranchIDs()), "SubtractedBranchIDs of %s should be %s but is %s in the Metadata", blockID, expectedDiffBranchID[1], blockMetadata.SubtractedBranchIDs())
		}))
	}
}
