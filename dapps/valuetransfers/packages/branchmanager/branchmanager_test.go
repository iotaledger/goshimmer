package branchmanager

import (
	"testing"

	"github.com/iotaledger/hive.go/kvstore/mapdb"
	"github.com/stretchr/testify/assert"
)

type sampleTree struct {
	cachedBranches [17]*CachedBranch
	branches       [17]*Branch
	numID          map[BranchID]int
}

func (st *sampleTree) Release() {
	for i := range st.cachedBranches {
		if st.cachedBranches[i] == nil {
			continue
		}
		st.cachedBranches[i].Release()
	}
}

// ./img/sample_tree.png
func createSampleTree(branchManager *BranchManager) *sampleTree {
	st := &sampleTree{
		numID: map[BranchID]int{},
	}
	cachedMasterBranch := branchManager.Branch(MasterBranchID)
	st.cachedBranches[1] = cachedMasterBranch
	st.branches[1] = cachedMasterBranch.Unwrap()
	st.numID[st.branches[1].ID()] = 1

	cachedBranch2, _ := branchManager.Fork(BranchID{2}, []BranchID{MasterBranchID}, []ConflictID{{0}})
	branch2 := cachedBranch2.Unwrap()
	st.branches[2] = branch2
	st.cachedBranches[2] = cachedBranch2
	st.numID[st.branches[2].ID()] = 2

	cachedBranch3, _ := branchManager.Fork(BranchID{3}, []BranchID{MasterBranchID}, []ConflictID{{0}})
	branch3 := cachedBranch3.Unwrap()
	st.branches[3] = branch3
	st.cachedBranches[3] = cachedBranch3
	st.numID[st.branches[3].ID()] = 3

	cachedBranch4, _ := branchManager.Fork(BranchID{4}, []BranchID{branch2.ID()}, []ConflictID{{1}})
	branch4 := cachedBranch4.Unwrap()
	st.branches[4] = branch4
	st.cachedBranches[4] = cachedBranch4
	st.numID[st.branches[4].ID()] = 4

	cachedBranch5, _ := branchManager.Fork(BranchID{5}, []BranchID{branch2.ID()}, []ConflictID{{1}})
	branch5 := cachedBranch5.Unwrap()
	st.branches[5] = branch5
	st.cachedBranches[5] = cachedBranch5
	st.numID[st.branches[5].ID()] = 5

	cachedBranch6, _ := branchManager.Fork(BranchID{6}, []BranchID{MasterBranchID}, []ConflictID{{2}})
	branch6 := cachedBranch6.Unwrap()
	st.branches[6] = branch6
	st.cachedBranches[6] = cachedBranch6
	st.numID[st.branches[6].ID()] = 6

	cachedBranch7, _ := branchManager.Fork(BranchID{7}, []BranchID{MasterBranchID}, []ConflictID{{2}})
	branch7 := cachedBranch7.Unwrap()
	st.branches[7] = branch7
	st.cachedBranches[7] = cachedBranch7
	st.numID[st.branches[7].ID()] = 7

	cachedBranch8, _ := branchManager.Fork(BranchID{8}, []BranchID{MasterBranchID}, []ConflictID{{3}})
	branch8 := cachedBranch8.Unwrap()
	st.branches[8] = branch8
	st.cachedBranches[8] = cachedBranch8
	st.numID[st.branches[8].ID()] = 8

	cachedBranch9, _ := branchManager.Fork(BranchID{9}, []BranchID{MasterBranchID}, []ConflictID{{3}})
	branch9 := cachedBranch9.Unwrap()
	st.branches[9] = branch9
	st.cachedBranches[9] = cachedBranch9
	st.numID[st.branches[9].ID()] = 9

	cachedAggrBranch10, err := branchManager.AggregateBranches(branch3.ID(), branch6.ID())
	if err != nil {
		panic(err)
	}
	aggrBranch10 := cachedAggrBranch10.Unwrap()
	st.branches[10] = aggrBranch10
	st.cachedBranches[10] = cachedAggrBranch10
	st.numID[st.branches[10].ID()] = 10

	cachedAggrBranch11, err := branchManager.AggregateBranches(branch6.ID(), branch8.ID())
	if err != nil {
		panic(err)
	}
	aggrBranch11 := cachedAggrBranch11.Unwrap()
	st.branches[11] = aggrBranch11
	st.cachedBranches[11] = cachedAggrBranch11
	st.numID[st.branches[11].ID()] = 11

	cachedBranch12, _ := branchManager.Fork(BranchID{12}, []BranchID{aggrBranch10.ID()}, []ConflictID{{4}})
	branch12 := cachedBranch12.Unwrap()
	st.branches[12] = branch12
	st.cachedBranches[12] = cachedBranch12
	st.numID[st.branches[12].ID()] = 12

	cachedBranch13, _ := branchManager.Fork(BranchID{13}, []BranchID{aggrBranch10.ID()}, []ConflictID{{4}})
	branch13 := cachedBranch13.Unwrap()
	st.branches[13] = branch13
	st.cachedBranches[13] = cachedBranch13
	st.numID[st.branches[13].ID()] = 13

	cachedBranch14, _ := branchManager.Fork(BranchID{14}, []BranchID{aggrBranch11.ID()}, []ConflictID{{5}})
	branch14 := cachedBranch14.Unwrap()
	st.branches[14] = branch14
	st.cachedBranches[14] = cachedBranch14
	st.numID[st.branches[14].ID()] = 14

	cachedBranch15, _ := branchManager.Fork(BranchID{15}, []BranchID{aggrBranch11.ID()}, []ConflictID{{5}})
	branch15 := cachedBranch15.Unwrap()
	st.branches[15] = branch15
	st.cachedBranches[15] = cachedBranch15
	st.numID[st.branches[15].ID()] = 15

	cachedAggrBranch16, err := branchManager.AggregateBranches(branch13.ID(), branch14.ID())
	if err != nil {
		panic(err)
	}
	aggrBranch16 := cachedAggrBranch16.Unwrap()
	st.branches[16] = aggrBranch16
	st.cachedBranches[16] = cachedAggrBranch16
	st.numID[st.branches[16].ID()] = 16
	return st
}

