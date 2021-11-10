package consensus

import (
	"fmt"

	"github.com/iotaledger/goshimmer/packages/ledgerstate"
)

// WeightFunc returns the approval weight for the given branch.
type WeightFunc func(branchID ledgerstate.BranchID) (weight float64)

// OpinionTuple expresses the root of an opinion in the BranchDAG.
type OpinionTuple struct {
	// Liked is the liked branch out of a conflict set.
	Liked ledgerstate.BranchID
	// Disliked is the disliked branch out of a conflict set.
	Disliked ledgerstate.BranchID
}

// String returns a human-readable version of the OpinionTuple.
func (ot OpinionTuple) String() string {
	return fmt.Sprintf("OpinionTuple(Liked:%s, Disliked:%s)", ot.Liked, ot.Disliked)
}

// Mechanism is a generic interface allowing to use different methods to reach consensus.
type Mechanism interface {
	// Opinion retrieves the opinion of the given branches.
	Opinion(branchIDs ledgerstate.BranchIDs) (liked, disliked ledgerstate.BranchIDs, err error)

	// LikedInstead returns the liked branch out of the conflict set of the given branch.
	LikedInstead(branchID ledgerstate.BranchID) (opinionTuple []OpinionTuple, err error)
}
