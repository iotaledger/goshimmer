package otv

import (
	"bytes"
	"sort"

	"github.com/iotaledger/hive.go/generics/set"
	"github.com/iotaledger/hive.go/generics/walker"

	"github.com/iotaledger/goshimmer/packages/consensus"
	"github.com/iotaledger/goshimmer/packages/ledger/branchdag"
	"github.com/iotaledger/goshimmer/packages/ledger/utxo"
)

// OnTangleVoting is a pluggable implementation of tangle.ConsensusMechanism2. On tangle voting is a generalized form of
// Nakamoto consensus for the parallel-reality-based ledger state where the heaviest branch according to approval weight
// is liked by any given node.
type OnTangleVoting struct {
	branchDAG  *branchdag.BranchDAG[utxo.TransactionID, utxo.OutputID]
	weightFunc consensus.WeightFunc
}

// NewOnTangleVoting is the constructor for OnTangleVoting.
func NewOnTangleVoting(branchDAG *branchdag.BranchDAG[utxo.TransactionID, utxo.OutputID], weightFunc consensus.WeightFunc) *OnTangleVoting {
	return &OnTangleVoting{
		branchDAG:  branchDAG,
		weightFunc: weightFunc,
	}
}

// LikedConflictMember returns the liked BranchID across the members of its conflict sets.
func (o *OnTangleVoting) LikedConflictMember(branchID utxo.TransactionID) (likedBranchID utxo.TransactionID, conflictMembers *set.AdvancedSet[utxo.TransactionID]) {
	conflictMembers = set.NewAdvancedSet[utxo.TransactionID]()
	o.branchDAG.Utils.ForEachConflictingBranchID(branchID, func(conflictingBranchID utxo.TransactionID) bool {
		if likedBranchID == utxo.EmptyTransactionID && o.BranchLiked(conflictingBranchID) {
			likedBranchID = conflictingBranchID
		}
		conflictMembers.Add(conflictingBranchID)

		return true
	})

	return
}

// BranchLiked returns whether the branch is the winner across all conflict sets (it is in the liked reality).
func (o *OnTangleVoting) BranchLiked(branchID utxo.TransactionID) (branchLiked bool) {
	branchLiked = true
	if branchID == utxo.EmptyTransactionID {
		return
	}
	for likeWalker := walker.New[utxo.TransactionID]().Push(branchID); likeWalker.HasNext(); {
		if branchLiked = branchLiked && o.branchPreferred(likeWalker.Next(), likeWalker); !branchLiked {
			return
		}
	}

	return
}

// branchPreferred returns whether the branch is the winner across its conflict sets.
func (o *OnTangleVoting) branchPreferred(branchID utxo.TransactionID, likeWalker *walker.Walker[utxo.TransactionID]) (preferred bool) {
	preferred = true
	if branchID == utxo.EmptyTransactionID {
		return
	}

	o.branchDAG.Storage.CachedBranch(branchID).Consume(func(branch *branchdag.Branch[utxo.TransactionID, utxo.OutputID]) {
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

func (o *OnTangleVoting) dislikedConnectedConflictingBranches(currentBranchID utxo.TransactionID) (dislikedBranches set.Set[utxo.TransactionID]) {
	dislikedBranches = set.New[utxo.TransactionID]()
	o.forEachConnectedConflictingBranchInDescendingOrder(currentBranchID, func(branchID utxo.TransactionID, weight float64) {
		if dislikedBranches.Has(branchID) {
			return
		}

		rejectionWalker := walker.New[utxo.TransactionID]()
		o.branchDAG.Utils.ForEachConflictingBranchID(branchID, func(conflictingBranchID utxo.TransactionID) bool {
			rejectionWalker.Push(conflictingBranchID)
			return true
		})

		for rejectionWalker.HasNext() {
			rejectedBranchID := rejectionWalker.Next()

			dislikedBranches.Add(rejectedBranchID)

			o.branchDAG.Storage.CachedChildBranches(rejectedBranchID).Consume(func(childBranch *branchdag.ChildBranch[utxo.TransactionID]) {
				rejectionWalker.Push(childBranch.ChildBranchID())
			})
		}
	})

	return dislikedBranches
}

// forEachConnectedConflictingBranchInDescendingOrder iterates over all branches connected via conflict sets
// and sorts them by weight. It calls the callback for each of them in that order.
func (o *OnTangleVoting) forEachConnectedConflictingBranchInDescendingOrder(branchID utxo.TransactionID, callback func(branchID utxo.TransactionID, weight float64)) {
	branchWeights := make(map[utxo.TransactionID]float64)
	branchesOrderedByWeight := make([]utxo.TransactionID, 0)
	o.branchDAG.Utils.ForEachConnectedConflictingBranchID(branchID, func(conflictingBranchID utxo.TransactionID) {
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
