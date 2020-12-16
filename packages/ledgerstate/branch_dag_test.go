package ledgerstate

import (
	"fmt"
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
	defer branchDAG.Shutdown()

	cachedConflictBranch1, newBranchCreated, err := branchDAG.RetrieveConflictBranch(BranchID{2}, NewBranchIDs(MasterBranchID), NewConflictIDs(ConflictID{0}, ConflictID{1}))
	require.NoError(t, err)
	defer cachedConflictBranch1.Release()
	conflictBranch1, err := cachedConflictBranch1.UnwrapConflictBranch()
	require.NoError(t, err)
	assert.True(t, newBranchCreated)
	assert.Equal(t, NewBranchIDs(MasterBranchID), conflictBranch1.Parents())
	assert.Equal(t, ConflictBranchType, conflictBranch1.Type())
	assert.False(t, conflictBranch1.Preferred())
	assert.False(t, conflictBranch1.Liked())
	assert.False(t, conflictBranch1.Finalized())
	assert.Equal(t, Pending, conflictBranch1.InclusionState())
	assert.Equal(t, NewConflictIDs(ConflictID{0}, ConflictID{1}), conflictBranch1.Conflicts())

	cachedConflictBranch2, _, err := branchDAG.RetrieveConflictBranch(BranchID{3}, NewBranchIDs(conflictBranch1.ID()), NewConflictIDs(ConflictID{0}, ConflictID{1}, ConflictID{2}))
	require.NoError(t, err)
	defer cachedConflictBranch2.Release()
	conflictBranch2, err := cachedConflictBranch2.UnwrapConflictBranch()
	require.NoError(t, err)
	assert.True(t, newBranchCreated)
	assert.Equal(t, NewBranchIDs(conflictBranch1.ID()), conflictBranch2.Parents())
	assert.Equal(t, ConflictBranchType, conflictBranch2.Type())
	assert.False(t, conflictBranch2.Preferred())
	assert.False(t, conflictBranch2.Liked())
	assert.False(t, conflictBranch2.Finalized())
	assert.Equal(t, Pending, conflictBranch2.InclusionState())
	assert.Equal(t, NewConflictIDs(ConflictID{0}, ConflictID{1}, ConflictID{2}), conflictBranch2.Conflicts())

	cachedConflictBranch3, newBranchCreated, err := branchDAG.RetrieveConflictBranch(BranchID{2}, NewBranchIDs(MasterBranchID), NewConflictIDs(ConflictID{0}, ConflictID{1}, ConflictID{2}))
	require.NoError(t, err)
	defer cachedConflictBranch3.Release()
	conflictBranch3, err := cachedConflictBranch1.UnwrapConflictBranch()
	require.NoError(t, err)
	assert.False(t, newBranchCreated)
	assert.Equal(t, NewConflictIDs(ConflictID{0}, ConflictID{1}, ConflictID{2}), conflictBranch3.Conflicts())
}

