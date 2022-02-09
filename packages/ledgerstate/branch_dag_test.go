package ledgerstate

import (
	"testing"

	"github.com/iotaledger/goshimmer/packages/database"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBranchDAG_RetrieveConflictBranch(t *testing.T) {
	ledgerstate := New(CacheTimeProvider(database.NewCacheTimeProvider(0)))
	defer ledgerstate.Shutdown()

	err := ledgerstate.Prune()
	require.NoError(t, err)

	cachedConflictBranch2, newBranchCreated, err := ledgerstate.CreateConflictBranch(BranchID{2}, NewBranchIDs(MasterBranchID), NewConflictIDs(ConflictID{0}, ConflictID{1}))
	require.NoError(t, err)
	defer cachedConflictBranch2.Release()
	conflictBranch2, err := cachedConflictBranch2.UnwrapConflictBranch()
	require.NoError(t, err)
	assert.True(t, newBranchCreated)
	assert.Equal(t, NewBranchIDs(MasterBranchID), conflictBranch2.Parents())
	assert.Equal(t, ConflictBranchType, conflictBranch2.Type())
	assert.Equal(t, NewConflictIDs(ConflictID{0}, ConflictID{1}), conflictBranch2.Conflicts())

	cachedConflictBranch3, _, err := ledgerstate.CreateConflictBranch(BranchID{3}, NewBranchIDs(conflictBranch2.ID()), NewConflictIDs(ConflictID{0}, ConflictID{1}, ConflictID{2}))
	require.NoError(t, err)
	defer cachedConflictBranch3.Release()
	conflictBranch3, err := cachedConflictBranch3.UnwrapConflictBranch()
	require.NoError(t, err)
	assert.True(t, newBranchCreated)
	assert.Equal(t, NewBranchIDs(conflictBranch2.ID()), conflictBranch3.Parents())
	assert.Equal(t, ConflictBranchType, conflictBranch3.Type())
	assert.Equal(t, NewConflictIDs(ConflictID{0}, ConflictID{1}, ConflictID{2}), conflictBranch3.Conflicts())

	cachedConflictBranch2, newBranchCreated, err = ledgerstate.CreateConflictBranch(BranchID{2}, NewBranchIDs(MasterBranchID), NewConflictIDs(ConflictID{0}, ConflictID{1}, ConflictID{2}))
	require.NoError(t, err)
	defer cachedConflictBranch2.Release()
	conflictBranch2, err = cachedConflictBranch2.UnwrapConflictBranch()
	require.NoError(t, err)
	assert.False(t, newBranchCreated)
	assert.Equal(t, NewConflictIDs(ConflictID{0}, ConflictID{1}, ConflictID{2}), conflictBranch2.Conflicts())

	cachedConflictBranch3, _, err = ledgerstate.CreateConflictBranch(BranchID{4}, NewBranchIDs(cachedConflictBranch2.ID(), cachedConflictBranch3.ID()), NewConflictIDs(ConflictID{3}))
	require.NoError(t, err)
	defer cachedConflictBranch3.Release()
	conflictBranch3, err = cachedConflictBranch3.UnwrapConflictBranch()
	require.NoError(t, err)
	assert.False(t, newBranchCreated)
	assert.Equal(t, NewConflictIDs(ConflictID{3}), conflictBranch3.Conflicts())
}

func TestBranchDAG_ConflictMembers(t *testing.T) {
	ledgerstate := New(CacheTimeProvider(database.NewCacheTimeProvider(0)))
	defer ledgerstate.Shutdown()

	err := ledgerstate.Prune()
	require.NoError(t, err)

	// create initial branches
	cachedBranch2, newBranchCreated, _ := ledgerstate.CreateConflictBranch(BranchID{2}, NewBranchIDs(MasterBranchID), NewConflictIDs(ConflictID{0}))
	defer cachedBranch2.Release()
	branch2 := cachedBranch2.Unwrap()
	assert.True(t, newBranchCreated)
	cachedBranch3, newBranchCreated, _ := ledgerstate.CreateConflictBranch(BranchID{3}, NewBranchIDs(MasterBranchID), NewConflictIDs(ConflictID{0}))
	defer cachedBranch3.Release()
	branch3 := cachedBranch3.Unwrap()
	assert.True(t, newBranchCreated)

	// assert conflict members
	expectedConflictMembers := map[BranchID]struct{}{
		branch2.ID(): {}, branch3.ID(): {},
	}
	actualConflictMembers := map[BranchID]struct{}{}
	ledgerstate.ConflictMembers(ConflictID{0}).Consume(func(conflictMember *ConflictMember) {
		actualConflictMembers[conflictMember.BranchID()] = struct{}{}
	})
	assert.Equal(t, expectedConflictMembers, actualConflictMembers)

	// add branch 4
	cachedBranch4, newBranchCreated, _ := ledgerstate.CreateConflictBranch(BranchID{4}, NewBranchIDs(MasterBranchID), NewConflictIDs(ConflictID{0}))
	defer cachedBranch4.Release()
	branch4 := cachedBranch4.Unwrap()
	assert.True(t, newBranchCreated)

	// branch 4 should now also be part of the conflict set
	expectedConflictMembers = map[BranchID]struct{}{
		branch2.ID(): {}, branch3.ID(): {}, branch4.ID(): {},
	}
	actualConflictMembers = map[BranchID]struct{}{}
	ledgerstate.ConflictMembers(ConflictID{0}).Consume(func(conflictMember *ConflictMember) {
		actualConflictMembers[conflictMember.BranchID()] = struct{}{}
	})
	assert.Equal(t, expectedConflictMembers, actualConflictMembers)
}

func TestBranchDAG_SetBranchConfirmed(t *testing.T) {
	ledgerstate := New(CacheTimeProvider(database.NewCacheTimeProvider(0)))
	defer ledgerstate.Shutdown()

	err := ledgerstate.Prune()
	require.NoError(t, err)

	branchIDs := make(map[string]BranchID)
	branchIDs["Branch2"] = createConflictBranch(t, ledgerstate, "Branch2", NewBranchIDs(MasterBranchID), NewConflictIDs(ConflictID{0}))
	branchIDs["Branch3"] = createConflictBranch(t, ledgerstate, "Branch3", NewBranchIDs(MasterBranchID), NewConflictIDs(ConflictID{0}))
	branchIDs["Branch4"] = createConflictBranch(t, ledgerstate, "Branch4", NewBranchIDs(branchIDs["Branch2"]), NewConflictIDs(ConflictID{1}))
	branchIDs["Branch5"] = createConflictBranch(t, ledgerstate, "Branch5", NewBranchIDs(branchIDs["Branch2"]), NewConflictIDs(ConflictID{1}))
	branchIDs["Branch6"] = createConflictBranch(t, ledgerstate, "Branch6", NewBranchIDs(MasterBranchID), NewConflictIDs(ConflictID{2}))
	branchIDs["Branch7"] = createConflictBranch(t, ledgerstate, "Branch7", NewBranchIDs(MasterBranchID), NewConflictIDs(ConflictID{2}))
	branchIDs["Branch8"] = createConflictBranch(t, ledgerstate, "Branch8", NewBranchIDs(MasterBranchID), NewConflictIDs(ConflictID{2}))
	branchIDs["Branch5+Branch7"] = createAggregatedBranch(ledgerstate, "Branch5+Branch7", NewBranchIDs(branchIDs["Branch5"], branchIDs["Branch7"]))
	branchIDs["Branch2+Branch7"] = createAggregatedBranch(ledgerstate, "Branch2+Branch7", NewBranchIDs(branchIDs["Branch2"], branchIDs["Branch7"]))
	branchIDs["Branch5+Branch8"] = createAggregatedBranch(ledgerstate, "Branch5+Branch8", NewBranchIDs(branchIDs["Branch5"], branchIDs["Branch8"]))

	assert.True(t, ledgerstate.BranchDAG.SetBranchConfirmed(branchIDs["Branch4"]))

	assertInclusionStates(t, ledgerstate, map[BranchID]InclusionState{
		branchIDs["Branch2"]:         Confirmed,
		branchIDs["Branch3"]:         Rejected,
		branchIDs["Branch4"]:         Confirmed,
		branchIDs["Branch5"]:         Rejected,
		branchIDs["Branch6"]:         Pending,
		branchIDs["Branch7"]:         Pending,
		branchIDs["Branch8"]:         Pending,
		branchIDs["Branch5+Branch7"]: Rejected,
		branchIDs["Branch2+Branch7"]: Pending,
		branchIDs["Branch5+Branch8"]: Rejected,
	})

	assert.True(t, ledgerstate.BranchDAG.SetBranchConfirmed(branchIDs["Branch8"]))

	// Spawning a new aggregated branch with Confirmed parents results in Confirmed
	branchIDs["Branch4+Branch8"] = createAggregatedBranch(ledgerstate, "Branch4+Branch8", NewBranchIDs(branchIDs["Branch4"], branchIDs["Branch8"]))

	// Spawning a new aggregated branch with any Rejected parent results in Rejected
	branchIDs["Branch3+Branch8"] = createAggregatedBranch(ledgerstate, "Branch3+Branch8", NewBranchIDs(branchIDs["Branch3"], branchIDs["Branch8"]))

	// Create a new ConflictBranch in an already-decided Conflict Set results in straight Reject
	branchIDs["Branch9"] = createConflictBranch(t, ledgerstate, "Branch9", NewBranchIDs(MasterBranchID), NewConflictIDs(ConflictID{2}))

	assertInclusionStates(t, ledgerstate, map[BranchID]InclusionState{
		branchIDs["Branch2"]:         Confirmed,
		branchIDs["Branch3"]:         Rejected,
		branchIDs["Branch4"]:         Confirmed,
		branchIDs["Branch5"]:         Rejected,
		branchIDs["Branch6"]:         Rejected,
		branchIDs["Branch7"]:         Rejected,
		branchIDs["Branch8"]:         Confirmed,
		branchIDs["Branch5+Branch7"]: Rejected,
		branchIDs["Branch2+Branch7"]: Rejected,
		branchIDs["Branch5+Branch8"]: Rejected,
		branchIDs["Branch4+Branch8"]: Confirmed,
		branchIDs["Branch3+Branch8"]: Rejected,
		branchIDs["Branch9"]:         Rejected,
	})

	// Pruning confirmed branches from aggregation
	branchIDs["Branch2+Branch8"] = createAggregatedBranch(ledgerstate, "Branch2+Branch8", NewBranchIDs(branchIDs["Branch2"], branchIDs["Branch8"]))

	ledgerstate.BranchDAG.Branch(branchIDs["Branch2+Branch8"]).Consume(func(branch Branch) {
		assert.Equal(t, NewBranchIDs(), branch.Parents())
		assert.Equal(t, MasterBranchID, branch.ID())

	})

	// Pruning confirmed branches from aggregation
	branchIDs["Branch2+Branch6"] = createAggregatedBranch(ledgerstate, "Branch2+Branch6", NewBranchIDs(branchIDs["Branch2"], branchIDs["Branch6"]))

	ledgerstate.BranchDAG.Branch(branchIDs["Branch2+Branch6"]).Consume(func(branch Branch) {
		assert.Equal(t, NewBranchIDs(MasterBranchID), branch.Parents())
		assert.Equal(t, branchIDs["Branch6"], branch.ID())
	})

	branchIDs["Branch10"] = createConflictBranch(t, ledgerstate, "Branch10", NewBranchIDs(MasterBranchID), NewConflictIDs(ConflictID{3}))
	branchIDs["Branch11"] = createConflictBranch(t, ledgerstate, "Branch11", NewBranchIDs(MasterBranchID), NewConflictIDs(ConflictID{3}))

	branchIDs["Branch2+Branch7+Branch11"] = createAggregatedBranch(ledgerstate, "Branch2+Branch7+Branch11", NewBranchIDs(branchIDs["Branch2"], branchIDs["Branch7"], branchIDs["Branch11"]))

	ledgerstate.BranchDAG.SetBranchConfirmed(branchIDs["Branch10"])

	ledgerstate.BranchDAG.Branch(branchIDs["Branch2+Branch7+Branch11"]).Consume(func(branch Branch) {
		assert.Equal(t, NewBranchIDs(branchIDs["Branch7"], branchIDs["Branch11"]), branch.Parents())
		assert.Equal(t, branch.Type(), AggregatedBranchType)
	})

	assertInclusionStates(t, ledgerstate, map[BranchID]InclusionState{
		branchIDs["Branch2"]:                  Confirmed,
		branchIDs["Branch3"]:                  Rejected,
		branchIDs["Branch4"]:                  Confirmed,
		branchIDs["Branch5"]:                  Rejected,
		branchIDs["Branch6"]:                  Rejected,
		branchIDs["Branch7"]:                  Rejected,
		branchIDs["Branch8"]:                  Confirmed,
		branchIDs["Branch5+Branch7"]:          Rejected,
		branchIDs["Branch2+Branch7"]:          Rejected,
		branchIDs["Branch5+Branch8"]:          Rejected,
		branchIDs["Branch4+Branch8"]:          Confirmed,
		branchIDs["Branch3+Branch8"]:          Rejected,
		branchIDs["Branch9"]:                  Rejected,
		branchIDs["Branch10"]:                 Confirmed,
		branchIDs["Branch11"]:                 Rejected,
		branchIDs["Branch2+Branch7+Branch11"]: Rejected,
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

func assertInclusionStates(t *testing.T, ledgerstate *Ledgerstate, expectedInclusionStates map[BranchID]InclusionState) {
	for branchID, expectedInclusionState := range expectedInclusionStates {
		assert.Equal(t, expectedInclusionState, ledgerstate.BranchDAG.InclusionState(branchID), "%s inclustionState is not %s", branchID, expectedInclusionState)
	}
}

func createConflictBranch(t *testing.T, ledgerstate *Ledgerstate, branchAlias string, parents BranchIDs, conflictIDs ConflictIDs) BranchID {
	cachedBranch, _, err := ledgerstate.CreateConflictBranch(BranchIDFromRandomness(), parents, conflictIDs)
	require.NoError(t, err)
	cachedBranch.Release()

	RegisterBranchIDAlias(cachedBranch.ID(), branchAlias)

	return cachedBranch.ID()
}

func createAggregatedBranch(ledgerstate *Ledgerstate, branchAlias string, parents BranchIDs) BranchID {
	branchID := ledgerstate.AggregateConflictBranchesID(parents)
	RegisterBranchIDAlias(branchID, branchAlias)

	return branchID
}
