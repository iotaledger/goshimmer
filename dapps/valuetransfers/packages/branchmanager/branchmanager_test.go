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

func TestConflictDetection(t *testing.T) {
	branchManager := New(testutil.DB(t))

	cachedBranch2, _ := branchManager.Fork(BranchID{2}, []BranchID{MasterBranchID}, []ConflictID{{0}})
	defer cachedBranch2.Release()
	branch2 := cachedBranch2.Unwrap()

	cachedBranch3, _ := branchManager.Fork(BranchID{3}, []BranchID{MasterBranchID}, []ConflictID{{0}})
	defer cachedBranch3.Release()
	branch3 := cachedBranch3.Unwrap()

	conflicting, err := branchManager.BranchesConflicting(MasterBranchID, branch2.ID())
	assert.NoError(t, err)
	assert.False(t, conflicting, "branch 2 should not be conflicting with master branch")

	conflicting, err = branchManager.BranchesConflicting(MasterBranchID, branch2.ID())
	assert.NoError(t, err)
	assert.False(t, conflicting, "branch 3 should not be conflicting with master branch")

	conflicting, err = branchManager.BranchesConflicting(branch2.ID(), branch3.ID())
	assert.NoError(t, err)
	assert.True(t, conflicting, "branch 2 & 3 should be conflicting with each other")

	// spawn of branch 4 and 5 from branch 2
	cachedBranch4, _ := branchManager.Fork(BranchID{4}, []BranchID{branch2.ID()}, []ConflictID{{1}})
	defer cachedBranch4.Release()
	branch4 := cachedBranch4.Unwrap()

	conflicting, err = branchManager.BranchesConflicting(MasterBranchID, branch4.ID())
	assert.NoError(t, err)
	assert.False(t, conflicting, "branch 4 should not be conflicting with master branch")

	conflicting, err = branchManager.BranchesConflicting(branch3.ID(), branch4.ID())
	assert.NoError(t, err)
	assert.True(t, conflicting, "branch 3 & 4 should be conflicting with each other")

	cachedBranch5, _ := branchManager.Fork(BranchID{5}, []BranchID{branch2.ID()}, []ConflictID{{1}})
	defer cachedBranch5.Release()
	branch5 := cachedBranch5.Unwrap()

	conflicting, err = branchManager.BranchesConflicting(MasterBranchID, branch5.ID())
	assert.NoError(t, err)
	assert.False(t, conflicting, "branch 5 should not be conflicting with master branch")

	conflicting, err = branchManager.BranchesConflicting(branch3.ID(), branch5.ID())
	assert.NoError(t, err)
	assert.True(t, conflicting, "branch 3 & 5 should be conflicting with each other")

	// branch 6, 7 is on the same level as 2 and 3 but are not part of that conflict set
	cachedBranch6, _ := branchManager.Fork(BranchID{6}, []BranchID{MasterBranchID}, []ConflictID{{2}})
	defer cachedBranch6.Release()
	branch6 := cachedBranch6.Unwrap()

	conflicting, err = branchManager.BranchesConflicting(branch2.ID(), branch6.ID())
	assert.NoError(t, err)
	assert.False(t, conflicting, "branch 6 should not be conflicting with branch 2")

	conflicting, err = branchManager.BranchesConflicting(branch3.ID(), branch6.ID())
	assert.NoError(t, err)
	assert.False(t, conflicting, "branch 6 should not be conflicting with branch 3")

	// branch 6, 7 is on the same level as 2 and 3 but are not part of that conflict set
	cachedBranch7, _ := branchManager.Fork(BranchID{7}, []BranchID{MasterBranchID}, []ConflictID{{2}})
	defer cachedBranch7.Release()
	branch7 := cachedBranch7.Unwrap()

	conflicting, err = branchManager.BranchesConflicting(branch2.ID(), branch7.ID())
	assert.NoError(t, err)
	assert.False(t, conflicting, "branch 7 should not be conflicting with branch 2")

	conflicting, err = branchManager.BranchesConflicting(branch3.ID(), branch7.ID())
	assert.NoError(t, err)
	assert.False(t, conflicting, "branch 7 should not be conflicting with branch 3")

	// aggregated branch out of branch 4 (child of branch 2) and branch 6
	cachedAggrBranch8, err := branchManager.AggregateBranches(branch4.ID(), branch6.ID())
	assert.NoError(t, err)
	defer cachedAggrBranch8.Release()
	aggrBranch8 := cachedAggrBranch8.Unwrap()

	conflicting, err = branchManager.BranchesConflicting(aggrBranch8.ID(), MasterBranchID)
	assert.NoError(t, err)
	assert.False(t, conflicting, "aggr. branch 8 should not be conflicting with master branch")

	// conflicting since branch 2 and branch 3 are
	conflicting, err = branchManager.BranchesConflicting(aggrBranch8.ID(), branch3.ID())
	assert.NoError(t, err)
	assert.True(t, conflicting, "aggr. branch 8 & branch 3 should be conflicting with each other")

	// aggregated branch out of aggr. branch 8 and branch 7,
	// should fail since branch 6 & 7 are conflicting
	_, err = branchManager.AggregateBranches(aggrBranch8.ID(), branch7.ID())
	assert.Error(t, err)

	// aggregated branch out of branch 5 (child of branch 2) and branch 7
	cachedAggrBranch9, err := branchManager.AggregateBranches(branch5.ID(), branch7.ID())
	assert.NoError(t, err)
	defer cachedAggrBranch9.Release()
	aggrBranch9 := cachedAggrBranch9.Unwrap()

	// aggr. branch 8 and 9 should be conflicting, since 4&5 and 6&7 are
	conflicting, err = branchManager.BranchesConflicting(aggrBranch8.ID(), aggrBranch9.ID())
	assert.NoError(t, err)
	assert.True(t, conflicting, "aggr. branch 8 & branch 9 should be conflicting with each other")
}
