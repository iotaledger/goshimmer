package conflictdag

import (
	"strings"
	"testing"

	"github.com/iotaledger/hive.go/generics/set"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConflictDAG_RetrieveBranch(t *testing.T) {
	branchDAG := New[MockedConflictID, MockedConflictSetID]()
	defer branchDAG.Shutdown()

	var (
		branchID2   MockedConflictID
		branchID3   MockedConflictID
		branchID4   MockedConflictID
		conflictID0 MockedConflictSetID
		conflictID1 MockedConflictSetID
		conflictID2 MockedConflictSetID
		conflictID3 MockedConflictSetID
	)
	require.NoError(t, branchID2.FromRandomness())
	require.NoError(t, branchID3.FromRandomness())
	require.NoError(t, branchID4.FromRandomness())
	require.NoError(t, conflictID0.FromRandomness())
	require.NoError(t, conflictID1.FromRandomness())
	require.NoError(t, conflictID2.FromRandomness())
	require.NoError(t, conflictID3.FromRandomness())

	assert.True(t, branchDAG.CreateConflict(branchID2, set.NewAdvancedSet(MockedConflictID{}), set.NewAdvancedSet(conflictID0, conflictID1)))
	cachedBranch2 := branchDAG.Storage.CachedConflict(branchID2)
	defer cachedBranch2.Release()
	Branch2, exists := cachedBranch2.Unwrap()
	require.True(t, exists)
	assert.Equal(t, set.NewAdvancedSet(MockedConflictID{}), Branch2.Parents())
	assert.True(t, set.NewAdvancedSet(conflictID0, conflictID1).Equal(Branch2.ConflictIDs()))

	assert.True(t, branchDAG.CreateConflict(branchID3, set.NewAdvancedSet(Branch2.ID()), set.NewAdvancedSet(conflictID0, conflictID1, conflictID2)))
	cachedBranch3 := branchDAG.Storage.CachedConflict(branchID3)
	defer cachedBranch3.Release()
	Branch3, exists := cachedBranch3.Unwrap()
	require.True(t, exists)

	assert.Equal(t, set.NewAdvancedSet(Branch2.ID()), Branch3.Parents())
	assert.Equal(t, set.NewAdvancedSet(conflictID0, conflictID1, conflictID2), Branch3.ConflictIDs())

	assert.False(t, branchDAG.CreateConflict(branchID2, set.NewAdvancedSet(MockedConflictID{}), set.NewAdvancedSet(conflictID0, conflictID1, conflictID2)))
	assert.True(t, branchDAG.UpdateConflictingResources(branchID2, set.NewAdvancedSet(conflictID0, conflictID1, conflictID2)))
	cachedBranch2 = branchDAG.Storage.CachedConflict(branchID2)
	defer cachedBranch2.Release()
	Branch2, exists = cachedBranch2.Unwrap()
	require.True(t, exists)

	assert.Equal(t, set.NewAdvancedSet(conflictID0, conflictID1, conflictID2), Branch2.ConflictIDs())

	assert.True(t, branchDAG.CreateConflict(branchID4, set.NewAdvancedSet(Branch3.ID(), Branch3.ID()), set.NewAdvancedSet(conflictID3)))
	cachedBranch4 := branchDAG.Storage.CachedConflict(branchID4)
	defer cachedBranch4.Release()
	Branch4, exists := cachedBranch4.Unwrap()
	require.True(t, exists)
	assert.Equal(t, set.NewAdvancedSet(conflictID3), Branch4.ConflictIDs())
}

func TestConflictDAG_ConflictMembers(t *testing.T) {
	branchDAG := New[MockedConflictID, MockedConflictSetID]()
	defer branchDAG.Shutdown()

	var (
		branchID2   MockedConflictID
		branchID3   MockedConflictID
		branchID4   MockedConflictID
		conflictID0 MockedConflictSetID
	)
	require.NoError(t, branchID2.FromRandomness())
	require.NoError(t, branchID3.FromRandomness())
	require.NoError(t, branchID4.FromRandomness())
	require.NoError(t, conflictID0.FromRandomness())

	// create initial branches
	assert.True(t, branchDAG.CreateConflict(branchID2, set.NewAdvancedSet(MockedConflictID{}), set.NewAdvancedSet(conflictID0)))
	cachedBranch2 := branchDAG.Storage.CachedConflict(branchID2)
	defer cachedBranch2.Release()
	branch2, exists := cachedBranch2.Unwrap()
	assert.True(t, exists)

	assert.True(t, branchDAG.CreateConflict(branchID3, set.NewAdvancedSet(MockedConflictID{}), set.NewAdvancedSet(conflictID0)))
	cachedBranch3 := branchDAG.Storage.CachedConflict(branchID3)
	defer cachedBranch3.Release()
	branch3, exists := cachedBranch3.Unwrap()
	assert.True(t, exists)

	// assert conflict members
	expectedConflictMembers := map[MockedConflictID]struct{}{
		branch2.ID(): {}, branch3.ID(): {},
	}
	actualConflictMembers := map[MockedConflictID]struct{}{}
	branchDAG.Storage.CachedConflictMembers(conflictID0).Consume(func(conflictMember *ConflictMember[MockedConflictID, MockedConflictSetID]) {
		actualConflictMembers[conflictMember.BranchID()] = struct{}{}
	})
	assert.Equal(t, expectedConflictMembers, actualConflictMembers)

	// add branch 4
	assert.True(t, branchDAG.CreateConflict(branchID4, set.NewAdvancedSet(MockedConflictID{}), set.NewAdvancedSet(conflictID0)))
	cachedBranch4 := branchDAG.Storage.CachedConflict(branchID4)
	defer cachedBranch4.Release()
	branch4, exists := cachedBranch4.Unwrap()
	assert.True(t, exists)

	// branch 4 should now also be part of the conflict set
	expectedConflictMembers = map[MockedConflictID]struct{}{
		branch2.ID(): {}, branch3.ID(): {}, branch4.ID(): {},
	}
	actualConflictMembers = map[MockedConflictID]struct{}{}
	branchDAG.Storage.CachedConflictMembers(conflictID0).Consume(func(conflictMember *ConflictMember[MockedConflictID, MockedConflictSetID]) {
		actualConflictMembers[conflictMember.BranchID()] = struct{}{}
	})
	assert.Equal(t, expectedConflictMembers, actualConflictMembers)
}

func TestConflictDAG_SetBranchConfirmed(t *testing.T) {
	branchDAG := New[MockedConflictID, MockedConflictSetID]()
	defer branchDAG.Shutdown()

	var (
		conflictID0 MockedConflictSetID
		conflictID1 MockedConflictSetID
		conflictID2 MockedConflictSetID
		conflictID3 MockedConflictSetID
	)
	require.NoError(t, conflictID0.FromRandomness())
	require.NoError(t, conflictID1.FromRandomness())
	require.NoError(t, conflictID2.FromRandomness())
	require.NoError(t, conflictID3.FromRandomness())

	branchIDs := make(map[string]MockedConflictID)
	branchIDs["Branch2"] = createBranch(t, branchDAG, "Branch2", set.NewAdvancedSet(MockedConflictID{}), set.NewAdvancedSet(conflictID0))
	branchIDs["Branch3"] = createBranch(t, branchDAG, "Branch3", set.NewAdvancedSet(MockedConflictID{}), set.NewAdvancedSet(conflictID0))
	branchIDs["Branch4"] = createBranch(t, branchDAG, "Branch4", set.NewAdvancedSet(branchIDs["Branch2"]), set.NewAdvancedSet(conflictID1))
	branchIDs["Branch5"] = createBranch(t, branchDAG, "Branch5", set.NewAdvancedSet(branchIDs["Branch2"]), set.NewAdvancedSet(conflictID1))
	branchIDs["Branch6"] = createBranch(t, branchDAG, "Branch6", set.NewAdvancedSet(MockedConflictID{}), set.NewAdvancedSet(conflictID2))
	branchIDs["Branch7"] = createBranch(t, branchDAG, "Branch7", set.NewAdvancedSet(MockedConflictID{}), set.NewAdvancedSet(conflictID2))
	branchIDs["Branch8"] = createBranch(t, branchDAG, "Branch8", set.NewAdvancedSet(MockedConflictID{}), set.NewAdvancedSet(conflictID2))

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

	// Create a new Conflict in an already-decided Conflict Set results in straight Reject
	branchIDs["Branch9"] = createBranch(t, branchDAG, "Branch9", set.NewAdvancedSet(MockedConflictID{}), set.NewAdvancedSet(conflictID2))

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

	branchIDs["Branch10"] = createBranch(t, branchDAG, "Branch10", set.NewAdvancedSet(MockedConflictID{}), set.NewAdvancedSet(conflictID3))
	branchIDs["Branch11"] = createBranch(t, branchDAG, "Branch11", set.NewAdvancedSet(MockedConflictID{}), set.NewAdvancedSet(conflictID3))

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

func assertInclusionStates[ConflictT set.AdvancedSetElement[ConflictT], ConflictSetT set.AdvancedSetElement[ConflictSetT]](t *testing.T, branchDAG *ConflictDAG[ConflictT, ConflictSetT], branchIDsMapping map[string]ConflictT, expectedInclusionStates map[string]InclusionState) {
	for branchIDStrings, expectedInclusionState := range expectedInclusionStates {
		branchIDs := set.NewAdvancedSet[ConflictT]()
		for _, branchString := range strings.Split(branchIDStrings, "+") {
			branchIDs.Add(branchIDsMapping[branchString])
		}

		assert.Equal(t, expectedInclusionState, branchDAG.InclusionState(branchIDs), "%s inclustionState is not %s", branchIDs, expectedInclusionState)
	}
}

func createBranch(t *testing.T, branchDAG *ConflictDAG[MockedConflictID, MockedConflictSetID], branchAlias string, parents *set.AdvancedSet[MockedConflictID], conflictIDs *set.AdvancedSet[MockedConflictSetID]) MockedConflictID {
	var randomBranchID MockedConflictID
	if err := randomBranchID.FromRandomness(); err != nil {
		t.Error(err)
		return MockedConflictID{}
	}

	assert.True(t, branchDAG.CreateConflict(randomBranchID, parents, conflictIDs))
	cachedBranch := branchDAG.Storage.CachedConflict(randomBranchID)
	cachedBranch.Release()

	// randomBranchID.RegisterAlias(branchAlias)

	return randomBranchID
}
