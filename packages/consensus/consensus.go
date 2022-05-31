package consensus

import (
	"github.com/iotaledger/hive.go/generics/set"

	"github.com/iotaledger/goshimmer/packages/ledger/utxo"
)

// WeightFunc returns the approval weight for the given branch.
type WeightFunc func(branchID utxo.TransactionID) (weight float64)

// Mechanism is a generic interface allowing to use different methods to reach consensus.
type Mechanism interface {
	// LikedConflictMember returns the liked BranchID across the members of its conflict sets.
	LikedConflictMember(branchID utxo.TransactionID) (likedBranchID utxo.TransactionID, conflictMembers *set.AdvancedSet[utxo.TransactionID])
	// BranchLiked returns true if the BranchID is liked.
	BranchLiked(branchID utxo.TransactionID) (branchLiked bool)
}
