package conflictdag

import (
	"github.com/iotaledger/hive.go/generics/objectstorage"
	"github.com/iotaledger/hive.go/generics/set"
	"github.com/iotaledger/hive.go/generics/walker"
)

// Utils is a ConflictDAG component that bundles utility related API to simplify common interactions with the ConflictDAG.
type Utils[ConflictID comparable, ConflictSetID comparable] struct {
	// branchDAG contains a reference to the ConflictDAG that created the Utils.
	branchDAG *ConflictDAG[ConflictID, ConflictSetID]
}

// newUtils returns a new Utils instance for the given ConflictDAG.
func newUtils[ConflictID comparable, ConflictSetID comparable](branchDAG *ConflictDAG[ConflictID, ConflictSetID]) (new *Utils[ConflictID, ConflictSetID]) {
	return &Utils[ConflictID, ConflictSetID]{
		branchDAG: branchDAG,
	}
}

func (u *Utils[ConflictID, ConflictSetID]) ForEachChildBranchID(branchID ConflictID, callback func(childBranchID ConflictID)) {
	u.branchDAG.Storage.CachedChildBranches(branchID).Consume(func(childBranch *ChildBranch[ConflictID]) {
		callback(childBranch.ChildBranchID())
	})
}

// ForEachBranch iterates over every existing Conflict in the entire Storage.
func (u *Utils[ConflictID, ConflictSetID]) ForEachBranch(consumer func(branch *Conflict[ConflictID, ConflictSetID])) {
	u.branchDAG.Storage.branchStorage.ForEach(func(key []byte, cachedObject *objectstorage.CachedObject[*Conflict[ConflictID, ConflictSetID]]) bool {
		cachedObject.Consume(func(branch *Conflict[ConflictID, ConflictSetID]) {
			consumer(branch)
		})

		return true
	})
}

// ForEachConflictingBranchID executes the callback for each Conflict that is conflicting with the named Conflict.
func (u *Utils[ConflictID, ConflictSetID]) ForEachConflictingBranchID(branchID ConflictID, callback func(conflictingBranchID ConflictID) bool) {
	u.branchDAG.Storage.CachedConflict(branchID).Consume(func(branch *Conflict[ConflictID, ConflictSetID]) {
		u.forEachConflictingBranchID(branch, callback)
	})
}

// ForEachConnectedConflictingBranchID executes the callback for each Conflict that is directly or indirectly connected to
// the named Conflict through a chain of intersecting conflicts.
func (u *Utils[ConflictID, ConflictSetID]) ForEachConnectedConflictingBranchID(branchID ConflictID, callback func(conflictingBranchID ConflictID)) {
	traversedBranches := set.New[ConflictID]()
	conflictSetsWalker := walker.New[ConflictSetID]()

	processBranchAndQueueConflictSets := func(branchID ConflictID) {
		if !traversedBranches.Add(branchID) {
			return
		}

		u.branchDAG.Storage.CachedConflict(branchID).Consume(func(branch *Conflict[ConflictID, ConflictSetID]) {
			_ = branch.ConflictIDs().ForEach(func(conflictID ConflictSetID) (err error) {
				conflictSetsWalker.Push(conflictID)
				return nil
			})
		})
	}

	processBranchAndQueueConflictSets(branchID)

	for conflictSetsWalker.HasNext() {
		u.branchDAG.Storage.CachedConflictMembers(conflictSetsWalker.Next()).Consume(func(conflictMember *ConflictMember[ConflictSetID, ConflictID]) {
			processBranchAndQueueConflictSets(conflictMember.ConflictID())
		})
	}

	traversedBranches.ForEach(callback)
}

// forEachConflictingBranchID executes the callback for each Conflict that is conflicting with the named Conflict.
func (u *Utils[ConflictID, ConflictSetID]) forEachConflictingBranchID(branch *Conflict[ConflictID, ConflictSetID], callback func(conflictingBranchID ConflictID) bool) {
	for it := branch.ConflictIDs().Iterator(); it.HasNext(); {
		abort := false
		u.branchDAG.Storage.CachedConflictMembers(it.Next()).Consume(func(conflictMember *ConflictMember[ConflictSetID, ConflictID]) {
			if abort || conflictMember.ConflictID() == branch.ID() {
				return
			}

			if abort = !callback(conflictMember.ConflictID()); abort {
				it.StopWalk()
			}
		})
	}
}
