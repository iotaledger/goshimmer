package ledgerstate

import (
	"testing"

	"github.com/iotaledger/goshimmer/packages/database"

	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/kvstore/mapdb"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestBranchDAG_RetrieveConflictBranch(t *testing.T) {
	branchDAG := NewBranchDAG(mapdb.NewMapDB(), database.NewCacheTimeProvider(0))
	err := branchDAG.Prune()
	require.NoError(t, err)
	defer branchDAG.Shutdown()

	cachedConflictBranch2, newBranchCreated, err := branchDAG.CreateConflictBranch(BranchID{2}, NewBranchIDs(MasterBranchID), NewConflictIDs(ConflictID{0}, ConflictID{1}))
	require.NoError(t, err)
	defer cachedConflictBranch2.Release()
	conflictBranch2, err := cachedConflictBranch2.UnwrapConflictBranch()
	require.NoError(t, err)
	assert.True(t, newBranchCreated)
	assert.Equal(t, NewBranchIDs(MasterBranchID), conflictBranch2.Parents())
	assert.Equal(t, ConflictBranchType, conflictBranch2.Type())
	assert.Equal(t, NewConflictIDs(ConflictID{0}, ConflictID{1}), conflictBranch2.Conflicts())

	cachedConflictBranch3, _, err := branchDAG.CreateConflictBranch(BranchID{3}, NewBranchIDs(conflictBranch2.ID()), NewConflictIDs(ConflictID{0}, ConflictID{1}, ConflictID{2}))
	require.NoError(t, err)
	defer cachedConflictBranch3.Release()
	conflictBranch3, err := cachedConflictBranch3.UnwrapConflictBranch()
	require.NoError(t, err)
	assert.True(t, newBranchCreated)
	assert.Equal(t, NewBranchIDs(conflictBranch2.ID()), conflictBranch3.Parents())
	assert.Equal(t, ConflictBranchType, conflictBranch3.Type())
	assert.Equal(t, NewConflictIDs(ConflictID{0}, ConflictID{1}, ConflictID{2}), conflictBranch3.Conflicts())

	cachedConflictBranch2, newBranchCreated, err = branchDAG.CreateConflictBranch(BranchID{2}, NewBranchIDs(MasterBranchID), NewConflictIDs(ConflictID{0}, ConflictID{1}, ConflictID{2}))
	require.NoError(t, err)
	defer cachedConflictBranch2.Release()
	conflictBranch2, err = cachedConflictBranch2.UnwrapConflictBranch()
	require.NoError(t, err)
	assert.False(t, newBranchCreated)
	assert.Equal(t, NewConflictIDs(ConflictID{0}, ConflictID{1}, ConflictID{2}), conflictBranch2.Conflicts())

	_, _, err = branchDAG.CreateConflictBranch(BranchID{4}, NewBranchIDs(cachedConflictBranch2.ID(), cachedConflictBranch3.ID()), NewConflictIDs(ConflictID{3}))
	require.Error(t, err)
}

