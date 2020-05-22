package branchmanager

import (
	"testing"

	"github.com/iotaledger/hive.go/kvstore/mapdb"
	"github.com/stretchr/testify/assert"
)

func TestBranchManager_ConflictMembers(t *testing.T) {
	branchManager := New(mapdb.NewMapDB())

	cachedBranch2, _ := branchManager.Fork(BranchID{2}, []BranchID{MasterBranchID}, []ConflictID{{0}})
	defer cachedBranch2.Release()
	branch2 := cachedBranch2.Unwrap()

	cachedBranch3, _ := branchManager.Fork(BranchID{3}, []BranchID{MasterBranchID}, []ConflictID{{0}})
	defer cachedBranch3.Release()
	branch3 := cachedBranch3.Unwrap()

	// assert conflict members
	expectedConflictMembers := map[BranchID]struct{}{
		branch2.ID(): {}, branch3.ID(): {},
	}
	actualConflictMembers := map[BranchID]struct{}{}
	branchManager.ConflictMembers(ConflictID{0}).Consume(func(conflictMember *ConflictMember) {
		actualConflictMembers[conflictMember.BranchID()] = struct{}{}
	})
	assert.Equal(t, expectedConflictMembers, actualConflictMembers)

	// add branch 4
	cachedBranch4, _ := branchManager.Fork(BranchID{4}, []BranchID{MasterBranchID}, []ConflictID{{0}})
	defer cachedBranch4.Release()
	branch4 := cachedBranch4.Unwrap()

	// branch 4 should now also be part of the conflict set
	expectedConflictMembers = map[BranchID]struct{}{
		branch2.ID(): {}, branch3.ID(): {}, branch4.ID(): {},
	}
	actualConflictMembers = map[BranchID]struct{}{}
	branchManager.ConflictMembers(ConflictID{0}).Consume(func(conflictMember *ConflictMember) {
		actualConflictMembers[conflictMember.BranchID()] = struct{}{}
	})
	assert.Equal(t, expectedConflictMembers, actualConflictMembers)
}