func TestGetAncestorBranches(t *testing.T) {
	branchManager := New(mapdb.NewMapDB())

	st := createSampleTree(branchManager)
	defer st.Release()

	{
		masterBranch := branchManager.Branch(MasterBranchID)
		assert.NotNil(t, masterBranch)
		ancestorBranches, err := branchManager.getAncestorBranches(masterBranch.Unwrap())
		assert.NoError(t, err)

		expectedAncestors := map[BranchID]struct{}{}
		actualAncestors := map[BranchID]struct{}{}
		ancestorBranches.Consume(func(branch *Branch) {
			actualAncestors[branch.ID()] = struct{}{}
		})
		assert.Equal(t, expectedAncestors, actualAncestors)
	}

	{
		ancestorBranches, err := branchManager.getAncestorBranches(st.branches[4])
		assert.NoError(t, err)

		expectedAncestors := map[BranchID]struct{}{
			st.branches[2].ID(): {}, MasterBranchID: {},
		}
		actualAncestors := map[BranchID]struct{}{}
		ancestorBranches.Consume(func(branch *Branch) {
			actualAncestors[branch.ID()] = struct{}{}
		})
		assert.Equal(t, expectedAncestors, actualAncestors)
	}

	{
		ancestorBranches, err := branchManager.getAncestorBranches(st.branches[16])
		assert.NoError(t, err)

		expectedAncestors := map[BranchID]struct{}{
			st.branches[13].ID(): {}, st.branches[14].ID(): {},
			st.branches[10].ID(): {}, st.branches[11].ID(): {},
			st.branches[3].ID(): {}, st.branches[6].ID(): {}, st.branches[8].ID(): {},
			MasterBranchID: {},
		}
		actualAncestors := map[BranchID]struct{}{}
		ancestorBranches.Consume(func(branch *Branch) {
			actualAncestors[branch.ID()] = struct{}{}
		})
		assert.Equal(t, expectedAncestors, actualAncestors)
	}

	{
		ancestorBranches, err := branchManager.getAncestorBranches(st.branches[12])
		assert.NoError(t, err)

		expectedAncestors := map[BranchID]struct{}{
			st.branches[10].ID(): {}, st.branches[3].ID(): {},
			st.branches[6].ID(): {}, MasterBranchID: {},
		}
		actualAncestors := map[BranchID]struct{}{}
		ancestorBranches.Consume(func(branch *Branch) {
			actualAncestors[branch.ID()] = struct{}{}
		})
		assert.Equal(t, expectedAncestors, actualAncestors)
	}

	{
		ancestorBranches, err := branchManager.getAncestorBranches(st.branches[11])
		assert.NoError(t, err)

		expectedAncestors := map[BranchID]struct{}{
			st.branches[8].ID(): {}, st.branches[6].ID(): {}, MasterBranchID: {},
		}
		actualAncestors := map[BranchID]struct{}{}
		ancestorBranches.Consume(func(branch *Branch) {
			actualAncestors[branch.ID()] = struct{}{}
		})
		assert.Equal(t, expectedAncestors, actualAncestors)
	}
}

