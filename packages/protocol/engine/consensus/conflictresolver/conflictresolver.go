package conflictresolver

import (
	"bytes"
	"sort"

	"github.com/iotaledger/goshimmer/packages/protocol/engine/ledger/mempool/conflictdag"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/ledger/utxo"
	"github.com/iotaledger/hive.go/ds/set"
	"github.com/iotaledger/hive.go/ds/walker"
	"github.com/iotaledger/hive.go/lo"
)

type WeightFunc func(conflictID utxo.TransactionID) (weight int64)

// ConflictResolver is a generalized form of Nakamoto consensus for the parallel-reality-based ledger state where the
// heaviest conflict according to approval weight is liked by any given node.
type ConflictResolver struct {
	conflictDAG *conflictdag.ConflictDAG[utxo.TransactionID, utxo.OutputID]
	weightFunc  WeightFunc
}

// New is the constructor for ConflictResolver.
func New(conflictDAG *conflictdag.ConflictDAG[utxo.TransactionID, utxo.OutputID], weightFunc WeightFunc) *ConflictResolver {
	return &ConflictResolver{
		conflictDAG: conflictDAG,
		weightFunc:  weightFunc,
	}
}

// likedConflictMember returns the liked ConflictID across the members of its conflict sets.
func (o *ConflictResolver) likedConflictMember(conflictID utxo.TransactionID) (likedConflict utxo.TransactionID, dislikedConflicts utxo.TransactionIDs) {
	dislikedConflicts = utxo.NewTransactionIDs()

	conflict, exists := o.conflictDAG.Conflict(conflictID)
	if !exists {
		return utxo.EmptyTransactionID, dislikedConflicts
	}

	if o.ConflictLiked(conflict) {
		likedConflict = conflictID
	} else {
		dislikedConflicts.Add(conflictID)
	}

	// Try to find a liked conflict across the "flat" intersecting conflict set: we don't try to find a liked conflict on the parent conflicts.
	conflict.ForEachConflictingConflict(func(conflictingConflict *conflictdag.Conflict[utxo.TransactionID, utxo.OutputID]) bool {
		if likedConflict.IsEmpty() && o.ConflictLiked(conflictingConflict) {
			likedConflict = conflictingConflict.ID()
		} else {
			dislikedConflicts.Add(conflictingConflict.ID())
		}
		return true
	})

	return likedConflict, dislikedConflicts
}

// AdjustOpinion returns the reference that is necessary to correct our opinion on the given conflict.
// It recursively walk to the parent conflicts to find the upmost liked conflict.
func (o *ConflictResolver) AdjustOpinion(conflictID utxo.TransactionID) (likedConflict utxo.TransactionID, dislikedConflicts utxo.TransactionIDs) {
	dislikedConflicts = utxo.NewTransactionIDs()

	for w := walker.New[utxo.TransactionID](false).Push(conflictID); w.HasNext(); {
		currentConflictID := w.Next()

		likedConflictID, dislikedConflictIDs := o.likedConflictMember(currentConflictID)

		dislikedConflicts.AddAll(dislikedConflictIDs)

		if !likedConflictID.IsEmpty() {
			likedConflict = likedConflictID
			break
		}
		// only walk deeper if we don't like "something else"
		conflict, exists := o.conflictDAG.Conflict(currentConflictID)
		if exists {
			w.PushFront(conflict.Parents().Slice()...)
		}
	}

	return likedConflict, dislikedConflicts
}

// ConflictLiked returns whether the conflict is the winner across all conflict sets (it is in the liked reality).
func (o *ConflictResolver) ConflictLiked(conflict *conflictdag.Conflict[utxo.TransactionID, utxo.OutputID]) (conflictLiked bool) {
	conflictLiked = true
	if conflict.ID().IsEmpty() {
		return
	}

	for likeWalker := walker.New[*conflictdag.Conflict[utxo.TransactionID, utxo.OutputID]]().Push(conflict); likeWalker.HasNext(); {
		if conflictLiked = conflictLiked && o.conflictPreferred(likeWalker.Next(), likeWalker); !conflictLiked {
			return
		}
	}

	return
}