// ./img/testelevation.png
func TestBranchManager_ElevateConflictBranch(t *testing.T) {
	branchManager := New(mapdb.NewMapDB())

	cachedBranch2, _ := branchManager.Fork(BranchID{2}, []BranchID{MasterBranchID}, []ConflictID{{0}})
	defer cachedBranch2.Release()
	branch2 := cachedBranch2.Unwrap()

	cachedBranch3, _ := branchManager.Fork(BranchID{3}, []BranchID{MasterBranchID}, []ConflictID{{0}})
	defer cachedBranch3.Release()
	branch3 := cachedBranch3.Unwrap()

	cachedBranch4, _ := branchManager.Fork(BranchID{4}, []BranchID{branch2.ID()}, []ConflictID{{1}})
	defer cachedBranch4.Release()
	branch4 := cachedBranch4.Unwrap()

	cachedBranch5, _ := branchManager.Fork(BranchID{5}, []BranchID{branch2.ID()}, []ConflictID{{1}})
	defer cachedBranch5.Release()
	branch5 := cachedBranch5.Unwrap()

	cachedBranch6, _ := branchManager.Fork(BranchID{6}, []BranchID{branch4.ID()}, []ConflictID{{2}})
	defer cachedBranch6.Release()
	branch6 := cachedBranch6.Unwrap()

	cachedBranch7, _ := branchManager.Fork(BranchID{7}, []BranchID{branch4.ID()}, []ConflictID{{2}})
	defer cachedBranch7.Release()
	branch7 := cachedBranch7.Unwrap()

	// lets assume the conflict between 2 and 3 is resolved and therefore we elevate
	// branches 4 and 5 to the master branch
	isConflictBranch, modified, err := branchManager.ElevateConflictBranch(branch4.ID(), MasterBranchID)
	assert.NoError(t, err, "branch 4 should have been elevated to the master branch")
	assert.True(t, isConflictBranch, "branch 4 should have been a conflict branch")
	assert.True(t, modified, "branch 4 should have been modified")
	assert.Equal(t, MasterBranchID, branch4.ParentBranches()[0], "branch 4's parent should now be the master branch")

	isConflictBranch, modified, err = branchManager.ElevateConflictBranch(branch5.ID(), MasterBranchID)
	assert.NoError(t, err, "branch 5 should have been elevated to the master branch")
	assert.True(t, isConflictBranch, "branch 5 should have been a conflict branch")
	assert.True(t, modified, "branch 5 should have been modified")
	assert.Equal(t, MasterBranchID, branch5.ParentBranches()[0], "branch 5's parent should now be the master branch")

	// check whether the child branches are what we expect them to be of the master branch
	expectedMasterChildBranches := map[BranchID]struct{}{
		branch2.ID(): {}, branch3.ID(): {},
		branch4.ID(): {}, branch5.ID(): {},
	}
	actualMasterChildBranches := map[BranchID]struct{}{}
	branchManager.ChildBranches(MasterBranchID).Consume(func(childBranch *ChildBranch) {
		actualMasterChildBranches[childBranch.ChildID()] = struct{}{}
	})
	assert.Equal(t, expectedMasterChildBranches, actualMasterChildBranches)

	// check that 4 and 5 no longer are children of branch 2
	expectedBranch2ChildBranches := map[BranchID]struct{}{}
	actualBranch2ChildBranches := map[BranchID]struct{}{}
	branchManager.ChildBranches(branch2.ID()).Consume(func(childBranch *ChildBranch) {
		actualBranch2ChildBranches[childBranch.ChildID()] = struct{}{}
	})
	assert.Equal(t, expectedBranch2ChildBranches, actualBranch2ChildBranches)

	// lets assume the conflict between 4 and 5 is resolved and therefore we elevate
	// branches 6 and 7 to the master branch
	isConflictBranch, modified, err = branchManager.ElevateConflictBranch(branch6.ID(), MasterBranchID)
	assert.NoError(t, err, "branch 6 should have been elevated to the master branch")
	assert.True(t, isConflictBranch, "branch 6 should have been a conflict branch")
	assert.True(t, modified, "branch 6 should have been modified")
	assert.Equal(t, MasterBranchID, branch6.ParentBranches()[0], "branch 6's parent should now be the  master branch")

	isConflictBranch, modified, err = branchManager.ElevateConflictBranch(branch7.ID(), MasterBranchID)
	assert.NoError(t, err, "branch 7 should have been elevated to the master branch")
	assert.True(t, isConflictBranch, "branch 7 should have been a conflict branch")
	assert.True(t, modified, "branch 7 should have been modified")
	assert.Equal(t, MasterBranchID, branch7.ParentBranches()[0], "branch 7's parent should now be the  master branch")

	// check whether the child branches are what we expect them to be of the master branch
	expectedMasterChildBranches = map[BranchID]struct{}{
		branch2.ID(): {}, branch3.ID(): {},
		branch4.ID(): {}, branch5.ID(): {},
		branch6.ID(): {}, branch7.ID(): {},
	}
	actualMasterChildBranches = map[BranchID]struct{}{}
	branchManager.ChildBranches(MasterBranchID).Consume(func(childBranch *ChildBranch) {
		actualMasterChildBranches[childBranch.ChildID()] = struct{}{}
	})
	assert.Equal(t, expectedMasterChildBranches, actualMasterChildBranches)

	// check that 6 and 7 no longer are children of branch 4
	expectedBranch4ChildBranches := map[BranchID]struct{}{}
	actualBranch4ChildBranches := map[BranchID]struct{}{}
	branchManager.ChildBranches(branch4.ID()).Consume(func(childBranch *ChildBranch) {
		actualBranch4ChildBranches[childBranch.ChildID()] = struct{}{}
	})
	assert.Equal(t, expectedBranch4ChildBranches, actualBranch4ChildBranches)

	// TODO: branches are never deleted?
}

