package branchdag

import (
	"github.com/iotaledger/hive.go/generics/objectstorage"
	"github.com/iotaledger/hive.go/generics/set"
	"github.com/iotaledger/hive.go/generics/walker"
)

// Utils is a ConflictDAG component that bundles utility related API to simplify common interactions with the ConflictDAG.
type Utils[ConflictID ConflictIDType[ConflictID], ConflictSetID ConflictSetIDType[ConflictSetID]] struct {
	// branchDAG contains a reference to the ConflictDAG that created the Utils.
	branchDAG *ConflictDAG[ConflictID, ConflictSetID]
}

// newUtils returns a new Utils instance for the given ConflictDAG.
func newUtils[ConflictID ConflictIDType[ConflictID], ConflictSetID ConflictSetIDType[ConflictSetID]](branchDAG *ConflictDAG[ConflictID, ConflictSetID]) (new *Utils[ConflictID, ConflictSetID]) {
	return &Utils[ConflictID, ConflictSetID]{
		branchDAG: branchDAG,
	}
}

func (u *Utils[ConflictID, ConflictSetID]) ForEachChildBranchID(branchID ConflictID, callback func(childBranchID ConflictID)) {
	u.branchDAG.Storage.CachedChildBranches(branchID).Consume(func(childBranch *ChildBranch[ConflictID]) {
		callback(childBranch.ChildBranchID())
	})
}

// ForEachBranch iterates over every existing Branch in the entire Storage.
func (u *Utils[ConflictID, ConflictSetID]) ForEachBranch(consumer func(branch *Branch[ConflictID, ConflictSetID])) {
	u.branchDAG.Storage.branchStorage.ForEach(func(key []byte, cachedObject *objectstorage.CachedObject[*Branch[ConflictID, ConflictSetID]]) bool {
		cachedObject.Consume(func(branch *Branch[ConflictID, ConflictSetID]) {
			consumer(branch)
		})

		return true
	})
}

// ForEachConflictingBranchID executes the callback for each Branch that is conflicting with the named Branch.
func (u *Utils[ConflictID, ConflictSetID]) ForEachConflictingBranchID(branchID ConflictID, callback func(conflictingBranchID ConflictID) bool) {
	u.branchDAG.Storage.CachedBranch(branchID).Consume(func(branch *Branch[ConflictID, ConflictSetID]) {
		u.forEachConflictingBranchID(branch, callback)
	})
}

// ForEachConnectedConflictingBranchID executes the callback for each Branch that is directly or indirectly connected to
// the named Branch through a chain of intersecting conflicts.
func (u *Utils[ConflictID, ConflictSetID]) ForEachConnectedConflictingBranchID(branchID ConflictID, callback func(conflictingBranchID ConflictID)) {
	traversedBranches := set.New[ConflictID]()
	conflictSetsWalker := walker.New[ConflictSetID]()

	processBranchAndQueueConflictSets := func(branchID ConflictID) {
		if !traversedBranches.Add(branchID) {
			return
		}

		u.branchDAG.Storage.CachedBranch(branchID).Consume(func(branch *Branch[ConflictID, ConflictSetID]) {
			_ = branch.ConflictIDs().ForEach(func(conflictID ConflictSetID) (err error) {
				conflictSetsWalker.Push(conflictID)
				return nil
			})
		})
	}

	processBranchAndQueueConflictSets(branchID)

	for conflictSetsWalker.HasNext() {
		u.branchDAG.Storage.CachedConflictMembers(conflictSetsWalker.Next()).Consume(func(conflictMember *ConflictMember[ConflictID, ConflictSetID]) {
			processBranchAndQueueConflictSets(conflictMember.BranchID())
		})
	}

	traversedBranches.ForEach(callback)
}

// forEachConflictingBranchID executes the callback for each Branch that is conflicting with the named Branch.
func (u *Utils[ConflictID, ConflictSetID]) forEachConflictingBranchID(branch *Branch[ConflictID, ConflictSetID], callback func(conflictingBranchID ConflictID) bool) {
	for it := branch.ConflictIDs().Iterator(); it.HasNext(); {
		abort := false
		u.branchDAG.Storage.CachedConflictMembers(it.Next()).Consume(func(conflictMember *ConflictMember[ConflictID, ConflictSetID]) {
			if abort || conflictMember.BranchID() == branch.ID() {
				return
			}

			if abort = !callback(conflictMember.BranchID()); abort {
				it.StopWalk()
			}
		})
	}
}
