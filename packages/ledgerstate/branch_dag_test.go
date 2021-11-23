package ledgerstate

import (
	"testing"

	"github.com/iotaledger/hive.go/types"

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

	_, _, err = ledgerstate.CreateConflictBranch(BranchID{4}, NewBranchIDs(cachedConflictBranch2.ID(), cachedConflictBranch3.ID()), NewConflictIDs(ConflictID{3}))
	require.Error(t, err)
}

func TestBranchDAG_normalizeBranches(t *testing.T) {
	ledgerstate := New(CacheTimeProvider(database.NewCacheTimeProvider(0)))
	defer ledgerstate.Shutdown()

	err := ledgerstate.Prune()
	require.NoError(t, err)

	cachedBranch2, newBranchCreated, _ := ledgerstate.CreateConflictBranch(BranchID{2}, NewBranchIDs(MasterBranchID), NewConflictIDs(ConflictID{0}))
	defer cachedBranch2.Release()
	branch2 := cachedBranch2.Unwrap()
	assert.True(t, newBranchCreated)

	cachedBranch3, newBranchCreated, _ := ledgerstate.CreateConflictBranch(BranchID{3}, NewBranchIDs(MasterBranchID), NewConflictIDs(ConflictID{0}))
	defer cachedBranch3.Release()
	branch3 := cachedBranch3.Unwrap()
	assert.True(t, newBranchCreated)

	{
		normalizedBranches, err := ledgerstate.normalizeBranches(NewBranchIDs(MasterBranchID, branch2.ID()))
		require.NoError(t, err)
		assert.Equal(t, normalizedBranches, NewBranchIDs(branch2.ID()))

		normalizedBranches, err = ledgerstate.normalizeBranches(NewBranchIDs(MasterBranchID, branch3.ID()))
		require.NoError(t, err)
		assert.Equal(t, normalizedBranches, NewBranchIDs(branch3.ID()))

		_, err = ledgerstate.normalizeBranches(NewBranchIDs(branch2.ID(), branch3.ID()))
		require.Error(t, err)
	}

	// spawn of branch 4 and 5 from branch 2
	cachedBranch4, newBranchCreated, _ := ledgerstate.CreateConflictBranch(BranchID{4}, NewBranchIDs(branch2.ID()), NewConflictIDs(ConflictID{1}))
	defer cachedBranch4.Release()
	branch4 := cachedBranch4.Unwrap()
	assert.True(t, newBranchCreated)

	cachedBranch5, newBranchCreated, _ := ledgerstate.CreateConflictBranch(BranchID{5}, NewBranchIDs(branch2.ID()), NewConflictIDs(ConflictID{1}))
	defer cachedBranch5.Release()
	branch5 := cachedBranch5.Unwrap()
	assert.True(t, newBranchCreated)

	{
		normalizedBranches, err := ledgerstate.normalizeBranches(NewBranchIDs(MasterBranchID, branch4.ID()))
		require.NoError(t, err)
		assert.Equal(t, NewBranchIDs(branch4.ID()), normalizedBranches)

		_, err = ledgerstate.normalizeBranches(NewBranchIDs(branch3.ID(), branch4.ID()))
		require.Error(t, err)

		normalizedBranches, err = ledgerstate.normalizeBranches(NewBranchIDs(MasterBranchID, branch5.ID()))
		require.NoError(t, err)
		assert.Equal(t, NewBranchIDs(branch5.ID()), normalizedBranches)

		_, err = ledgerstate.normalizeBranches(NewBranchIDs(branch3.ID(), branch5.ID()))
		require.Error(t, err)

		// since both consume the same output
		_, err = ledgerstate.normalizeBranches(NewBranchIDs(branch4.ID(), branch5.ID()))
		require.Error(t, err)
	}

	// branch 6, 7 are on the same level as 2 and 3 but are not part of that conflict set
	cachedBranch6, newBranchCreated, _ := ledgerstate.CreateConflictBranch(BranchID{6}, NewBranchIDs(MasterBranchID), NewConflictIDs(ConflictID{2}))
	defer cachedBranch6.Release()
	branch6 := cachedBranch6.Unwrap()
	assert.True(t, newBranchCreated)

	cachedBranch7, newBranchCreated, _ := ledgerstate.CreateConflictBranch(BranchID{7}, NewBranchIDs(MasterBranchID), NewConflictIDs(ConflictID{2}))
	defer cachedBranch7.Release()
	branch7 := cachedBranch7.Unwrap()
	assert.True(t, newBranchCreated)

	{
		normalizedBranches, err := ledgerstate.normalizeBranches(NewBranchIDs(branch2.ID(), branch6.ID()))
		require.NoError(t, err)
		assert.Equal(t, NewBranchIDs(branch2.ID(), branch6.ID()), normalizedBranches)

		normalizedBranches, err = ledgerstate.normalizeBranches(NewBranchIDs(branch3.ID(), branch6.ID()))
		require.NoError(t, err)
		assert.Equal(t, NewBranchIDs(branch3.ID(), branch6.ID()), normalizedBranches)

		normalizedBranches, err = ledgerstate.normalizeBranches(NewBranchIDs(branch2.ID(), branch7.ID()))
		require.NoError(t, err)
		assert.Equal(t, NewBranchIDs(branch2.ID(), branch7.ID()), normalizedBranches)

		normalizedBranches, err = ledgerstate.normalizeBranches(NewBranchIDs(branch3.ID(), branch7.ID()))
		require.NoError(t, err)
		assert.Equal(t, NewBranchIDs(branch3.ID(), branch7.ID()), normalizedBranches)

		_, err = ledgerstate.normalizeBranches(NewBranchIDs(branch6.ID(), branch7.ID()))
		require.Error(t, err)

		normalizedBranches, err = ledgerstate.normalizeBranches(NewBranchIDs(branch4.ID(), branch6.ID()))
		require.NoError(t, err)
		assert.Equal(t, NewBranchIDs(branch4.ID(), branch6.ID()), normalizedBranches)

		normalizedBranches, err = ledgerstate.normalizeBranches(NewBranchIDs(branch5.ID(), branch6.ID()))
		require.NoError(t, err)
		assert.Equal(t, NewBranchIDs(branch5.ID(), branch6.ID()), normalizedBranches)

		normalizedBranches, err = ledgerstate.normalizeBranches(NewBranchIDs(branch4.ID(), branch7.ID()))
		require.NoError(t, err)
		assert.Equal(t, NewBranchIDs(branch4.ID(), branch7.ID()), normalizedBranches)

		normalizedBranches, err = ledgerstate.normalizeBranches(NewBranchIDs(branch5.ID(), branch7.ID()))
		require.NoError(t, err)
		assert.Equal(t, NewBranchIDs(branch5.ID(), branch7.ID()), normalizedBranches)
	}

	// aggregated branch out of branch 4 (child of branch 2) and branch 6
	cachedAggrBranch8, newBranchCreated, aggrBranchErr := ledgerstate.AggregateBranches(NewBranchIDs(branch4.ID(), branch6.ID()))
	require.NoError(t, aggrBranchErr)
	defer cachedAggrBranch8.Release()
	aggrBranch8 := cachedAggrBranch8.Unwrap()
	assert.True(t, newBranchCreated)

	{
		normalizedBranches, err := ledgerstate.normalizeBranches(NewBranchIDs(aggrBranch8.ID(), MasterBranchID))
		require.NoError(t, err)
		assert.Equal(t, NewBranchIDs(branch4.ID(), branch6.ID()), normalizedBranches)

		normalizedBranches, err = ledgerstate.normalizeBranches(NewBranchIDs(aggrBranch8.ID(), branch2.ID()))
		require.NoError(t, err)
		assert.Equal(t, NewBranchIDs(branch4.ID(), branch6.ID()), normalizedBranches)

		// conflicting since branch 2 and branch 3 are
		_, err = ledgerstate.normalizeBranches(NewBranchIDs(aggrBranch8.ID(), branch3.ID()))
		require.Error(t, err)

		// conflicting since branch 4 and branch 5 are
		_, err = ledgerstate.normalizeBranches(NewBranchIDs(aggrBranch8.ID(), branch5.ID()))
		require.Error(t, err)

		// conflicting since branch 6 and branch 7 are
		_, err = ledgerstate.normalizeBranches(NewBranchIDs(aggrBranch8.ID(), branch7.ID()))
		require.Error(t, err)
	}

	// aggregated branch out of aggr. branch 8 and branch 7:
	// should fail since branch 6 & 7 are conflicting
	_, newBrachCreated, aggrBranchErr := ledgerstate.AggregateBranches(NewBranchIDs(aggrBranch8.ID(), branch7.ID()))
	require.Error(t, aggrBranchErr)
	assert.False(t, newBrachCreated)

	// aggregated branch out of branch 5 (child of branch 2) and branch 7
	cachedAggrBranch9, newBranchCreated, aggrBranchErr := ledgerstate.AggregateBranches(NewBranchIDs(branch5.ID(), branch7.ID()))
	require.NoError(t, aggrBranchErr)
	defer cachedAggrBranch9.Release()
	aggrBranch9 := cachedAggrBranch9.Unwrap()
	assert.True(t, newBranchCreated)

	{
		normalizedBranches, err := ledgerstate.normalizeBranches(NewBranchIDs(aggrBranch9.ID(), MasterBranchID))
		require.NoError(t, err)
		assert.Equal(t, NewBranchIDs(branch5.ID(), branch7.ID()), normalizedBranches)

		// aggr. branch 8 and 9 should be conflicting, since 4 & 5 and 6 & 7 are
		_, err = ledgerstate.normalizeBranches(NewBranchIDs(aggrBranch8.ID(), aggrBranch9.ID()))
		require.Error(t, err)

		// conflicting since branch 3 & 2 are
		_, err = ledgerstate.normalizeBranches(NewBranchIDs(branch3.ID(), aggrBranch9.ID()))
		require.Error(t, err)
	}

	// aggregated branch out of branch 3 and branch 6
	cachedAggrBranch10, newBranchCreated, aggrBranchErr := ledgerstate.AggregateBranches(NewBranchIDs(branch3.ID(), branch6.ID()))
	require.NoError(t, aggrBranchErr)
	defer cachedAggrBranch10.Release()
	aggrBranch10 := cachedAggrBranch10.Unwrap()
	assert.True(t, newBranchCreated)

	{
		normalizedBranches, err := ledgerstate.normalizeBranches(NewBranchIDs(aggrBranch10.ID(), MasterBranchID))
		require.NoError(t, err)
		assert.Equal(t, NewBranchIDs(branch3.ID(), branch6.ID()), normalizedBranches)

		// aggr. branch 8 and 10 should be conflicting, since 2 & 3 are
		_, err = ledgerstate.normalizeBranches(NewBranchIDs(aggrBranch8.ID(), aggrBranch10.ID()))
		require.Error(t, err)

		// aggr. branch 9 and 10 should be conflicting, since 2 & 3 and 6 & 7 are
		_, err = ledgerstate.normalizeBranches(NewBranchIDs(aggrBranch9.ID(), aggrBranch10.ID()))
		require.Error(t, err)
	}

	// branch 11, 12 are on the same level as 2 & 3 and 6 & 7 but are not part of either conflict set
	cachedBranch11, newBranchCreated, _ := ledgerstate.CreateConflictBranch(BranchID{11}, NewBranchIDs(MasterBranchID), NewConflictIDs(ConflictID{3}))
	defer cachedBranch11.Release()
	branch11 := cachedBranch11.Unwrap()
	assert.True(t, newBranchCreated)

	cachedBranch12, newBranchCreated, _ := ledgerstate.CreateConflictBranch(BranchID{12}, NewBranchIDs(MasterBranchID), NewConflictIDs(ConflictID{3}))
	defer cachedBranch12.Release()
	branch12 := cachedBranch12.Unwrap()
	assert.True(t, newBranchCreated)

	{
		normalizedBranches, err := ledgerstate.normalizeBranches(NewBranchIDs(MasterBranchID, branch11.ID()))
		require.NoError(t, err)
		assert.Equal(t, NewBranchIDs(branch11.ID()), normalizedBranches)

		normalizedBranches, err = ledgerstate.normalizeBranches(NewBranchIDs(MasterBranchID, branch12.ID()))
		require.NoError(t, err)
		assert.Equal(t, NewBranchIDs(branch12.ID()), normalizedBranches)

		_, err = ledgerstate.normalizeBranches(NewBranchIDs(branch11.ID(), branch12.ID()))
		require.Error(t, err)
	}

	// aggr. branch 13 out of branch 6 and 11
	cachedAggrBranch13, newBranchCreated, aggrBranchErr := ledgerstate.AggregateBranches(NewBranchIDs(branch6.ID(), branch11.ID()))
	require.NoError(t, aggrBranchErr)
	defer cachedAggrBranch13.Release()
	aggrBranch13 := cachedAggrBranch13.Unwrap()
	assert.True(t, newBranchCreated)

	{
		_, err := ledgerstate.normalizeBranches(NewBranchIDs(aggrBranch13.ID(), aggrBranch9.ID()))
		require.Error(t, err)

		normalizedBranches, err := ledgerstate.normalizeBranches(NewBranchIDs(aggrBranch13.ID(), aggrBranch8.ID()))
		require.NoError(t, err)
		assert.Equal(t, NewBranchIDs(branch4.ID(), branch6.ID(), branch11.ID()), normalizedBranches)

		normalizedBranches, err = ledgerstate.normalizeBranches(NewBranchIDs(aggrBranch13.ID(), aggrBranch10.ID()))
		require.NoError(t, err)
		assert.Equal(t, NewBranchIDs(branch3.ID(), branch6.ID(), branch11.ID()), normalizedBranches)
	}

	// aggr. branch 14 out of aggr. branch 10 and 13
	cachedAggrBranch14, newBranchCreated, aggrBranchErr := ledgerstate.AggregateBranches(NewBranchIDs(aggrBranch10.ID(), aggrBranch13.ID()))
	require.NoError(t, aggrBranchErr)
	defer cachedAggrBranch14.Release()
	aggrBranch14 := cachedAggrBranch14.Unwrap()
	assert.True(t, newBranchCreated)

	{
		// aggr. branch 9 has parent branch 7 which conflicts with ancestor branch 6 of aggr. branch 14
		_, err := ledgerstate.normalizeBranches(NewBranchIDs(aggrBranch14.ID(), aggrBranch9.ID()))
		require.Error(t, err)

		// aggr. branch has ancestor branch 2 which conflicts with ancestor branch 3 of aggr. branch 14
		_, err = ledgerstate.normalizeBranches(NewBranchIDs(aggrBranch14.ID(), aggrBranch8.ID()))
		require.Error(t, err)
	}

	// aggr. branch 15 out of branch 2, 7 and 12
	cachedAggrBranch15, newBranchCreated, aggrBranchErr := ledgerstate.AggregateBranches(NewBranchIDs(branch2.ID(), branch7.ID(), branch12.ID()))
	require.NoError(t, aggrBranchErr)
	defer cachedAggrBranch15.Release()
	aggrBranch15 := cachedAggrBranch15.Unwrap()
	assert.True(t, newBranchCreated)

	{
		// aggr. branch 13 has parent branches 11 & 6 which conflicts which conflicts with ancestor branches 12 & 7 of aggr. branch 15
		_, err := ledgerstate.normalizeBranches(NewBranchIDs(aggrBranch15.ID(), aggrBranch13.ID()))
		require.Error(t, err)

		// aggr. branch 10 has parent branches 3 & 6 which conflicts with ancestor branches 2 & 7 of aggr. branch 15
		_, err = ledgerstate.normalizeBranches(NewBranchIDs(aggrBranch15.ID(), aggrBranch10.ID()))
		require.Error(t, err)

		// aggr. branch 8 has parent branch 6 which conflicts with ancestor branch 7 of aggr. branch 15
		_, err = ledgerstate.normalizeBranches(NewBranchIDs(aggrBranch15.ID(), aggrBranch8.ID()))
		require.Error(t, err)
	}

	// aggr. branch 16 out of aggr. branches 15 and 9
	cachedAggrBranch16, newBranchCreated, aggrBranchErr := ledgerstate.AggregateBranches(NewBranchIDs(aggrBranch15.ID(), aggrBranch9.ID()))
	require.NoError(t, aggrBranchErr)
	defer cachedAggrBranch16.Release()
	aggrBranch16 := cachedAggrBranch16.Unwrap()
	assert.True(t, newBranchCreated)

	{
		// sanity check
		normalizedBranches, err := ledgerstate.normalizeBranches(NewBranchIDs(aggrBranch16.ID(), aggrBranch9.ID()))
		require.NoError(t, err)
		assert.Equal(t, NewBranchIDs(branch5.ID(), branch7.ID(), branch12.ID()), normalizedBranches)

		// sanity check
		normalizedBranches, err = ledgerstate.normalizeBranches(NewBranchIDs(aggrBranch16.ID(), branch7.ID()))
		require.NoError(t, err)
		assert.Equal(t, NewBranchIDs(branch5.ID(), branch7.ID(), branch12.ID()), normalizedBranches)

		_, err = ledgerstate.normalizeBranches(NewBranchIDs(aggrBranch16.ID(), aggrBranch13.ID()))
		require.Error(t, err)

		_, err = ledgerstate.normalizeBranches(NewBranchIDs(aggrBranch16.ID(), aggrBranch14.ID()))
		require.Error(t, err)

		_, err = ledgerstate.normalizeBranches(NewBranchIDs(aggrBranch16.ID(), aggrBranch8.ID()))
		require.Error(t, err)
	}
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
	ledgerstate.conflictMembers(ConflictID{0}).Consume(func(conflictMember *ConflictMember) {
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
	ledgerstate.conflictMembers(ConflictID{0}).Consume(func(conflictMember *ConflictMember) {
		actualConflictMembers[conflictMember.BranchID()] = struct{}{}
	})
	assert.Equal(t, expectedConflictMembers, actualConflictMembers)
}

func TestBranchDAG_MergeToMaster(t *testing.T) {
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
	branchIDs["Branch5+Branch7"] = createAggregatedBranch(t, ledgerstate, "Branch5+Branch7", NewBranchIDs(branchIDs["Branch5"], branchIDs["Branch7"]))
	branchIDs["Branch2+Branch7"] = createAggregatedBranch(t, ledgerstate, "Branch2+Branch7", NewBranchIDs(branchIDs["Branch2"], branchIDs["Branch7"]))
	branchIDs["Branch5+Branch8"] = createAggregatedBranch(t, ledgerstate, "Branch5+Branch8", NewBranchIDs(branchIDs["Branch5"], branchIDs["Branch8"]))

	movedBranches, err := ledgerstate.BranchDAG.MergeToMaster(branchIDs["Branch2"])
	require.NoError(t, err)

	assert.Equal(t, map[BranchID]BranchID{
		branchIDs["Branch2"]:         MasterBranchID,
		branchIDs["Branch2+Branch7"]: branchIDs["Branch7"],
	}, movedBranches)

	assertConflictMembers(t, ledgerstate, ConflictID{0}, map[BranchID]types.Empty{
		branchIDs["Branch3"]: types.Void,
	})
}

func assertConflictMembers(t *testing.T, ledgerstate *Ledgerstate, conflictID ConflictID, expectedMembers map[BranchID]types.Empty) {
	assert.True(t, ledgerstate.Conflict(conflictID).Consume(func(conflict *Conflict) {
		assert.Equal(t, len(expectedMembers), conflict.MemberCount())

		conflictMembers := make(map[BranchID]types.Empty)
		ledgerstate.conflictMembers(conflict.ID()).Consume(func(conflictMember *ConflictMember) {
			conflictMembers[conflictMember.BranchID()] = types.Void
		})

		assert.Equal(t, expectedMembers, conflictMembers)
	}))
}

func createConflictBranch(t *testing.T, ledgerstate *Ledgerstate, branchAlias string, parents BranchIDs, conflictIDs ConflictIDs) BranchID {
	cachedBranch, _, err := ledgerstate.CreateConflictBranch(BranchIDFromRandomness(), parents, conflictIDs)
	require.NoError(t, err)
	cachedBranch.Release()

	RegisterBranchIDAlias(cachedBranch.ID(), branchAlias)

	return cachedBranch.ID()
}

func createAggregatedBranch(t *testing.T, ledgerstate *Ledgerstate, branchAlias string, parents BranchIDs) BranchID {
	cachedBranch, _, err := ledgerstate.AggregateBranches(parents)
	require.NoError(t, err)
	cachedBranch.Release()

	RegisterBranchIDAlias(cachedBranch.ID(), branchAlias)

	return cachedBranch.ID()
}
