package branchdag

import (
	"github.com/cockroachdb/errors"
	"github.com/iotaledger/hive.go/generics/set"
	"github.com/iotaledger/hive.go/generics/walker"
)

type utils struct {
	branchDAG *BranchDAG
}

func newUtil(branchDAG *BranchDAG) (new *utils) {
	return &utils{
		branchDAG: branchDAG,
	}
}

// ForEachConflictingBranchID executes the callback for each Branch that is conflicting with the Branch
// identified by the given BranchID.
func (u *utils) ForEachConflictingBranchID(branchID BranchID, callback func(conflictingBranchID BranchID) bool) {
	abort := false
	u.branchDAG.Storage.Branch(branchID).Consume(func(branch *Branch) {
		_ = branch.Conflicts().ForEach(func(conflictID ConflictID) (err error) {
			u.branchDAG.Storage.ConflictMembers(conflictID).Consume(func(conflictMember *ConflictMember) {
				if abort || conflictMember.BranchID() == branchID {
					return
				}

				abort = !callback(conflictMember.BranchID())
			})

			if abort {
				return errors.New("abort")
			}

			return nil
		})
	})
}

// ForEachConnectedConflictingBranchID executes the callback for each Branch that is connected through a chain
// of intersecting ConflictSets.
func (u *utils) ForEachConnectedConflictingBranchID(branchID BranchID, callback func(conflictingBranchID BranchID)) {
	traversedBranches := set.New[BranchID]()
	conflictSetsWalker := walker.New[ConflictID]()

	processBranchAndQueueConflictSets := func(branchID BranchID) {
		if !traversedBranches.Add(branchID) {
			return
		}

		u.branchDAG.Storage.Branch(branchID).Consume(func(branch *Branch) {
			_ = branch.Conflicts().ForEach(func(conflictID ConflictID) (err error) {
				conflictSetsWalker.Push(conflictID)
				return nil
			})
		})
	}

	processBranchAndQueueConflictSets(branchID)

	for conflictSetsWalker.HasNext() {
		u.branchDAG.Storage.ConflictMembers(conflictSetsWalker.Next()).Consume(func(conflictMember *ConflictMember) {
			processBranchAndQueueConflictSets(conflictMember.BranchID())
		})
	}

	traversedBranches.ForEach(func(element BranchID) {
		callback(element)
	})
}
