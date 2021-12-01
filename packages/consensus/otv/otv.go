package otv

import (
	"bytes"
	"sort"

	"github.com/cockroachdb/errors"
	"github.com/iotaledger/hive.go/datastructure/set"
	"github.com/iotaledger/hive.go/datastructure/walker"

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

// LikedInstead determines what vote should be cast given the provided branchID.
func (o *OnTangleVoting) LikedInstead(branchIDs ledgerstate.BranchIDs) (likedBranchIDs ledgerstate.BranchIDs, err error) {
	resolvedConflictBranchIDs, err := o.branchDAG.ResolveConflictBranchIDs(branchIDs)
	if err != nil {
		return nil, errors.Errorf("unable to resolve conflict branch IDs of %s: %w", err)
	}

	likedBranchIDs = ledgerstate.NewBranchIDs()
	for resolvedConflictBranchID := range resolvedConflictBranchIDs {
		if o.branchLiked(resolvedConflictBranchID) {
			continue
		}

		likedBranchIDs.Add(o.conflictBranchLikedInstead(resolvedConflictBranchID))
	}

	return
}

func (o *OnTangleVoting) conflictBranchLikedInstead(branchID ledgerstate.BranchID) (likedBranchID ledgerstate.BranchID) {
	for parentWalker := walker.New().Push(branchID); parentWalker.HasNext(); {
		currentBranchID := parentWalker.Next().(ledgerstate.BranchID)

		o.branchDAG.ForEachConflictingBranchID(currentBranchID, func(conflictingBranchID ledgerstate.BranchID) {
			if likedBranchID != ledgerstate.UndefinedBranchID {
				return
			}

			if o.branchLiked(conflictingBranchID) {
				likedBranchID = conflictingBranchID
			}
		})
		if likedBranchID != ledgerstate.UndefinedBranchID {
			return
		}

		o.branchDAG.Branch(currentBranchID).ConsumeConflictBranch(func(conflictBranch *ledgerstate.ConflictBranch) {
			for parentBranchID := range conflictBranch.Parents() {
				parentWalker.Push(parentBranchID)
			}
		})
	}

	return
}

func (o *OnTangleVoting) branchLiked(branchID ledgerstate.BranchID) (branchLiked bool) {
	branchLiked = true
	for likeWalker := walker.New().Push(branchID); likeWalker.HasNext(); {
		if branchLiked = branchLiked && o.branchPreferred(likeWalker.Next().(ledgerstate.BranchID), likeWalker); !branchLiked {
			return
		}
	}

	return
}

func (o *OnTangleVoting) branchPreferred(branchID ledgerstate.BranchID, likeWalker *walker.Walker) (preferred bool) {
	preferred = true
	if branchID == ledgerstate.MasterBranchID {
		return
	}

	o.branchDAG.Branch(branchID).ConsumeConflictBranch(func(currentBranch *ledgerstate.ConflictBranch) {
		switch currentBranch.InclusionState() {
		case ledgerstate.Rejected:
			preferred = false
			return
		case ledgerstate.Confirmed:
			return
		}

		if preferred = !o.dislikedConnectedConflictingBranches(branchID).Has(branchID); preferred {
			for parentBranchID := range currentBranch.Parents() {
				likeWalker.Push(parentBranchID)
			}
		}
	})

	return
}

func (o *OnTangleVoting) dislikedConnectedConflictingBranches(currentBranchID ledgerstate.BranchID) (dislikedBranches set.Set) {
	dislikedBranches = set.New()
	o.forEachConnectedConflictingBranchInDescendingOrder(currentBranchID, func(branchID ledgerstate.BranchID, weight float64) {
		if dislikedBranches.Has(branchID) {
			return
		}

		rejectionWalker := walker.New()
		o.branchDAG.ForEachConflictingBranchID(branchID, func(conflictingBranchID ledgerstate.BranchID) {
			rejectionWalker.Push(conflictingBranchID)
		})

		for rejectionWalker.HasNext() {
			rejectedBranchID := rejectionWalker.Next().(ledgerstate.BranchID)

			dislikedBranches.Add(rejectedBranchID)

			o.branchDAG.ChildBranches(rejectedBranchID).Consume(func(childBranch *ledgerstate.ChildBranch) {
				if childBranch.ChildBranchType() == ledgerstate.ConflictBranchType {
					rejectionWalker.Push(childBranch.ChildBranchID())
				}
			})
		}
	})

	return dislikedBranches
}

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