func TestIsAncestorOfBranch(t *testing.T) {
	branchManager := New(mapdb.NewMapDB())

	st := createSampleTree(branchManager)
	defer st.Release()

	type testcase struct {
		ancestor   *Branch
		descendent *Branch
		is         bool
	}

	cases := []testcase{
		{st.branches[2], st.branches[4], true},
		{st.branches[4], st.branches[2], false},
		{st.branches[3], st.branches[4], false},
		{st.branches[3], st.branches[12], true},
		{st.branches[3], st.branches[10], true},
		{st.branches[3], st.branches[16], true},
		{st.branches[1], st.branches[16], true},
		{st.branches[11], st.branches[16], true},
		{st.branches[9], st.branches[16], false},
		{st.branches[6], st.branches[15], true},
	}

	for _, testCase := range cases {
		isAncestor, err := branchManager.branchIsAncestorOfBranch(testCase.ancestor, testCase.descendent)
		assert.NoError(t, err)
		if testCase.is {
			assert.True(t, isAncestor, "branch %d is an ancestor of branch %d", st.numID[testCase.ancestor.ID()], st.numID[testCase.descendent.ID()])
			continue
		}
		assert.False(t, isAncestor, "branch %d is not an ancestor of branch %d", st.numID[testCase.ancestor.ID()], st.numID[testCase.descendent.ID()])
	}
}

