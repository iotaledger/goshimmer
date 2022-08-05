package consensus

import (
	"github.com/iotaledger/hive.go/core/generics/set"

	"github.com/iotaledger/goshimmer/packages/core/ledger/utxo"
)

// WeightFunc returns the approval weight for the given conflict.
type WeightFunc func(conflictID utxo.TransactionID) (weight float64)

// Mechanism is a generic interface allowing to use different methods to reach consensus.
type Mechanism interface {
	// LikedConflictMember returns the liked ConflictID across the members of its conflict sets.
	LikedConflictMember(conflictID utxo.TransactionID) (likedConflictID utxo.TransactionID, conflictMembers *set.AdvancedSet[utxo.TransactionID])
	// ConflictLiked returns true if the ConflictID is liked.
	ConflictLiked(conflictID utxo.TransactionID) (conflictLiked bool)
}