// ./img/testconflictdetection.png
func TestBranchManager_BranchesConflicting(t *testing.T) {
	branchManager := New(mapdb.NewMapDB())

	cachedBranch2, _ := branchManager.Fork(BranchID{2}, []BranchID{MasterBranchID}, []ConflictID{{0}})
	defer cachedBranch2.Release()
	branch2 := cachedBranch2.Unwrap()

	cachedBranch3, _ := branchManager.Fork(BranchID{3}, []BranchID{MasterBranchID}, []ConflictID{{0}})
	defer cachedBranch3.Release()
	branch3 := cachedBranch3.Unwrap()

	{
		conflicting, err := branchManager.BranchesConflicting(MasterBranchID, branch2.ID())
		assert.NoError(t, err)
		assert.False(t, conflicting, "branch 2 should not be conflicting with master branch")

		conflicting, err = branchManager.BranchesConflicting(MasterBranchID, branch2.ID())
		assert.NoError(t, err)
		assert.False(t, conflicting, "branch 3 should not be conflicting with master branch")

		conflicting, err = branchManager.BranchesConflicting(branch2.ID(), branch3.ID())
		assert.NoError(t, err)
		assert.True(t, conflicting, "branch 2 & 3 should be conflicting with each other")
	}

	// spawn of branch 4 and 5 from branch 2
	cachedBranch4, _ := branchManager.Fork(BranchID{4}, []BranchID{branch2.ID()}, []ConflictID{{1}})
	defer cachedBranch4.Release()
	branch4 := cachedBranch4.Unwrap()

	cachedBranch5, _ := branchManager.Fork(BranchID{5}, []BranchID{branch2.ID()}, []ConflictID{{1}})
	defer cachedBranch5.Release()
	branch5 := cachedBranch5.Unwrap()

	{
		conflicting, err := branchManager.BranchesConflicting(MasterBranchID, branch4.ID())
		assert.NoError(t, err)
		assert.False(t, conflicting, "branch 4 should not be conflicting with master branch")

		conflicting, err = branchManager.BranchesConflicting(branch3.ID(), branch4.ID())
		assert.NoError(t, err)
		assert.True(t, conflicting, "branch 3 & 4 should be conflicting with each other")

		conflicting, err = branchManager.BranchesConflicting(MasterBranchID, branch5.ID())
		assert.NoError(t, err)
		assert.False(t, conflicting, "branch 5 should not be conflicting with master branch")

		conflicting, err = branchManager.BranchesConflicting(branch3.ID(), branch5.ID())
		assert.NoError(t, err)
		assert.True(t, conflicting, "branch 3 & 5 should be conflicting with each other")

		// since both consume the same output
		conflicting, err = branchManager.BranchesConflicting(branch4.ID(), branch5.ID())
		assert.NoError(t, err)
		assert.True(t, conflicting, "branch 4 & 5 should be conflicting with each other")
	}

	// branch 6, 7 are on the same level as 2 and 3 but are not part of that conflict set
	cachedBranch6, _ := branchManager.Fork(BranchID{6}, []BranchID{MasterBranchID}, []ConflictID{{2}})
	defer cachedBranch6.Release()
	branch6 := cachedBranch6.Unwrap()

	cachedBranch7, _ := branchManager.Fork(BranchID{7}, []BranchID{MasterBranchID}, []ConflictID{{2}})
	defer cachedBranch7.Release()
	branch7 := cachedBranch7.Unwrap()

	{
		conflicting, err := branchManager.BranchesConflicting(branch2.ID(), branch6.ID())
		assert.NoError(t, err)
		assert.False(t, conflicting, "branch 6 should not be conflicting with branch 2")

		conflicting, err = branchManager.BranchesConflicting(branch3.ID(), branch6.ID())
		assert.NoError(t, err)
		assert.False(t, conflicting, "branch 6 should not be conflicting with branch 3")

		conflicting, err = branchManager.BranchesConflicting(branch2.ID(), branch7.ID())
		assert.NoError(t, err)
		assert.False(t, conflicting, "branch 7 should not be conflicting with branch 2")

		conflicting, err = branchManager.BranchesConflicting(branch3.ID(), branch7.ID())
		assert.NoError(t, err)
		assert.False(t, conflicting, "branch 7 should not be conflicting with branch 3")

		conflicting, err = branchManager.BranchesConflicting(branch6.ID(), branch7.ID())
		assert.NoError(t, err)
		assert.True(t, conflicting, "branch 6 & 7 should be conflicting with each other")

		conflicting, err = branchManager.BranchesConflicting(branch4.ID(), branch6.ID())
		assert.NoError(t, err)
		assert.False(t, conflicting, "branch 6 should not be conflicting with branch 4")

		conflicting, err = branchManager.BranchesConflicting(branch5.ID(), branch6.ID())
		assert.NoError(t, err)
		assert.False(t, conflicting, "branch 6 should not be conflicting with branch 5")

		conflicting, err = branchManager.BranchesConflicting(branch4.ID(), branch7.ID())
		assert.NoError(t, err)
		assert.False(t, conflicting, "branch 7 should not be conflicting with branch 4")

		conflicting, err = branchManager.BranchesConflicting(branch5.ID(), branch7.ID())
		assert.NoError(t, err)
		assert.False(t, conflicting, "branch 7 should not be conflicting with branch 5")
	}

	// aggregated branch out of branch 4 (child of branch 2) and branch 6
	cachedAggrBranch8, err := branchManager.AggregateBranches(branch4.ID(), branch6.ID())
	assert.NoError(t, err)
	defer cachedAggrBranch8.Release()
	aggrBranch8 := cachedAggrBranch8.Unwrap()

	{
		conflicting, err := branchManager.BranchesConflicting(aggrBranch8.ID(), MasterBranchID)
		assert.NoError(t, err)
		assert.False(t, conflicting, "aggr. branch 8 should not be conflicting with master branch")

		conflicting, err = branchManager.BranchesConflicting(aggrBranch8.ID(), branch2.ID())
		assert.NoError(t, err)
		assert.False(t, conflicting, "aggr. branch 8 should not be conflicting with branch 2")

		// conflicting since branch 2 and branch 3 are
		conflicting, err = branchManager.BranchesConflicting(aggrBranch8.ID(), branch3.ID())
		assert.NoError(t, err)
		assert.True(t, conflicting, "aggr. branch 8 & branch 3 should be conflicting with each other")

		// conflicting since branch 4 and branch 5 are
		conflicting, err = branchManager.BranchesConflicting(aggrBranch8.ID(), branch5.ID())
		assert.NoError(t, err)
		assert.True(t, conflicting, "aggr. branch 8 & branch 5 should be conflicting with each other")

		// conflicting since branch 6 and branch 7 are
		conflicting, err = branchManager.BranchesConflicting(aggrBranch8.ID(), branch7.ID())
		assert.NoError(t, err)
		assert.True(t, conflicting, "aggr. branch 8 & branch 7 should be conflicting with each other")
	}

	// aggregated branch out of aggr. branch 8 and branch 7:
	// should fail since branch 6 & 7 are conflicting
	_, err = branchManager.AggregateBranches(aggrBranch8.ID(), branch7.ID())
	assert.Error(t, err, "can't aggregate branches aggr. branch 8 & conflict branch 7")

	// aggregated branch out of branch 5 (child of branch 2) and branch 7
	cachedAggrBranch9, err := branchManager.AggregateBranches(branch5.ID(), branch7.ID())
	assert.NoError(t, err)
	defer cachedAggrBranch9.Release()
	aggrBranch9 := cachedAggrBranch9.Unwrap()

	assert.NotEqual(t, aggrBranch8.ID().String(), aggrBranch9.ID().String(), "aggr. branches 8 & 9 should  have different IDs")

	{
		conflicting, err := branchManager.BranchesConflicting(aggrBranch9.ID(), MasterBranchID)
		assert.NoError(t, err)
		assert.False(t, conflicting, "aggr. branch 9 should not be conflicting with master branch")

		// aggr. branch 8 and 9 should be conflicting, since 4 & 5 and 6 & 7 are
		conflicting, err = branchManager.BranchesConflicting(aggrBranch8.ID(), aggrBranch9.ID())
		assert.NoError(t, err)
		assert.True(t, conflicting, "aggr. branch 8 & branch 9 should be conflicting with each other")

		// conflicting since branch 3 & 2 are
		conflicting, err = branchManager.BranchesConflicting(branch3.ID(), aggrBranch9.ID())
		assert.NoError(t, err)
		assert.True(t, conflicting, "aggr. branch 9 & branch 3 should be conflicting with each other")
	}

	// aggregated branch out of branch 3 and branch 6
	cachedAggrBranch10, err := branchManager.AggregateBranches(branch3.ID(), branch6.ID())
	assert.NoError(t, err)
	defer cachedAggrBranch10.Release()
	aggrBranch10 := cachedAggrBranch10.Unwrap()

	{
		conflicting, err := branchManager.BranchesConflicting(aggrBranch10.ID(), MasterBranchID)
		assert.NoError(t, err)
		assert.False(t, conflicting, "aggr. branch 10 should not be conflicting with master branch")

		// aggr. branch 8 and 10 should be conflicting, since 2 & 3 are
		conflicting, err = branchManager.BranchesConflicting(aggrBranch8.ID(), aggrBranch10.ID())
		assert.NoError(t, err)
		assert.True(t, conflicting, "aggr. branch 8 & branch 10 should be conflicting with each other")

		// aggr. branch 9 and 10 should be conflicting, since 2 & 3 and 6 & 7 are
		conflicting, err = branchManager.BranchesConflicting(aggrBranch9.ID(), aggrBranch10.ID())
		assert.NoError(t, err)
		assert.True(t, conflicting, "aggr. branch 9 & branch 10 should be conflicting with each other")
	}

	// branch 11, 12 are on the same level as 2 & 3 and 6 & 7 but are not part of either conflict set
	cachedBranch11, _ := branchManager.Fork(BranchID{11}, []BranchID{MasterBranchID}, []ConflictID{{3}})
	defer cachedBranch11.Release()
	branch11 := cachedBranch11.Unwrap()

	cachedBranch12, _ := branchManager.Fork(BranchID{12}, []BranchID{MasterBranchID}, []ConflictID{{3}})
	defer cachedBranch12.Release()
	branch12 := cachedBranch12.Unwrap()

	{
		conflicting, err := branchManager.BranchesConflicting(MasterBranchID, branch11.ID())
		assert.NoError(t, err)
		assert.False(t, conflicting, "branch 11 should not be conflicting with master branch")

		conflicting, err = branchManager.BranchesConflicting(MasterBranchID, branch12.ID())
		assert.NoError(t, err)
		assert.False(t, conflicting, "branch 12 should not be conflicting with master branch")

		conflicting, err = branchManager.BranchesConflicting(branch11.ID(), branch12.ID())
		assert.NoError(t, err)
		assert.True(t, conflicting, "branch 11 & 12 should be conflicting with each other")
	}

	// aggr. branch 13 out of branch 6 and 11
	cachedAggrBranch13, err := branchManager.AggregateBranches(branch6.ID(), branch11.ID())
	assert.NoError(t, err)
	defer cachedAggrBranch13.Release()
	aggrBranch13 := cachedAggrBranch13.Unwrap()

	{
		conflicting, err := branchManager.BranchesConflicting(aggrBranch13.ID(), aggrBranch9.ID())
		assert.NoError(t, err)
		assert.True(t, conflicting, "aggr. branch 9 & 13 should be conflicting with each other")

		conflicting, err = branchManager.BranchesConflicting(aggrBranch13.ID(), aggrBranch8.ID())
		assert.NoError(t, err)
		assert.False(t, conflicting, "aggr. branch 8 & 13 should not be conflicting with each other")

		conflicting, err = branchManager.BranchesConflicting(aggrBranch13.ID(), aggrBranch10.ID())
		assert.NoError(t, err)
		assert.False(t, conflicting, "aggr. branch 10 & 13 should not be conflicting with each other")
	}

	// aggr. branch 14 out of aggr. branch 10 and 13
	cachedAggrBranch14, err := branchManager.AggregateBranches(aggrBranch10.ID(), aggrBranch13.ID())
	assert.NoError(t, err)
	defer cachedAggrBranch14.Release()
	aggrBranch14 := cachedAggrBranch14.Unwrap()

	{
		// aggr. branch 9 has parent branch 7 which conflicts with ancestor branch 6 of aggr. branch 14
		conflicting, err := branchManager.BranchesConflicting(aggrBranch14.ID(), aggrBranch9.ID())
		assert.NoError(t, err)
		assert.True(t, conflicting, "aggr. branch 14 & 9 should be conflicting with each other")

		// aggr. branch has ancestor branch 2 which conflicts with ancestor branch 3 of aggr. branch 14
		conflicting, err = branchManager.BranchesConflicting(aggrBranch14.ID(), aggrBranch8.ID())
		assert.NoError(t, err)
		assert.True(t, conflicting, "aggr. branch 14 & 8 should be conflicting with each other")
	}

	// aggr. branch 15 out of branch 2, 7 and 12
	cachedAggrBranch15, err := branchManager.AggregateBranches(branch2.ID(), branch7.ID(), branch12.ID())
	assert.NoError(t, err)
	defer cachedAggrBranch15.Release()
	aggrBranch15 := cachedAggrBranch15.Unwrap()

	{
		// aggr. branch 13 has parent branches 11 & 6 which conflicts which conflicts with ancestor branches 12 & 7 of aggr. branch 15
		conflicting, err := branchManager.BranchesConflicting(aggrBranch15.ID(), aggrBranch13.ID())
		assert.NoError(t, err)
		assert.True(t, conflicting, "aggr. branch 15 & 13 should be conflicting with each other")

		// aggr. branch 10 has parent branches 3 & 6 which conflicts with ancestor branches 2 & 7 of aggr. branch 15
		conflicting, err = branchManager.BranchesConflicting(aggrBranch15.ID(), aggrBranch10.ID())
		assert.NoError(t, err)
		assert.True(t, conflicting, "aggr. branch 15 & 10 should be conflicting with each other")

		// aggr. branch 8 has parent branch 6 which conflicts with ancestor branch 7 of aggr. branch 15
		conflicting, err = branchManager.BranchesConflicting(aggrBranch15.ID(), aggrBranch8.ID())
		assert.NoError(t, err)
		assert.True(t, conflicting, "aggr. branch 15 & 8 should be conflicting with each other")
	}

	// aggr. branch 16 out of aggr. branches 15 and 9
	cachedAggrBranch16, err := branchManager.AggregateBranches(aggrBranch15.ID(), aggrBranch9.ID())
	assert.NoError(t, err)
	defer cachedAggrBranch16.Release()
	aggrBranch16 := cachedAggrBranch16.Unwrap()

	{
		// sanity check
		conflicting, err := branchManager.BranchesConflicting(aggrBranch16.ID(), aggrBranch9.ID())
		assert.NoError(t, err)
		assert.False(t, conflicting, "aggr. branch 16 & 9 should not be conflicting with each other")

		// sanity check
		conflicting, err = branchManager.BranchesConflicting(aggrBranch16.ID(), branch7.ID())
		assert.NoError(t, err)
		assert.False(t, conflicting, "aggr. branch 16 & 9 should not be conflicting with each other")

		conflicting, err = branchManager.BranchesConflicting(aggrBranch16.ID(), aggrBranch13.ID())
		assert.NoError(t, err)
		assert.True(t, conflicting, "aggr. branch 16 & 13 should be conflicting with each other")

		conflicting, err = branchManager.BranchesConflicting(aggrBranch16.ID(), aggrBranch14.ID())
		assert.NoError(t, err)
		assert.True(t, conflicting, "aggr. branch 16 & 14 should be conflicting with each other")

		conflicting, err = branchManager.BranchesConflicting(aggrBranch16.ID(), aggrBranch8.ID())
		assert.NoError(t, err)
		assert.True(t, conflicting, "aggr. branch 16 & 8 should be conflicting with each other")
	}

}

