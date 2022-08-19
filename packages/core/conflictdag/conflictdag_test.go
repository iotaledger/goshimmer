package conflictdag

import (
	"strings"
	"testing"

	"github.com/iotaledger/hive.go/core/generics/set"
	"github.com/iotaledger/hive.go/core/types"
	"github.com/iotaledger/hive.go/core/types/confirmation"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConflictDAG_RetrieveConflict(t *testing.T) {
	conflictDAG := New[types.Identifier, types.Identifier]()
	defer conflictDAG.Shutdown()

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

	assert.True(t, conflictDAG.CreateConflict(branchID2, set.NewAdvancedSet(types.Identifier{}), set.NewAdvancedSet(conflictID0, conflictID1)))
	cachedBranch2 := conflictDAG.Storage.CachedConflict(branchID2)
	defer cachedBranch2.Release()
	Branch2, exists := cachedBranch2.Unwrap()
	require.True(t, exists)
	assert.Equal(t, set.NewAdvancedSet(types.Identifier{}), Branch2.Parents())
	assert.True(t, set.NewAdvancedSet(conflictID0, conflictID1).Equal(Branch2.ConflictSetIDs()))

	assert.True(t, conflictDAG.CreateConflict(branchID3, set.NewAdvancedSet(Branch2.ID()), set.NewAdvancedSet(conflictID0, conflictID1, conflictID2)))
	cachedBranch3 := conflictDAG.Storage.CachedConflict(branchID3)
	defer cachedBranch3.Release()
	Branch3, exists := cachedBranch3.Unwrap()
	require.True(t, exists)

	assert.Equal(t, set.NewAdvancedSet(Branch2.ID()), Branch3.Parents())
	assert.Equal(t, set.NewAdvancedSet(conflictID0, conflictID1, conflictID2), Branch3.ConflictSetIDs())

	assert.False(t, conflictDAG.CreateConflict(branchID2, set.NewAdvancedSet(types.Identifier{}), set.NewAdvancedSet(conflictID0, conflictID1, conflictID2)))
	assert.True(t, conflictDAG.UpdateConflictingResources(branchID2, set.NewAdvancedSet(conflictID0, conflictID1, conflictID2)))
	cachedBranch2 = conflictDAG.Storage.CachedConflict(branchID2)
	defer cachedBranch2.Release()
	Branch2, exists = cachedBranch2.Unwrap()
	require.True(t, exists)

	assert.Equal(t, set.NewAdvancedSet(conflictID0, conflictID1, conflictID2), Branch2.ConflictSetIDs())

	assert.True(t, conflictDAG.CreateConflict(branchID4, set.NewAdvancedSet(Branch3.ID(), Branch3.ID()), set.NewAdvancedSet(conflictID3)))
	cachedBranch4 := conflictDAG.Storage.CachedConflict(branchID4)
	defer cachedBranch4.Release()
	Branch4, exists := cachedBranch4.Unwrap()
	require.True(t, exists)
	assert.Equal(t, set.NewAdvancedSet(conflictID3), Branch4.ConflictSetIDs())
}

func TestConflictDAG_ConflictMembers(t *testing.T) {
	conflictDAG := New[types.Identifier, types.Identifier]()
	defer conflictDAG.Shutdown()

	var (
		conflictID2 types.Identifier
		conflictID3 types.Identifier
		conflictID4 types.Identifier
		conflictID0 types.Identifier
	)
	require.NoError(t, conflictID2.FromRandomness())
	require.NoError(t, conflictID3.FromRandomness())
	require.NoError(t, conflictID4.FromRandomness())
	require.NoError(t, conflictID0.FromRandomness())

	// create initial conflicts
	assert.True(t, conflictDAG.CreateConflict(conflictID2, set.NewAdvancedSet(types.Identifier{}), set.NewAdvancedSet(conflictID0)))
	cachedConflict2 := conflictDAG.Storage.CachedConflict(conflictID2)
	defer cachedConflict2.Release()
	conflict2, exists := cachedConflict2.Unwrap()
	assert.True(t, exists)

	assert.True(t, conflictDAG.CreateConflict(conflictID3, set.NewAdvancedSet(types.Identifier{}), set.NewAdvancedSet(conflictID0)))
	cachedConflict3 := conflictDAG.Storage.CachedConflict(conflictID3)
	defer cachedConflict3.Release()
	conflict3, exists := cachedConflict3.Unwrap()
	assert.True(t, exists)

	// assert conflict members
	expectedConflictMembers := map[types.Identifier]struct{}{
		conflict2.ID(): {}, conflict3.ID(): {},
	}
	actualConflictMembers := map[types.Identifier]struct{}{}
	conflictDAG.Storage.CachedConflictMembers(conflictID0).Consume(func(conflictMember *ConflictMember[types.Identifier, types.Identifier]) {
		actualConflictMembers[conflictMember.ConflictID()] = struct{}{}
	})
	assert.Equal(t, expectedConflictMembers, actualConflictMembers)

	// add conflict 4
	assert.True(t, conflictDAG.CreateConflict(conflictID4, set.NewAdvancedSet(types.Identifier{}), set.NewAdvancedSet(conflictID0)))
	cachedConflict4 := conflictDAG.Storage.CachedConflict(conflictID4)
	defer cachedConflict4.Release()
	conflict4, exists := cachedConflict4.Unwrap()
	assert.True(t, exists)

	// conflict 4 should now also be part of the conflict set
	expectedConflictMembers = map[types.Identifier]struct{}{
		conflict2.ID(): {}, conflict3.ID(): {}, conflict4.ID(): {},
	}
	actualConflictMembers = map[types.Identifier]struct{}{}
	conflictDAG.Storage.CachedConflictMembers(conflictID0).Consume(func(conflictMember *ConflictMember[types.Identifier, types.Identifier]) {
		actualConflictMembers[conflictMember.ConflictID()] = struct{}{}
	})
	assert.Equal(t, expectedConflictMembers, actualConflictMembers)
}

func TestConflictDAG_SetConflictAccepted(t *testing.T) {
	conflictDAG := New[types.Identifier, types.Identifier]()
	defer conflictDAG.Shutdown()

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

	conflictIDs := make(map[string]types.Identifier)
	conflictIDs["Conflict2"] = createConflict(t, conflictDAG, "Conflict2", set.NewAdvancedSet(types.Identifier{}), set.NewAdvancedSet(conflictID0))
	conflictIDs["Conflict3"] = createConflict(t, conflictDAG, "Conflict3", set.NewAdvancedSet(types.Identifier{}), set.NewAdvancedSet(conflictID0))
	conflictIDs["Conflict4"] = createConflict(t, conflictDAG, "Conflict4", set.NewAdvancedSet(conflictIDs["Conflict2"]), set.NewAdvancedSet(conflictID1))
	conflictIDs["Conflict5"] = createConflict(t, conflictDAG, "Conflict5", set.NewAdvancedSet(conflictIDs["Conflict2"]), set.NewAdvancedSet(conflictID1))
	conflictIDs["Conflict6"] = createConflict(t, conflictDAG, "Conflict6", set.NewAdvancedSet(types.Identifier{}), set.NewAdvancedSet(conflictID2))
	conflictIDs["Conflict7"] = createConflict(t, conflictDAG, "Conflict7", set.NewAdvancedSet(types.Identifier{}), set.NewAdvancedSet(conflictID2))
	conflictIDs["Conflict8"] = createConflict(t, conflictDAG, "Conflict8", set.NewAdvancedSet(types.Identifier{}), set.NewAdvancedSet(conflictID2))

	assert.True(t, conflictDAG.SetConflictAccepted(conflictIDs["Conflict4"]))

	assertConfirmationStates(t, conflictDAG, conflictIDs, map[string]confirmation.State{
		"Conflict2":           confirmation.Accepted,
		"Conflict3":           confirmation.Rejected,
		"Conflict4":           confirmation.Accepted,
		"Conflict5":           confirmation.Rejected,
		"Conflict6":           confirmation.Pending,
		"Conflict7":           confirmation.Pending,
		"Conflict8":           confirmation.Pending,
		"Conflict5+Conflict7": confirmation.Rejected,
		"Conflict2+Conflict7": confirmation.Pending,
		"Conflict5+Conflict8": confirmation.Rejected,
	})

	assert.True(t, conflictDAG.SetConflictAccepted(conflictIDs["Conflict8"]))

	// Create a new Conflict in an already-decided Conflict Set results in straight Reject
	conflictIDs["Conflict9"] = createConflict(t, conflictDAG, "Conflict9", set.NewAdvancedSet(types.Identifier{}), set.NewAdvancedSet(conflictID2))

	assertConfirmationStates(t, conflictDAG, conflictIDs, map[string]confirmation.State{
		"Conflict2":           confirmation.Accepted,
		"Conflict3":           confirmation.Rejected,
		"Conflict4":           confirmation.Accepted,
		"Conflict5":           confirmation.Rejected,
		"Conflict6":           confirmation.Rejected,
		"Conflict7":           confirmation.Rejected,
		"Conflict8":           confirmation.Accepted,
		"Conflict5+Conflict7": confirmation.Rejected,
		"Conflict2+Conflict7": confirmation.Rejected,
		"Conflict5+Conflict8": confirmation.Rejected,
		// Spawning a new aggregated conflict with confirmation.Accepted parents results in confirmation.Accepted
		"Conflict4+Conflict8": confirmation.Accepted,
		// Spawning a new aggregated conflict with any confirmation.Rejected parent results in confirmation.Rejected
		"Conflict3+Conflict8": confirmation.Rejected,
		"Conflict9":           confirmation.Rejected,
	})

	conflictIDs["Conflict10"] = createConflict(t, conflictDAG, "Conflict10", set.NewAdvancedSet(types.Identifier{}), set.NewAdvancedSet(conflictID3))
	conflictIDs["Conflict11"] = createConflict(t, conflictDAG, "Conflict11", set.NewAdvancedSet(types.Identifier{}), set.NewAdvancedSet(conflictID3))

	conflictDAG.SetConflictAccepted(conflictIDs["Conflict10"])

	assertConfirmationStates(t, conflictDAG, conflictIDs, map[string]confirmation.State{
		"Conflict2":                      confirmation.Accepted,
		"Conflict3":                      confirmation.Rejected,
		"Conflict4":                      confirmation.Accepted,
		"Conflict5":                      confirmation.Rejected,
		"Conflict6":                      confirmation.Rejected,
		"Conflict7":                      confirmation.Rejected,
		"Conflict8":                      confirmation.Accepted,
		"Conflict5+Conflict7":            confirmation.Rejected,
		"Conflict2+Conflict7":            confirmation.Rejected,
		"Conflict5+Conflict8":            confirmation.Rejected,
		"Conflict4+Conflict8":            confirmation.Accepted,
		"Conflict3+Conflict8":            confirmation.Rejected,
		"Conflict9":                      confirmation.Rejected,
		"Conflict10":                     confirmation.Accepted,
		"Conflict11":                     confirmation.Rejected,
		"Conflict2+Conflict7+Conflict11": confirmation.Rejected,
	})
}

func assertConfirmationStates[ConflictT, ConflictSetT comparable](t *testing.T, conflictDAG *ConflictDAG[ConflictT, ConflictSetT], conflictIDsMapping map[string]ConflictT, expectedConfirmationStates map[string]confirmation.State) {
	for conflictIDStrings, expectedConfirmationState := range expectedConfirmationStates {
		conflictIDs := set.NewAdvancedSet[ConflictT]()
		for _, conflictString := range strings.Split(conflictIDStrings, "+") {
			conflictIDs.Add(conflictIDsMapping[conflictString])
		}

		assert.Equal(t, expectedConfirmationState, conflictDAG.ConfirmationState(conflictIDs), "%s inclustionState is not %s", conflictIDs, expectedConfirmationState)
	}
}

func createConflict(t *testing.T, conflictDAG *ConflictDAG[types.Identifier, types.Identifier], conflictAlias string, parents *set.AdvancedSet[types.Identifier], conflictIDs *set.AdvancedSet[types.Identifier]) types.Identifier {
	var randomConflictID types.Identifier
	if err := randomConflictID.FromRandomness(); err != nil {
		t.Error(err)
		return types.Identifier{}
	}

	assert.True(t, conflictDAG.CreateConflict(randomConflictID, parents, conflictIDs))
	cachedConflict := conflictDAG.Storage.CachedConflict(randomConflictID)
	cachedConflict.Release()

	// randomConflictID.RegisterAlias(conflictAlias)

	return randomConflictID
}
