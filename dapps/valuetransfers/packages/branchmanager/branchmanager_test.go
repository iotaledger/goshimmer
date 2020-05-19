package branchmanager

import (
	"fmt"
	"testing"

	"github.com/iotaledger/goshimmer/packages/binary/testutil"
	"github.com/stretchr/testify/assert"
)

func TestSomething(t *testing.T) {
	branchManager := New(testutil.DB(t))

	cachedBranch1, _ := branchManager.Fork(BranchID{2}, []BranchID{MasterBranchID}, []ConflictID{{4}})
	defer cachedBranch1.Release()
	_ = cachedBranch1.Unwrap()

	cachedBranch2, _ := branchManager.Fork(BranchID{3}, []BranchID{MasterBranchID}, []ConflictID{{4}})
	defer cachedBranch2.Release()
	branch2 := cachedBranch2.Unwrap()

	fmt.Println(branchManager.BranchesConflicting(MasterBranchID, branch2.ID()))
}

// ./img/testconflictdetection.png
func TestConflictDetection(t *testing.T) {
	branchManager := New(testutil.DB(t))

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