func TestBranchDAG_normalizeBranches(t *testing.T) {
	branchDAG := NewBranchDAG(mapdb.NewMapDB())
	defer branchDAG.Shutdown()

	cachedBranch2, newBranchCreated, _ := branchDAG.RetrieveConflictBranch(BranchID{2}, NewBranchIDs(MasterBranchID), NewConflictIDs(ConflictID{0}))
	defer cachedBranch2.Release()
	branch2 := cachedBranch2.Unwrap()
	assert.True(t, newBranchCreated)

	cachedBranch3, newBranchCreated, _ := branchDAG.RetrieveConflictBranch(BranchID{3}, NewBranchIDs(MasterBranchID), NewConflictIDs(ConflictID{0}))
	defer cachedBranch3.Release()
	branch3 := cachedBranch3.Unwrap()
	assert.True(t, newBranchCreated)

	{
		normalizedBranches, err := branchDAG.normalizeBranches(NewBranchIDs(MasterBranchID, branch2.ID()))
		assert.NoError(t, err)
		assert.Equal(t, normalizedBranches, NewBranchIDs(branch2.ID()))

		normalizedBranches, err = branchDAG.normalizeBranches(NewBranchIDs(MasterBranchID, branch3.ID()))
		assert.NoError(t, err)
		assert.Equal(t, normalizedBranches, NewBranchIDs(branch3.ID()))

		_, err = branchDAG.normalizeBranches(NewBranchIDs(branch2.ID(), branch3.ID()))
		assert.Error(t, err)
	}

	// spawn of branch 4 and 5 from branch 2
	cachedBranch4, newBranchCreated, _ := branchDAG.RetrieveConflictBranch(BranchID{4}, NewBranchIDs(branch2.ID()), NewConflictIDs(ConflictID{1}))
	defer cachedBranch4.Release()
	branch4 := cachedBranch4.Unwrap()
	assert.True(t, newBranchCreated)

	cachedBranch5, newBranchCreated, _ := branchDAG.RetrieveConflictBranch(BranchID{5}, NewBranchIDs(branch2.ID()), NewConflictIDs(ConflictID{1}))
	defer cachedBranch5.Release()
	branch5 := cachedBranch5.Unwrap()
	assert.True(t, newBranchCreated)

	{
		normalizedBranches, err := branchDAG.normalizeBranches(NewBranchIDs(MasterBranchID, branch4.ID()))
		assert.NoError(t, err)
		assert.Equal(t, NewBranchIDs(branch4.ID()), normalizedBranches)

		_, err = branchDAG.normalizeBranches(NewBranchIDs(branch3.ID(), branch4.ID()))
		assert.Error(t, err)

		normalizedBranches, err = branchDAG.normalizeBranches(NewBranchIDs(MasterBranchID, branch5.ID()))
		assert.NoError(t, err)
		assert.Equal(t, NewBranchIDs(branch5.ID()), normalizedBranches)

		_, err = branchDAG.normalizeBranches(NewBranchIDs(branch3.ID(), branch5.ID()))
		assert.Error(t, err)

		// since both consume the same output
		_, err = branchDAG.normalizeBranches(NewBranchIDs(branch4.ID(), branch5.ID()))
		assert.Error(t, err)
	}

	// branch 6, 7 are on the same level as 2 and 3 but are not part of that conflict set
	cachedBranch6, newBranchCreated, _ := branchDAG.RetrieveConflictBranch(BranchID{6}, NewBranchIDs(MasterBranchID), NewConflictIDs(ConflictID{2}))
	defer cachedBranch6.Release()
	branch6 := cachedBranch6.Unwrap()
	assert.True(t, newBranchCreated)

	cachedBranch7, newBranchCreated, _ := branchDAG.RetrieveConflictBranch(BranchID{7}, NewBranchIDs(MasterBranchID), NewConflictIDs(ConflictID{2}))
	defer cachedBranch7.Release()
	branch7 := cachedBranch7.Unwrap()
	assert.True(t, newBranchCreated)

	{
		normalizedBranches, err := branchDAG.normalizeBranches(NewBranchIDs(branch2.ID(), branch6.ID()))
		assert.NoError(t, err)
		assert.Equal(t, NewBranchIDs(branch2.ID(), branch6.ID()), normalizedBranches)

		normalizedBranches, err = branchDAG.normalizeBranches(NewBranchIDs(branch3.ID(), branch6.ID()))
		assert.NoError(t, err)
		assert.Equal(t, NewBranchIDs(branch3.ID(), branch6.ID()), normalizedBranches)

		normalizedBranches, err = branchDAG.normalizeBranches(NewBranchIDs(branch2.ID(), branch7.ID()))
		assert.NoError(t, err)
		assert.Equal(t, NewBranchIDs(branch2.ID(), branch7.ID()), normalizedBranches)

		normalizedBranches, err = branchDAG.normalizeBranches(NewBranchIDs(branch3.ID(), branch7.ID()))
		assert.NoError(t, err)
		assert.Equal(t, NewBranchIDs(branch3.ID(), branch7.ID()), normalizedBranches)

		_, err = branchDAG.normalizeBranches(NewBranchIDs(branch6.ID(), branch7.ID()))
		assert.Error(t, err)

		normalizedBranches, err = branchDAG.normalizeBranches(NewBranchIDs(branch4.ID(), branch6.ID()))
		assert.NoError(t, err)
		assert.Equal(t, NewBranchIDs(branch4.ID(), branch6.ID()), normalizedBranches)

		normalizedBranches, err = branchDAG.normalizeBranches(NewBranchIDs(branch5.ID(), branch6.ID()))
		assert.NoError(t, err)
		assert.Equal(t, NewBranchIDs(branch5.ID(), branch6.ID()), normalizedBranches)

		normalizedBranches, err = branchDAG.normalizeBranches(NewBranchIDs(branch4.ID(), branch7.ID()))
		assert.NoError(t, err)
		assert.Equal(t, NewBranchIDs(branch4.ID(), branch7.ID()), normalizedBranches)

		normalizedBranches, err = branchDAG.normalizeBranches(NewBranchIDs(branch5.ID(), branch7.ID()))
		assert.NoError(t, err)
		assert.Equal(t, NewBranchIDs(branch5.ID(), branch7.ID()), normalizedBranches)
	}

	// aggregated branch out of branch 4 (child of branch 2) and branch 6
	cachedAggrBranch8, newBranchCreated, aggrBranchErr := branchDAG.RetrieveAggregatedBranch(NewBranchIDs(branch4.ID(), branch6.ID()))
	assert.NoError(t, aggrBranchErr)
	defer cachedAggrBranch8.Release()
	aggrBranch8 := cachedAggrBranch8.Unwrap()
	assert.True(t, newBranchCreated)

	{
		normalizedBranches, err := branchDAG.normalizeBranches(NewBranchIDs(aggrBranch8.ID(), MasterBranchID))
		assert.NoError(t, err)
		assert.Equal(t, NewBranchIDs(branch4.ID(), branch6.ID()), normalizedBranches)

		normalizedBranches, err = branchDAG.normalizeBranches(NewBranchIDs(aggrBranch8.ID(), branch2.ID()))
		assert.NoError(t, err)
		assert.Equal(t, NewBranchIDs(branch4.ID(), branch6.ID()), normalizedBranches)

		// conflicting since branch 2 and branch 3 are
		_, err = branchDAG.normalizeBranches(NewBranchIDs(aggrBranch8.ID(), branch3.ID()))
		assert.Error(t, err)

		// conflicting since branch 4 and branch 5 are
		_, err = branchDAG.normalizeBranches(NewBranchIDs(aggrBranch8.ID(), branch5.ID()))
		assert.Error(t, err)

		// conflicting since branch 6 and branch 7 are
		_, err = branchDAG.normalizeBranches(NewBranchIDs(aggrBranch8.ID(), branch7.ID()))
		assert.Error(t, err)
	}

	// aggregated branch out of aggr. branch 8 and branch 7:
	// should fail since branch 6 & 7 are conflicting
	_, newBrachCreated, aggrBranchErr := branchDAG.RetrieveAggregatedBranch(NewBranchIDs(aggrBranch8.ID(), branch7.ID()))
	assert.Error(t, aggrBranchErr)
	assert.False(t, newBrachCreated)

	// aggregated branch out of branch 5 (child of branch 2) and branch 7
	cachedAggrBranch9, newBranchCreated, aggrBranchErr := branchDAG.RetrieveAggregatedBranch(NewBranchIDs(branch5.ID(), branch7.ID()))
	assert.NoError(t, aggrBranchErr)
	defer cachedAggrBranch9.Release()
	aggrBranch9 := cachedAggrBranch9.Unwrap()
	assert.True(t, newBranchCreated)

	{
		normalizedBranches, err := branchDAG.normalizeBranches(NewBranchIDs(aggrBranch9.ID(), MasterBranchID))
		assert.NoError(t, err)
		assert.Equal(t, NewBranchIDs(branch5.ID(), branch7.ID()), normalizedBranches)

		// aggr. branch 8 and 9 should be conflicting, since 4 & 5 and 6 & 7 are
		_, err = branchDAG.normalizeBranches(NewBranchIDs(aggrBranch8.ID(), aggrBranch9.ID()))
		assert.Error(t, err)

		// conflicting since branch 3 & 2 are
		_, err = branchDAG.normalizeBranches(NewBranchIDs(branch3.ID(), aggrBranch9.ID()))
		assert.Error(t, err)
	}

	// aggregated branch out of branch 3 and branch 6
	cachedAggrBranch10, newBranchCreated, aggrBranchErr := branchDAG.RetrieveAggregatedBranch(NewBranchIDs(branch3.ID(), branch6.ID()))
	assert.NoError(t, aggrBranchErr)
	defer cachedAggrBranch10.Release()
	aggrBranch10 := cachedAggrBranch10.Unwrap()
	assert.True(t, newBranchCreated)

	{
		normalizedBranches, err := branchDAG.normalizeBranches(NewBranchIDs(aggrBranch10.ID(), MasterBranchID))
		assert.NoError(t, err)
		assert.Equal(t, NewBranchIDs(branch3.ID(), branch6.ID()), normalizedBranches)

		// aggr. branch 8 and 10 should be conflicting, since 2 & 3 are
		_, err = branchDAG.normalizeBranches(NewBranchIDs(aggrBranch8.ID(), aggrBranch10.ID()))
		assert.Error(t, err)

		// aggr. branch 9 and 10 should be conflicting, since 2 & 3 and 6 & 7 are
		_, err = branchDAG.normalizeBranches(NewBranchIDs(aggrBranch9.ID(), aggrBranch10.ID()))
		assert.Error(t, err)
	}

	// branch 11, 12 are on the same level as 2 & 3 and 6 & 7 but are not part of either conflict set
	cachedBranch11, newBranchCreated, _ := branchDAG.RetrieveConflictBranch(BranchID{11}, NewBranchIDs(MasterBranchID), NewConflictIDs(ConflictID{3}))
	defer cachedBranch11.Release()
	branch11 := cachedBranch11.Unwrap()
	assert.True(t, newBranchCreated)

	cachedBranch12, newBranchCreated, _ := branchDAG.RetrieveConflictBranch(BranchID{12}, NewBranchIDs(MasterBranchID), NewConflictIDs(ConflictID{3}))
	defer cachedBranch12.Release()
	branch12 := cachedBranch12.Unwrap()
	assert.True(t, newBranchCreated)

	{
		normalizedBranches, err := branchDAG.normalizeBranches(NewBranchIDs(MasterBranchID, branch11.ID()))
		assert.NoError(t, err)
		assert.Equal(t, NewBranchIDs(branch11.ID()), normalizedBranches)

		normalizedBranches, err = branchDAG.normalizeBranches(NewBranchIDs(MasterBranchID, branch12.ID()))
		assert.NoError(t, err)
		assert.Equal(t, NewBranchIDs(branch12.ID()), normalizedBranches)

		_, err = branchDAG.normalizeBranches(NewBranchIDs(branch11.ID(), branch12.ID()))
		assert.Error(t, err)
	}

	// aggr. branch 13 out of branch 6 and 11
	cachedAggrBranch13, newBranchCreated, aggrBranchErr := branchDAG.RetrieveAggregatedBranch(NewBranchIDs(branch6.ID(), branch11.ID()))
	assert.NoError(t, aggrBranchErr)
	defer cachedAggrBranch13.Release()
	aggrBranch13 := cachedAggrBranch13.Unwrap()
	assert.True(t, newBranchCreated)

	{
		_, err := branchDAG.normalizeBranches(NewBranchIDs(aggrBranch13.ID(), aggrBranch9.ID()))
		assert.Error(t, err)

		normalizedBranches, err := branchDAG.normalizeBranches(NewBranchIDs(aggrBranch13.ID(), aggrBranch8.ID()))
		assert.NoError(t, err)
		assert.Equal(t, NewBranchIDs(branch4.ID(), branch6.ID(), branch11.ID()), normalizedBranches)

		normalizedBranches, err = branchDAG.normalizeBranches(NewBranchIDs(aggrBranch13.ID(), aggrBranch10.ID()))
		assert.NoError(t, err)
		assert.Equal(t, NewBranchIDs(branch3.ID(), branch6.ID(), branch11.ID()), normalizedBranches)
	}

	// aggr. branch 14 out of aggr. branch 10 and 13
	cachedAggrBranch14, newBranchCreated, aggrBranchErr := branchDAG.RetrieveAggregatedBranch(NewBranchIDs(aggrBranch10.ID(), aggrBranch13.ID()))
	assert.NoError(t, aggrBranchErr)
	defer cachedAggrBranch14.Release()
	aggrBranch14 := cachedAggrBranch14.Unwrap()
	assert.True(t, newBranchCreated)

	{
		// aggr. branch 9 has parent branch 7 which conflicts with ancestor branch 6 of aggr. branch 14
		_, err := branchDAG.normalizeBranches(NewBranchIDs(aggrBranch14.ID(), aggrBranch9.ID()))
		assert.Error(t, err)

		// aggr. branch has ancestor branch 2 which conflicts with ancestor branch 3 of aggr. branch 14
		_, err = branchDAG.normalizeBranches(NewBranchIDs(aggrBranch14.ID(), aggrBranch8.ID()))
		assert.Error(t, err)
	}

	// aggr. branch 15 out of branch 2, 7 and 12
	cachedAggrBranch15, newBranchCreated, aggrBranchErr := branchDAG.RetrieveAggregatedBranch(NewBranchIDs(branch2.ID(), branch7.ID(), branch12.ID()))
	assert.NoError(t, aggrBranchErr)
	defer cachedAggrBranch15.Release()
	aggrBranch15 := cachedAggrBranch15.Unwrap()
	assert.True(t, newBranchCreated)

	{
		// aggr. branch 13 has parent branches 11 & 6 which conflicts which conflicts with ancestor branches 12 & 7 of aggr. branch 15
		_, err := branchDAG.normalizeBranches(NewBranchIDs(aggrBranch15.ID(), aggrBranch13.ID()))
		assert.Error(t, err)

		// aggr. branch 10 has parent branches 3 & 6 which conflicts with ancestor branches 2 & 7 of aggr. branch 15
		_, err = branchDAG.normalizeBranches(NewBranchIDs(aggrBranch15.ID(), aggrBranch10.ID()))
		assert.Error(t, err)

		// aggr. branch 8 has parent branch 6 which conflicts with ancestor branch 7 of aggr. branch 15
		_, err = branchDAG.normalizeBranches(NewBranchIDs(aggrBranch15.ID(), aggrBranch8.ID()))
		assert.Error(t, err)
	}

	// aggr. branch 16 out of aggr. branches 15 and 9
	cachedAggrBranch16, newBranchCreated, aggrBranchErr := branchDAG.RetrieveAggregatedBranch(NewBranchIDs(aggrBranch15.ID(), aggrBranch9.ID()))
	assert.NoError(t, aggrBranchErr)
	defer cachedAggrBranch16.Release()
	aggrBranch16 := cachedAggrBranch16.Unwrap()
	assert.True(t, newBranchCreated)

	{
		// sanity check
		normalizedBranches, err := branchDAG.normalizeBranches(NewBranchIDs(aggrBranch16.ID(), aggrBranch9.ID()))
		assert.NoError(t, err)
		assert.Equal(t, NewBranchIDs(branch5.ID(), branch7.ID(), branch12.ID()), normalizedBranches)

		// sanity check
		normalizedBranches, err = branchDAG.normalizeBranches(NewBranchIDs(aggrBranch16.ID(), branch7.ID()))
		assert.NoError(t, err)
		assert.Equal(t, NewBranchIDs(branch5.ID(), branch7.ID(), branch12.ID()), normalizedBranches)

		_, err = branchDAG.normalizeBranches(NewBranchIDs(aggrBranch16.ID(), aggrBranch13.ID()))
		assert.Error(t, err)

		_, err = branchDAG.normalizeBranches(NewBranchIDs(aggrBranch16.ID(), aggrBranch14.ID()))
		assert.Error(t, err)

		_, err = branchDAG.normalizeBranches(NewBranchIDs(aggrBranch16.ID(), aggrBranch8.ID()))
		assert.Error(t, err)
	}
}

