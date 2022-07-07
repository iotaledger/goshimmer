package conflictdag

import (
	"strings"
	"testing"

	"github.com/iotaledger/hive.go/generics/set"
	"github.com/iotaledger/hive.go/types"
	"github.com/iotaledger/hive.go/types/confirmation"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConflictDAG_RetrieveBranch(t *testing.T) {
	branchDAG := New[types.Identifier, types.Identifier]()
	defer branchDAG.Shutdown()

	var (
		branchID2   types.Identifier
		branchID3   types.Identifier
		branchID4   types.Identifier
		conflictID0 types.Identifier
		conflictID1 types.Identifier
		conflictID2 types.Identifier
		conflictID3 types.Identifier
	)
	require.NoError(t, branchID2.FromRandomness())
	require.NoError(t, branchID3.FromRandomness())
	require.NoError(t, branchID4.FromRandomness())
	require.NoError(t, conflictID0.FromRandomness())
	require.NoError(t, conflictID1.FromRandomness())
	require.NoError(t, conflictID2.FromRandomness())
	require.NoError(t, conflictID3.FromRandomness())

	assert.True(t, branchDAG.CreateConflict(branchID2, set.NewAdvancedSet(types.Identifier{}), set.NewAdvancedSet(conflictID0, conflictID1)))
	cachedBranch2 := branchDAG.Storage.CachedConflict(branchID2)
	defer cachedBranch2.Release()
	Branch2, exists := cachedBranch2.Unwrap()
	require.True(t, exists)
	assert.Equal(t, set.NewAdvancedSet(types.Identifier{}), Branch2.Parents())
	assert.True(t, set.NewAdvancedSet(conflictID0, conflictID1).Equal(Branch2.ConflictSetIDs()))

	assert.True(t, branchDAG.CreateConflict(branchID3, set.NewAdvancedSet(Branch2.ID()), set.NewAdvancedSet(conflictID0, conflictID1, conflictID2)))
	cachedBranch3 := branchDAG.Storage.CachedConflict(branchID3)
	defer cachedBranch3.Release()
	Branch3, exists := cachedBranch3.Unwrap()
	require.True(t, exists)

	assert.Equal(t, set.NewAdvancedSet(Branch2.ID()), Branch3.Parents())
	assert.Equal(t, set.NewAdvancedSet(conflictID0, conflictID1, conflictID2), Branch3.ConflictSetIDs())

	assert.False(t, branchDAG.CreateConflict(branchID2, set.NewAdvancedSet(types.Identifier{}), set.NewAdvancedSet(conflictID0, conflictID1, conflictID2)))
	assert.True(t, branchDAG.UpdateConflictingResources(branchID2, set.NewAdvancedSet(conflictID0, conflictID1, conflictID2)))
	cachedBranch2 = branchDAG.Storage.CachedConflict(branchID2)
	defer cachedBranch2.Release()
	Branch2, exists = cachedBranch2.Unwrap()
	require.True(t, exists)

	assert.Equal(t, set.NewAdvancedSet(conflictID0, conflictID1, conflictID2), Branch2.ConflictSetIDs())

	assert.True(t, branchDAG.CreateConflict(branchID4, set.NewAdvancedSet(Branch3.ID(), Branch3.ID()), set.NewAdvancedSet(conflictID3)))
	cachedBranch4 := branchDAG.Storage.CachedConflict(branchID4)
	defer cachedBranch4.Release()
	Branch4, exists := cachedBranch4.Unwrap()
	require.True(t, exists)
	assert.Equal(t, set.NewAdvancedSet(conflictID3), Branch4.ConflictSetIDs())
}

func TestConflictDAG_ConflictMembers(t *testing.T) {
	branchDAG := New[types.Identifier, types.Identifier]()
	defer branchDAG.Shutdown()

	var (
		branchID2   types.Identifier
		branchID3   types.Identifier
		branchID4   types.Identifier
		conflictID0 types.Identifier
	)
	require.NoError(t, branchID2.FromRandomness())
	require.NoError(t, branchID3.FromRandomness())
	require.NoError(t, branchID4.FromRandomness())
	require.NoError(t, conflictID0.FromRandomness())

	// create initial branches
	assert.True(t, branchDAG.CreateConflict(branchID2, set.NewAdvancedSet(types.Identifier{}), set.NewAdvancedSet(conflictID0)))
	cachedBranch2 := branchDAG.Storage.CachedConflict(branchID2)
	defer cachedBranch2.Release()
	branch2, exists := cachedBranch2.Unwrap()
	assert.True(t, exists)

	assert.True(t, branchDAG.CreateConflict(branchID3, set.NewAdvancedSet(types.Identifier{}), set.NewAdvancedSet(conflictID0)))
	cachedBranch3 := branchDAG.Storage.CachedConflict(branchID3)
	defer cachedBranch3.Release()
	branch3, exists := cachedBranch3.Unwrap()
	assert.True(t, exists)

	// assert conflict members
	expectedConflictMembers := map[types.Identifier]struct{}{
		branch2.ID(): {}, branch3.ID(): {},
	}
	actualConflictMembers := map[types.Identifier]struct{}{}
	branchDAG.Storage.CachedConflictMembers(conflictID0).Consume(func(conflictMember *ConflictMember[types.Identifier, types.Identifier]) {
		actualConflictMembers[conflictMember.ConflictID()] = struct{}{}
	})
	assert.Equal(t, expectedConflictMembers, actualConflictMembers)

	// add branch 4
	assert.True(t, branchDAG.CreateConflict(branchID4, set.NewAdvancedSet(types.Identifier{}), set.NewAdvancedSet(conflictID0)))
	cachedBranch4 := branchDAG.Storage.CachedConflict(branchID4)
	defer cachedBranch4.Release()
	branch4, exists := cachedBranch4.Unwrap()
	assert.True(t, exists)

	// branch 4 should now also be part of the conflict set
	expectedConflictMembers = map[types.Identifier]struct{}{
		branch2.ID(): {}, branch3.ID(): {}, branch4.ID(): {},
	}
	actualConflictMembers = map[types.Identifier]struct{}{}
	branchDAG.Storage.CachedConflictMembers(conflictID0).Consume(func(conflictMember *ConflictMember[types.Identifier, types.Identifier]) {
		actualConflictMembers[conflictMember.ConflictID()] = struct{}{}
	})
	assert.Equal(t, expectedConflictMembers, actualConflictMembers)
}

func TestConflictDAG_SetBranchAccepted(t *testing.T) {
	branchDAG := New[types.Identifier, types.Identifier]()
	defer branchDAG.Shutdown()

	var (
		conflictID0 types.Identifier
		conflictID1 types.Identifier
		conflictID2 types.Identifier
		conflictID3 types.Identifier
	)
	require.NoError(t, conflictID0.FromRandomness())
	require.NoError(t, conflictID1.FromRandomness())
	require.NoError(t, conflictID2.FromRandomness())
	require.NoError(t, conflictID3.FromRandomness())

	branchIDs := make(map[string]types.Identifier)
	branchIDs["Branch2"] = createBranch(t, branchDAG, "Branch2", set.NewAdvancedSet(types.Identifier{}), set.NewAdvancedSet(conflictID0))
	branchIDs["Branch3"] = createBranch(t, branchDAG, "Branch3", set.NewAdvancedSet(types.Identifier{}), set.NewAdvancedSet(conflictID0))
	branchIDs["Branch4"] = createBranch(t, branchDAG, "Branch4", set.NewAdvancedSet(branchIDs["Branch2"]), set.NewAdvancedSet(conflictID1))
	branchIDs["Branch5"] = createBranch(t, branchDAG, "Branch5", set.NewAdvancedSet(branchIDs["Branch2"]), set.NewAdvancedSet(conflictID1))
	branchIDs["Branch6"] = createBranch(t, branchDAG, "Branch6", set.NewAdvancedSet(types.Identifier{}), set.NewAdvancedSet(conflictID2))
	branchIDs["Branch7"] = createBranch(t, branchDAG, "Branch7", set.NewAdvancedSet(types.Identifier{}), set.NewAdvancedSet(conflictID2))
	branchIDs["Branch8"] = createBranch(t, branchDAG, "Branch8", set.NewAdvancedSet(types.Identifier{}), set.NewAdvancedSet(conflictID2))

	assert.True(t, branchDAG.SetBranchAccepted(branchIDs["Branch4"]))

	assertConfirmationStates(t, branchDAG, branchIDs, map[string]confirmation.State{
		"Branch2":         confirmation.Accepted,
		"Branch3":         confirmation.Rejected,
		"Branch4":         confirmation.Accepted,
		"Branch5":         confirmation.Rejected,
		"Branch6":         confirmation.Pending,
		"Branch7":         confirmation.Pending,
		"Branch8":         confirmation.Pending,
		"Branch5+Branch7": confirmation.Rejected,
		"Branch2+Branch7": confirmation.Pending,
		"Branch5+Branch8": confirmation.Rejected,
	})

	assert.True(t, branchDAG.SetBranchAccepted(branchIDs["Branch8"]))

	// Create a new Conflict in an already-decided Conflict Set results in straight Reject
	branchIDs["Branch9"] = createBranch(t, branchDAG, "Branch9", set.NewAdvancedSet(types.Identifier{}), set.NewAdvancedSet(conflictID2))

	assertConfirmationStates(t, branchDAG, branchIDs, map[string]confirmation.State{
		"Branch2":         confirmation.Accepted,
		"Branch3":         confirmation.Rejected,
		"Branch4":         confirmation.Accepted,
		"Branch5":         confirmation.Rejected,
		"Branch6":         confirmation.Rejected,
		"Branch7":         confirmation.Rejected,
		"Branch8":         confirmation.Accepted,
		"Branch5+Branch7": confirmation.Rejected,
		"Branch2+Branch7": confirmation.Rejected,
		"Branch5+Branch8": confirmation.Rejected,
		// Spawning a new aggregated branch with confirmation.Accepted parents results in confirmation.Accepted
		"Branch4+Branch8": confirmation.Accepted,
		// Spawning a new aggregated branch with any confirmation.Rejected parent results in confirmation.Rejected
		"Branch3+Branch8": confirmation.Rejected,
		"Branch9":         confirmation.Rejected,
	})

	branchIDs["Branch10"] = createBranch(t, branchDAG, "Branch10", set.NewAdvancedSet(types.Identifier{}), set.NewAdvancedSet(conflictID3))
	branchIDs["Branch11"] = createBranch(t, branchDAG, "Branch11", set.NewAdvancedSet(types.Identifier{}), set.NewAdvancedSet(conflictID3))

	branchDAG.SetBranchAccepted(branchIDs["Branch10"])

	assertConfirmationStates(t, branchDAG, branchIDs, map[string]confirmation.State{
		"Branch2":                  confirmation.Accepted,
		"Branch3":                  confirmation.Rejected,
		"Branch4":                  confirmation.Accepted,
		"Branch5":                  confirmation.Rejected,
		"Branch6":                  confirmation.Rejected,
		"Branch7":                  confirmation.Rejected,
		"Branch8":                  confirmation.Accepted,
		"Branch5+Branch7":          confirmation.Rejected,
		"Branch2+Branch7":          confirmation.Rejected,
		"Branch5+Branch8":          confirmation.Rejected,
		"Branch4+Branch8":          confirmation.Accepted,
		"Branch3+Branch8":          confirmation.Rejected,
		"Branch9":                  confirmation.Rejected,
		"Branch10":                 confirmation.Accepted,
		"Branch11":                 confirmation.Rejected,
		"Branch2+Branch7+Branch11": confirmation.Rejected,
	})
}

func assertConfirmationStates[ConflictT, ConflictSetT comparable](t *testing.T, branchDAG *ConflictDAG[ConflictT, ConflictSetT], branchIDsMapping map[string]ConflictT, expectedConfirmationStates map[string]confirmation.State) {
	for branchIDStrings, expectedConfirmationState := range expectedConfirmationStates {
		branchIDs := set.NewAdvancedSet[ConflictT]()
		for _, branchString := range strings.Split(branchIDStrings, "+") {
			branchIDs.Add(branchIDsMapping[branchString])
		}

		assert.Equal(t, expectedConfirmationState, branchDAG.ConfirmationState(branchIDs), "%s inclustionState is not %s", branchIDs, expectedConfirmationState)
	}
}

func createBranch(t *testing.T, branchDAG *ConflictDAG[types.Identifier, types.Identifier], branchAlias string, parents *set.AdvancedSet[types.Identifier], conflictIDs *set.AdvancedSet[types.Identifier]) types.Identifier {
	var randomBranchID types.Identifier
	if err := randomBranchID.FromRandomness(); err != nil {
		t.Error(err)
		return types.Identifier{}
	}

	assert.True(t, branchDAG.CreateConflict(randomBranchID, parents, conflictIDs))
	cachedBranch := branchDAG.Storage.CachedConflict(randomBranchID)
	cachedBranch.Release()

	// randomBranchID.RegisterAlias(branchAlias)

	return randomBranchID
}