// conflictPreferred returns whether the conflict is the winner across its conflict sets.
func (o *ConflictResolver) conflictPreferred(conflict *conflictdag.Conflict[utxo.TransactionID, utxo.OutputID], likeWalker *walker.Walker[*conflictdag.Conflict[utxo.TransactionID, utxo.OutputID]]) (preferred bool) {
	preferred = true
	if conflict.ID().IsEmpty() {
		return
	}

	if conflict.ConfirmationState().IsAccepted() {
		return
	}

	if preferred = !o.dislikedConnectedConflictingConflicts(conflict).Has(conflict.ID()); preferred {
		for it := conflict.Parents().Iterator(); it.HasNext(); {
			parentConflict, exists := o.conflictDAG.Conflict(it.Next())
			if !exists {
				continue
			}
			likeWalker.Push(parentConflict)
		}
	}

	return
}

func (o *ConflictResolver) dislikedConnectedConflictingConflicts(currentConflict *conflictdag.Conflict[utxo.TransactionID, utxo.OutputID]) (dislikedConflicts set.Set[utxo.TransactionID]) {
	dislikedConflicts = set.New[utxo.TransactionID]()

	o.ForEachConnectedConflictingConflictInDescendingOrder(currentConflict, func(conflict *conflictdag.Conflict[utxo.TransactionID, utxo.OutputID]) {
		if dislikedConflicts.Has(conflict.ID()) {
			return
		}

		rejectionWalker := walker.New[*conflictdag.Conflict[utxo.TransactionID, utxo.OutputID]]()
		conflict.ForEachConflictingConflict(func(conflictingConflict *conflictdag.Conflict[utxo.TransactionID, utxo.OutputID]) bool {
			rejectionWalker.Push(conflictingConflict)
			return true
		})

		for rejectionWalker.HasNext() {
			rejectedConflict := rejectionWalker.Next()

			dislikedConflicts.Add(rejectedConflict.ID())

			rejectionWalker.PushAll(rejectedConflict.Children().Slice()...)
		}
	})

	return dislikedConflicts
}

// ForEachConnectedConflictingConflictInDescendingOrder iterates over all conflicts connected via conflict sets
// and sorts them by weight. It calls the callback for each of them in that order.
func (o *ConflictResolver) ForEachConnectedConflictingConflictInDescendingOrder(conflict *conflictdag.Conflict[utxo.TransactionID, utxo.OutputID], callback func(conflict *conflictdag.Conflict[utxo.TransactionID, utxo.OutputID])) {
	conflictWeights := make(map[utxo.TransactionID]int64)
	conflictsOrderedByWeight := make([]*conflictdag.Conflict[utxo.TransactionID, utxo.OutputID], 0)

	o.conflictDAG.ForEachConnectedConflictingConflictID(conflict, func(conflictingConflict *conflictdag.Conflict[utxo.TransactionID, utxo.OutputID]) {
		// TODO: possible race condition on weight function state because it's not locked
		conflictWeights[conflictingConflict.ID()] = o.weightFunc(conflictingConflict.ID())
		conflictsOrderedByWeight = append(conflictsOrderedByWeight, conflictingConflict)
	})

	sort.Slice(conflictsOrderedByWeight, func(i, j int) bool {
		conflictI := conflictsOrderedByWeight[i].ID()
		conflictJ := conflictsOrderedByWeight[j].ID()

		return !(conflictWeights[conflictI] < conflictWeights[conflictJ] || (conflictWeights[conflictI] == conflictWeights[conflictJ] && bytes.Compare(lo.PanicOnErr(conflictI.Bytes()), lo.PanicOnErr(conflictJ.Bytes())) > 0))
	})

	for _, orderedConflictID := range conflictsOrderedByWeight {
		callback(orderedConflictID)
	}
}