func TestBranchDAG_SetBranchPreferred(t *testing.T) {
	branchDAG := NewBranchDAG(mapdb.NewMapDB())
	defer branchDAG.Shutdown()

	event := newEventMock(t, branchDAG)
	defer event.DetachAll()

	cachedBranch2, newBranchCreated, _ := branchDAG.RetrieveConflictBranch(BranchID{2}, NewBranchIDs(MasterBranchID), NewConflictIDs(ConflictID{0}))
	defer cachedBranch2.Release()
	branch2 := cachedBranch2.Unwrap()
	assert.True(t, newBranchCreated)

	cachedBranch3, newBranchCreated, _ := branchDAG.RetrieveConflictBranch(BranchID{3}, NewBranchIDs(MasterBranchID), NewConflictIDs(ConflictID{0}))
	defer cachedBranch3.Release()
	branch3 := cachedBranch3.Unwrap()
	assert.True(t, newBranchCreated)

	assert.False(t, branch2.Preferred(), "branch 2 should not be preferred")
	assert.False(t, branch2.Liked(), "branch 2 should not be liked")
	assert.False(t, branch3.Preferred(), "branch 3 should not be preferred")
	assert.False(t, branch3.Liked(), "branch 3 should not be liked")

	cachedBranch4, newBranchCreated, _ := branchDAG.RetrieveConflictBranch(BranchID{4}, NewBranchIDs(branch2.ID()), NewConflictIDs(ConflictID{1}))
	defer cachedBranch4.Release()
	branch4 := cachedBranch4.Unwrap()
	assert.True(t, newBranchCreated)

	cachedBranch5, newBranchCreated, _ := branchDAG.RetrieveConflictBranch(BranchID{5}, NewBranchIDs(branch2.ID()), NewConflictIDs(ConflictID{1}))
	defer cachedBranch5.Release()
	branch5 := cachedBranch5.Unwrap()
	assert.True(t, newBranchCreated)

	// lets assume branch 4 is preferred since its underlying transaction was longer
	// solid than the avg. network delay before the conflicting transaction which created
	// the conflict set was received

	event.Expect("BranchPreferred", branch4)

	modified, err := branchDAG.SetBranchPreferred(branch4.ID(), true)
	assert.NoError(t, err)
	assert.True(t, modified)

	assert.True(t, branch4.Preferred(), "branch 4 should be preferred")
	// is not liked because its parents aren't liked, respectively branch 2
	assert.False(t, branch4.Liked(), "branch 4 should not be liked")
	assert.False(t, branch5.Preferred(), "branch 5 should not be preferred")
	assert.False(t, branch5.Liked(), "branch 5 should not be liked")

	// now branch 2 becomes preferred via FPC, this causes branch 2 to be liked (since
	// the master branch is liked) and its liked state propagates to branch 4 (but not branch 5)

	event.Expect("BranchPreferred", branch2)
	event.Expect("BranchLiked", branch2)
	event.Expect("BranchLiked", branch4)

	modified, err = branchDAG.SetBranchPreferred(branch2.ID(), true)
	assert.NoError(t, err)
	assert.True(t, modified)

	assert.True(t, branch2.Liked(), "branch 2 should be liked")
	assert.True(t, branch2.Preferred(), "branch 2 should be preferred")
	assert.True(t, branch4.Liked(), "branch 4 should be liked")
	assert.True(t, branch4.Preferred(), "branch 4 should still be preferred")
	assert.False(t, branch5.Liked(), "branch 5 should not be liked")
	assert.False(t, branch5.Preferred(), "branch 5 should not be preferred")

	// now the network decides that branch 5 is preferred (via FPC), thus branch 4 should lose its
	// preferred and liked state and branch 5 should instead become preferred and liked

	event.Expect("BranchPreferred", branch5)
	event.Expect("BranchLiked", branch5)
	event.Expect("BranchUnpreferred", branch4)
	event.Expect("BranchDisliked", branch4)

	modified, err = branchDAG.SetBranchPreferred(branch5.ID(), true)
	assert.NoError(t, err)
	assert.True(t, modified)

	// sanity check for branch 2 state
	assert.True(t, branch2.Liked(), "branch 2 should be liked")
	assert.True(t, branch2.Preferred(), "branch 2 should be preferred")

	// check that branch 4 is disliked and not preferred
	assert.False(t, branch4.Liked(), "branch 4 should be disliked")
	assert.False(t, branch4.Preferred(), "branch 4 should not be preferred")
	assert.True(t, branch5.Liked(), "branch 5 should be liked")
	assert.True(t, branch5.Preferred(), "branch 5 should be preferred")

	// check that all events have been triggered
	event.AssertExpectations(t)
}

