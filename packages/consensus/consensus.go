package consensus

import (
	"github.com/iotaledger/goshimmer/packages/ledger/branchdag"
)

// WeightFunc returns the approval weight for the given branch.
type WeightFunc func(branchID branchdag.BranchID) (weight float64)

// Mechanism is a generic interface allowing to use different methods to reach consensus.
type Mechanism interface {
	// LikedConflictMember returns the liked BranchID across the members of its conflict sets.
	LikedConflictMember(branchID branchdag.BranchID) (likedBranchID branchdag.BranchID, conflictMembers branchdag.BranchIDs)
	// BranchLiked returns true if the BranchID is liked.
	BranchLiked(branchID branchdag.BranchID) (branchLiked bool)
}
