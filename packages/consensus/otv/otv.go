package otv

import (
	"bytes"
	"sort"

	"github.com/iotaledger/hive.go/generics/set"
	"github.com/iotaledger/hive.go/generics/walker"

	"github.com/iotaledger/goshimmer/packages/consensus"
	"github.com/iotaledger/goshimmer/packages/ledgerstate"
)

// OnTangleVoting is a pluggable implementation of tangle.ConsensusMechanism2. On tangle voting is a generalized form of
// Nakamoto consensus for the parallel-reality-based ledger state where the heaviest branch according to approval weight
// is liked by any given node.
type OnTangleVoting struct {
	branchDAG  *ledgerstate.BranchDAG
	weightFunc consensus.WeightFunc
}

// NewOnTangleVoting is the constructor for OnTangleVoting.
func NewOnTangleVoting(branchDAG *ledgerstate.BranchDAG, weightFunc consensus.WeightFunc) *OnTangleVoting {
	return &OnTangleVoting{
		branchDAG:  branchDAG,
		weightFunc: weightFunc,
	}
}

// LikedConflictMember returns the liked BranchID across the members of its conflict sets.
func (o *OnTangleVoting) LikedConflictMember(branchID ledgerstate.BranchID) (likedBranchID ledgerstate.BranchID, conflictMembers ledgerstate.BranchIDs) {
	conflictMembers = ledgerstate.NewBranchIDs(branchID)
	if o.BranchLiked(branchID) {
		likedBranchID = branchID
	}

	o.branchDAG.ForEachConflictingBranchID(branchID, func(conflictingBranchID ledgerstate.BranchID) bool {
		if likedBranchID == ledgerstate.UndefinedBranchID && o.BranchLiked(conflictingBranchID) {
			likedBranchID = conflictingBranchID
		}
		conflictMembers.Add(conflictingBranchID)

		return true
	})

	return
}

// BranchLiked returns whether the branch is the winner across all conflict sets (it is in the liked reality).
func (o *OnTangleVoting) BranchLiked(branchID ledgerstate.BranchID) (branchLiked bool) {
	branchLiked = true
	if branchID == ledgerstate.MasterBranchID {
		return
	}
	for likeWalker := walker.New[ledgerstate.BranchID]().Push(branchID); likeWalker.HasNext(); {
		if branchLiked = branchLiked && o.branchPreferred(likeWalker.Next(), likeWalker); !branchLiked {
			return
		}
	}

	return
}

// branchPreferred returns whether the branch is the winner across its conflict sets.
func (o *OnTangleVoting) branchPreferred(branchID ledgerstate.BranchID, likeWalker *walker.Walker[ledgerstate.BranchID]) (preferred bool) {
	preferred = true
	if branchID == ledgerstate.MasterBranchID {
		return
	}

	o.branchDAG.Branch(branchID).Consume(func(branch *ledgerstate.Branch) {
		switch branch.InclusionState() {
		case ledgerstate.Rejected:
			preferred = false
			return
		case ledgerstate.Confirmed:
			return
		}

		if preferred = !o.dislikedConnectedConflictingBranches(branchID).Has(branchID); preferred {
			for parentBranchID := range branch.Parents() {
				likeWalker.Push(parentBranchID)
			}
		}
	})

	return
}

func (o *OnTangleVoting) dislikedConnectedConflictingBranches(currentBranchID ledgerstate.BranchID) (dislikedBranches set.Set[ledgerstate.BranchID]) {
	dislikedBranches = set.New[ledgerstate.BranchID]()
	o.forEachConnectedConflictingBranchInDescendingOrder(currentBranchID, func(branchID ledgerstate.BranchID, weight float64) {
		if dislikedBranches.Has(branchID) {
			return
		}

		rejectionWalker := walker.New[ledgerstate.BranchID]()
		o.branchDAG.ForEachConflictingBranchID(branchID, func(conflictingBranchID ledgerstate.BranchID) bool {
			rejectionWalker.Push(conflictingBranchID)
			return true
		})

		for rejectionWalker.HasNext() {
			rejectedBranchID := rejectionWalker.Next()

			dislikedBranches.Add(rejectedBranchID)

			o.branchDAG.ChildBranches(rejectedBranchID).Consume(func(childBranch *ledgerstate.ChildBranch) {
				rejectionWalker.Push(childBranch.ChildBranchID())
			})
		}
	})

	return dislikedBranches
}

// forEachConnectedConflictingBranchInDescendingOrder iterates over all branches connected via conflict sets
// and sorts them by weight. It calls the callback for each of them in that order.
func (o *OnTangleVoting) forEachConnectedConflictingBranchInDescendingOrder(branchID ledgerstate.BranchID, callback func(branchID ledgerstate.BranchID, weight float64)) {
	branchWeights := make(map[ledgerstate.BranchID]float64)
	branchesOrderedByWeight := make([]ledgerstate.BranchID, 0)
	o.branchDAG.ForEachConnectedConflictingBranchID(branchID, func(conflictingBranchID ledgerstate.BranchID) {
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