func TestBranchDAG_normalizeBranches(t *testing.T) {
	branchDAG := NewBranchDAG(mapdb.NewMapDB(), database.NewCacheTimeProvider(0))
	err := branchDAG.Prune()
	require.NoError(t, err)
	defer branchDAG.Shutdown()

	cachedBranch2, newBranchCreated, _ := branchDAG.CreateConflictBranch(BranchID{2}, NewBranchIDs(MasterBranchID), NewConflictIDs(ConflictID{0}))
	defer cachedBranch2.Release()
	branch2 := cachedBranch2.Unwrap()
	assert.True(t, newBranchCreated)

	cachedBranch3, newBranchCreated, _ := branchDAG.CreateConflictBranch(BranchID{3}, NewBranchIDs(MasterBranchID), NewConflictIDs(ConflictID{0}))
	defer cachedBranch3.Release()
	branch3 := cachedBranch3.Unwrap()
	assert.True(t, newBranchCreated)

	{
		normalizedBranches, err := branchDAG.normalizeBranches(NewBranchIDs(MasterBranchID, branch2.ID()))
		require.NoError(t, err)
		assert.Equal(t, normalizedBranches, NewBranchIDs(branch2.ID()))

		normalizedBranches, err = branchDAG.normalizeBranches(NewBranchIDs(MasterBranchID, branch3.ID()))
		require.NoError(t, err)
		assert.Equal(t, normalizedBranches, NewBranchIDs(branch3.ID()))

		_, err = branchDAG.normalizeBranches(NewBranchIDs(branch2.ID(), branch3.ID()))
		require.Error(t, err)
	}

	// spawn of branch 4 and 5 from branch 2
	cachedBranch4, newBranchCreated, _ := branchDAG.CreateConflictBranch(BranchID{4}, NewBranchIDs(branch2.ID()), NewConflictIDs(ConflictID{1}))
	defer cachedBranch4.Release()
	branch4 := cachedBranch4.Unwrap()
	assert.True(t, newBranchCreated)

	cachedBranch5, newBranchCreated, _ := branchDAG.CreateConflictBranch(BranchID{5}, NewBranchIDs(branch2.ID()), NewConflictIDs(ConflictID{1}))
	defer cachedBranch5.Release()
	branch5 := cachedBranch5.Unwrap()
	assert.True(t, newBranchCreated)

	{
		normalizedBranches, err := branchDAG.normalizeBranches(NewBranchIDs(MasterBranchID, branch4.ID()))
		require.NoError(t, err)
		assert.Equal(t, NewBranchIDs(branch4.ID()), normalizedBranches)

		_, err = branchDAG.normalizeBranches(NewBranchIDs(branch3.ID(), branch4.ID()))
		require.Error(t, err)

		normalizedBranches, err = branchDAG.normalizeBranches(NewBranchIDs(MasterBranchID, branch5.ID()))
		require.NoError(t, err)
		assert.Equal(t, NewBranchIDs(branch5.ID()), normalizedBranches)

		_, err = branchDAG.normalizeBranches(NewBranchIDs(branch3.ID(), branch5.ID()))
		require.Error(t, err)

		// since both consume the same output
		_, err = branchDAG.normalizeBranches(NewBranchIDs(branch4.ID(), branch5.ID()))
		require.Error(t, err)
	}

	// branch 6, 7 are on the same level as 2 and 3 but are not part of that conflict set
	cachedBranch6, newBranchCreated, _ := branchDAG.CreateConflictBranch(BranchID{6}, NewBranchIDs(MasterBranchID), NewConflictIDs(ConflictID{2}))
	defer cachedBranch6.Release()
	branch6 := cachedBranch6.Unwrap()
	assert.True(t, newBranchCreated)

	cachedBranch7, newBranchCreated, _ := branchDAG.CreateConflictBranch(BranchID{7}, NewBranchIDs(MasterBranchID), NewConflictIDs(ConflictID{2}))
	defer cachedBranch7.Release()
	branch7 := cachedBranch7.Unwrap()
	assert.True(t, newBranchCreated)

	{
		normalizedBranches, err := branchDAG.normalizeBranches(NewBranchIDs(branch2.ID(), branch6.ID()))
		require.NoError(t, err)
		assert.Equal(t, NewBranchIDs(branch2.ID(), branch6.ID()), normalizedBranches)

		normalizedBranches, err = branchDAG.normalizeBranches(NewBranchIDs(branch3.ID(), branch6.ID()))
		require.NoError(t, err)
		assert.Equal(t, NewBranchIDs(branch3.ID(), branch6.ID()), normalizedBranches)

		normalizedBranches, err = branchDAG.normalizeBranches(NewBranchIDs(branch2.ID(), branch7.ID()))
		require.NoError(t, err)
		assert.Equal(t, NewBranchIDs(branch2.ID(), branch7.ID()), normalizedBranches)

		normalizedBranches, err = branchDAG.normalizeBranches(NewBranchIDs(branch3.ID(), branch7.ID()))
		require.NoError(t, err)
		assert.Equal(t, NewBranchIDs(branch3.ID(), branch7.ID()), normalizedBranches)

		_, err = branchDAG.normalizeBranches(NewBranchIDs(branch6.ID(), branch7.ID()))
		require.Error(t, err)

		normalizedBranches, err = branchDAG.normalizeBranches(NewBranchIDs(branch4.ID(), branch6.ID()))
		require.NoError(t, err)
		assert.Equal(t, NewBranchIDs(branch4.ID(), branch6.ID()), normalizedBranches)

		normalizedBranches, err = branchDAG.normalizeBranches(NewBranchIDs(branch5.ID(), branch6.ID()))
		require.NoError(t, err)
		assert.Equal(t, NewBranchIDs(branch5.ID(), branch6.ID()), normalizedBranches)

		normalizedBranches, err = branchDAG.normalizeBranches(NewBranchIDs(branch4.ID(), branch7.ID()))
		require.NoError(t, err)
		assert.Equal(t, NewBranchIDs(branch4.ID(), branch7.ID()), normalizedBranches)

		normalizedBranches, err = branchDAG.normalizeBranches(NewBranchIDs(branch5.ID(), branch7.ID()))
		require.NoError(t, err)
		assert.Equal(t, NewBranchIDs(branch5.ID(), branch7.ID()), normalizedBranches)
	}

	// aggregated branch out of branch 4 (child of branch 2) and branch 6
	cachedAggrBranch8, newBranchCreated, aggrBranchErr := branchDAG.AggregateBranches(NewBranchIDs(branch4.ID(), branch6.ID()))
	require.NoError(t, aggrBranchErr)
	defer cachedAggrBranch8.Release()
	aggrBranch8 := cachedAggrBranch8.Unwrap()
	assert.True(t, newBranchCreated)

	{
		normalizedBranches, err := branchDAG.normalizeBranches(NewBranchIDs(aggrBranch8.ID(), MasterBranchID))
		require.NoError(t, err)
		assert.Equal(t, NewBranchIDs(branch4.ID(), branch6.ID()), normalizedBranches)

		normalizedBranches, err = branchDAG.normalizeBranches(NewBranchIDs(aggrBranch8.ID(), branch2.ID()))
		require.NoError(t, err)
		assert.Equal(t, NewBranchIDs(branch4.ID(), branch6.ID()), normalizedBranches)

		// conflicting since branch 2 and branch 3 are
		_, err = branchDAG.normalizeBranches(NewBranchIDs(aggrBranch8.ID(), branch3.ID()))
		require.Error(t, err)

		// conflicting since branch 4 and branch 5 are
		_, err = branchDAG.normalizeBranches(NewBranchIDs(aggrBranch8.ID(), branch5.ID()))
		require.Error(t, err)

		// conflicting since branch 6 and branch 7 are
		_, err = branchDAG.normalizeBranches(NewBranchIDs(aggrBranch8.ID(), branch7.ID()))
		require.Error(t, err)
	}

	// aggregated branch out of aggr. branch 8 and branch 7:
	// should fail since branch 6 & 7 are conflicting
	_, newBrachCreated, aggrBranchErr := branchDAG.AggregateBranches(NewBranchIDs(aggrBranch8.ID(), branch7.ID()))
	require.Error(t, aggrBranchErr)
	assert.False(t, newBrachCreated)

	// aggregated branch out of branch 5 (child of branch 2) and branch 7
	cachedAggrBranch9, newBranchCreated, aggrBranchErr := branchDAG.AggregateBranches(NewBranchIDs(branch5.ID(), branch7.ID()))
	require.NoError(t, aggrBranchErr)
	defer cachedAggrBranch9.Release()
	aggrBranch9 := cachedAggrBranch9.Unwrap()
	assert.True(t, newBranchCreated)

	{
		normalizedBranches, err := branchDAG.normalizeBranches(NewBranchIDs(aggrBranch9.ID(), MasterBranchID))
		require.NoError(t, err)
		assert.Equal(t, NewBranchIDs(branch5.ID(), branch7.ID()), normalizedBranches)

		// aggr. branch 8 and 9 should be conflicting, since 4 & 5 and 6 & 7 are
		_, err = branchDAG.normalizeBranches(NewBranchIDs(aggrBranch8.ID(), aggrBranch9.ID()))
		require.Error(t, err)

		// conflicting since branch 3 & 2 are
		_, err = branchDAG.normalizeBranches(NewBranchIDs(branch3.ID(), aggrBranch9.ID()))
		require.Error(t, err)
	}

	// aggregated branch out of branch 3 and branch 6
	cachedAggrBranch10, newBranchCreated, aggrBranchErr := branchDAG.AggregateBranches(NewBranchIDs(branch3.ID(), branch6.ID()))
	require.NoError(t, aggrBranchErr)
	defer cachedAggrBranch10.Release()
	aggrBranch10 := cachedAggrBranch10.Unwrap()
	assert.True(t, newBranchCreated)

	{
		normalizedBranches, err := branchDAG.normalizeBranches(NewBranchIDs(aggrBranch10.ID(), MasterBranchID))
		require.NoError(t, err)
		assert.Equal(t, NewBranchIDs(branch3.ID(), branch6.ID()), normalizedBranches)

		// aggr. branch 8 and 10 should be conflicting, since 2 & 3 are
		_, err = branchDAG.normalizeBranches(NewBranchIDs(aggrBranch8.ID(), aggrBranch10.ID()))
		require.Error(t, err)

		// aggr. branch 9 and 10 should be conflicting, since 2 & 3 and 6 & 7 are
		_, err = branchDAG.normalizeBranches(NewBranchIDs(aggrBranch9.ID(), aggrBranch10.ID()))
		require.Error(t, err)
	}

	// branch 11, 12 are on the same level as 2 & 3 and 6 & 7 but are not part of either conflict set
	cachedBranch11, newBranchCreated, _ := branchDAG.CreateConflictBranch(BranchID{11}, NewBranchIDs(MasterBranchID), NewConflictIDs(ConflictID{3}))
	defer cachedBranch11.Release()
	branch11 := cachedBranch11.Unwrap()
	assert.True(t, newBranchCreated)

	cachedBranch12, newBranchCreated, _ := branchDAG.CreateConflictBranch(BranchID{12}, NewBranchIDs(MasterBranchID), NewConflictIDs(ConflictID{3}))
	defer cachedBranch12.Release()
	branch12 := cachedBranch12.Unwrap()
	assert.True(t, newBranchCreated)

	{
		normalizedBranches, err := branchDAG.normalizeBranches(NewBranchIDs(MasterBranchID, branch11.ID()))
		require.NoError(t, err)
		assert.Equal(t, NewBranchIDs(branch11.ID()), normalizedBranches)

		normalizedBranches, err = branchDAG.normalizeBranches(NewBranchIDs(MasterBranchID, branch12.ID()))
		require.NoError(t, err)
		assert.Equal(t, NewBranchIDs(branch12.ID()), normalizedBranches)

		_, err = branchDAG.normalizeBranches(NewBranchIDs(branch11.ID(), branch12.ID()))
		require.Error(t, err)
	}

	// aggr. branch 13 out of branch 6 and 11
	cachedAggrBranch13, newBranchCreated, aggrBranchErr := branchDAG.AggregateBranches(NewBranchIDs(branch6.ID(), branch11.ID()))
	require.NoError(t, aggrBranchErr)
	defer cachedAggrBranch13.Release()
	aggrBranch13 := cachedAggrBranch13.Unwrap()
	assert.True(t, newBranchCreated)

	{
		_, err := branchDAG.normalizeBranches(NewBranchIDs(aggrBranch13.ID(), aggrBranch9.ID()))
		require.Error(t, err)

		normalizedBranches, err := branchDAG.normalizeBranches(NewBranchIDs(aggrBranch13.ID(), aggrBranch8.ID()))
		require.NoError(t, err)
		assert.Equal(t, NewBranchIDs(branch4.ID(), branch6.ID(), branch11.ID()), normalizedBranches)

		normalizedBranches, err = branchDAG.normalizeBranches(NewBranchIDs(aggrBranch13.ID(), aggrBranch10.ID()))
		require.NoError(t, err)
		assert.Equal(t, NewBranchIDs(branch3.ID(), branch6.ID(), branch11.ID()), normalizedBranches)
	}

	// aggr. branch 14 out of aggr. branch 10 and 13
	cachedAggrBranch14, newBranchCreated, aggrBranchErr := branchDAG.AggregateBranches(NewBranchIDs(aggrBranch10.ID(), aggrBranch13.ID()))
	require.NoError(t, aggrBranchErr)
	defer cachedAggrBranch14.Release()
	aggrBranch14 := cachedAggrBranch14.Unwrap()
	assert.True(t, newBranchCreated)

	{
		// aggr. branch 9 has parent branch 7 which conflicts with ancestor branch 6 of aggr. branch 14
		_, err := branchDAG.normalizeBranches(NewBranchIDs(aggrBranch14.ID(), aggrBranch9.ID()))
		require.Error(t, err)

		// aggr. branch has ancestor branch 2 which conflicts with ancestor branch 3 of aggr. branch 14
		_, err = branchDAG.normalizeBranches(NewBranchIDs(aggrBranch14.ID(), aggrBranch8.ID()))
		require.Error(t, err)
	}

	// aggr. branch 15 out of branch 2, 7 and 12
	cachedAggrBranch15, newBranchCreated, aggrBranchErr := branchDAG.AggregateBranches(NewBranchIDs(branch2.ID(), branch7.ID(), branch12.ID()))
	require.NoError(t, aggrBranchErr)
	defer cachedAggrBranch15.Release()
	aggrBranch15 := cachedAggrBranch15.Unwrap()
	assert.True(t, newBranchCreated)

	{
		// aggr. branch 13 has parent branches 11 & 6 which conflicts which conflicts with ancestor branches 12 & 7 of aggr. branch 15
		_, err := branchDAG.normalizeBranches(NewBranchIDs(aggrBranch15.ID(), aggrBranch13.ID()))
		require.Error(t, err)

		// aggr. branch 10 has parent branches 3 & 6 which conflicts with ancestor branches 2 & 7 of aggr. branch 15
		_, err = branchDAG.normalizeBranches(NewBranchIDs(aggrBranch15.ID(), aggrBranch10.ID()))
		require.Error(t, err)

		// aggr. branch 8 has parent branch 6 which conflicts with ancestor branch 7 of aggr. branch 15
		_, err = branchDAG.normalizeBranches(NewBranchIDs(aggrBranch15.ID(), aggrBranch8.ID()))
		require.Error(t, err)
	}

	// aggr. branch 16 out of aggr. branches 15 and 9
	cachedAggrBranch16, newBranchCreated, aggrBranchErr := branchDAG.AggregateBranches(NewBranchIDs(aggrBranch15.ID(), aggrBranch9.ID()))
	require.NoError(t, aggrBranchErr)
	defer cachedAggrBranch16.Release()
	aggrBranch16 := cachedAggrBranch16.Unwrap()
	assert.True(t, newBranchCreated)

	{
		// sanity check
		normalizedBranches, err := branchDAG.normalizeBranches(NewBranchIDs(aggrBranch16.ID(), aggrBranch9.ID()))
		require.NoError(t, err)
		assert.Equal(t, NewBranchIDs(branch5.ID(), branch7.ID(), branch12.ID()), normalizedBranches)

		// sanity check
		normalizedBranches, err = branchDAG.normalizeBranches(NewBranchIDs(aggrBranch16.ID(), branch7.ID()))
		require.NoError(t, err)
		assert.Equal(t, NewBranchIDs(branch5.ID(), branch7.ID(), branch12.ID()), normalizedBranches)

		_, err = branchDAG.normalizeBranches(NewBranchIDs(aggrBranch16.ID(), aggrBranch13.ID()))
		require.Error(t, err)

		_, err = branchDAG.normalizeBranches(NewBranchIDs(aggrBranch16.ID(), aggrBranch14.ID()))
		require.Error(t, err)

		_, err = branchDAG.normalizeBranches(NewBranchIDs(aggrBranch16.ID(), aggrBranch8.ID()))
		require.Error(t, err)
	}
}

