package ledgerstate

import (
	"reflect"
	"testing"

	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/kvstore/mapdb"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func TestBranchDAG_RetrieveConflictBranch(t *testing.T) {
	branchDAG := NewBranchDAG(mapdb.NewMapDB())
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
	assert.False(t, conflictBranch2.Liked())
	assert.False(t, conflictBranch2.MonotonicallyLiked())
	assert.False(t, conflictBranch2.Finalized())
	assert.Equal(t, Pending, conflictBranch2.InclusionState())
	assert.Equal(t, NewConflictIDs(ConflictID{0}, ConflictID{1}), conflictBranch2.Conflicts())

	cachedConflictBranch3, _, err := branchDAG.CreateConflictBranch(BranchID{3}, NewBranchIDs(conflictBranch2.ID()), NewConflictIDs(ConflictID{0}, ConflictID{1}, ConflictID{2}))
	require.NoError(t, err)
	defer cachedConflictBranch3.Release()
	conflictBranch3, err := cachedConflictBranch3.UnwrapConflictBranch()
	require.NoError(t, err)
	assert.True(t, newBranchCreated)
	assert.Equal(t, NewBranchIDs(conflictBranch2.ID()), conflictBranch3.Parents())
	assert.Equal(t, ConflictBranchType, conflictBranch3.Type())
	assert.False(t, conflictBranch3.Liked())
	assert.False(t, conflictBranch3.MonotonicallyLiked())
	assert.False(t, conflictBranch3.Finalized())
	assert.Equal(t, Pending, conflictBranch3.InclusionState())
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
	branchDAG := NewBranchDAG(mapdb.NewMapDB())
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

func TestBranchDAG_SetBranchLiked(t *testing.T) {
	branchDAG := NewBranchDAG(mapdb.NewMapDB())
	err := branchDAG.Prune()
	require.NoError(t, err)
	defer branchDAG.Shutdown()

	event := newEventMock(t, branchDAG)
	defer event.DetachAll()

	cachedBranch2, newBranchCreated, _ := branchDAG.CreateConflictBranch(BranchID{2}, NewBranchIDs(MasterBranchID), NewConflictIDs(ConflictID{0}))
	defer cachedBranch2.Release()
	branch2 := cachedBranch2.Unwrap()
	assert.True(t, newBranchCreated)

	cachedBranch3, newBranchCreated, _ := branchDAG.CreateConflictBranch(BranchID{3}, NewBranchIDs(MasterBranchID), NewConflictIDs(ConflictID{0}))
	defer cachedBranch3.Release()
	branch3 := cachedBranch3.Unwrap()
	assert.True(t, newBranchCreated)

	assert.False(t, branch2.Liked(), "branch 2 should not be liked")
	assert.False(t, branch2.MonotonicallyLiked(), "branch 2 should not be monotonically liked")
	assert.False(t, branch3.Liked(), "branch 3 should not be liked")
	assert.False(t, branch3.MonotonicallyLiked(), "branch 3 should not be monotonically liked")

	cachedBranch4, newBranchCreated, _ := branchDAG.CreateConflictBranch(BranchID{4}, NewBranchIDs(branch2.ID()), NewConflictIDs(ConflictID{1}))
	defer cachedBranch4.Release()
	branch4 := cachedBranch4.Unwrap()
	assert.True(t, newBranchCreated)

	cachedBranch5, newBranchCreated, _ := branchDAG.CreateConflictBranch(BranchID{5}, NewBranchIDs(branch2.ID()), NewConflictIDs(ConflictID{1}))
	defer cachedBranch5.Release()
	branch5 := cachedBranch5.Unwrap()
	assert.True(t, newBranchCreated)

	// lets assume branch 4 is liked since its underlying transaction was longer
	// solid than the avg. network delay before the conflicting transaction which created
	// the conflict set was received

	event.Expect("BranchLiked", branch4.ID())

	modified, err := branchDAG.SetBranchLiked(branch4.ID(), true)
	require.NoError(t, err)
	assert.True(t, modified)

	assert.True(t, branch4.Liked(), "branch 4 should be liked")
	// is not liked because its parents aren't liked, respectively branch 2
	assert.False(t, branch4.MonotonicallyLiked(), "branch 4 should not be monotonically liked")
	assert.False(t, branch5.Liked(), "branch 5 should not be liked")
	assert.False(t, branch5.MonotonicallyLiked(), "branch 5 should not be monotonically liked")

	// now branch 2 becomes liked via FPC, this causes branch 2 to be monotonically liked (since
	// the master branch is monotonically liked) and its monotonically liked state propagates to branch 4 (but not
	// branch 5)

	event.Expect("BranchLiked", branch2.ID())
	event.Expect("BranchMonotonicallyLiked", branch2.ID())
	event.Expect("BranchMonotonicallyLiked", branch4.ID())

	modified, err = branchDAG.SetBranchLiked(branch2.ID(), true)
	require.NoError(t, err)
	assert.True(t, modified)

	assert.True(t, branch2.MonotonicallyLiked(), "branch 2 should be monotonically liked")
	assert.True(t, branch2.Liked(), "branch 2 should be liked")
	assert.True(t, branch4.MonotonicallyLiked(), "branch 4 should be monotonically liked")
	assert.True(t, branch4.Liked(), "branch 4 should still be liked")
	assert.False(t, branch5.MonotonicallyLiked(), "branch 5 should not be monotonically liked")
	assert.False(t, branch5.Liked(), "branch 5 should not be liked")

	// now the network decides that branch 5 is liked (via FPC), thus branch 4 should lose its
	// liked and monotonically liked state and branch 5 should instead become liked and monotonically liked

	event.Expect("BranchLiked", branch5.ID())
	event.Expect("BranchMonotonicallyLiked", branch5.ID())
	event.Expect("BranchDisliked", branch4.ID())
	event.Expect("BranchMonotonicallyDisliked", branch4.ID())

	modified, err = branchDAG.SetBranchLiked(branch5.ID(), true)
	require.NoError(t, err)
	assert.True(t, modified)

	// sanity check for branch 2 state
	assert.True(t, branch2.MonotonicallyLiked(), "branch 2 should be monotonically liked")
	assert.True(t, branch2.Liked(), "branch 2 should be liked")

	// check that branch 4 is disliked and not liked
	assert.False(t, branch4.MonotonicallyLiked(), "branch 4 should be monotonically disliked")
	assert.False(t, branch4.Liked(), "branch 4 should not be liked")
	assert.True(t, branch5.MonotonicallyLiked(), "branch 5 should be monotonically liked")
	assert.True(t, branch5.Liked(), "branch 5 should be liked")

	// check that all events have been triggered
	event.AssertExpectations(t)
}

func TestBranchDAG_SetBranchMonotonicallyLiked(t *testing.T) {
	branchDAG := NewBranchDAG(mapdb.NewMapDB())
	err := branchDAG.Prune()
	require.NoError(t, err)
	defer branchDAG.Shutdown()

	eventMock := newEventMock(t, branchDAG)
	defer eventMock.DetachAll()

	testBranchDAG, err := newTestBranchDAG(branchDAG)
	require.NoError(t, err)
	defer testBranchDAG.Release()
	testBranchDAG.AssertInitialState(t)
	testBranchDAG.RegisterDebugAliases(eventMock)

	{
		eventMock.Expect("BranchLiked", testBranchDAG.branch2.ID())
		eventMock.Expect("BranchLiked", testBranchDAG.branch7.ID())
		eventMock.Expect("BranchLiked", testBranchDAG.branch12.ID())
		eventMock.Expect("BranchMonotonicallyLiked", testBranchDAG.branch2.ID())
		eventMock.Expect("BranchMonotonicallyLiked", testBranchDAG.branch7.ID())
		eventMock.Expect("BranchMonotonicallyLiked", testBranchDAG.branch12.ID())
		eventMock.Expect("BranchLiked", testBranchDAG.branch5.ID())
		eventMock.Expect("BranchMonotonicallyLiked", testBranchDAG.branch5.ID())
		eventMock.Expect("BranchLiked", testBranchDAG.branch9.ID())
		eventMock.Expect("BranchLiked", testBranchDAG.branch15.ID())
		eventMock.Expect("BranchLiked", testBranchDAG.branch16.ID())
		eventMock.Expect("BranchMonotonicallyLiked", testBranchDAG.branch9.ID())
		eventMock.Expect("BranchMonotonicallyLiked", testBranchDAG.branch15.ID())
		eventMock.Expect("BranchMonotonicallyLiked", testBranchDAG.branch16.ID())

		modified, err := branchDAG.SetBranchMonotonicallyLiked(testBranchDAG.branch16.ID(), true)
		require.NoError(t, err)
		assert.True(t, modified)

		assert.True(t, testBranchDAG.branch2.Liked())
		assert.True(t, testBranchDAG.branch2.MonotonicallyLiked())
		assert.True(t, testBranchDAG.branch7.Liked())
		assert.True(t, testBranchDAG.branch7.MonotonicallyLiked())
		assert.True(t, testBranchDAG.branch12.Liked())
		assert.True(t, testBranchDAG.branch12.MonotonicallyLiked())
		assert.True(t, testBranchDAG.branch5.Liked())
		assert.True(t, testBranchDAG.branch5.MonotonicallyLiked())
		assert.True(t, testBranchDAG.branch9.Liked())
		assert.True(t, testBranchDAG.branch9.MonotonicallyLiked())
		assert.True(t, testBranchDAG.branch15.Liked())
		assert.True(t, testBranchDAG.branch15.MonotonicallyLiked())
		assert.True(t, testBranchDAG.branch16.Liked())
		assert.True(t, testBranchDAG.branch16.MonotonicallyLiked())
	}

	{
		eventMock.Expect("BranchDisliked", testBranchDAG.branch5.ID())
		eventMock.Expect("BranchDisliked", testBranchDAG.branch7.ID())
		eventMock.Expect("BranchDisliked", testBranchDAG.branch9.ID())
		eventMock.Expect("BranchDisliked", testBranchDAG.branch15.ID())
		eventMock.Expect("BranchDisliked", testBranchDAG.branch16.ID())
		eventMock.Expect("BranchMonotonicallyDisliked", testBranchDAG.branch5.ID())
		eventMock.Expect("BranchMonotonicallyDisliked", testBranchDAG.branch7.ID())
		eventMock.Expect("BranchMonotonicallyDisliked", testBranchDAG.branch9.ID())
		eventMock.Expect("BranchMonotonicallyDisliked", testBranchDAG.branch15.ID())
		eventMock.Expect("BranchMonotonicallyDisliked", testBranchDAG.branch16.ID())
		eventMock.Expect("BranchLiked", testBranchDAG.branch4.ID())
		eventMock.Expect("BranchMonotonicallyLiked", testBranchDAG.branch4.ID())
		eventMock.Expect("BranchLiked", testBranchDAG.branch6.ID())
		eventMock.Expect("BranchMonotonicallyLiked", testBranchDAG.branch6.ID())
		eventMock.Expect("BranchLiked", testBranchDAG.branch8.ID())
		eventMock.Expect("BranchMonotonicallyLiked", testBranchDAG.branch8.ID())

		modified, err := branchDAG.SetBranchMonotonicallyLiked(testBranchDAG.branch8.ID(), true)
		require.NoError(t, err)
		assert.True(t, modified)

		assert.False(t, testBranchDAG.branch7.Liked())
		assert.False(t, testBranchDAG.branch7.MonotonicallyLiked())
		assert.False(t, testBranchDAG.branch5.Liked())
		assert.False(t, testBranchDAG.branch5.MonotonicallyLiked())
		assert.False(t, testBranchDAG.branch9.Liked())
		assert.False(t, testBranchDAG.branch9.MonotonicallyLiked())
		assert.False(t, testBranchDAG.branch15.Liked())
		assert.False(t, testBranchDAG.branch15.MonotonicallyLiked())
		assert.False(t, testBranchDAG.branch16.Liked())
		assert.False(t, testBranchDAG.branch16.MonotonicallyLiked())
		assert.True(t, testBranchDAG.branch4.Liked())
		assert.True(t, testBranchDAG.branch4.MonotonicallyLiked())
		assert.True(t, testBranchDAG.branch6.Liked())
		assert.True(t, testBranchDAG.branch6.MonotonicallyLiked())
		assert.True(t, testBranchDAG.branch8.Liked())
		assert.True(t, testBranchDAG.branch8.MonotonicallyLiked())
	}

	{
		eventMock.Expect("BranchDisliked", testBranchDAG.branch6.ID())
		eventMock.Expect("BranchMonotonicallyDisliked", testBranchDAG.branch6.ID())
		eventMock.Expect("BranchMonotonicallyDisliked", testBranchDAG.branch8.ID())

		modified, err := branchDAG.SetBranchMonotonicallyLiked(testBranchDAG.branch6.ID(), false)
		require.NoError(t, err)
		assert.True(t, modified)
	}

	// check that all events have been triggered
	eventMock.AssertExpectations(t)
}

func TestBranchDAG_SetBranchFinalized(t *testing.T) {
	branchDAG := NewBranchDAG(mapdb.NewMapDB())
	err := branchDAG.Prune()
	require.NoError(t, err)
	defer branchDAG.Shutdown()

	eventMock := newEventMock(t, branchDAG)
	defer eventMock.DetachAll()

	testBranchDAG, err := newTestBranchDAG(branchDAG)
	require.NoError(t, err)
	defer testBranchDAG.Release()
	testBranchDAG.AssertInitialState(t)
	testBranchDAG.RegisterDebugAliases(eventMock)

	{
		eventMock.Expect("BranchLiked", testBranchDAG.branch2.ID())
		eventMock.Expect("BranchLiked", testBranchDAG.branch7.ID())
		eventMock.Expect("BranchLiked", testBranchDAG.branch12.ID())
		eventMock.Expect("BranchMonotonicallyLiked", testBranchDAG.branch2.ID())
		eventMock.Expect("BranchMonotonicallyLiked", testBranchDAG.branch7.ID())
		eventMock.Expect("BranchMonotonicallyLiked", testBranchDAG.branch12.ID())
		eventMock.Expect("BranchLiked", testBranchDAG.branch5.ID())
		eventMock.Expect("BranchMonotonicallyLiked", testBranchDAG.branch5.ID())
		eventMock.Expect("BranchLiked", testBranchDAG.branch9.ID())
		eventMock.Expect("BranchLiked", testBranchDAG.branch15.ID())
		eventMock.Expect("BranchLiked", testBranchDAG.branch16.ID())
		eventMock.Expect("BranchMonotonicallyLiked", testBranchDAG.branch9.ID())
		eventMock.Expect("BranchMonotonicallyLiked", testBranchDAG.branch15.ID())
		eventMock.Expect("BranchMonotonicallyLiked", testBranchDAG.branch16.ID())

		modified, err := branchDAG.SetBranchMonotonicallyLiked(testBranchDAG.branch16.ID(), true)
		require.NoError(t, err)
		assert.True(t, modified)

		assert.True(t, testBranchDAG.branch2.Liked())
		assert.True(t, testBranchDAG.branch2.MonotonicallyLiked())
		assert.True(t, testBranchDAG.branch7.Liked())
		assert.True(t, testBranchDAG.branch7.MonotonicallyLiked())
		assert.True(t, testBranchDAG.branch12.Liked())
		assert.True(t, testBranchDAG.branch12.MonotonicallyLiked())
		assert.True(t, testBranchDAG.branch5.Liked())
		assert.True(t, testBranchDAG.branch5.MonotonicallyLiked())
		assert.True(t, testBranchDAG.branch9.Liked())
		assert.True(t, testBranchDAG.branch9.MonotonicallyLiked())
		assert.True(t, testBranchDAG.branch15.Liked())
		assert.True(t, testBranchDAG.branch15.MonotonicallyLiked())
		assert.True(t, testBranchDAG.branch16.Liked())
		assert.True(t, testBranchDAG.branch16.MonotonicallyLiked())
	}

	{
		eventMock.Expect("BranchFinalized", testBranchDAG.branch4.ID())
		eventMock.Expect("BranchFinalized", testBranchDAG.branch5.ID())
		eventMock.Expect("BranchFinalized", testBranchDAG.branch6.ID())
		eventMock.Expect("BranchFinalized", testBranchDAG.branch7.ID())
		eventMock.Expect("BranchFinalized", testBranchDAG.branch8.ID())
		eventMock.Expect("BranchFinalized", testBranchDAG.branch9.ID())
		eventMock.Expect("BranchRejected", testBranchDAG.branch4.ID())
		eventMock.Expect("BranchRejected", testBranchDAG.branch6.ID())
		eventMock.Expect("BranchRejected", testBranchDAG.branch8.ID())
		eventMock.Expect("BranchRejected", testBranchDAG.branch10.ID())
		eventMock.Expect("BranchRejected", testBranchDAG.branch13.ID())
		eventMock.Expect("BranchRejected", testBranchDAG.branch14.ID())
		eventMock.Expect("BranchConfirmed", testBranchDAG.branch7.ID())

		modified, err := branchDAG.SetBranchFinalized(testBranchDAG.branch9.ID(), true)
		require.NoError(t, err)
		assert.True(t, modified)

		assert.True(t, testBranchDAG.branch4.Finalized())
		assert.True(t, testBranchDAG.branch5.Finalized())
		assert.True(t, testBranchDAG.branch6.Finalized())
		assert.True(t, testBranchDAG.branch7.Finalized())
		assert.True(t, testBranchDAG.branch8.Finalized())
		assert.True(t, testBranchDAG.branch9.Finalized())
		assert.Equal(t, Rejected, testBranchDAG.branch4.InclusionState())
		assert.Equal(t, Rejected, testBranchDAG.branch6.InclusionState())
		assert.Equal(t, Rejected, testBranchDAG.branch8.InclusionState())
		assert.Equal(t, Rejected, testBranchDAG.branch10.InclusionState())
		assert.Equal(t, Rejected, testBranchDAG.branch13.InclusionState())
		assert.Equal(t, Rejected, testBranchDAG.branch14.InclusionState())
		assert.Equal(t, Confirmed, testBranchDAG.branch7.InclusionState())
	}

	{
		eventMock.Expect("BranchFinalized", testBranchDAG.branch2.ID())
		eventMock.Expect("BranchFinalized", testBranchDAG.branch3.ID())
		eventMock.Expect("BranchFinalized", testBranchDAG.branch10.ID())
		eventMock.Expect("BranchRejected", testBranchDAG.branch3.ID())
		eventMock.Expect("BranchConfirmed", testBranchDAG.branch2.ID())
		eventMock.Expect("BranchConfirmed", testBranchDAG.branch5.ID())
		eventMock.Expect("BranchConfirmed", testBranchDAG.branch9.ID())

		modified, err := branchDAG.SetBranchFinalized(testBranchDAG.branch2.ID(), true)
		require.NoError(t, err)
		assert.True(t, modified)

		assert.True(t, testBranchDAG.branch2.Finalized())
		assert.True(t, testBranchDAG.branch3.Finalized())
		assert.True(t, testBranchDAG.branch10.Finalized())
		assert.Equal(t, Rejected, testBranchDAG.branch3.InclusionState())
		assert.Equal(t, Confirmed, testBranchDAG.branch2.InclusionState())
		assert.Equal(t, Confirmed, testBranchDAG.branch5.InclusionState())
		assert.Equal(t, Confirmed, testBranchDAG.branch9.InclusionState())
	}

	{
		eventMock.Expect("BranchUnfinalized", testBranchDAG.branch2.ID())
		eventMock.Expect("BranchPending", testBranchDAG.branch2.ID())
		eventMock.Expect("BranchPending", testBranchDAG.branch4.ID())
		eventMock.Expect("BranchPending", testBranchDAG.branch5.ID())
		eventMock.Expect("BranchPending", testBranchDAG.branch8.ID())
		eventMock.Expect("BranchPending", testBranchDAG.branch9.ID())

		modified, err := branchDAG.SetBranchFinalized(testBranchDAG.branch2.ID(), false)
		require.NoError(t, err)
		assert.True(t, modified)

		assert.False(t, testBranchDAG.branch2.Finalized())
		assert.Equal(t, Pending, testBranchDAG.branch2.InclusionState())
		assert.Equal(t, Pending, testBranchDAG.branch4.InclusionState())
		assert.Equal(t, Pending, testBranchDAG.branch5.InclusionState())
		assert.Equal(t, Pending, testBranchDAG.branch8.InclusionState())
		assert.Equal(t, Pending, testBranchDAG.branch9.InclusionState())
	}

	{
		eventMock.Expect("BranchUnfinalized", testBranchDAG.branch3.ID())
		eventMock.Expect("BranchUnfinalized", testBranchDAG.branch10.ID())
		eventMock.Expect("BranchPending", testBranchDAG.branch3.ID())
		eventMock.Expect("BranchPending", testBranchDAG.branch10.ID())
		eventMock.Expect("BranchPending", testBranchDAG.branch14.ID())

		modified, err := branchDAG.SetBranchFinalized(testBranchDAG.branch3.ID(), false)
		require.NoError(t, err)
		assert.True(t, modified)

		assert.False(t, testBranchDAG.branch3.Finalized())
		assert.False(t, testBranchDAG.branch10.Finalized())
		assert.Equal(t, Pending, testBranchDAG.branch3.InclusionState())
		assert.Equal(t, Pending, testBranchDAG.branch10.InclusionState())
		assert.Equal(t, Pending, testBranchDAG.branch14.InclusionState())
	}

	// check that all events have been triggered
	eventMock.AssertExpectations(t)
}

func TestBranchDAG_SetBranchLiked2(t *testing.T) {
	branchDAG := NewBranchDAG(mapdb.NewMapDB())
	err := branchDAG.Prune()
	require.NoError(t, err)
	defer branchDAG.Shutdown()

	event := newEventMock(t, branchDAG)
	defer event.DetachAll()

	cachedBranch2, newBranchCreated, _ := branchDAG.CreateConflictBranch(BranchID{2}, NewBranchIDs(MasterBranchID), NewConflictIDs(ConflictID{0}))
	defer cachedBranch2.Release()
	branch2 := cachedBranch2.Unwrap()
	assert.True(t, newBranchCreated)

	cachedBranch3, newBranchCreated, _ := branchDAG.CreateConflictBranch(BranchID{3}, NewBranchIDs(MasterBranchID), NewConflictIDs(ConflictID{0}))
	defer cachedBranch3.Release()
	branch3 := cachedBranch3.Unwrap()
	assert.True(t, newBranchCreated)

	cachedBranch4, newBranchCreated, _ := branchDAG.CreateConflictBranch(BranchID{4}, NewBranchIDs(branch2.ID()), NewConflictIDs(ConflictID{1}))
	defer cachedBranch4.Release()
	branch4 := cachedBranch4.Unwrap()
	assert.True(t, newBranchCreated)

	cachedBranch5, newBranchCreated, _ := branchDAG.CreateConflictBranch(BranchID{5}, NewBranchIDs(branch2.ID()), NewConflictIDs(ConflictID{1}))
	defer cachedBranch5.Release()
	branch5 := cachedBranch5.Unwrap()
	assert.True(t, newBranchCreated)

	cachedBranch6, newBranchCreated, _ := branchDAG.CreateConflictBranch(BranchID{6}, NewBranchIDs(MasterBranchID), NewConflictIDs(ConflictID{2}))
	defer cachedBranch6.Release()
	branch6 := cachedBranch6.Unwrap()
	assert.True(t, newBranchCreated)

	cachedBranch7, newBranchCreated, _ := branchDAG.CreateConflictBranch(BranchID{7}, NewBranchIDs(MasterBranchID), NewConflictIDs(ConflictID{2}))
	defer cachedBranch7.Release()
	branch7 := cachedBranch7.Unwrap()
	assert.True(t, newBranchCreated)

	event.Expect("BranchLiked", branch2.ID())
	event.Expect("BranchMonotonicallyLiked", branch2.ID())
	event.Expect("BranchLiked", branch6.ID())
	event.Expect("BranchMonotonicallyLiked", branch6.ID())

	// assume branch 2 liked since solid longer than avg. network delay
	modified, err := branchDAG.SetBranchLiked(branch2.ID(), true)
	require.NoError(t, err)
	assert.True(t, modified)

	// assume branch 6 liked since solid longer than avg. network delay
	modified, err = branchDAG.SetBranchLiked(branch6.ID(), true)
	require.NoError(t, err)
	assert.True(t, modified)

	{
		assert.True(t, branch2.MonotonicallyLiked(), "branch 2 should be monotonically liked")
		assert.True(t, branch2.Liked(), "branch 2 should be liked")
		assert.False(t, branch3.MonotonicallyLiked(), "branch 3 should not be monotonically liked")
		assert.False(t, branch3.Liked(), "branch 3 should not be likedliked")
		assert.False(t, branch4.MonotonicallyLiked(), "branch 4 should not be monotonically liked")
		assert.False(t, branch4.Liked(), "branch 4 should not be likedliked")
		assert.False(t, branch5.MonotonicallyLiked(), "branch 5 should not be monotonically liked")
		assert.False(t, branch5.Liked(), "branch 5 should not be likedliked")
		assert.True(t, branch6.MonotonicallyLiked(), "branch 6 should be monotonically liked")
		assert.True(t, branch6.Liked(), "branch 6 should be likedliked")
		assert.False(t, branch7.MonotonicallyLiked(), "branch 7 should not be monotonically liked")
		assert.False(t, branch7.Liked(), "branch 7 should not be liked")
	}

	// throw some aggregated branches into the mix
	cachedAggrBranch8, newBranchCreated, err := branchDAG.AggregateBranches(NewBranchIDs(branch4.ID(), branch6.ID()))
	require.NoError(t, err)
	defer cachedAggrBranch8.Release()
	aggrBranch8 := cachedAggrBranch8.Unwrap()
	assert.True(t, newBranchCreated)

	// should not be liked because only 6 is liked but not 4
	assert.False(t, aggrBranch8.MonotonicallyLiked(), "aggr. branch 8 should not be monotonically liked")
	assert.False(t, aggrBranch8.Liked(), "aggr. branch 8 should not be liked")

	cachedAggrBranch9, newBranchCreated, err := branchDAG.AggregateBranches(NewBranchIDs(branch5.ID(), branch7.ID()))
	require.NoError(t, err)
	defer cachedAggrBranch9.Release()
	aggrBranch9 := cachedAggrBranch9.Unwrap()
	assert.True(t, newBranchCreated)

	// branch 5 and 7 are neither liked or liked
	assert.False(t, aggrBranch9.MonotonicallyLiked(), "aggr. branch 9 should not be monotonically liked")
	assert.False(t, aggrBranch9.Liked(), "aggr. branch 9 should not be liked")

	// should not be liked because only 6 is is liked but not 3
	cachedAggrBranch10, newBranchCreated, err := branchDAG.AggregateBranches(NewBranchIDs(branch3.ID(), branch6.ID()))
	require.NoError(t, err)
	defer cachedAggrBranch10.Release()
	aggrBranch10 := cachedAggrBranch10.Unwrap()
	assert.True(t, newBranchCreated)

	assert.False(t, aggrBranch10.MonotonicallyLiked(), "aggr. branch 10 should not be monotonically liked")
	assert.False(t, aggrBranch10.Liked(), "aggr. branch 10 should not be liked")

	// spawn off conflict branch 11 and 12
	cachedBranch11, newBranchCreated, _ := branchDAG.CreateConflictBranch(BranchID{11}, NewBranchIDs(aggrBranch8.ID()), NewConflictIDs(ConflictID{3}))
	defer cachedBranch11.Release()
	branch11 := cachedBranch11.Unwrap()
	assert.True(t, newBranchCreated)

	assert.False(t, branch11.MonotonicallyLiked(), "aggr. branch 11 should not be monotonically liked")
	assert.False(t, branch11.Liked(), "aggr. branch 11 should not be liked")

	cachedBranch12, newBranchCreated, _ := branchDAG.CreateConflictBranch(BranchID{12}, NewBranchIDs(aggrBranch8.ID()), NewConflictIDs(ConflictID{3}))
	defer cachedBranch12.Release()
	branch12 := cachedBranch12.Unwrap()
	assert.True(t, newBranchCreated)

	assert.False(t, branch12.MonotonicallyLiked(), "aggr. branch 12 should not be monotonically liked")
	assert.False(t, branch12.Liked(), "aggr. branch 12 should not be liked")

	cachedAggrBranch13, newBranchCreated, err := branchDAG.AggregateBranches(NewBranchIDs(branch4.ID(), branch12.ID()))
	require.NoError(t, err)
	defer cachedAggrBranch13.Release()
	aggrBranch13 := cachedAggrBranch13.Unwrap()
	assert.False(t, newBranchCreated)

	assert.False(t, aggrBranch13.MonotonicallyLiked(), "aggr. branch 13 should not be monotonically liked")
	assert.False(t, aggrBranch13.Liked(), "aggr. branch 13 should not be liked")

	// now lets assume FPC finalized on branch 2, 6 and 4 to be liked.
	// branches 2 and 6 are already liked but 4 is newly liked. Branch 4 therefore
	// should also become liked, since branch 2 of which it spawns off is liked too.

	// simulate branch 3 being not liked from FPC vote
	// this does not trigger any events as branch 3 was never liked
	modified, err = branchDAG.SetBranchLiked(branch3.ID(), false)
	require.NoError(t, err)
	assert.False(t, modified)
	// simulate branch 7 being not liked from FPC vote
	// this does not trigger any events as branch 7 was never liked
	modified, err = branchDAG.SetBranchLiked(branch7.ID(), false)
	require.NoError(t, err)
	assert.False(t, modified)

	event.Expect("BranchLiked", branch4.ID())
	event.Expect("BranchMonotonicallyLiked", branch4.ID())
	event.Expect("BranchLiked", aggrBranch8.ID())
	event.Expect("BranchMonotonicallyLiked", aggrBranch8.ID())

	// simulate branch 4 being liked by FPC vote
	modified, err = branchDAG.SetBranchLiked(branch4.ID(), true)
	require.NoError(t, err)
	assert.True(t, modified)
	assert.True(t, branch4.MonotonicallyLiked(), "branch 4 should be monotonically liked")
	assert.True(t, branch4.Liked(), "branch 4 should be liked")

	// this should cause aggr. branch 8 to also be liked and monotonically liked, since branch 6 and 4
	// of which it spawns off are.
	assert.True(t, aggrBranch8.MonotonicallyLiked(), "aggr. branch 8 should be monotonically liked")
	assert.True(t, aggrBranch8.Liked(), "aggr. branch 8 should be liked")

	// check that all events have been triggered
	event.AssertExpectations(t)
}

func TestBranchDAG_ConflictMembers(t *testing.T) {
	branchDAG := NewBranchDAG(mapdb.NewMapDB())
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

func TestBranchDAG_MergeToMaster(t *testing.T) {
	branchDAG := NewBranchDAG(mapdb.NewMapDB())
	err := branchDAG.Prune()
	require.NoError(t, err)
	defer branchDAG.Shutdown()

	eventMock := newEventMock(t, branchDAG)
	defer eventMock.DetachAll()

	testBranchDAG, err := newTestBranchDAG(branchDAG)
	require.NoError(t, err)
	defer testBranchDAG.Release()
	testBranchDAG.AssertInitialState(t)
	testBranchDAG.RegisterDebugAliases(eventMock)

	_, err = branchDAG.MergeToMaster(testBranchDAG.branch12.ID())
	require.Error(t, err)

	eventMock.Expect("BranchLiked", testBranchDAG.branch2.ID())
	eventMock.Expect("BranchMonotonicallyLiked", testBranchDAG.branch2.ID())
	eventMock.Expect("BranchLiked", testBranchDAG.branch7.ID())
	eventMock.Expect("BranchMonotonicallyLiked", testBranchDAG.branch7.ID())
	eventMock.Expect("BranchLiked", testBranchDAG.branch12.ID())
	eventMock.Expect("BranchMonotonicallyLiked", testBranchDAG.branch12.ID())
	eventMock.Expect("BranchLiked", testBranchDAG.branch15.ID())
	eventMock.Expect("BranchMonotonicallyLiked", testBranchDAG.branch15.ID())

	modified, err := branchDAG.SetBranchMonotonicallyLiked(testBranchDAG.branch15.ID(), true)
	require.NoError(t, err)
	assert.True(t, modified)

	eventMock.Expect("BranchFinalized", testBranchDAG.branch2.ID())
	eventMock.Expect("BranchFinalized", testBranchDAG.branch3.ID())
	eventMock.Expect("BranchFinalized", testBranchDAG.branch6.ID())
	eventMock.Expect("BranchFinalized", testBranchDAG.branch7.ID())
	eventMock.Expect("BranchFinalized", testBranchDAG.branch10.ID())
	eventMock.Expect("BranchFinalized", testBranchDAG.branch11.ID())
	eventMock.Expect("BranchFinalized", testBranchDAG.branch12.ID())
	eventMock.Expect("BranchFinalized", testBranchDAG.branch13.ID())
	eventMock.Expect("BranchFinalized", testBranchDAG.branch14.ID())
	eventMock.Expect("BranchFinalized", testBranchDAG.branch15.ID())
	eventMock.Expect("BranchRejected", testBranchDAG.branch3.ID())
	eventMock.Expect("BranchRejected", testBranchDAG.branch6.ID())
	eventMock.Expect("BranchRejected", testBranchDAG.branch8.ID())
	eventMock.Expect("BranchRejected", testBranchDAG.branch10.ID())
	eventMock.Expect("BranchRejected", testBranchDAG.branch11.ID())
	eventMock.Expect("BranchRejected", testBranchDAG.branch13.ID())
	eventMock.Expect("BranchRejected", testBranchDAG.branch14.ID())
	eventMock.Expect("BranchConfirmed", testBranchDAG.branch2.ID())
	eventMock.Expect("BranchConfirmed", testBranchDAG.branch7.ID())
	eventMock.Expect("BranchConfirmed", testBranchDAG.branch12.ID())
	eventMock.Expect("BranchConfirmed", testBranchDAG.branch15.ID())

	modified, err = branchDAG.SetBranchFinalized(testBranchDAG.branch15.ID(), true)
	require.NoError(t, err)
	assert.True(t, modified)

	eventMock.Expect("BranchLiked", testBranchDAG.branch5.ID())
	eventMock.Expect("BranchMonotonicallyLiked", testBranchDAG.branch5.ID())
	eventMock.Expect("BranchLiked", testBranchDAG.branch9.ID())
	eventMock.Expect("BranchMonotonicallyLiked", testBranchDAG.branch9.ID())
	eventMock.Expect("BranchLiked", testBranchDAG.branch16.ID())
	eventMock.Expect("BranchMonotonicallyLiked", testBranchDAG.branch16.ID())

	modified, err = branchDAG.SetBranchMonotonicallyLiked(testBranchDAG.branch5.ID(), true)
	require.NoError(t, err)
	assert.True(t, modified)

	eventMock.Expect("BranchFinalized", testBranchDAG.branch4.ID())
	eventMock.Expect("BranchFinalized", testBranchDAG.branch5.ID())
	eventMock.Expect("BranchFinalized", testBranchDAG.branch8.ID())
	eventMock.Expect("BranchFinalized", testBranchDAG.branch9.ID())
	eventMock.Expect("BranchFinalized", testBranchDAG.branch16.ID())
	eventMock.Expect("BranchRejected", testBranchDAG.branch4.ID())
	eventMock.Expect("BranchRejected", testBranchDAG.branch8.ID())
	eventMock.Expect("BranchConfirmed", testBranchDAG.branch5.ID())
	eventMock.Expect("BranchConfirmed", testBranchDAG.branch9.ID())
	eventMock.Expect("BranchConfirmed", testBranchDAG.branch16.ID())

	modified, err = branchDAG.SetBranchFinalized(testBranchDAG.branch5.ID(), true)
	require.NoError(t, err)
	assert.True(t, modified)

	_, err = branchDAG.MergeToMaster(testBranchDAG.branch15.ID())
	require.Error(t, err)

	_, err = branchDAG.MergeToMaster(testBranchDAG.branch5.ID())
	require.Error(t, err)

	aggregatedBranch7Plus12 := NewAggregatedBranch(NewBranchIDs(testBranchDAG.branch7.ID(), testBranchDAG.branch12.ID()))
	eventMock.RegisterDebugAlias(aggregatedBranch7Plus12.ID(), "AggregatedBranch(7 + 12)")

	eventMock.Expect("BranchLiked", aggregatedBranch7Plus12.ID())
	eventMock.Expect("BranchMonotonicallyLiked", aggregatedBranch7Plus12.ID())
	eventMock.Expect("BranchFinalized", aggregatedBranch7Plus12.ID())
	eventMock.Expect("BranchConfirmed", aggregatedBranch7Plus12.ID())

	movedBranches, err := branchDAG.MergeToMaster(testBranchDAG.branch2.ID())
	require.NoError(t, err)

	assert.Equal(t, map[BranchID]BranchID{
		testBranchDAG.branch2.ID():  MasterBranchID,
		testBranchDAG.branch15.ID(): NewAggregatedBranch(NewBranchIDs(testBranchDAG.branch7.ID(), testBranchDAG.branch12.ID())).ID(),
	}, movedBranches)

	movedBranches, err = branchDAG.MergeToMaster(testBranchDAG.branch12.ID())
	require.NoError(t, err)

	assert.Equal(t, map[BranchID]BranchID{
		testBranchDAG.branch12.ID(): MasterBranchID,
		NewAggregatedBranch(NewBranchIDs(testBranchDAG.branch7.ID(), testBranchDAG.branch12.ID())).ID(): testBranchDAG.branch7.ID(),
		testBranchDAG.branch16.ID(): testBranchDAG.branch9.ID(),
	}, movedBranches)

	cachedAggregatedBranch, _, err := branchDAG.AggregateBranches(NewBranchIDs(testBranchDAG.branch11.ID(), MasterBranchID))
	require.NoError(t, err)
	cachedAggregatedBranch.Release()
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

	if result.cachedBranch3, _, err = branchDAG.CreateConflictBranch(BranchID{3}, NewBranchIDs(MasterBranchID), NewConflictIDs(ConflictID{0})); err != nil {
		return
	}
	if result.branch3, err = result.cachedBranch3.UnwrapConflictBranch(); err != nil {
		return
	}

	if result.cachedBranch4, _, err = branchDAG.CreateConflictBranch(BranchID{4}, NewBranchIDs(result.branch2.ID()), NewConflictIDs(ConflictID{1})); err != nil {
		return
	}
	if result.branch4, err = result.cachedBranch4.UnwrapConflictBranch(); err != nil {
		return
	}

	if result.cachedBranch5, _, err = branchDAG.CreateConflictBranch(BranchID{5}, NewBranchIDs(result.branch2.ID()), NewConflictIDs(ConflictID{1})); err != nil {
		return
	}
	if result.branch5, err = result.cachedBranch5.UnwrapConflictBranch(); err != nil {
		return
	}

	if result.cachedBranch6, _, err = branchDAG.CreateConflictBranch(BranchID{6}, NewBranchIDs(MasterBranchID), NewConflictIDs(ConflictID{2})); err != nil {
		return
	}
	if result.branch6, err = result.cachedBranch6.UnwrapConflictBranch(); err != nil {
		return
	}

	if result.cachedBranch7, _, err = branchDAG.CreateConflictBranch(BranchID{7}, NewBranchIDs(MasterBranchID), NewConflictIDs(ConflictID{2})); err != nil {
		return
	}
	if result.branch7, err = result.cachedBranch7.UnwrapConflictBranch(); err != nil {
		return
	}

	if result.cachedBranch8, _, err = branchDAG.AggregateBranches(NewBranchIDs(result.branch4.ID(), result.branch6.ID())); err != nil {
		return
	}
	if result.branch8, err = result.cachedBranch8.UnwrapAggregatedBranch(); err != nil {
		return
	}

	if result.cachedBranch9, _, err = branchDAG.AggregateBranches(NewBranchIDs(result.branch5.ID(), result.branch7.ID())); err != nil {
		return
	}
	if result.branch9, err = result.cachedBranch9.UnwrapAggregatedBranch(); err != nil {
		return
	}

	if result.cachedBranch10, _, err = branchDAG.AggregateBranches(NewBranchIDs(result.branch3.ID(), result.branch6.ID())); err != nil {
		return
	}
	if result.branch10, err = result.cachedBranch10.UnwrapAggregatedBranch(); err != nil {
		return
	}

	if result.cachedBranch11, _, err = branchDAG.CreateConflictBranch(BranchID{11}, NewBranchIDs(MasterBranchID), NewConflictIDs(ConflictID{3})); err != nil {
		return
	}
	if result.branch11, err = result.cachedBranch11.UnwrapConflictBranch(); err != nil {
		return
	}

	if result.cachedBranch12, _, err = branchDAG.CreateConflictBranch(BranchID{12}, NewBranchIDs(MasterBranchID), NewConflictIDs(ConflictID{3})); err != nil {
		return
	}
	if result.branch12, err = result.cachedBranch12.UnwrapConflictBranch(); err != nil {
		return
	}

	if result.cachedBranch13, _, err = branchDAG.AggregateBranches(NewBranchIDs(result.branch6.ID(), result.branch11.ID())); err != nil {
		return
	}
	if result.branch13, err = result.cachedBranch13.UnwrapAggregatedBranch(); err != nil {
		return
	}

	if result.cachedBranch14, _, err = branchDAG.AggregateBranches(NewBranchIDs(result.branch10.ID(), result.branch13.ID())); err != nil {
		return
	}
	if result.branch14, err = result.cachedBranch14.UnwrapAggregatedBranch(); err != nil {
		return
	}

	if result.cachedBranch15, _, err = branchDAG.AggregateBranches(NewBranchIDs(result.branch2.ID(), result.branch7.ID(), result.branch12.ID())); err != nil {
		return
	}
	if result.branch15, err = result.cachedBranch15.UnwrapAggregatedBranch(); err != nil {
		return
	}

	if result.cachedBranch16, _, err = branchDAG.AggregateBranches(NewBranchIDs(result.branch9.ID(), result.branch15.ID())); err != nil {
		return
	}
	if result.branch16, err = result.cachedBranch16.UnwrapAggregatedBranch(); err != nil {
		return
	}

	return
}

func (t *testBranchDAG) AssertInitialState(testingT *testing.T) {
	for _, branch := range []Branch{
		t.branch2,
		t.branch3,
		t.branch4,
		t.branch5,
		t.branch6,
		t.branch7,
		t.branch8,
		t.branch9,
		t.branch10,
		t.branch11,
		t.branch12,
		t.branch13,
		t.branch14,
		t.branch15,
		t.branch16,
	} {
		assert.False(testingT, branch.Liked())
		assert.False(testingT, branch.Finalized())
		assert.False(testingT, branch.MonotonicallyLiked())
		assert.Equal(testingT, Pending, branch.InclusionState())
	}
}

func (t *testBranchDAG) RegisterDebugAliases(eventMock *eventMock) {
	eventMock.RegisterDebugAlias(t.branch2.ID(), "ConflictBranch(2)")
	eventMock.RegisterDebugAlias(t.branch3.ID(), "ConflictBranch(3)")
	eventMock.RegisterDebugAlias(t.branch4.ID(), "ConflictBranch(4)")
	eventMock.RegisterDebugAlias(t.branch5.ID(), "ConflictBranch(5)")
	eventMock.RegisterDebugAlias(t.branch6.ID(), "ConflictBranch(6)")
	eventMock.RegisterDebugAlias(t.branch7.ID(), "ConflictBranch(7)")
	eventMock.RegisterDebugAlias(t.branch8.ID(), "AggregatedBranch(8)")
	eventMock.RegisterDebugAlias(t.branch9.ID(), "AggregatedBranch(9)")
	eventMock.RegisterDebugAlias(t.branch10.ID(), "AggregatedBranch(10)")
	eventMock.RegisterDebugAlias(t.branch11.ID(), "ConflictBranch(11)")
	eventMock.RegisterDebugAlias(t.branch12.ID(), "ConflictBranch(12)")
	eventMock.RegisterDebugAlias(t.branch13.ID(), "AggregatedBranch(13)")
	eventMock.RegisterDebugAlias(t.branch14.ID(), "AggregatedBranch(14)")
	eventMock.RegisterDebugAlias(t.branch15.ID(), "AggregatedBranch(15)")
	eventMock.RegisterDebugAlias(t.branch16.ID(), "AggregatedBranch(16)")
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

func newEventMock(t *testing.T, mgr *BranchDAG) *eventMock {
	e := &eventMock{
		debugAlias: make(map[BranchID]string),
		test:       t,
	}
	e.Test(t)

	// attach all events
	e.attach(mgr.Events.BranchLiked, e.BranchLiked)
	e.attach(mgr.Events.BranchDisliked, e.BranchDisliked)
	e.attach(mgr.Events.BranchMonotonicallyLiked, e.BranchMonotonicallyLiked)
	e.attach(mgr.Events.BranchMonotonicallyDisliked, e.BranchMonotonicallyDisliked)
	e.attach(mgr.Events.BranchFinalized, e.BranchFinalized)
	e.attach(mgr.Events.BranchUnfinalized, e.BranchUnfinalized)
	e.attach(mgr.Events.BranchConfirmed, e.BranchConfirmed)
	e.attach(mgr.Events.BranchRejected, e.BranchRejected)
	e.attach(mgr.Events.BranchPending, e.BranchPending)

	// assure that all available events are mocked
	numEvents := reflect.ValueOf(mgr.Events).Elem().NumField()
	assert.Equalf(t, len(e.attached), numEvents, "not all events in BranchManager.Events have been attached")

	return e
}

func (e *eventMock) RegisterDebugAlias(branchID BranchID, debugAlias string) {
	e.debugAlias[branchID] = debugAlias
}

func (e *eventMock) DetachAll() {
	for _, a := range e.attached {
		a.Event.Detach(a.Closure)
	}
}

func (e *eventMock) Expect(eventName string, arguments ...interface{}) {
	e.On(eventName, arguments...)
	e.expectedEvents++
}

func (e *eventMock) attach(event *events.Event, f interface{}) {
	closure := events.NewClosure(f)
	event.Attach(closure)
	e.attached = append(e.attached, struct {
		*events.Event
		*events.Closure
	}{event, closure})
}

func (e *eventMock) AssertExpectations(t mock.TestingT) bool {
	if e.calledEvents != e.expectedEvents {
		t.Errorf("number of called (%d) events is not equal to number of expected events (%d)", e.calledEvents, e.expectedEvents)
		return false
	}

	return e.Mock.AssertExpectations(t)
}

func (e *eventMock) BranchLiked(cachedBranch *BranchDAGEvent) {
	if debugAlias, exists := e.debugAlias[cachedBranch.Branch.Unwrap().ID()]; exists {
		e.test.Logf("EVENT TRIGGERED:\tBranchLiked(%s)", debugAlias)
	}

	defer cachedBranch.Release()
	e.Called(cachedBranch.Branch.Unwrap().ID())

	e.calledEvents++
}

func (e *eventMock) BranchDisliked(cachedBranch *BranchDAGEvent) {
	if debugAlias, exists := e.debugAlias[cachedBranch.Branch.Unwrap().ID()]; exists {
		e.test.Logf("EVENT TRIGGERED:\tBranchDisliked(%s)", debugAlias)
	}

	defer cachedBranch.Release()
	e.Called(cachedBranch.Branch.Unwrap().ID())

	e.calledEvents++
}

func (e *eventMock) BranchMonotonicallyLiked(cachedBranch *BranchDAGEvent) {
	if debugAlias, exists := e.debugAlias[cachedBranch.Branch.Unwrap().ID()]; exists {
		e.test.Logf("EVENT TRIGGERED:\tBranchMonotonicallyLiked(%s)", debugAlias)
	}

	defer cachedBranch.Release()
	e.Called(cachedBranch.Branch.Unwrap().ID())

	e.calledEvents++
}

func (e *eventMock) BranchMonotonicallyDisliked(cachedBranch *BranchDAGEvent) {
	if debugAlias, exists := e.debugAlias[cachedBranch.Branch.Unwrap().ID()]; exists {
		e.test.Logf("EVENT TRIGGERED:\tBranchMonotonicallyDisliked(%s)", debugAlias)
	}

	defer cachedBranch.Release()
	e.Called(cachedBranch.Branch.Unwrap().ID())

	e.calledEvents++
}

func (e *eventMock) BranchFinalized(cachedBranch *BranchDAGEvent) {
	if debugAlias, exists := e.debugAlias[cachedBranch.Branch.Unwrap().ID()]; exists {
		e.test.Logf("EVENT TRIGGERED:\tBranchFinalized(%s)", debugAlias)
	}

	defer cachedBranch.Release()
	e.Called(cachedBranch.Branch.Unwrap().ID())

	e.calledEvents++
}

func (e *eventMock) BranchUnfinalized(cachedBranch *BranchDAGEvent) {
	if debugAlias, exists := e.debugAlias[cachedBranch.Branch.Unwrap().ID()]; exists {
		e.test.Logf("EVENT TRIGGERED:\tBranchUnfinalized(%s)", debugAlias)
	}

	defer cachedBranch.Release()
	e.Called(cachedBranch.Branch.Unwrap().ID())

	e.calledEvents++
}

func (e *eventMock) BranchConfirmed(cachedBranch *BranchDAGEvent) {
	if debugAlias, exists := e.debugAlias[cachedBranch.Branch.Unwrap().ID()]; exists {
		e.test.Logf("EVENT TRIGGERED:\tBranchConfirmed(%s)", debugAlias)
	}

	defer cachedBranch.Release()
	e.Called(cachedBranch.Branch.Unwrap().ID())

	e.calledEvents++
}

func (e *eventMock) BranchRejected(cachedBranch *BranchDAGEvent) {
	if debugAlias, exists := e.debugAlias[cachedBranch.Branch.Unwrap().ID()]; exists {
		e.test.Logf("EVENT TRIGGERED:\tBranchRejected(%s)", debugAlias)
	}

	defer cachedBranch.Release()
	e.Called(cachedBranch.Branch.Unwrap().ID())

	e.calledEvents++
}

func (e *eventMock) BranchPending(cachedBranch *BranchDAGEvent) {
	if debugAlias, exists := e.debugAlias[cachedBranch.Branch.Unwrap().ID()]; exists {
		e.test.Logf("EVENT TRIGGERED:\tBranchPending(%s)", debugAlias)
	}

	defer cachedBranch.Release()
	e.Called(cachedBranch.Branch.Unwrap().ID())

	e.calledEvents++
}
