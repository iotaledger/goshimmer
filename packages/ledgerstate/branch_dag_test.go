package ledgerstate

import (
	"testing"

	"github.com/iotaledger/goshimmer/packages/database"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBranchDAG_RetrieveConflictBranch(t *testing.T) {
	ledgerState := New(CacheTimeProvider(database.NewCacheTimeProvider(0)))
	defer ledgerState.Shutdown()

	err := ledgerState.Prune()
	require.NoError(t, err)

	cachedConflictBranch2, newBranchCreated, err := ledgerState.CreateConflictBranch(BranchID{2}, NewBranchIDs(MasterBranchID), NewConflictIDs(ConflictID{0}, ConflictID{1}))
	require.NoError(t, err)
	defer cachedConflictBranch2.Release()
	conflictBranch2 := cachedConflictBranch2.Unwrap()
	assert.True(t, newBranchCreated)
	assert.Equal(t, NewBranchIDs(MasterBranchID), conflictBranch2.Parents())
	assert.Equal(t, NewConflictIDs(ConflictID{0}, ConflictID{1}), conflictBranch2.Conflicts())

	cachedConflictBranch3, _, err := ledgerState.CreateConflictBranch(BranchID{3}, NewBranchIDs(conflictBranch2.ID()), NewConflictIDs(ConflictID{0}, ConflictID{1}, ConflictID{2}))
	require.NoError(t, err)
	defer cachedConflictBranch3.Release()
	conflictBranch3 := cachedConflictBranch3.Unwrap()
	assert.True(t, newBranchCreated)
	assert.Equal(t, NewBranchIDs(conflictBranch2.ID()), conflictBranch3.Parents())
	assert.Equal(t, NewConflictIDs(ConflictID{0}, ConflictID{1}, ConflictID{2}), conflictBranch3.Conflicts())

	cachedConflictBranch2, newBranchCreated, err = ledgerState.CreateConflictBranch(BranchID{2}, NewBranchIDs(MasterBranchID), NewConflictIDs(ConflictID{0}, ConflictID{1}, ConflictID{2}))
	require.NoError(t, err)
	defer cachedConflictBranch2.Release()
	conflictBranch2 = cachedConflictBranch2.Unwrap()
	assert.False(t, newBranchCreated)
	assert.Equal(t, NewConflictIDs(ConflictID{0}, ConflictID{1}, ConflictID{2}), conflictBranch2.Conflicts())

	cachedConflictBranch3, _, err = ledgerState.CreateConflictBranch(BranchID{4}, NewBranchIDs(conflictBranch2.ID(), conflictBranch3.ID()), NewConflictIDs(ConflictID{3}))
	require.NoError(t, err)
	defer cachedConflictBranch3.Release()
	conflictBranch3 = cachedConflictBranch3.Unwrap()
	assert.False(t, newBranchCreated)
	assert.Equal(t, NewConflictIDs(ConflictID{3}), conflictBranch3.Conflicts())
}

func TestBranchDAG_ConflictMembers(t *testing.T) {
	ledgerState := New(CacheTimeProvider(database.NewCacheTimeProvider(0)))
	defer ledgerState.Shutdown()

	err := ledgerState.Prune()
	require.NoError(t, err)

	// create initial branches
	cachedBranch2, newBranchCreated, _ := ledgerState.CreateConflictBranch(BranchID{2}, NewBranchIDs(MasterBranchID), NewConflictIDs(ConflictID{0}))
	defer cachedBranch2.Release()
	branch2 := cachedBranch2.Unwrap()
	assert.True(t, newBranchCreated)
	cachedBranch3, newBranchCreated, _ := ledgerState.CreateConflictBranch(BranchID{3}, NewBranchIDs(MasterBranchID), NewConflictIDs(ConflictID{0}))
	defer cachedBranch3.Release()
	branch3 := cachedBranch3.Unwrap()
	assert.True(t, newBranchCreated)

	// assert conflict members
	expectedConflictMembers := map[BranchID]struct{}{
		branch2.ID(): {}, branch3.ID(): {},
	}
	actualConflictMembers := map[BranchID]struct{}{}
	ledgerState.ConflictMembers(ConflictID{0}).Consume(func(conflictMember *ConflictMember) {
		actualConflictMembers[conflictMember.BranchID()] = struct{}{}
	})
	assert.Equal(t, expectedConflictMembers, actualConflictMembers)

	// add branch 4
	cachedBranch4, newBranchCreated, _ := ledgerState.CreateConflictBranch(BranchID{4}, NewBranchIDs(MasterBranchID), NewConflictIDs(ConflictID{0}))
	defer cachedBranch4.Release()
	branch4 := cachedBranch4.Unwrap()
	assert.True(t, newBranchCreated)

	// branch 4 should now also be part of the conflict set
	expectedConflictMembers = map[BranchID]struct{}{
		branch2.ID(): {}, branch3.ID(): {}, branch4.ID(): {},
	}
	actualConflictMembers = map[BranchID]struct{}{}
	ledgerState.ConflictMembers(ConflictID{0}).Consume(func(conflictMember *ConflictMember) {
		actualConflictMembers[conflictMember.BranchID()] = struct{}{}
	})
	assert.Equal(t, expectedConflictMembers, actualConflictMembers)
}

func TestBranchDAG_SetBranchConfirmed(t *testing.T) {
	ledgerState := New(CacheTimeProvider(database.NewCacheTimeProvider(0)))
	defer ledgerState.Shutdown()

	err := ledgerState.Prune()
	require.NoError(t, err)

	branchIDs := make(map[string]BranchID)
	branchIDs["Branch2"] = createConflictBranch(t, ledgerState, "Branch2", NewBranchIDs(MasterBranchID), NewConflictIDs(ConflictID{0}))
	branchIDs["Branch3"] = createConflictBranch(t, ledgerState, "Branch3", NewBranchIDs(MasterBranchID), NewConflictIDs(ConflictID{0}))
	branchIDs["Branch4"] = createConflictBranch(t, ledgerState, "Branch4", NewBranchIDs(branchIDs["Branch2"]), NewConflictIDs(ConflictID{1}))
	branchIDs["Branch5"] = createConflictBranch(t, ledgerState, "Branch5", NewBranchIDs(branchIDs["Branch2"]), NewConflictIDs(ConflictID{1}))
	branchIDs["Branch6"] = createConflictBranch(t, ledgerState, "Branch6", NewBranchIDs(MasterBranchID), NewConflictIDs(ConflictID{2}))
	branchIDs["Branch7"] = createConflictBranch(t, ledgerState, "Branch7", NewBranchIDs(MasterBranchID), NewConflictIDs(ConflictID{2}))
	branchIDs["Branch8"] = createConflictBranch(t, ledgerState, "Branch8", NewBranchIDs(MasterBranchID), NewConflictIDs(ConflictID{2}))

	assert.True(t, ledgerState.BranchDAG.SetBranchConfirmed(branchIDs["Branch4"]))

	assertInclusionStates(t, ledgerState, map[BranchID]InclusionState{
		branchIDs["Branch2"]: Confirmed,
		branchIDs["Branch3"]: Rejected,
		branchIDs["Branch4"]: Confirmed,
		branchIDs["Branch5"]: Rejected,
		branchIDs["Branch6"]: Pending,
		branchIDs["Branch7"]: Pending,
		branchIDs["Branch8"]: Pending,
	})

	assert.True(t, ledgerState.BranchDAG.SetBranchConfirmed(branchIDs["Branch8"]))

	// Create a new Branch in an already-decided Conflict Set results in straight Reject
	branchIDs["Branch9"] = createConflictBranch(t, ledgerState, "Branch9", NewBranchIDs(MasterBranchID), NewConflictIDs(ConflictID{2}))

	assertInclusionStates(t, ledgerState, map[BranchID]InclusionState{
		branchIDs["Branch2"]: Confirmed,
		branchIDs["Branch3"]: Rejected,
		branchIDs["Branch4"]: Confirmed,
		branchIDs["Branch5"]: Rejected,
		branchIDs["Branch6"]: Rejected,
		branchIDs["Branch7"]: Rejected,
		branchIDs["Branch8"]: Confirmed,
		branchIDs["Branch9"]: Rejected,
	})

	ledgerState.BranchDAG.Branch(branchIDs["Branch2+Branch8"]).Consume(func(branch *Branch) {
		assert.Equal(t, NewBranchIDs(), branch.Parents())
		assert.Equal(t, MasterBranchID, branch.ID())

	})

	ledgerState.BranchDAG.Branch(branchIDs["Branch2+Branch6"]).Consume(func(branch *Branch) {
		assert.Equal(t, NewBranchIDs(MasterBranchID), branch.Parents())
		assert.Equal(t, branchIDs["Branch6"], branch.ID())
	})

	branchIDs["Branch10"] = createConflictBranch(t, ledgerState, "Branch10", NewBranchIDs(MasterBranchID), NewConflictIDs(ConflictID{3}))
	branchIDs["Branch11"] = createConflictBranch(t, ledgerState, "Branch11", NewBranchIDs(MasterBranchID), NewConflictIDs(ConflictID{3}))

	ledgerState.BranchDAG.SetBranchConfirmed(branchIDs["Branch10"])

	ledgerState.BranchDAG.Branch(branchIDs["Branch2+Branch7+Branch11"]).Consume(func(branch *Branch) {
		assert.Equal(t, NewBranchIDs(branchIDs["Branch7"], branchIDs["Branch11"]), branch.Parents())
	})

	assertInclusionStates(t, ledgerState, map[BranchID]InclusionState{
		branchIDs["Branch2"]:  Confirmed,
		branchIDs["Branch3"]:  Rejected,
		branchIDs["Branch4"]:  Confirmed,
		branchIDs["Branch5"]:  Rejected,
		branchIDs["Branch6"]:  Rejected,
		branchIDs["Branch7"]:  Rejected,
		branchIDs["Branch8"]:  Confirmed,
		branchIDs["Branch9"]:  Rejected,
		branchIDs["Branch10"]: Confirmed,
		branchIDs["Branch11"]: Rejected,
	})

}

func TestArithmeticBranchIDs_Add(t *testing.T) {
	branchID1 := BranchIDFromRandomness()
	branchID2 := BranchIDFromRandomness()
	branchID3 := BranchIDFromRandomness()

	RegisterBranchIDAlias(branchID1, "branchID1")
	RegisterBranchIDAlias(branchID2, "branchID2")
	RegisterBranchIDAlias(branchID3, "branchID3")

	arithmeticBranchIDs := NewArithmeticBranchIDs()

	arithmeticBranchIDs.Add(NewBranchIDs(branchID1, branchID2))
	arithmeticBranchIDs.Add(NewBranchIDs(branchID1, branchID3))
	arithmeticBranchIDs.Subtract(NewBranchIDs(branchID2, branchID2))

	assert.Equal(t, NewBranchIDs(branchID1, branchID3), arithmeticBranchIDs.BranchIDs())
}

func assertInclusionStates(t *testing.T, ledgerState *Ledgerstate, expectedInclusionStates map[BranchID]InclusionState) {
	for branchID, expectedInclusionState := range expectedInclusionStates {
		assert.Equal(t, expectedInclusionState, ledgerState.BranchDAG.InclusionState(branchID), "%s inclusionState is not %s", branchID, expectedInclusionState)
	}
}

func createConflictBranch(t *testing.T, ledgerState *Ledgerstate, branchAlias string, parents BranchIDs, conflictIDs ConflictIDs) BranchID {
	cachedBranch, _, err := ledgerState.CreateConflictBranch(BranchIDFromRandomness(), parents, conflictIDs)
	require.NoError(t, err)
	branch := cachedBranch.Unwrap()
	cachedBranch.Release()

	RegisterBranchIDAlias(branch.ID(), branchAlias)

	return branch.ID()
}
