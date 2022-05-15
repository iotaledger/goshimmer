package conflictdag

import (
	"github.com/iotaledger/hive.go/generics/objectstorage"
	"github.com/iotaledger/hive.go/generics/set"
	"github.com/iotaledger/hive.go/generics/walker"
)

// Utils is a ConflictDAG component that bundles utility related API to simplify common interactions with the ConflictDAG.
type Utils struct {
	// branchDAG contains a reference to the ConflictDAG that created the Utils.
	branchDAG *ConflictDAG
}

// newUtils returns a new Utils instance for the given ConflictDAG.
func newUtils(branchDAG *ConflictDAG) (new *Utils) {
	return &Utils{
		branchDAG: branchDAG,
	}
}

func (u *Utils) ForEachChildBranchID(branchID BranchID, callback func(childBranchID BranchID)) {
	u.branchDAG.Storage.CachedChildBranches(branchID).Consume(func(childBranch *ChildBranch) {
		callback(childBranch.ChildBranchID())
	})
}

// ForEachBranch iterates over every existing Branch in the entire Storage.
func (u *Utils) ForEachBranch(consumer func(branch *Branch)) {
	u.branchDAG.Storage.branchStorage.ForEach(func(key []byte, cachedObject *objectstorage.CachedObject[*Branch]) bool {
		cachedObject.Consume(func(branch *Branch) {
			consumer(branch)
		})

		return true
	})
}

// ForEachConflictingBranchID executes the callback for each Branch that is conflicting with the named Branch.
func (u *Utils) ForEachConflictingBranchID(branchID BranchID, callback func(conflictingBranchID BranchID) bool) {
	u.branchDAG.Storage.CachedBranch(branchID).Consume(func(branch *Branch) {
		u.forEachConflictingBranchID(branch, callback)
	})
}

// ForEachConnectedConflictingBranchID executes the callback for each Branch that is directly or indirectly connected to
// the named Branch through a chain of intersecting conflicts.
func (u *Utils) ForEachConnectedConflictingBranchID(branchID BranchID, callback func(conflictingBranchID BranchID)) {
	traversedBranches := set.New[BranchID]()
	conflictSetsWalker := walker.New[ConflictID]()

	processBranchAndQueueConflictSets := func(branchID BranchID) {
		if !traversedBranches.Add(branchID) {
			return
		}

		u.branchDAG.Storage.CachedBranch(branchID).Consume(func(branch *Branch) {
			_ = branch.ConflictIDs().ForEach(func(conflictID ConflictID) (err error) {
				conflictSetsWalker.Push(conflictID)
				return nil
			})
		})
	}

	processBranchAndQueueConflictSets(branchID)

	for conflictSetsWalker.HasNext() {
		u.branchDAG.Storage.CachedConflictMembers(conflictSetsWalker.Next()).Consume(func(conflictMember *ConflictMember) {
			processBranchAndQueueConflictSets(conflictMember.BranchID())
		})
	}

	traversedBranches.ForEach(callback)
}

// forEachConflictingBranchID executes the callback for each Branch that is conflicting with the named Branch.
func (u *Utils) forEachConflictingBranchID(branch *Branch, callback func(conflictingBranchID BranchID) bool) {
	for it := branch.ConflictIDs().Iterator(); it.HasNext(); {
		abort := false
		u.branchDAG.Storage.CachedConflictMembers(it.Next()).Consume(func(conflictMember *ConflictMember) {
			if abort || conflictMember.BranchID() == branch.ID() {
				return
			}

			if abort = !callback(conflictMember.BranchID()); abort {
				it.StopWalk()
			}
		})
	}
}