func TestBranchDAG_SetBranchLiked(t *testing.T) {
	branchDAG := NewBranchDAG(mapdb.NewMapDB())
	defer branchDAG.Shutdown()

	event := newEventMock(t, branchDAG)
	defer event.DetachAll()

	cachedBranch2, newBranchCreated, _ := branchDAG.RetrieveConflictBranch(BranchID{2}, NewBranchIDs(MasterBranchID), NewConflictIDs(ConflictID{0}))
	defer cachedBranch2.Release()
	branch2 := cachedBranch2.Unwrap()
	event.RegisterDebugAlias(branch2.ID(), "branch2")
	assert.True(t, newBranchCreated)
	assert.False(t, branch2.Preferred())
	assert.False(t, branch2.Finalized())
	assert.False(t, branch2.Liked())
	assert.Equal(t, Pending, branch2.InclusionState())

	cachedBranch3, newBranchCreated, _ := branchDAG.RetrieveConflictBranch(BranchID{3}, NewBranchIDs(MasterBranchID), NewConflictIDs(ConflictID{0}))
	defer cachedBranch3.Release()
	branch3 := cachedBranch3.Unwrap()
	event.RegisterDebugAlias(branch3.ID(), "branch3")
	assert.True(t, newBranchCreated)
	assert.False(t, branch3.Preferred())
	assert.False(t, branch3.Finalized())
	assert.False(t, branch3.Liked())
	assert.Equal(t, Pending, branch3.InclusionState())

	cachedBranch4, newBranchCreated, _ := branchDAG.RetrieveConflictBranch(BranchID{4}, NewBranchIDs(branch2.ID()), NewConflictIDs(ConflictID{1}))
	defer cachedBranch4.Release()
	branch4 := cachedBranch4.Unwrap()
	event.RegisterDebugAlias(branch4.ID(), "branch4")
	assert.True(t, newBranchCreated)
	assert.False(t, branch4.Preferred())
	assert.False(t, branch4.Finalized())
	assert.False(t, branch4.Liked())
	assert.Equal(t, Pending, branch4.InclusionState())

	cachedBranch5, newBranchCreated, _ := branchDAG.RetrieveConflictBranch(BranchID{5}, NewBranchIDs(branch2.ID()), NewConflictIDs(ConflictID{1}))
	defer cachedBranch5.Release()
	branch5 := cachedBranch5.Unwrap()
	event.RegisterDebugAlias(branch5.ID(), "branch5")
	assert.True(t, newBranchCreated)
	assert.False(t, branch5.Preferred())
	assert.False(t, branch5.Finalized())
	assert.False(t, branch5.Liked())
	assert.Equal(t, Pending, branch5.InclusionState())

	cachedBranch6, newBranchCreated, _ := branchDAG.RetrieveConflictBranch(BranchID{6}, NewBranchIDs(MasterBranchID), NewConflictIDs(ConflictID{2}))
	defer cachedBranch6.Release()
	branch6 := cachedBranch6.Unwrap()
	event.RegisterDebugAlias(branch6.ID(), "branch6")
	assert.True(t, newBranchCreated)
	assert.False(t, branch6.Preferred())
	assert.False(t, branch6.Finalized())
	assert.False(t, branch6.Liked())
	assert.Equal(t, Pending, branch6.InclusionState())

	cachedBranch7, newBranchCreated, _ := branchDAG.RetrieveConflictBranch(BranchID{7}, NewBranchIDs(MasterBranchID), NewConflictIDs(ConflictID{2}))
	defer cachedBranch7.Release()
	branch7 := cachedBranch7.Unwrap()
	event.RegisterDebugAlias(branch7.ID(), "branch7")
	assert.True(t, newBranchCreated)
	assert.False(t, branch7.Preferred())
	assert.False(t, branch7.Finalized())
	assert.False(t, branch7.Liked())
	assert.Equal(t, Pending, branch7.InclusionState())

	cachedAggrBranch8, newBranchCreated, err := branchDAG.RetrieveAggregatedBranch(NewBranchIDs(branch4.ID(), branch6.ID()))
	assert.NoError(t, err)
	defer cachedAggrBranch8.Release()
	aggrBranch8 := cachedAggrBranch8.Unwrap()
	event.RegisterDebugAlias(aggrBranch8.ID(), "aggrBranch8")
	assert.True(t, newBranchCreated)
	assert.False(t, aggrBranch8.Preferred())
	assert.False(t, aggrBranch8.Finalized())
	assert.False(t, aggrBranch8.Liked())
	assert.Equal(t, Pending, aggrBranch8.InclusionState())

	cachedAggrBranch9, newBranchCreated, err := branchDAG.RetrieveAggregatedBranch(NewBranchIDs(branch5.ID(), branch7.ID()))
	assert.NoError(t, err)
	defer cachedAggrBranch9.Release()
	aggrBranch9 := cachedAggrBranch9.Unwrap()
	event.RegisterDebugAlias(aggrBranch9.ID(), "aggrBranch9")
	assert.True(t, newBranchCreated)
	assert.False(t, aggrBranch9.Preferred())
	assert.False(t, aggrBranch9.Finalized())
	assert.False(t, aggrBranch9.Liked())
	assert.Equal(t, Pending, aggrBranch9.InclusionState())

	// should not be preferred because only 6 is is preferred but not 3
	cachedAggrBranch10, newBranchCreated, err := branchDAG.RetrieveAggregatedBranch(NewBranchIDs(branch3.ID(), branch6.ID()))
	assert.NoError(t, err)
	defer cachedAggrBranch10.Release()
	aggrBranch10 := cachedAggrBranch10.Unwrap()
	event.RegisterDebugAlias(aggrBranch10.ID(), "aggrBranch10")
	assert.True(t, newBranchCreated)
	assert.False(t, aggrBranch10.Preferred())
	assert.False(t, aggrBranch10.Finalized())
	assert.False(t, aggrBranch10.Liked())
	assert.Equal(t, Pending, aggrBranch10.InclusionState())

	cachedBranch11, newBranchCreated, _ := branchDAG.RetrieveConflictBranch(BranchID{11}, NewBranchIDs(MasterBranchID), NewConflictIDs(ConflictID{3}))
	defer cachedBranch11.Release()
	branch11 := cachedBranch11.Unwrap()
	event.RegisterDebugAlias(branch11.ID(), "branch11")
	assert.True(t, newBranchCreated)
	assert.False(t, branch11.Preferred())
	assert.False(t, branch11.Finalized())
	assert.False(t, branch11.Liked())
	assert.Equal(t, Pending, branch11.InclusionState())

	cachedBranch12, newBranchCreated, _ := branchDAG.RetrieveConflictBranch(BranchID{12}, NewBranchIDs(MasterBranchID), NewConflictIDs(ConflictID{3}))
	defer cachedBranch12.Release()
	branch12 := cachedBranch12.Unwrap()
	event.RegisterDebugAlias(branch12.ID(), "branch12")
	assert.True(t, newBranchCreated)
	assert.False(t, branch12.Preferred())
	assert.False(t, branch12.Finalized())
	assert.False(t, branch12.Liked())
	assert.Equal(t, Pending, branch12.InclusionState())

	cachedAggrBranch13, newBranchCreated, err := branchDAG.RetrieveAggregatedBranch(NewBranchIDs(branch6.ID(), branch11.ID()))
	assert.NoError(t, err)
	defer cachedAggrBranch13.Release()
	aggrBranch13 := cachedAggrBranch13.Unwrap()
	event.RegisterDebugAlias(aggrBranch13.ID(), "aggrBranch13")
	assert.True(t, newBranchCreated)
	assert.False(t, aggrBranch13.Preferred())
	assert.False(t, aggrBranch13.Finalized())
	assert.False(t, aggrBranch13.Liked())
	assert.Equal(t, Pending, aggrBranch13.InclusionState())

	cachedAggrBranch14, newBranchCreated, err := branchDAG.RetrieveAggregatedBranch(NewBranchIDs(aggrBranch10.ID(), aggrBranch13.ID()))
	assert.NoError(t, err)
	defer cachedAggrBranch14.Release()
	aggrBranch14 := cachedAggrBranch14.Unwrap()
	event.RegisterDebugAlias(aggrBranch14.ID(), "aggrBranch14")
	assert.True(t, newBranchCreated)
	assert.False(t, aggrBranch14.Preferred())
	assert.False(t, aggrBranch14.Finalized())
	assert.False(t, aggrBranch14.Liked())
	assert.Equal(t, Pending, aggrBranch14.InclusionState())

	cachedAggrBranch15, newBranchCreated, err := branchDAG.RetrieveAggregatedBranch(NewBranchIDs(branch2.ID(), branch7.ID(), branch12.ID()))
	assert.NoError(t, err)
	defer cachedAggrBranch15.Release()
	aggrBranch15 := cachedAggrBranch15.Unwrap()
	event.RegisterDebugAlias(aggrBranch15.ID(), "aggrBranch15")
	assert.True(t, newBranchCreated)
	assert.False(t, aggrBranch15.Preferred())
	assert.False(t, aggrBranch15.Finalized())
	assert.False(t, aggrBranch15.Liked())
	assert.Equal(t, Pending, aggrBranch15.InclusionState())

	cachedAggrBranch16, newBranchCreated, err := branchDAG.RetrieveAggregatedBranch(NewBranchIDs(aggrBranch9.ID(), aggrBranch15.ID()))
	assert.NoError(t, err)
	defer cachedAggrBranch16.Release()
	aggrBranch16 := cachedAggrBranch16.Unwrap()
	event.RegisterDebugAlias(aggrBranch16.ID(), "aggrBranch16")
	assert.True(t, newBranchCreated)
	assert.False(t, aggrBranch16.Preferred())
	assert.False(t, aggrBranch16.Finalized())
	assert.False(t, aggrBranch16.Liked())
	assert.Equal(t, Pending, aggrBranch16.InclusionState())

	event.Expect("BranchPreferred", branch2)
	event.Expect("BranchPreferred", branch7)
	event.Expect("BranchPreferred", branch12)
	event.Expect("BranchLiked", branch2)
	event.Expect("BranchLiked", branch7)
	event.Expect("BranchLiked", branch12)
	event.Expect("BranchPreferred", branch5)
	event.Expect("BranchLiked", branch5)
	event.Expect("BranchPreferred", aggrBranch9)
	event.Expect("BranchPreferred", aggrBranch15)
	event.Expect("BranchPreferred", aggrBranch16)
	event.Expect("BranchLiked", aggrBranch9)
	event.Expect("BranchLiked", aggrBranch15)
	event.Expect("BranchLiked", aggrBranch16)

	modified, err := branchDAG.SetBranchLiked(aggrBranch16.ID(), true)
	assert.NoError(t, err)
	assert.True(t, modified)

	assert.True(t, branch2.Preferred())
	assert.True(t, branch2.Liked())
	assert.True(t, branch7.Preferred())
	assert.True(t, branch7.Liked())
	assert.True(t, branch12.Preferred())
	assert.True(t, branch12.Liked())
	assert.True(t, branch5.Preferred())
	assert.True(t, branch5.Liked())
	assert.True(t, aggrBranch9.Preferred())
	assert.True(t, aggrBranch9.Liked())
	assert.True(t, aggrBranch15.Preferred())
	assert.True(t, aggrBranch15.Liked())
	assert.True(t, aggrBranch16.Preferred())
	assert.True(t, aggrBranch16.Liked())

	event.Expect("BranchUnpreferred", branch5)
	event.Expect("BranchUnpreferred", branch7)
	event.Expect("BranchUnpreferred", aggrBranch9)
	event.Expect("BranchUnpreferred", aggrBranch15)
	event.Expect("BranchUnpreferred", aggrBranch16)
	event.Expect("BranchDisliked", branch5)
	event.Expect("BranchDisliked", branch7)
	event.Expect("BranchDisliked", aggrBranch9)
	event.Expect("BranchDisliked", aggrBranch15)
	event.Expect("BranchDisliked", aggrBranch16)
	event.Expect("BranchPreferred", branch4)
	event.Expect("BranchLiked", branch4)
	event.Expect("BranchPreferred", branch6)
	event.Expect("BranchLiked", branch6)
	event.Expect("BranchPreferred", aggrBranch8)
	event.Expect("BranchLiked", aggrBranch8)

	modified, err = branchDAG.SetBranchLiked(aggrBranch8.ID(), true)
	assert.NoError(t, err)
	assert.True(t, modified)

	assert.False(t, branch7.Preferred())
	assert.False(t, branch7.Liked())
	assert.False(t, branch5.Preferred())
	assert.False(t, branch5.Liked())
	assert.False(t, aggrBranch9.Preferred())
	assert.False(t, aggrBranch9.Liked())
	assert.False(t, aggrBranch15.Preferred())
	assert.False(t, aggrBranch15.Liked())
	assert.False(t, aggrBranch16.Preferred())
	assert.False(t, aggrBranch16.Liked())
	assert.True(t, branch4.Preferred())
	assert.True(t, branch4.Liked())
	assert.True(t, branch6.Preferred())
	assert.True(t, branch6.Liked())
	assert.True(t, aggrBranch8.Preferred())
	assert.True(t, aggrBranch8.Liked())

	// check that all events have been triggered
	event.AssertExpectations(t)
}