func TestFindDeepestCommonDescendants(t *testing.T) {
	branchManager := New(mapdb.NewMapDB())

	st := createSampleTree(branchManager)
	defer st.Release()

	{
		deepestCommonDescendants, err := branchManager.findDeepestCommonDescendants(MasterBranchID, st.branches[2].ID())
		assert.NoError(t, err)

		expectedDeepestCommonDescendants := map[BranchID]struct{}{
			st.branches[2].ID(): {},
		}
		actualDeepestCommonDescendants := map[BranchID]struct{}{}
		deepestCommonDescendants.Consume(func(branch *Branch) {
			actualDeepestCommonDescendants[branch.ID()] = struct{}{}
		})
		assert.Equal(t, expectedDeepestCommonDescendants, actualDeepestCommonDescendants)
	}

	{
		deepestCommonDescendants, err := branchManager.findDeepestCommonDescendants(st.branches[2].ID(), st.branches[3].ID())
		assert.NoError(t, err)

		expectedDeepestCommonDescendants := map[BranchID]struct{}{
			st.branches[2].ID(): {}, st.branches[3].ID(): {},
		}
		actualDeepestCommonDescendants := map[BranchID]struct{}{}
		deepestCommonDescendants.Consume(func(branch *Branch) {
			actualDeepestCommonDescendants[branch.ID()] = struct{}{}
		})
		assert.Equal(t, expectedDeepestCommonDescendants, actualDeepestCommonDescendants)
	}

	{
		deepestCommonDescendants, err := branchManager.findDeepestCommonDescendants(st.branches[2].ID(), st.branches[4].ID(), st.branches[5].ID())
		assert.NoError(t, err)

		expectedDeepestCommonDescendants := map[BranchID]struct{}{
			st.branches[4].ID(): {}, st.branches[5].ID(): {},
		}
		actualDeepestCommonDescendants := map[BranchID]struct{}{}
		deepestCommonDescendants.Consume(func(branch *Branch) {
			actualDeepestCommonDescendants[branch.ID()] = struct{}{}
		})
		assert.Equal(t, expectedDeepestCommonDescendants, actualDeepestCommonDescendants)
	}

	// breaks: should only be aggr. branch 11 which consists out of branch 6 and 8
	{
		deepestCommonDescendants, err := branchManager.findDeepestCommonDescendants(
			st.branches[6].ID(), st.branches[8].ID(), st.branches[11].ID())
		assert.NoError(t, err)

		expectedDeepestCommonDescendants := map[BranchID]struct{}{
			st.branches[11].ID(): {},
		}
		actualDeepestCommonDescendants := map[BranchID]struct{}{}
		deepestCommonDescendants.Consume(func(branch *Branch) {
			actualDeepestCommonDescendants[branch.ID()] = struct{}{}
		})
		assert.Equal(t, expectedDeepestCommonDescendants, actualDeepestCommonDescendants)
	}

	// breaks: thinks that branch 13 is one of the deepest common descendants
	{
		deepestCommonDescendants, err := branchManager.findDeepestCommonDescendants(
			st.branches[6].ID(), st.branches[8].ID(), st.branches[10].ID(), st.branches[11].ID(),
			st.branches[12].ID(), st.branches[15].ID(), st.branches[14].ID(), st.branches[13].ID(),
			st.branches[16].ID())
		assert.NoError(t, err)

		expectedDeepestCommonDescendants := map[BranchID]struct{}{
			st.branches[12].ID(): {}, st.branches[15].ID(): {}, st.branches[16].ID(): {},
		}
		actualDeepestCommonDescendants := map[BranchID]struct{}{}
		deepestCommonDescendants.Consume(func(branch *Branch) {
			actualDeepestCommonDescendants[branch.ID()] = struct{}{}
		})
		assert.Equal(t, expectedDeepestCommonDescendants, actualDeepestCommonDescendants)
	}
}

