package branchdag

import (
	"strings"
	"testing"

	"github.com/iotaledger/hive.go/kvstore/mapdb"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/iotaledger/goshimmer/packages/database"
	"github.com/iotaledger/goshimmer/packages/refactored/ledger/utxo"
)

func TestBranchDAG_RetrieveBranch(t *testing.T) {
	branchDAG := NewBranchDAG(mapdb.NewMapDB(), database.NewCacheTimeProvider(0))
	defer branchDAG.Shutdown()

	err := branchDAG.Prune()
	require.NoError(t, err)

	assert.True(t, branchDAG.CreateBranch(utxo.TransactionID{2}, NewBranchIDs(MasterBranchID), NewConflictIDs(ConflictID{0}, ConflictID{1})))
	cachedBranch2 := branchDAG.Branch(utxo.TransactionID{2})
	defer cachedBranch2.Release()
	Branch2, exists := cachedBranch2.Unwrap()
	require.True(t, exists)
	assert.Equal(t, NewBranchIDs(MasterBranchID), Branch2.Parents())
	assert.True(t, NewConflictIDs(ConflictID{0}, ConflictID{1}).Equal(Branch2.Conflicts()))

	assert.True(t, branchDAG.CreateBranch(utxo.TransactionID{3}, NewBranchIDs(Branch2.ID()), NewConflictIDs(ConflictID{0}, ConflictID{1}, ConflictID{2})))
	cachedBranch3 := branchDAG.Branch(utxo.TransactionID{3})
	defer cachedBranch3.Release()
	Branch3, exists := cachedBranch3.Unwrap()
	require.True(t, exists)

	assert.Equal(t, NewBranchIDs(Branch2.ID()), Branch3.Parents())
	assert.Equal(t, NewConflictIDs(ConflictID{0}, ConflictID{1}, ConflictID{2}), Branch3.Conflicts())

	assert.False(t, branchDAG.CreateBranch(utxo.TransactionID{2}, NewBranchIDs(MasterBranchID), NewConflictIDs(ConflictID{0}, ConflictID{1}, ConflictID{2})))
	cachedBranch2 = branchDAG.Branch(utxo.TransactionID{2})
	defer cachedBranch2.Release()
	Branch2, exists = cachedBranch2.Unwrap()
	require.True(t, exists)

	assert.Equal(t, NewConflictIDs(ConflictID{0}, ConflictID{1}, ConflictID{2}), Branch2.Conflicts())

	assert.True(t, branchDAG.CreateBranch(utxo.TransactionID{4}, NewBranchIDs(Branch3.ID(), Branch3.ID()), NewConflictIDs(ConflictID{3})))
	cachedBranch4 := branchDAG.Branch(utxo.TransactionID{4})
	defer cachedBranch4.Release()
	Branch4, exists := cachedBranch4.Unwrap()
	require.True(t, exists)
	assert.Equal(t, NewConflictIDs(ConflictID{3}), Branch4.Conflicts())
}