func TestBranchManager_SetBranchPreferred(t *testing.T) {
	branchManager := New(mapdb.NewMapDB())

	cachedBranch2, _ := branchManager.Fork(BranchID{2}, []BranchID{MasterBranchID}, []ConflictID{{0}})
	defer cachedBranch2.Release()
	branch2 := cachedBranch2.Unwrap()

	cachedBranch3, _ := branchManager.Fork(BranchID{3}, []BranchID{MasterBranchID}, []ConflictID{{0}})
	defer cachedBranch3.Release()
	branch3 := cachedBranch3.Unwrap()

	assert.False(t, branch2.Preferred(), "branch 2 should not be preferred")
	assert.False(t, branch2.Liked(), "branch 2 should not be liked")
	assert.False(t, branch3.Preferred(), "branch 3 should not be preferred")
	assert.False(t, branch3.Liked(), "branch 3 should not be liked")

	cachedBranch4, _ := branchManager.Fork(BranchID{4}, []BranchID{branch2.ID()}, []ConflictID{{1}})
	defer cachedBranch4.Release()
	branch4 := cachedBranch4.Unwrap()

	cachedBranch5, _ := branchManager.Fork(BranchID{5}, []BranchID{branch2.ID()}, []ConflictID{{1}})
	defer cachedBranch5.Release()
	branch5 := cachedBranch5.Unwrap()

	// lets assume branch 4 is preferred since its underlying transaction was longer
	// solid than the avg. network delay before the conflicting transaction which created
	// the conflict set was received
	modified, err := branchManager.SetBranchPreferred(branch4.ID(), true)
	assert.NoError(t, err)
	assert.True(t, modified)

	assert.True(t, branch4.Preferred(), "branch 4 should be preferred")
	// is not liked because its parents aren't liked, respectively branch 2
	assert.False(t, branch4.Liked(), "branch 4 should not be liked")
	assert.False(t, branch5.Preferred(), "branch 5 should not be preferred")
	assert.False(t, branch5.Liked(), "branch 5 should not be liked")

	// now branch 2 becomes preferred via FPC, this causes branch 2 to be liked (since
	// the master branch is liked) and its liked state propagates to branch 4 (but not branch 5)
	modified, err = branchManager.SetBranchPreferred(branch2.ID(), true)
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
	modified, err = branchManager.SetBranchPreferred(branch5.ID(), true)
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
}