func TestBranchDAG_SetBranchPreferred2(t *testing.T) {
	branchDAG := NewBranchDAG(mapdb.NewMapDB())
	defer branchDAG.Shutdown()

	event := newEventMock(t, branchDAG)
	defer event.DetachAll()

	cachedBranch2, newBranchCreated, _ := branchDAG.RetrieveConflictBranch(BranchID{2}, NewBranchIDs(MasterBranchID), NewConflictIDs(ConflictID{0}))
	defer cachedBranch2.Release()
	branch2 := cachedBranch2.Unwrap()
	assert.True(t, newBranchCreated)

	cachedBranch3, newBranchCreated, _ := branchDAG.RetrieveConflictBranch(BranchID{3}, NewBranchIDs(MasterBranchID), NewConflictIDs(ConflictID{0}))
	defer cachedBranch3.Release()
	branch3 := cachedBranch3.Unwrap()
	assert.True(t, newBranchCreated)

	cachedBranch4, newBranchCreated, _ := branchDAG.RetrieveConflictBranch(BranchID{4}, NewBranchIDs(branch2.ID()), NewConflictIDs(ConflictID{1}))
	defer cachedBranch4.Release()
	branch4 := cachedBranch4.Unwrap()
	assert.True(t, newBranchCreated)

	cachedBranch5, newBranchCreated, _ := branchDAG.RetrieveConflictBranch(BranchID{5}, NewBranchIDs(branch2.ID()), NewConflictIDs(ConflictID{1}))
	defer cachedBranch5.Release()
	branch5 := cachedBranch5.Unwrap()
	assert.True(t, newBranchCreated)

	cachedBranch6, newBranchCreated, _ := branchDAG.RetrieveConflictBranch(BranchID{6}, NewBranchIDs(MasterBranchID), NewConflictIDs(ConflictID{2}))
	defer cachedBranch6.Release()
	branch6 := cachedBranch6.Unwrap()
	assert.True(t, newBranchCreated)

	cachedBranch7, newBranchCreated, _ := branchDAG.RetrieveConflictBranch(BranchID{7}, NewBranchIDs(MasterBranchID), NewConflictIDs(ConflictID{2}))
	defer cachedBranch7.Release()
	branch7 := cachedBranch7.Unwrap()
	assert.True(t, newBranchCreated)

	event.Expect("BranchPreferred", branch2)
	event.Expect("BranchLiked", branch2)
	event.Expect("BranchPreferred", branch6)
	event.Expect("BranchLiked", branch6)

	// assume branch 2 preferred since solid longer than avg. network delay
	modified, err := branchDAG.SetBranchPreferred(branch2.ID(), true)
	assert.NoError(t, err)
	assert.True(t, modified)

	// assume branch 6 preferred since solid longer than avg. network delay
	modified, err = branchDAG.SetBranchPreferred(branch6.ID(), true)
	assert.NoError(t, err)
	assert.True(t, modified)

	{
		assert.True(t, branch2.Liked(), "branch 2 should be liked")
		assert.True(t, branch2.Preferred(), "branch 2 should be preferred")
		assert.False(t, branch3.Liked(), "branch 3 should not be liked")
		assert.False(t, branch3.Preferred(), "branch 3 should not be preferred")
		assert.False(t, branch4.Liked(), "branch 4 should not be liked")
		assert.False(t, branch4.Preferred(), "branch 4 should not be preferred")
		assert.False(t, branch5.Liked(), "branch 5 should not be liked")
		assert.False(t, branch5.Preferred(), "branch 5 should not be preferred")
		assert.True(t, branch6.Liked(), "branch 6 should be liked")
		assert.True(t, branch6.Preferred(), "branch 6 should be preferred")
		assert.False(t, branch7.Liked(), "branch 7 should not be liked")
		assert.False(t, branch7.Preferred(), "branch 7 should not be preferred")
	}

	// throw some aggregated branches into the mix
	cachedAggrBranch8, newBranchCreated, err := branchDAG.RetrieveAggregatedBranch(NewBranchIDs(branch4.ID(), branch6.ID()))
	assert.NoError(t, err)
	defer cachedAggrBranch8.Release()
	aggrBranch8 := cachedAggrBranch8.Unwrap()
	assert.True(t, newBranchCreated)

	// should not be preferred because only 6 is is preferred but not 4
	assert.False(t, aggrBranch8.Liked(), "aggr. branch 8 should not be liked")
	assert.False(t, aggrBranch8.Preferred(), "aggr. branch 8 should not be preferred")

	cachedAggrBranch9, newBranchCreated, err := branchDAG.RetrieveAggregatedBranch(NewBranchIDs(branch5.ID(), branch7.ID()))
	assert.NoError(t, err)
	defer cachedAggrBranch9.Release()
	aggrBranch9 := cachedAggrBranch9.Unwrap()
	assert.True(t, newBranchCreated)

	// branch 5 and 7 are neither liked or preferred
	assert.False(t, aggrBranch9.Liked(), "aggr. branch 9 should not be liked")
	assert.False(t, aggrBranch9.Preferred(), "aggr. branch 9 should not be preferred")

	// should not be preferred because only 6 is is preferred but not 3
	cachedAggrBranch10, newBranchCreated, err := branchDAG.RetrieveAggregatedBranch(NewBranchIDs(branch3.ID(), branch6.ID()))
	assert.NoError(t, err)
	defer cachedAggrBranch10.Release()
	aggrBranch10 := cachedAggrBranch10.Unwrap()
	assert.True(t, newBranchCreated)

	assert.False(t, aggrBranch10.Liked(), "aggr. branch 10 should not be liked")
	assert.False(t, aggrBranch10.Preferred(), "aggr. branch 10 should not be preferred")

	// spawn off conflict branch 11 and 12
	cachedBranch11, newBranchCreated, _ := branchDAG.RetrieveConflictBranch(BranchID{11}, NewBranchIDs(aggrBranch8.ID()), NewConflictIDs(ConflictID{3}))
	defer cachedBranch11.Release()
	branch11 := cachedBranch11.Unwrap()
	assert.True(t, newBranchCreated)

	assert.False(t, branch11.Liked(), "aggr. branch 11 should not be liked")
	assert.False(t, branch11.Preferred(), "aggr. branch 11 should not be preferred")

	cachedBranch12, newBranchCreated, _ := branchDAG.RetrieveConflictBranch(BranchID{12}, NewBranchIDs(aggrBranch8.ID()), NewConflictIDs(ConflictID{3}))
	defer cachedBranch12.Release()
	branch12 := cachedBranch12.Unwrap()
	assert.True(t, newBranchCreated)

	assert.False(t, branch12.Liked(), "aggr. branch 12 should not be liked")
	assert.False(t, branch12.Preferred(), "aggr. branch 12 should not be preferred")

	cachedAggrBranch13, newBranchCreated, err := branchDAG.RetrieveAggregatedBranch(NewBranchIDs(branch4.ID(), branch12.ID()))
	assert.NoError(t, err)
	defer cachedAggrBranch13.Release()
	aggrBranch13 := cachedAggrBranch13.Unwrap()
	assert.True(t, newBranchCreated)

	assert.False(t, aggrBranch13.Liked(), "aggr. branch 13 should not be liked")
	assert.False(t, aggrBranch13.Preferred(), "aggr. branch 13 should not be preferred")

	// now lets assume FPC finalized on branch 2, 6 and 4 to be preferred.
	// branches 2 and 6 are already preferred but 4 is newly preferred. Branch 4 therefore
	// should also become liked, since branch 2 of which it spawns off is liked too.

	// simulate branch 3 being not preferred from FPC vote
	// this does not trigger any events as branch 3 was never preferred
	modified, err = branchDAG.SetBranchPreferred(branch3.ID(), false)
	assert.NoError(t, err)
	assert.False(t, modified)
	// simulate branch 7 being not preferred from FPC vote
	// this does not trigger any events as branch 7 was never preferred
	modified, err = branchDAG.SetBranchPreferred(branch7.ID(), false)
	assert.NoError(t, err)
	assert.False(t, modified)

	event.Expect("BranchPreferred", branch4)
	event.Expect("BranchLiked", branch4)
	event.Expect("BranchPreferred", aggrBranch8)
	event.Expect("BranchLiked", aggrBranch8)

	// simulate branch 4 being preferred by FPC vote
	modified, err = branchDAG.SetBranchPreferred(branch4.ID(), true)
	assert.NoError(t, err)
	assert.True(t, modified)
	assert.True(t, branch4.Liked(), "branch 4 should be liked")
	assert.True(t, branch4.Preferred(), "branch 4 should be preferred")

	// this should cause aggr. branch 8 to also be preferred and liked, since branch 6 and 4
	// of which it spawns off are.
	assert.True(t, aggrBranch8.Liked(), "aggr. branch 8 should be liked")
	assert.True(t, aggrBranch8.Preferred(), "aggr. branch 8 should be preferred")

	// check that all events have been triggered
	event.AssertExpectations(t)
}