func TestBranchDAG_ConflictMembers(t *testing.T) {
	branchDAG := NewBranchDAG(mapdb.NewMapDB(), database.NewCacheTimeProvider(0))
	err := branchDAG.Prune()
	require.NoError(t, err)
	defer branchDAG.Shutdown()

	// create initial branches
	cachedBranch2, newBranchCreated, _ := branchDAG.CreateConflictBranch(BranchID{2}, NewBranchIDs(MasterBranchID), NewConflictIDs(ConflictID{0}))
	defer cachedBranch2.Release()
	branch2 := cachedBranch2.Unwrap()
	assert.True(t, newBranchCreated)
	cachedBranch3, newBranchCreated, _ := branchDAG.CreateConflictBranch(BranchID{3}, NewBranchIDs(MasterBranchID), NewConflictIDs(ConflictID{0}))
	defer cachedBranch3.Release()
	branch3 := cachedBranch3.Unwrap()
	assert.True(t, newBranchCreated)

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
	cachedBranch4, newBranchCreated, _ := branchDAG.CreateConflictBranch(BranchID{4}, NewBranchIDs(MasterBranchID), NewConflictIDs(ConflictID{0}))
	defer cachedBranch4.Release()
	branch4 := cachedBranch4.Unwrap()
	assert.True(t, newBranchCreated)

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

type testBranchDAG struct {
	branch2        *ConflictBranch
	cachedBranch2  *CachedBranch
	branch3        *ConflictBranch
	cachedBranch3  *CachedBranch
	branch4        *ConflictBranch
	cachedBranch4  *CachedBranch
	branch5        *ConflictBranch
	cachedBranch5  *CachedBranch
	branch6        *ConflictBranch
	cachedBranch6  *CachedBranch
	branch7        *ConflictBranch
	cachedBranch7  *CachedBranch
	branch8        *AggregatedBranch
	cachedBranch8  *CachedBranch
	branch9        *AggregatedBranch
	cachedBranch9  *CachedBranch
	branch10       *AggregatedBranch
	cachedBranch10 *CachedBranch
	branch11       *ConflictBranch
	cachedBranch11 *CachedBranch
	branch12       *ConflictBranch
	cachedBranch12 *CachedBranch
	branch13       *AggregatedBranch
	cachedBranch13 *CachedBranch
	branch14       *AggregatedBranch
	cachedBranch14 *CachedBranch
	branch15       *AggregatedBranch
	cachedBranch15 *CachedBranch
	branch16       *AggregatedBranch
	cachedBranch16 *CachedBranch
}

func newTestBranchDAG(branchDAG *BranchDAG) (result *testBranchDAG, err error) {
	result = &testBranchDAG{}

	if result.cachedBranch2, _, err = branchDAG.CreateConflictBranch(BranchID{2}, NewBranchIDs(MasterBranchID), NewConflictIDs(ConflictID{0})); err != nil {
		return
	}
	if result.branch2, err = result.cachedBranch2.UnwrapConflictBranch(); err != nil {
		return
	}
	RegisterBranchIDAlias(result.branch2.ID(), "Branch2")

	if result.cachedBranch3, _, err = branchDAG.CreateConflictBranch(BranchID{3}, NewBranchIDs(MasterBranchID), NewConflictIDs(ConflictID{0})); err != nil {
		return
	}
	if result.branch3, err = result.cachedBranch3.UnwrapConflictBranch(); err != nil {
		return
	}
	RegisterBranchIDAlias(result.branch3.ID(), "Branch3")

	if result.cachedBranch4, _, err = branchDAG.CreateConflictBranch(BranchID{4}, NewBranchIDs(result.branch2.ID()), NewConflictIDs(ConflictID{1})); err != nil {
		return
	}
	if result.branch4, err = result.cachedBranch4.UnwrapConflictBranch(); err != nil {
		return
	}
	RegisterBranchIDAlias(result.branch4.ID(), "Branch4")

	if result.cachedBranch5, _, err = branchDAG.CreateConflictBranch(BranchID{5}, NewBranchIDs(result.branch2.ID()), NewConflictIDs(ConflictID{1})); err != nil {
		return
	}
	if result.branch5, err = result.cachedBranch5.UnwrapConflictBranch(); err != nil {
		return
	}
	RegisterBranchIDAlias(result.branch5.ID(), "Branch5")

	if result.cachedBranch6, _, err = branchDAG.CreateConflictBranch(BranchID{6}, NewBranchIDs(MasterBranchID), NewConflictIDs(ConflictID{2})); err != nil {
		return
	}
	if result.branch6, err = result.cachedBranch6.UnwrapConflictBranch(); err != nil {
		return
	}
	RegisterBranchIDAlias(result.branch6.ID(), "Branch6")

	if result.cachedBranch7, _, err = branchDAG.CreateConflictBranch(BranchID{7}, NewBranchIDs(MasterBranchID), NewConflictIDs(ConflictID{2})); err != nil {
		return
	}
	if result.branch7, err = result.cachedBranch7.UnwrapConflictBranch(); err != nil {
		return
	}
	RegisterBranchIDAlias(result.branch7.ID(), "Branch7")

	if result.cachedBranch8, _, err = branchDAG.AggregateBranches(NewBranchIDs(result.branch4.ID(), result.branch6.ID())); err != nil {
		return
	}
	if result.branch8, err = result.cachedBranch8.UnwrapAggregatedBranch(); err != nil {
		return
	}
	RegisterBranchIDAlias(result.branch8.ID(), "Branch8 = Branch4 + Branch6")

	if result.cachedBranch9, _, err = branchDAG.AggregateBranches(NewBranchIDs(result.branch5.ID(), result.branch7.ID())); err != nil {
		return
	}
	if result.branch9, err = result.cachedBranch9.UnwrapAggregatedBranch(); err != nil {
		return
	}
	RegisterBranchIDAlias(result.branch8.ID(), "Branch9 = Branch5 + Branch7")

	if result.cachedBranch10, _, err = branchDAG.AggregateBranches(NewBranchIDs(result.branch3.ID(), result.branch6.ID())); err != nil {
		return
	}
	if result.branch10, err = result.cachedBranch10.UnwrapAggregatedBranch(); err != nil {
		return
	}
	RegisterBranchIDAlias(result.branch10.ID(), "Branch10 = Branch3 + Branch6")

	if result.cachedBranch11, _, err = branchDAG.CreateConflictBranch(BranchID{11}, NewBranchIDs(MasterBranchID), NewConflictIDs(ConflictID{3})); err != nil {
		return
	}
	if result.branch11, err = result.cachedBranch11.UnwrapConflictBranch(); err != nil {
		return
	}
	RegisterBranchIDAlias(result.branch11.ID(), "Branch11")

	if result.cachedBranch12, _, err = branchDAG.CreateConflictBranch(BranchID{12}, NewBranchIDs(MasterBranchID), NewConflictIDs(ConflictID{3})); err != nil {
		return
	}
	if result.branch12, err = result.cachedBranch12.UnwrapConflictBranch(); err != nil {
		return
	}
	RegisterBranchIDAlias(result.branch12.ID(), "Branch12")

	if result.cachedBranch13, _, err = branchDAG.AggregateBranches(NewBranchIDs(result.branch6.ID(), result.branch11.ID())); err != nil {
		return
	}
	if result.branch13, err = result.cachedBranch13.UnwrapAggregatedBranch(); err != nil {
		return
	}
	RegisterBranchIDAlias(result.branch13.ID(), "Branch13 = Branch6 + Branch11")

	if result.cachedBranch14, _, err = branchDAG.AggregateBranches(NewBranchIDs(result.branch10.ID(), result.branch13.ID())); err != nil {
		return
	}
	if result.branch14, err = result.cachedBranch14.UnwrapAggregatedBranch(); err != nil {
		return
	}
	RegisterBranchIDAlias(result.branch14.ID(), "Branch14 = Branch3 + Branch6 + Branch11")

	if result.cachedBranch15, _, err = branchDAG.AggregateBranches(NewBranchIDs(result.branch2.ID(), result.branch7.ID(), result.branch12.ID())); err != nil {
		return
	}
	if result.branch15, err = result.cachedBranch15.UnwrapAggregatedBranch(); err != nil {
		return
	}
	RegisterBranchIDAlias(result.branch15.ID(), "Branch15 = Branch2 + Branch7 + Branch12")

	if result.cachedBranch16, _, err = branchDAG.AggregateBranches(NewBranchIDs(result.branch9.ID(), result.branch15.ID())); err != nil {
		return
	}
	if result.branch16, err = result.cachedBranch16.UnwrapAggregatedBranch(); err != nil {
		return
	}
	RegisterBranchIDAlias(result.branch16.ID(), "Branch16 = Branch5 + Branch7 + Branch12")

	return
}

func (t *testBranchDAG) Release(force ...bool) {
	t.cachedBranch2.Release(force...)
	t.cachedBranch3.Release(force...)
	t.cachedBranch4.Release(force...)
	t.cachedBranch5.Release(force...)
	t.cachedBranch6.Release(force...)
	t.cachedBranch7.Release(force...)
	t.cachedBranch8.Release(force...)
	t.cachedBranch9.Release(force...)
	t.cachedBranch10.Release(force...)
	t.cachedBranch11.Release(force...)
	t.cachedBranch12.Release(force...)
	t.cachedBranch13.Release(force...)
	t.cachedBranch14.Release(force...)
	t.cachedBranch15.Release(force...)
	t.cachedBranch16.Release(force...)
}

type eventMock struct {
	mock.Mock
	expectedEvents int
	calledEvents   int
	debugAlias     map[BranchID]string
	test           *testing.T

	attached []struct {
		*events.Event
		*events.Closure
	}
}
