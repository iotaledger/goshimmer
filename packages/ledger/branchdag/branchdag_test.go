package branchdag

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBranchDAG_RetrieveBranch(t *testing.T) {
	branchDAG := New()
	defer branchDAG.Shutdown()

	var (
		branchID2   BranchID
		branchID3   BranchID
		branchID4   BranchID
		conflictID0 ConflictID
		conflictID1 ConflictID
		conflictID2 ConflictID
		conflictID3 ConflictID
	)
	require.NoError(t, branchID2.FromRandomness())
	require.NoError(t, branchID3.FromRandomness())
	require.NoError(t, branchID4.FromRandomness())
	require.NoError(t, conflictID0.TransactionID.FromRandomness())
	require.NoError(t, conflictID1.TransactionID.FromRandomness())
	require.NoError(t, conflictID2.TransactionID.FromRandomness())
	require.NoError(t, conflictID3.TransactionID.FromRandomness())

	assert.True(t, branchDAG.CreateBranch(branchID2, NewBranchIDs(MasterBranchID), NewConflictIDs(conflictID0, conflictID1)))
	cachedBranch2 := branchDAG.Storage.CachedBranch(branchID2)
	defer cachedBranch2.Release()
	Branch2, exists := cachedBranch2.Unwrap()
	require.True(t, exists)
	assert.Equal(t, NewBranchIDs(MasterBranchID), Branch2.Parents())
	assert.True(t, NewConflictIDs(conflictID0, conflictID1).Equal(Branch2.ConflictIDs()))

	assert.True(t, branchDAG.CreateBranch(branchID3, NewBranchIDs(Branch2.ID()), NewConflictIDs(conflictID0, conflictID1, conflictID2)))
	cachedBranch3 := branchDAG.Storage.CachedBranch(branchID3)
	defer cachedBranch3.Release()
	Branch3, exists := cachedBranch3.Unwrap()
	require.True(t, exists)

	assert.Equal(t, NewBranchIDs(Branch2.ID()), Branch3.Parents())
	assert.Equal(t, NewConflictIDs(conflictID0, conflictID1, conflictID2), Branch3.ConflictIDs())

	assert.False(t, branchDAG.CreateBranch(branchID2, NewBranchIDs(MasterBranchID), NewConflictIDs(conflictID0, conflictID1, conflictID2)))
	assert.True(t, branchDAG.AddBranchToConflicts(branchID2, NewConflictIDs(conflictID0, conflictID1, conflictID2)))
	cachedBranch2 = branchDAG.Storage.CachedBranch(branchID2)
	defer cachedBranch2.Release()
	Branch2, exists = cachedBranch2.Unwrap()
	require.True(t, exists)

	assert.Equal(t, NewConflictIDs(conflictID0, conflictID1, conflictID2), Branch2.ConflictIDs())

	assert.True(t, branchDAG.CreateBranch(branchID4, NewBranchIDs(Branch3.ID(), Branch3.ID()), NewConflictIDs(conflictID3)))
	cachedBranch4 := branchDAG.Storage.CachedBranch(branchID4)
	defer cachedBranch4.Release()
	Branch4, exists := cachedBranch4.Unwrap()
	require.True(t, exists)
	assert.Equal(t, NewConflictIDs(conflictID3), Branch4.ConflictIDs())
}

