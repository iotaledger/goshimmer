package otv

import (
	"bytes"
	"sort"

	"github.com/iotaledger/hive.go/generics/set"
	"github.com/iotaledger/hive.go/generics/walker"

	"github.com/iotaledger/goshimmer/packages/consensus"
	"github.com/iotaledger/goshimmer/packages/ledger/branchdag"
)

// OnTangleVoting is a pluggable implementation of tangle.ConsensusMechanism2. On tangle voting is a generalized form of
// Nakamoto consensus for the parallel-reality-based ledger state where the heaviest branch according to approval weight
// is liked by any given node.
type OnTangleVoting struct {
	branchDAG  *branchdag.BranchDAG
	weightFunc consensus.WeightFunc
}

// NewOnTangleVoting is the constructor for OnTangleVoting.
func NewOnTangleVoting(branchDAG *branchdag.BranchDAG, weightFunc consensus.WeightFunc) *OnTangleVoting {
	return &OnTangleVoting{
		branchDAG:  branchDAG,
		weightFunc: weightFunc,
	}
}

// LikedConflictMember returns the liked BranchID across the members of its conflict sets.
func (o *OnTangleVoting) LikedConflictMember(branchID branchdag.BranchID) (likedBranchID branchdag.BranchID, conflictMembers branchdag.BranchIDs) {
	conflictMembers = branchdag.NewBranchIDs()
	o.branchDAG.Utils.ForEachConflictingBranchID(branchID, func(conflictingBranchID branchdag.BranchID) bool {
		if likedBranchID == branchdag.UndefinedBranchID && o.BranchLiked(conflictingBranchID) {
			likedBranchID = conflictingBranchID
		}
		conflictMembers.Add(conflictingBranchID)

		return true
	})

	return
}

// BranchLiked returns whether the branch is the winner across all conflict sets (it is in the liked reality).
func (o *OnTangleVoting) BranchLiked(branchID branchdag.BranchID) (branchLiked bool) {
	branchLiked = true
	if branchID == branchdag.MasterBranchID {
		return
	}
	for likeWalker := walker.New[branchdag.BranchID]().Push(branchID); likeWalker.HasNext(); {
		if branchLiked = branchLiked && o.branchPreferred(likeWalker.Next(), likeWalker); !branchLiked {
			return
		}
	}

	return
}

// branchPreferred returns whether the branch is the winner across its conflict sets.
func (o *OnTangleVoting) branchPreferred(branchID branchdag.BranchID, likeWalker *walker.Walker[branchdag.BranchID]) (preferred bool) {
	preferred = true
	if branchID == branchdag.MasterBranchID {
		return
	}

	o.branchDAG.Storage.CachedBranch(branchID).Consume(func(branch *branchdag.Branch) {
		switch branch.InclusionState() {
		case branchdag.Rejected:
			preferred = false
			return
		case branchdag.Confirmed:
			return
		}

		if preferred = !o.dislikedConnectedConflictingBranches(branchID).Has(branchID); preferred {
			for it := branch.Parents().Iterator(); it.HasNext(); {
				likeWalker.Push(it.Next())
			}
		}
	})

	return
}

func (o *OnTangleVoting) dislikedConnectedConflictingBranches(currentBranchID branchdag.BranchID) (dislikedBranches set.Set[branchdag.BranchID]) {
	dislikedBranches = set.New[branchdag.BranchID]()
	o.forEachConnectedConflictingBranchInDescendingOrder(currentBranchID, func(branchID branchdag.BranchID, weight float64) {
		if dislikedBranches.Has(branchID) {
			return
		}

		rejectionWalker := walker.New[branchdag.BranchID]()
		o.branchDAG.Utils.ForEachConflictingBranchID(branchID, func(conflictingBranchID branchdag.BranchID) bool {
			rejectionWalker.Push(conflictingBranchID)
			return true
		})

		for rejectionWalker.HasNext() {
			rejectedBranchID := rejectionWalker.Next()

			dislikedBranches.Add(rejectedBranchID)

			o.branchDAG.Storage.CachedChildBranches(rejectedBranchID).Consume(func(childBranch *branchdag.ChildBranch) {
				rejectionWalker.Push(childBranch.ChildBranchID())
			})
		}
	})

	return dislikedBranches
}

// forEachConnectedConflictingBranchInDescendingOrder iterates over all branches connected via conflict sets
// and sorts them by weight. It calls the callback for each of them in that order.
func (o *OnTangleVoting) forEachConnectedConflictingBranchInDescendingOrder(branchID branchdag.BranchID, callback func(branchID branchdag.BranchID, weight float64)) {
	branchWeights := make(map[branchdag.BranchID]float64)
	branchesOrderedByWeight := make([]branchdag.BranchID, 0)
	o.branchDAG.Utils.ForEachConnectedConflictingBranchID(branchID, func(conflictingBranchID branchdag.BranchID) {
		branchWeights[conflictingBranchID] = o.weightFunc(conflictingBranchID)
		branchesOrderedByWeight = append(branchesOrderedByWeight, conflictingBranchID)
	})

	sort.Slice(branchesOrderedByWeight, func(i, j int) bool {
		branchI := branchesOrderedByWeight[i]
		branchJ := branchesOrderedByWeight[j]

		return !(branchWeights[branchI] < branchWeights[branchJ] || (branchWeights[branchI] == branchWeights[branchJ] && bytes.Compare(branchI.Bytes(), branchJ.Bytes()) > 0))
	})

	for _, orderedBranchID := range branchesOrderedByWeight {
		callback(orderedBranchID, branchWeights[orderedBranchID])
	}
}
