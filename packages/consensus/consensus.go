package consensus

import (
	"github.com/iotaledger/goshimmer/packages/ledgerstate"
)

// WeightFunc returns the approval weight for the given branch.
type WeightFunc func(branchID ledgerstate.BranchID) (weight float64)

// Mechanism is a generic interface allowing to use different methods to reach consensus.
type Mechanism interface {
	// LikedConflictMember returns the liked CompressedBranches across the members of its conflict sets.
	LikedConflictMember(branchID ledgerstate.BranchID) (likedBranchID ledgerstate.BranchID, conflictMembers ledgerstate.BranchIDs)
	// BranchLiked returns true if the CompressedBranches is liked.
	BranchLiked(branchID ledgerstate.BranchID) (branchLiked bool)
}
