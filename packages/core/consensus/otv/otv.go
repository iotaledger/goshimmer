package otv

import (
	"bytes"
	"sort"

	"github.com/iotaledger/hive.go/core/generics/set"
	"github.com/iotaledger/hive.go/core/generics/walker"
	"github.com/iotaledger/hive.go/core/types/confirmation"

	"github.com/iotaledger/goshimmer/packages/core/conflictdag"

	"github.com/iotaledger/goshimmer/packages/core/consensus"
	"github.com/iotaledger/goshimmer/packages/core/ledger/utxo"
)

// OnTangleVoting is a pluggable implementation of tangle.ConsensusMechanism2. On tangle voting is a generalized form of
// Nakamoto consensus for the parallel-reality-based ledger state where the heaviest conflict according to approval weight
// is liked by any given node.
type OnTangleVoting struct {
	conflictDAG *conflictdag.ConflictDAG[utxo.TransactionID, utxo.OutputID]
	weightFunc  consensus.WeightFunc
}

// NewOnTangleVoting is the constructor for OnTangleVoting.
func NewOnTangleVoting(conflictDAG *conflictdag.ConflictDAG[utxo.TransactionID, utxo.OutputID], weightFunc consensus.WeightFunc) *OnTangleVoting {
	return &OnTangleVoting{
		conflictDAG: conflictDAG,
		weightFunc:  weightFunc,
	}
}

// LikedConflictMember returns the liked ConflictID across the members of its conflict sets.
func (o *OnTangleVoting) LikedConflictMember(conflictID utxo.TransactionID) (likedConflict utxo.TransactionID, dislikedConflicts utxo.TransactionIDs) {
	dislikedConflicts = utxo.NewTransactionIDs()

	if o.ConflictLiked(conflictID) {
		likedConflict = conflictID
	} else {
		dislikedConflicts.Add(conflictID)
	}

	o.conflictDAG.Utils.ForEachConflictingConflictID(conflictID, func(conflictingConflictID utxo.TransactionID) bool {
		if likedConflict.IsEmpty() && o.ConflictLiked(conflictingConflictID) {
			likedConflict = conflictingConflictID
		} else {
			dislikedConflicts.Add(conflictingConflictID)
		}

		return true
	})

	return
}

// ConflictLiked returns whether the conflict is the winner across all conflict sets (it is in the liked reality).
func (o *OnTangleVoting) ConflictLiked(conflictID utxo.TransactionID) (conflictLiked bool) {
	conflictLiked = true
	if conflictID == utxo.EmptyTransactionID {
		return
	}
	for likeWalker := walker.New[utxo.TransactionID]().Push(conflictID); likeWalker.HasNext(); {
		if conflictLiked = conflictLiked && o.conflictPreferred(likeWalker.Next(), likeWalker); !conflictLiked {
			return
		}
	}

	return
}

// conflictPreferred returns whether the conflict is the winner across its conflict sets.
func (o *OnTangleVoting) conflictPreferred(conflictID utxo.TransactionID, likeWalker *walker.Walker[utxo.TransactionID]) (preferred bool) {
	preferred = true
	if conflictID == utxo.EmptyTransactionID {
		return
	}

	o.conflictDAG.Storage.CachedConflict(conflictID).Consume(func(conflict *conflictdag.Conflict[utxo.TransactionID, utxo.OutputID]) {
		switch conflict.ConfirmationState() {
		case confirmation.Rejected:
			preferred = false
			return
		case confirmation.Accepted:
		case confirmation.Confirmed:
			return
		}

		if preferred = !o.dislikedConnectedConflictingConflicts(conflictID).Has(conflictID); preferred {
			for it := conflict.Parents().Iterator(); it.HasNext(); {
				likeWalker.Push(it.Next())
			}
		}
	})

	return
}

func (o *OnTangleVoting) dislikedConnectedConflictingConflicts(currentConflictID utxo.TransactionID) (dislikedConflicts set.Set[utxo.TransactionID]) {
	dislikedConflicts = set.New[utxo.TransactionID]()
	o.forEachConnectedConflictingConflictInDescendingOrder(currentConflictID, func(conflictID utxo.TransactionID, weight float64) {
		if dislikedConflicts.Has(conflictID) {
			return
		}

		rejectionWalker := walker.New[utxo.TransactionID]()
		o.conflictDAG.Utils.ForEachConflictingConflictID(conflictID, func(conflictingConflictID utxo.TransactionID) bool {
			rejectionWalker.Push(conflictingConflictID)
			return true
		})

		for rejectionWalker.HasNext() {
			rejectedConflictID := rejectionWalker.Next()

			dislikedConflicts.Add(rejectedConflictID)

			o.conflictDAG.Storage.CachedChildConflicts(rejectedConflictID).Consume(func(childConflict *conflictdag.ChildConflict[utxo.TransactionID]) {
				rejectionWalker.Push(childConflict.ChildConflictID())
			})
		}
	})

	return dislikedConflicts
}

// forEachConnectedConflictingConflictInDescendingOrder iterates over all conflicts connected via conflict sets
// and sorts them by weight. It calls the callback for each of them in that order.
func (o *OnTangleVoting) forEachConnectedConflictingConflictInDescendingOrder(conflictID utxo.TransactionID, callback func(conflictID utxo.TransactionID, weight float64)) {
	conflictWeights := make(map[utxo.TransactionID]float64)
	conflictsOrderedByWeight := make([]utxo.TransactionID, 0)
	o.conflictDAG.Utils.ForEachConnectedConflictingConflictID(conflictID, func(conflictingConflictID utxo.TransactionID) {
		conflictWeights[conflictingConflictID] = o.weightFunc(conflictingConflictID)
		conflictsOrderedByWeight = append(conflictsOrderedByWeight, conflictingConflictID)
	})

	sort.Slice(conflictsOrderedByWeight, func(i, j int) bool {
		conflictI := conflictsOrderedByWeight[i]
		conflictJ := conflictsOrderedByWeight[j]

		return !(conflictWeights[conflictI] < conflictWeights[conflictJ] || (conflictWeights[conflictI] == conflictWeights[conflictJ] && bytes.Compare(conflictI.Bytes(), conflictJ.Bytes()) > 0))
	})

	for _, orderedConflictID := range conflictsOrderedByWeight {
		callback(orderedConflictID, conflictWeights[orderedConflictID])
	}
}