func TestBranchDAG_ConflictMembers(t *testing.T) {
	branchDAG := NewBranchDAG(mapdb.NewMapDB(), database.NewCacheTimeProvider(0))
	defer branchDAG.Shutdown()

	err := branchDAG.Prune()
	require.NoError(t, err)

	// create initial branches
	assert.True(t, branchDAG.CreateBranch(utxo.TransactionID{2}, NewBranchIDs(MasterBranchID), NewConflictIDs(ConflictID{0})))
	cachedBranch2 := branchDAG.Branch(utxo.TransactionID{2})
	defer cachedBranch2.Release()
	branch2, exists := cachedBranch2.Unwrap()
	assert.True(t, exists)

	assert.True(t, branchDAG.CreateBranch(utxo.TransactionID{3}, NewBranchIDs(MasterBranchID), NewConflictIDs(ConflictID{0})))
	cachedBranch3 := branchDAG.Branch(utxo.TransactionID{3})
	defer cachedBranch3.Release()
	branch3, exists := cachedBranch3.Unwrap()
	assert.True(t, exists)

	// assert conflict members
	expectedConflictMembers := map[BranchID]struct{}{
		branch2.ID(): {}, branch3.ID(): {},
	}
	actualConflictMembers := map[BranchID]struct{}{}
	branchDAG.ConflictMembers(ConflictID{0}).Consume(func(conflictMember *ConflictMember) {
		actualConflictMembers[conflictMember.BranchID()] = struct{}{}
	})
	assert.Equal(t, expectedConflictMembers, actualConflictMembers)

	// add branch 4
	assert.True(t, branchDAG.CreateBranch(utxo.TransactionID{4}, NewBranchIDs(MasterBranchID), NewConflictIDs(ConflictID{0})))
	cachedBranch4 := branchDAG.Branch(utxo.TransactionID{4})
	defer cachedBranch4.Release()
	branch4, exists := cachedBranch4.Unwrap()
	assert.True(t, exists)

	// branch 4 should now also be part of the conflict set
	expectedConflictMembers = map[BranchID]struct{}{
		branch2.ID(): {}, branch3.ID(): {}, branch4.ID(): {},
	}
	actualConflictMembers = map[BranchID]struct{}{}
	branchDAG.ConflictMembers(ConflictID{0}).Consume(func(conflictMember *ConflictMember) {
		actualConflictMembers[conflictMember.BranchID()] = struct{}{}
	})
	assert.Equal(t, expectedConflictMembers, actualConflictMembers)
}

func TestBranchDAG_SetBranchConfirmed(t *testing.T) {
	branchDAG := NewBranchDAG(mapdb.NewMapDB(), database.NewCacheTimeProvider(0))
	defer branchDAG.Shutdown()

	err := branchDAG.Prune()
	require.NoError(t, err)

	branchIDs := make(map[string]BranchID)
	branchIDs["Branch2"] = createBranch(t, branchDAG, "Branch2", NewBranchIDs(MasterBranchID), NewConflictIDs(ConflictID{0}))
	branchIDs["Branch3"] = createBranch(t, branchDAG, "Branch3", NewBranchIDs(MasterBranchID), NewConflictIDs(ConflictID{0}))
	branchIDs["Branch4"] = createBranch(t, branchDAG, "Branch4", NewBranchIDs(branchIDs["Branch2"]), NewConflictIDs(ConflictID{1}))
	branchIDs["Branch5"] = createBranch(t, branchDAG, "Branch5", NewBranchIDs(branchIDs["Branch2"]), NewConflictIDs(ConflictID{1}))
	branchIDs["Branch6"] = createBranch(t, branchDAG, "Branch6", NewBranchIDs(MasterBranchID), NewConflictIDs(ConflictID{2}))
	branchIDs["Branch7"] = createBranch(t, branchDAG, "Branch7", NewBranchIDs(MasterBranchID), NewConflictIDs(ConflictID{2}))
	branchIDs["Branch8"] = createBranch(t, branchDAG, "Branch8", NewBranchIDs(MasterBranchID), NewConflictIDs(ConflictID{2}))

	assert.True(t, branchDAG.SetBranchConfirmed(branchIDs["Branch4"]))

	assertInclusionStates(t, branchDAG, branchIDs, map[string]InclusionState{
		"Branch2":         Confirmed,
		"Branch3":         Rejected,
		"Branch4":         Confirmed,
		"Branch5":         Rejected,
		"Branch6":         Pending,
		"Branch7":         Pending,
		"Branch8":         Pending,
		"Branch5+Branch7": Rejected,
		"Branch2+Branch7": Pending,
		"Branch5+Branch8": Rejected,
	})

	assert.True(t, branchDAG.SetBranchConfirmed(branchIDs["Branch8"]))

	// Create a new Branch in an already-decided Conflict Set results in straight Reject
	branchIDs["Branch9"] = createBranch(t, branchDAG, "Branch9", NewBranchIDs(MasterBranchID), NewConflictIDs(ConflictID{2}))

	assertInclusionStates(t, branchDAG, branchIDs, map[string]InclusionState{
		"Branch2":         Confirmed,
		"Branch3":         Rejected,
		"Branch4":         Confirmed,
		"Branch5":         Rejected,
		"Branch6":         Rejected,
		"Branch7":         Rejected,
		"Branch8":         Confirmed,
		"Branch5+Branch7": Rejected,
		"Branch2+Branch7": Rejected,
		"Branch5+Branch8": Rejected,
		// Spawning a new aggregated branch with Confirmed parents results in Confirmed
		"Branch4+Branch8": Confirmed,
		// Spawning a new aggregated branch with any Rejected parent results in Rejected
		"Branch3+Branch8": Rejected,
		"Branch9":         Rejected,
	})

	branchIDs["Branch10"] = createBranch(t, branchDAG, "Branch10", NewBranchIDs(MasterBranchID), NewConflictIDs(ConflictID{3}))
	branchIDs["Branch11"] = createBranch(t, branchDAG, "Branch11", NewBranchIDs(MasterBranchID), NewConflictIDs(ConflictID{3}))

	branchDAG.SetBranchConfirmed(branchIDs["Branch10"])

	assertInclusionStates(t, branchDAG, branchIDs, map[string]InclusionState{
		"Branch2":                  Confirmed,
		"Branch3":                  Rejected,
		"Branch4":                  Confirmed,
		"Branch5":                  Rejected,
		"Branch6":                  Rejected,
		"Branch7":                  Rejected,
		"Branch8":                  Confirmed,
		"Branch5+Branch7":          Rejected,
		"Branch2+Branch7":          Rejected,
		"Branch5+Branch8":          Rejected,
		"Branch4+Branch8":          Confirmed,
		"Branch3+Branch8":          Rejected,
		"Branch9":                  Rejected,
		"Branch10":                 Confirmed,
		"Branch11":                 Rejected,
		"Branch2+Branch7+Branch11": Rejected,
	})
}

