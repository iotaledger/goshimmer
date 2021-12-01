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

// Opinion splits the given branch IDs by examining all the conflict sets for each branch and checking whether
// it is the branch with the highest approval weight across all its conflict sets of which it is a member.
func (o *OnTangleVoting) Opinion(branchIDs ledgerstate.BranchIDs) (liked, disliked ledgerstate.BranchIDs, err error) {
	liked, disliked = ledgerstate.NewBranchIDs(), ledgerstate.NewBranchIDs()
	for branchID := range branchIDs {
		resolvedConflictBranchIDs, err := o.branchDAG.ResolveConflictBranchIDs(ledgerstate.NewBranchIDs(branchID))
		if err != nil {
			return nil, nil, errors.Wrapf(err, "unable to resolve conflict branch IDs of %s", branchID)
		}

		allParentsLiked := true
		for resolvedBranch := range resolvedConflictBranchIDs {
			if !o.doILike(resolvedBranch) {
				allParentsLiked = false
				break
			}
		}

		if allParentsLiked {
			liked.Add(branchID)
			continue
		}

		parentsOpinionTuple, err := o.LikedInstead(branchID)
		if err != nil {
			return nil, nil, err
		}
		for _, k := range parentsOpinionTuple {
			liked.Add(k.Liked)
			disliked.Add(k.Disliked)
		}
	}
	return liked, disliked, nil
}

// LikedInstead determines what vote should be cast given the provided branchID.
func (o *OnTangleVoting) LikedInstead(branchID ledgerstate.BranchID) (opinionTuple []consensus.OpinionTuple, err error) {
	opinionTuple = make([]consensus.OpinionTuple, 0)
	resolvedConflictBranchIDs, err := o.branchDAG.ResolveConflictBranchIDs(ledgerstate.NewBranchIDs(branchID))
	if err != nil {
		return opinionTuple, errors.Wrapf(err, "unable to resolve conflict branch IDs of %s", branchID)
	}

	for resolvedConflictBranchID := range resolvedConflictBranchIDs {
		// I like myself
		if o.doILike(resolvedConflictBranchID) {
			continue
		}

		alternativeFound := false
		o.branchDAG.ForEachConflictingBranchID(resolvedConflictBranchID, func(conflictingBranchID ledgerstate.BranchID) {
			if !alternativeFound && o.doILike(conflictingBranchID) {
				opinionTuple = append(opinionTuple, consensus.OpinionTuple{
					Liked:    conflictingBranchID,
					Disliked: resolvedConflictBranchID,
				})
			}
		})

		// here any direct conflicting branch is also disliked
		// which means that instead the liked branches have to be derived from branch's parents
		o.branchDAG.Branch(resolvedConflictBranchID).Consume(func(branch ledgerstate.Branch) {
			for parent := range branch.Parents() {
				parentsOpinionTuple, likedErr := o.LikedInstead(parent)
				if likedErr != nil {
					err = errors.Wrapf(likedErr, "unable to determine liked instead of parent %s of %s", parent, branchID)
					return
				}
				// If I have multiple parents I have to add all of them to my tuple
				opinionTuple = append(opinionTuple, parentsOpinionTuple...)
			}
		})
		if err != nil {
			return nil, err
		}
	}

	return opinionTuple, nil
}

func (o *OnTangleVoting) doILike(branchID ledgerstate.BranchID) (branchLiked bool) {
	branchLiked = true

	likeWalker := walker.New()
	for likeWalker.Push(branchID); likeWalker.HasNext(); {
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
