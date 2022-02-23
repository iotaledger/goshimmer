package ledgerstate

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/iotaledger/goshimmer/packages/database"
)

func TestBranchDAG_RetrieveConflictBranch(t *testing.T) {
	ledgerstate := New(CacheTimeProvider(database.NewCacheTimeProvider(0)))
	defer ledgerstate.Shutdown()

	err := ledgerstate.Prune()
	require.NoError(t, err)

	cachedConflictBranch2, newBranchCreated, err := ledgerstate.CreateConflictBranch(BranchID{2}, NewBranchIDs(MasterBranchID), NewConflictIDs(ConflictID{0}, ConflictID{1}))
	require.NoError(t, err)
	defer cachedConflictBranch2.Release()
	conflictBranch2 := cachedConflictBranch2.Unwrap()
	assert.True(t, newBranchCreated)
	assert.Equal(t, NewBranchIDs(MasterBranchID), conflictBranch2.Parents())
	assert.Equal(t, NewConflictIDs(ConflictID{0}, ConflictID{1}), conflictBranch2.Conflicts())

	cachedConflictBranch3, _, err := ledgerstate.CreateConflictBranch(BranchID{3}, NewBranchIDs(conflictBranch2.ID()), NewConflictIDs(ConflictID{0}, ConflictID{1}, ConflictID{2}))
	require.NoError(t, err)
	defer cachedConflictBranch3.Release()
	conflictBranch3 := cachedConflictBranch3.Unwrap()
	assert.True(t, newBranchCreated)
	assert.Equal(t, NewBranchIDs(conflictBranch2.ID()), conflictBranch3.Parents())
	assert.Equal(t, NewConflictIDs(ConflictID{0}, ConflictID{1}, ConflictID{2}), conflictBranch3.Conflicts())

	cachedConflictBranch2, newBranchCreated, err = ledgerstate.CreateConflictBranch(BranchID{2}, NewBranchIDs(MasterBranchID), NewConflictIDs(ConflictID{0}, ConflictID{1}, ConflictID{2}))
	require.NoError(t, err)
	defer cachedConflictBranch2.Release()
	conflictBranch2 = cachedConflictBranch2.Unwrap()
	assert.False(t, newBranchCreated)
	assert.Equal(t, NewConflictIDs(ConflictID{0}, ConflictID{1}, ConflictID{2}), conflictBranch2.Conflicts())

	cachedConflictBranch4, newBranchCreated, err := ledgerstate.CreateConflictBranch(BranchID{4}, NewBranchIDs(conflictBranch3.ID(), conflictBranch3.ID()), NewConflictIDs(ConflictID{3}))
	require.NoError(t, err)
	defer cachedConflictBranch4.Release()
	conflictBranch4 := cachedConflictBranch4.Unwrap()
	require.NoError(t, err)
	assert.True(t, newBranchCreated)
	assert.Equal(t, NewConflictIDs(ConflictID{3}), conflictBranch4.Conflicts())
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

	assert.True(t, ledgerstate.BranchDAG.SetBranchConfirmed(branchIDs["Branch4"]))

	assertInclusionStates(t, ledgerstate, branchIDs, map[string]InclusionState{
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

	assert.True(t, ledgerstate.BranchDAG.SetBranchConfirmed(branchIDs["Branch8"]))

	// Create a new Branch in an already-decided Conflict Set results in straight Reject
	branchIDs["Branch9"] = createConflictBranch(t, ledgerstate, "Branch9", NewBranchIDs(MasterBranchID), NewConflictIDs(ConflictID{2}))

	assertInclusionStates(t, ledgerstate, branchIDs, map[string]InclusionState{
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

	branchIDs["Branch10"] = createConflictBranch(t, ledgerstate, "Branch10", NewBranchIDs(MasterBranchID), NewConflictIDs(ConflictID{3}))
	branchIDs["Branch11"] = createConflictBranch(t, ledgerstate, "Branch11", NewBranchIDs(MasterBranchID), NewConflictIDs(ConflictID{3}))

	ledgerstate.BranchDAG.SetBranchConfirmed(branchIDs["Branch10"])

	assertInclusionStates(t, ledgerstate, branchIDs, map[string]InclusionState{
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

func assertInclusionStates(t *testing.T, ledgerstate *Ledgerstate, branchIDsMapping map[string]BranchID, expectedInclusionStates map[string]InclusionState) {
	for branchIDStrings, expectedInclusionState := range expectedInclusionStates {
		branchIDs := NewBranchIDs()
		for _, branchString := range strings.Split(branchIDStrings, "+") {
			branchIDs.Add(branchIDsMapping[branchString])
		}

		assert.Equal(t, expectedInclusionState, ledgerstate.BranchDAG.InclusionState(branchIDs), "%s inclustionState is not %s", branchIDs, expectedInclusionState)
	}
}

func createConflictBranch(t *testing.T, ledgerstate *Ledgerstate, branchAlias string, parents BranchIDs, conflictIDs ConflictIDs) BranchID {
	branchID := BranchIDFromRandomness()
	cachedBranch, _, err := ledgerstate.CreateConflictBranch(branchID, parents, conflictIDs)
	require.NoError(t, err)
	cachedBranch.Release()

	RegisterBranchIDAlias(branchID, branchAlias)

	return branchID
}