func TestBranchDAG_ConflictMembers(t *testing.T) {
	branchDAG := New()
	defer branchDAG.Shutdown()

	var (
		branchID2   BranchID
		branchID3   BranchID
		branchID4   BranchID
		conflictID0 ConflictID
	)
	require.NoError(t, branchID2.FromRandomness())
	require.NoError(t, branchID3.FromRandomness())
	require.NoError(t, branchID4.FromRandomness())
	require.NoError(t, conflictID0.TransactionID.FromRandomness())

	return

	// create initial branches
	assert.True(t, branchDAG.CreateBranch(branchID2, NewBranchIDs(MasterBranchID), NewConflictIDs(conflictID0)))
	cachedBranch2 := branchDAG.Storage.CachedBranch(branchID2)
	defer cachedBranch2.Release()
	branch2, exists := cachedBranch2.Unwrap()
	assert.True(t, exists)

	return

	assert.True(t, branchDAG.CreateBranch(branchID3, NewBranchIDs(MasterBranchID), NewConflictIDs(conflictID0)))
	cachedBranch3 := branchDAG.Storage.CachedBranch(branchID3)
	defer cachedBranch3.Release()
	branch3, exists := cachedBranch3.Unwrap()
	assert.True(t, exists)

	// assert conflict members
	expectedConflictMembers := map[BranchID]struct{}{
		branch2.ID(): {}, branch3.ID(): {},
	}
	actualConflictMembers := map[BranchID]struct{}{}
	branchDAG.Storage.CachedConflictMembers(conflictID0).Consume(func(conflictMember *ConflictMember) {
		actualConflictMembers[conflictMember.BranchID()] = struct{}{}
	})
	assert.Equal(t, expectedConflictMembers, actualConflictMembers)

	// add branch 4
	assert.True(t, branchDAG.CreateBranch(branchID4, NewBranchIDs(MasterBranchID), NewConflictIDs(conflictID0)))
	cachedBranch4 := branchDAG.Storage.CachedBranch(branchID4)
	defer cachedBranch4.Release()
	branch4, exists := cachedBranch4.Unwrap()
	assert.True(t, exists)

	// branch 4 should now also be part of the conflict set
	expectedConflictMembers = map[BranchID]struct{}{
		branch2.ID(): {}, branch3.ID(): {}, branch4.ID(): {},
	}
	actualConflictMembers = map[BranchID]struct{}{}
	branchDAG.Storage.CachedConflictMembers(conflictID0).Consume(func(conflictMember *ConflictMember) {
		actualConflictMembers[conflictMember.BranchID()] = struct{}{}
	})
	assert.Equal(t, expectedConflictMembers, actualConflictMembers)
}

func TestBranchDAG_SetBranchConfirmed(t *testing.T) {
	branchDAG := New()
	defer branchDAG.Shutdown()

	var (
		conflictID0 ConflictID
		conflictID1 ConflictID
		conflictID2 ConflictID
		conflictID3 ConflictID
	)
	require.NoError(t, conflictID0.TransactionID.FromRandomness())
	require.NoError(t, conflictID1.TransactionID.FromRandomness())
	require.NoError(t, conflictID2.TransactionID.FromRandomness())
	require.NoError(t, conflictID3.TransactionID.FromRandomness())

	branchIDs := make(map[string]BranchID)
	branchIDs["Branch2"] = createBranch(t, branchDAG, "Branch2", NewBranchIDs(MasterBranchID), NewConflictIDs(conflictID0))
	branchIDs["Branch3"] = createBranch(t, branchDAG, "Branch3", NewBranchIDs(MasterBranchID), NewConflictIDs(conflictID0))
	branchIDs["Branch4"] = createBranch(t, branchDAG, "Branch4", NewBranchIDs(branchIDs["Branch2"]), NewConflictIDs(conflictID1))
	branchIDs["Branch5"] = createBranch(t, branchDAG, "Branch5", NewBranchIDs(branchIDs["Branch2"]), NewConflictIDs(conflictID1))
	branchIDs["Branch6"] = createBranch(t, branchDAG, "Branch6", NewBranchIDs(MasterBranchID), NewConflictIDs(conflictID2))
	branchIDs["Branch7"] = createBranch(t, branchDAG, "Branch7", NewBranchIDs(MasterBranchID), NewConflictIDs(conflictID2))
	branchIDs["Branch8"] = createBranch(t, branchDAG, "Branch8", NewBranchIDs(MasterBranchID), NewConflictIDs(conflictID2))

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
	branchIDs["Branch9"] = createBranch(t, branchDAG, "Branch9", NewBranchIDs(MasterBranchID), NewConflictIDs(conflictID2))

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

	branchIDs["Branch10"] = createBranch(t, branchDAG, "Branch10", NewBranchIDs(MasterBranchID), NewConflictIDs(conflictID3))
	branchIDs["Branch11"] = createBranch(t, branchDAG, "Branch11", NewBranchIDs(MasterBranchID), NewConflictIDs(conflictID3))

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
	var randomBranchID BranchID
	if err := randomBranchID.FromRandomness(); err != nil {
		t.Error(err)
		return BranchID{}
	}

	assert.True(t, branchDAG.CreateBranch(randomBranchID, parents, conflictIDs))
	cachedBranch := branchDAG.Storage.CachedBranch(randomBranchID)
	cachedBranch.Release()

	randomBranchID.RegisterAlias(branchAlias)

	return randomBranchID
}