func TestBranchManager_SetBranchPreferred2(t *testing.T) {
	branchManager := New(mapdb.NewMapDB())

	cachedBranch2, _ := branchManager.Fork(BranchID{2}, []BranchID{MasterBranchID}, []ConflictID{{0}})
	defer cachedBranch2.Release()
	branch2 := cachedBranch2.Unwrap()

	cachedBranch3, _ := branchManager.Fork(BranchID{3}, []BranchID{MasterBranchID}, []ConflictID{{0}})
	defer cachedBranch3.Release()
	branch3 := cachedBranch3.Unwrap()

	cachedBranch4, _ := branchManager.Fork(BranchID{4}, []BranchID{branch2.ID()}, []ConflictID{{1}})
	defer cachedBranch4.Release()
	branch4 := cachedBranch4.Unwrap()

	cachedBranch5, _ := branchManager.Fork(BranchID{5}, []BranchID{branch2.ID()}, []ConflictID{{1}})
	defer cachedBranch5.Release()
	branch5 := cachedBranch5.Unwrap()

	cachedBranch6, _ := branchManager.Fork(BranchID{6}, []BranchID{MasterBranchID}, []ConflictID{{2}})
	defer cachedBranch6.Release()
	branch6 := cachedBranch6.Unwrap()

	cachedBranch7, _ := branchManager.Fork(BranchID{7}, []BranchID{MasterBranchID}, []ConflictID{{2}})
	defer cachedBranch7.Release()
	branch7 := cachedBranch7.Unwrap()

	// assume branch 2 preferred since solid longer than avg. network delay
	modified, err := branchManager.SetBranchPreferred(branch2.ID(), true)
	assert.NoError(t, err)
	assert.True(t, modified)

	// assume branch 6 preferred since solid longer than avg. network delay
	modified, err = branchManager.SetBranchPreferred(branch6.ID(), true)
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
	cachedAggrBranch8, err := branchManager.AggregateBranches(branch4.ID(), branch6.ID())
	assert.NoError(t, err)
	defer cachedAggrBranch8.Release()
	aggrBranch8 := cachedAggrBranch8.Unwrap()

	// should not be preferred because only 6 is is preferred but not 4
	assert.False(t, aggrBranch8.Liked(), "aggr. branch 8 should not be liked")
	assert.False(t, aggrBranch8.Preferred(), "aggr. branch 8 should not be preferred")

	cachedAggrBranch9, err := branchManager.AggregateBranches(branch5.ID(), branch7.ID())
	assert.NoError(t, err)
	defer cachedAggrBranch9.Release()
	aggrBranch9 := cachedAggrBranch9.Unwrap()

	// branch 5 and 7 are neither liked or preferred
	assert.False(t, aggrBranch9.Liked(), "aggr. branch 9 should not be liked")
	assert.False(t, aggrBranch9.Preferred(), "aggr. branch 9 should not be preferred")

	// should not be preferred because only 6 is is preferred but not 3
	cachedAggrBranch10, err := branchManager.AggregateBranches(branch3.ID(), branch6.ID())
	assert.NoError(t, err)
	defer cachedAggrBranch10.Release()
	aggrBranch10 := cachedAggrBranch10.Unwrap()

	assert.False(t, aggrBranch10.Liked(), "aggr. branch 10 should not be liked")
	assert.False(t, aggrBranch10.Preferred(), "aggr. branch 10 should not be preferred")

	// spawn off conflict branch 11 and 12
	cachedBranch11, _ := branchManager.Fork(BranchID{11}, []BranchID{aggrBranch8.ID()}, []ConflictID{{3}})
	defer cachedBranch11.Release()
	branch11 := cachedBranch11.Unwrap()

	assert.False(t, branch11.Liked(), "aggr. branch 11 should not be liked")
	assert.False(t, branch11.Preferred(), "aggr. branch 11 should not be preferred")

	cachedBranch12, _ := branchManager.Fork(BranchID{12}, []BranchID{aggrBranch8.ID()}, []ConflictID{{3}})
	defer cachedBranch12.Release()
	branch12 := cachedBranch12.Unwrap()

	assert.False(t, branch12.Liked(), "aggr. branch 12 should not be liked")
	assert.False(t, branch12.Preferred(), "aggr. branch 12 should not be preferred")

	cachedAggrBranch13, err := branchManager.AggregateBranches(branch4.ID(), branch12.ID())
	assert.NoError(t, err)
	defer cachedAggrBranch13.Release()
	aggrBranch13 := cachedAggrBranch13.Unwrap()

	assert.False(t, aggrBranch13.Liked(), "aggr. branch 13 should not be liked")
	assert.False(t, aggrBranch13.Preferred(), "aggr. branch 13 should not be preferred")

	// now lets assume FPC finalized on branch 2, 6 and 4 to be preferred.
	// branches 2 and 6 are already preferred but 4 is newly preferred. Branch 4 therefore
	// should also become liked, since branch 2 of which it spawns off is liked too.

	// simulate branch 3 being not preferred from FPC vote
	modified, err = branchManager.SetBranchPreferred(branch3.ID(), false)
	assert.NoError(t, err)
	// simulate branch 7 being not preferred from FPC vote
	modified, err = branchManager.SetBranchPreferred(branch7.ID(), false)
	assert.NoError(t, err)
	// simulate branch 4 being preferred by FPC vote
	modified, err = branchManager.SetBranchPreferred(branch4.ID(), true)
	assert.NoError(t, err)
	assert.True(t, modified)
	assert.True(t, branch4.Liked(), "branch 4 should be liked")
	assert.True(t, branch4.Preferred(), "branch 4 should be preferred")

	// this should cause aggr. branch 8 to also be preferred and liked, since branch 6 and 4
	// of which it spawns off are.
	assert.True(t, aggrBranch8.Liked(), "aggr. branch 8 should be liked")
	assert.True(t, aggrBranch8.Preferred(), "aggr. branch 8 should be preferred")
}