func TestArithmeticBranchIDs_Add(t *testing.T) {
	var branchID1 BranchID
	_ = branchID1.FromRandomness()
	var branchID2 BranchID
	_ = branchID2.FromRandomness()
	var branchID3 BranchID
	_ = branchID3.FromRandomness()

	branchID1.RegisterAlias("branchID1")
	branchID2.RegisterAlias("branchID2")
	branchID3.RegisterAlias("branchID3")

	arithmeticBranchIDs := NewArithmeticBranchIDs()
	arithmeticBranchIDs.Add(NewBranchIDs(branchID1, branchID2))
	arithmeticBranchIDs.Add(NewBranchIDs(branchID1, branchID3))
	arithmeticBranchIDs.Subtract(NewBranchIDs(branchID2, branchID2))

	assert.True(t, NewBranchIDs(branchID1, branchID3).Equal(arithmeticBranchIDs.BranchIDs()))
}

func assertInclusionStates(t *testing.T, branchDAG *BranchDAG, branchIDsMapping map[string]BranchID, expectedInclusionStates map[string]InclusionState) {
	for branchIDStrings, expectedInclusionState := range expectedInclusionStates {
		branchIDs := NewBranchIDs()
		for _, branchString := range strings.Split(branchIDStrings, "+") {
			branchIDs.Add(branchIDsMapping[branchString])
		}

		assert.Equal(t, expectedInclusionState, branchDAG.InclusionState(branchIDs), "%s inclustionState is not %s", branchIDs, expectedInclusionState)
	}
}

func createBranch(t *testing.T, branchDAG *BranchDAG, branchAlias string, parents BranchIDs, conflictIDs ConflictIDs) BranchID {
	var randomTxID utxo.TransactionID
	if err := randomTxID.FromRandomness(); err != nil {
		t.Error(err)
		return BranchID{}
	}

	assert.True(t, branchDAG.CreateBranch(randomTxID, parents, conflictIDs))
	cachedBranch := branchDAG.Branch(randomTxID)
	cachedBranch.Release()

	randomTxID.RegisterAlias(branchAlias)

	return randomTxID
}