func TestCollectClosestConflictAncestors(t *testing.T) {
	branchManager := New(mapdb.NewMapDB())

	st := createSampleTree(branchManager)
	defer st.Release()

	{
		aggregatedBranchConflictParents := make(CachedBranches)
		err := branchManager.collectClosestConflictAncestors(st.branches[10], aggregatedBranchConflictParents)
		assert.NoError(t, err)

		expectedClosestConflictAncestors := map[BranchID]struct{}{
			st.branches[3].ID(): {}, st.branches[6].ID(): {},
		}
		actualClosestConflictAncestors := map[BranchID]struct{}{}
		aggregatedBranchConflictParents.Consume(func(branch *Branch) {
			actualClosestConflictAncestors[branch.ID()] = struct{}{}
		})
		assert.Equal(t, expectedClosestConflictAncestors, actualClosestConflictAncestors)
	}

	{
		aggregatedBranchConflictParents := make(CachedBranches)
		err := branchManager.collectClosestConflictAncestors(st.branches[13], aggregatedBranchConflictParents)
		assert.NoError(t, err)

		expectedClosestConflictAncestors := map[BranchID]struct{}{
			st.branches[3].ID(): {}, st.branches[6].ID(): {},
		}
		actualClosestConflictAncestors := map[BranchID]struct{}{}
		aggregatedBranchConflictParents.Consume(func(branch *Branch) {
			actualClosestConflictAncestors[branch.ID()] = struct{}{}
		})
		assert.Equal(t, expectedClosestConflictAncestors, actualClosestConflictAncestors)
	}

	{
		aggregatedBranchConflictParents := make(CachedBranches)
		err := branchManager.collectClosestConflictAncestors(st.branches[14], aggregatedBranchConflictParents)
		assert.NoError(t, err)

		expectedClosestConflictAncestors := map[BranchID]struct{}{
			st.branches[8].ID(): {}, st.branches[6].ID(): {},
		}
		actualClosestConflictAncestors := map[BranchID]struct{}{}
		aggregatedBranchConflictParents.Consume(func(branch *Branch) {
			actualClosestConflictAncestors[branch.ID()] = struct{}{}
		})
		assert.Equal(t, expectedClosestConflictAncestors, actualClosestConflictAncestors)
	}

	{
		aggregatedBranchConflictParents := make(CachedBranches)
		err := branchManager.collectClosestConflictAncestors(st.branches[16], aggregatedBranchConflictParents)
		assert.NoError(t, err)

		expectedClosestConflictAncestors := map[BranchID]struct{}{
			st.branches[13].ID(): {}, st.branches[14].ID(): {},
		}
		actualClosestConflictAncestors := map[BranchID]struct{}{}
		aggregatedBranchConflictParents.Consume(func(branch *Branch) {
			actualClosestConflictAncestors[branch.ID()] = struct{}{}
		})
		assert.Equal(t, expectedClosestConflictAncestors, actualClosestConflictAncestors)
	}

	{
		// lets check whether an aggregated branch out of branch 2 and aggr. branch 11
		// resolves to the same ID as when the actual parents (6 and 8) of aggr. branch 11
		// are used in conjunction with branch 2.
		parentsBranches := CachedBranches{
			st.branches[2].ID(): st.cachedBranches[2].Retain(),
			st.branches[6].ID(): st.cachedBranches[6].Retain(),
			st.branches[8].ID(): st.cachedBranches[8].Retain(),
		}
		expectedAggrBranchID := branchManager.generateAggregatedBranchID(parentsBranches)

		cachedAggrBranch17, err := branchManager.AggregateBranches(st.branches[2].ID(), st.branches[11].ID())
		assert.NoError(t, err)
		assert.Equal(t, expectedAggrBranchID, cachedAggrBranch17.Unwrap().ID())
		cachedAggrBranch17.Release()
	}
}

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
	cachedAggrBranch8, aggrBranchErr := branchManager.AggregateBranches(branch4.ID(), branch6.ID())
	assert.NoError(t, aggrBranchErr)
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
	_, aggrBranchErr = branchManager.AggregateBranches(aggrBranch8.ID(), branch7.ID())
	assert.Error(t, aggrBranchErr, "can't aggregate branches aggr. branch 8 & conflict branch 7")

	// aggregated branch out of branch 5 (child of branch 2) and branch 7
	cachedAggrBranch9, aggrBranchErr := branchManager.AggregateBranches(branch5.ID(), branch7.ID())
	assert.NoError(t, aggrBranchErr)
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
	cachedAggrBranch10, aggrBranchErr := branchManager.AggregateBranches(branch3.ID(), branch6.ID())
	assert.NoError(t, aggrBranchErr)
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
	cachedAggrBranch13, aggrBranchErr := branchManager.AggregateBranches(branch6.ID(), branch11.ID())
	assert.NoError(t, aggrBranchErr)
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
	cachedAggrBranch14, aggrBranchErr := branchManager.AggregateBranches(aggrBranch10.ID(), aggrBranch13.ID())
	assert.NoError(t, aggrBranchErr)
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
	cachedAggrBranch15, aggrBranchErr := branchManager.AggregateBranches(branch2.ID(), branch7.ID(), branch12.ID())
	assert.NoError(t, aggrBranchErr)
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
	cachedAggrBranch16, aggrBranchErr := branchManager.AggregateBranches(aggrBranch15.ID(), aggrBranch9.ID())
	assert.NoError(t, aggrBranchErr)
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
	assert.False(t, modified)
	// simulate branch 7 being not preferred from FPC vote
	modified, err = branchManager.SetBranchPreferred(branch7.ID(), false)
	assert.NoError(t, err)
	assert.False(t, modified)
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