func TestBranchDAG_ConflictMembers(t *testing.T) {
	branchDAG := NewBranchDAG(mapdb.NewMapDB())
	defer branchDAG.Shutdown()

	// create initial branches
	cachedBranch2, newBranchCreated, _ := branchDAG.RetrieveConflictBranch(BranchID{2}, NewBranchIDs(MasterBranchID), NewConflictIDs(ConflictID{0}))
	defer cachedBranch2.Release()
	branch2 := cachedBranch2.Unwrap()
	assert.True(t, newBranchCreated)
	cachedBranch3, newBranchCreated, _ := branchDAG.RetrieveConflictBranch(BranchID{3}, NewBranchIDs(MasterBranchID), NewConflictIDs(ConflictID{0}))
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
	cachedBranch4, newBranchCreated, _ := branchDAG.RetrieveConflictBranch(BranchID{4}, NewBranchIDs(MasterBranchID), NewConflictIDs(ConflictID{0}))
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

// eventMock is a wrapper around mock.Mock used to test the triggered events.
type eventMock struct {
	mock.Mock
	expectedEvents int
	calledEvents   int
	debugAlias     map[BranchID]string

	attached []struct {
		*events.Event
		*events.Closure
	}
}

func newEventMock(t *testing.T, mgr *BranchDAG) *eventMock {
	e := &eventMock{
		debugAlias: make(map[BranchID]string),
	}
	e.Test(t)

	// attach all events
	e.attach(mgr.Events.BranchPreferred, e.BranchPreferred)
	e.attach(mgr.Events.BranchUnpreferred, e.BranchUnpreferred)
	e.attach(mgr.Events.BranchLiked, e.BranchLiked)
	e.attach(mgr.Events.BranchDisliked, e.BranchDisliked)
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

// DetachAll detaches all attached event mocks.
func (e *eventMock) DetachAll() {
	for _, a := range e.attached {
		a.Event.Detach(a.Closure)
	}
}

// Expect starts a description of an expectation of the specified event being triggered.
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

func (e *eventMock) BranchPreferred(cachedBranch *BranchDAGEvent) {
	defer cachedBranch.Release()
	e.Called(cachedBranch.Branch.Unwrap())

	if debugAlias, exists := e.debugAlias[cachedBranch.Branch.Unwrap().ID()]; exists {
		fmt.Println("Called BranchPreferred(" + debugAlias + ")")
	}

	e.calledEvents++
}

func (e *eventMock) BranchUnpreferred(cachedBranch *BranchDAGEvent) {
	defer cachedBranch.Release()
	e.Called(cachedBranch.Branch.Unwrap())

	if debugAlias, exists := e.debugAlias[cachedBranch.Branch.Unwrap().ID()]; exists {
		fmt.Println("Called BranchUnpreferred(" + debugAlias + ")")
	}

	e.calledEvents++
}

func (e *eventMock) BranchLiked(cachedBranch *BranchDAGEvent) {
	defer cachedBranch.Release()
	e.Called(cachedBranch.Branch.Unwrap())

	if debugAlias, exists := e.debugAlias[cachedBranch.Branch.Unwrap().ID()]; exists {
		fmt.Println("Called BranchLiked(" + debugAlias + ")")
	}

	e.calledEvents++
}

func (e *eventMock) BranchDisliked(cachedBranch *BranchDAGEvent) {
	defer cachedBranch.Release()
	e.Called(cachedBranch.Branch.Unwrap())

	if debugAlias, exists := e.debugAlias[cachedBranch.Branch.Unwrap().ID()]; exists {
		fmt.Println("Called BranchDisliked(" + debugAlias + ")")
	}

	e.calledEvents++
}

func (e *eventMock) BranchFinalized(cachedBranch *BranchDAGEvent) {
	defer cachedBranch.Release()
	e.Called(cachedBranch.Branch.Unwrap())

	if debugAlias, exists := e.debugAlias[cachedBranch.Branch.Unwrap().ID()]; exists {
		fmt.Println("Called BranchFinalized(" + debugAlias + ")")
	}

	e.calledEvents++
}

func (e *eventMock) BranchUnfinalized(cachedBranch *BranchDAGEvent) {
	defer cachedBranch.Release()
	e.Called(cachedBranch.Branch.Unwrap())

	if debugAlias, exists := e.debugAlias[cachedBranch.Branch.Unwrap().ID()]; exists {
		fmt.Println("Called BranchUnfinalized(" + debugAlias + ")")
	}

	e.calledEvents++
}

func (e *eventMock) BranchConfirmed(cachedBranch *BranchDAGEvent) {
	defer cachedBranch.Release()
	e.Called(cachedBranch.Branch.Unwrap())

	if debugAlias, exists := e.debugAlias[cachedBranch.Branch.Unwrap().ID()]; exists {
		fmt.Println("Called BranchConfirmed(" + debugAlias + ")")
	}

	e.calledEvents++
}

func (e *eventMock) BranchRejected(cachedBranch *BranchDAGEvent) {
	defer cachedBranch.Release()
	e.Called(cachedBranch.Branch.Unwrap())

	if debugAlias, exists := e.debugAlias[cachedBranch.Branch.Unwrap().ID()]; exists {
		fmt.Println("Called BranchRejected(" + debugAlias + ")")
	}

	e.calledEvents++
}

func (e *eventMock) BranchPending(cachedBranch *BranchDAGEvent) {
	defer cachedBranch.Release()
	e.Called(cachedBranch.Branch.Unwrap())

	if debugAlias, exists := e.debugAlias[cachedBranch.Branch.Unwrap().ID()]; exists {
		fmt.Println("Called BranchPending(" + debugAlias + ")")
	}

	e.calledEvents++
}
