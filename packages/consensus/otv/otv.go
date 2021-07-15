package otv

import (
	"bytes"

	"github.com/cockroachdb/errors"

	"github.com/iotaledger/goshimmer/packages/ledgerstate"
)

// WeightFunc returns the approval weight for the given branch.
type WeightFunc func(branchID ledgerstate.BranchID) (weight float64)

// OnTangleVoting is a pluggable implementation of tangle.ConsensusMechanism2. On tangle voting is a generalized form of
// Nakamoto consensus for the parallel-reality-based ledger state where the heaviest branch according to approval weight
// is liked by any given node.
type OnTangleVoting struct {
	branchDAG  *ledgerstate.BranchDAG
	weightFunc WeightFunc
}

// NewOnTangleVoting is the constructor for OnTangleVoting.
func NewOnTangleVoting(weightFunc WeightFunc, branchDAG *ledgerstate.BranchDAG) *OnTangleVoting {
	return &OnTangleVoting{
		weightFunc: weightFunc,
		branchDAG:  branchDAG,
	}
}

// Opinion splits the given branch IDs by examining all the conflict sets for each branch and checking whether
// it is the branch with the highest approval weight across all its conflict sets of which it is a member.
func (o *OnTangleVoting) Opinion(branchIDs ledgerstate.BranchIDs) (liked, disliked ledgerstate.BranchIDs, likedInstead map[ledgerstate.BranchID]ledgerstate.BranchIDs, err error) {
	liked, disliked = ledgerstate.NewBranchIDs(), ledgerstate.NewBranchIDs()
	likedInstead = make(map[ledgerstate.BranchID]ledgerstate.BranchIDs)
	for branchID := range branchIDs {
		resolvedConflictBranchIDs, err := o.branchDAG.ResolveConflictBranchIDs(ledgerstate.NewBranchIDs(branchID))
		if err != nil {
			return nil, nil, nil, errors.Wrapf(err, "unable to resolve conflict branch IDs of %s", branchID)
		}

		allParentsLiked := true
		for resolvedBranch := range resolvedConflictBranchIDs {
			if !o.doILike(resolvedBranch, ledgerstate.NewConflictIDs()) {
				allParentsLiked = false
				break
			}
		}

		if allParentsLiked {
			liked.Add(branchID)
			continue
		}

		disliked.Add(branchID)
		innerLikedInstead, err := o.LikedInstead(branchID)
		if err != nil {
			return nil, nil, nil, err
		}
		likedInstead[branchID] = innerLikedInstead
	}
	return
}

func (o *OnTangleVoting) LikedInstead(branchID ledgerstate.BranchID) (likedInstead ledgerstate.BranchIDs, err error) {
	likedInstead = ledgerstate.NewBranchIDs()
	resolvedConflictBranchIDs, err := o.branchDAG.ResolveConflictBranchIDs(ledgerstate.NewBranchIDs(branchID))
	if err != nil {
		return likedInstead, errors.Wrapf(err, "unable to resolve conflict branch IDs of %s", branchID)
	}

	for resolvedConflictBranchID := range resolvedConflictBranchIDs {
		o.branchDAG.ForEachConflictingBranchID(resolvedConflictBranchID, func(conflictingBranchID ledgerstate.BranchID) {
			if o.doILike(conflictingBranchID, ledgerstate.NewConflictIDs()) {
				likedInstead.Add(conflictingBranchID)
			}
		})

		if len(likedInstead) > 0 {
			continue
		}

		if o.doILike(resolvedConflictBranchID, ledgerstate.NewConflictIDs()) {
			continue
		}

		// here any direct conflicting branch is also disliked
		// which means that instead the liked branches have to be derived from branch's parents
		cachedBranch := o.branchDAG.Branch(resolvedConflictBranchID)
		for parent := range cachedBranch.Unwrap().Parents() {
			parentsLikedInstead, err := o.LikedInstead(parent)
			if err != nil {
				cachedBranch.Release()
				return nil, errors.Wrapf(err, "unable to determine liked instead of parent %s of %s", parent, branchID)
			}
			for k := range parentsLikedInstead {
				likedInstead.Add(k)
			}
		}
		cachedBranch.Release()
	}

	return
}

func (o *OnTangleVoting) doILike(branchID ledgerstate.BranchID, visitedConflicts ledgerstate.ConflictIDs) bool {
	// if any parent in the branch DAG is not liked, the current branch can't be liked either
	if parentsLiked := o.areParentsLiked(branchID, visitedConflicts); !parentsLiked {
		return false
	}

	for conflictSet := range o.conflictsSets(branchID) {
		// don't visit same conflict sets again
		if _, ok := visitedConflicts[conflictSet]; ok {
			continue
		}

		innerVisitedConflicts := visitedConflicts.Clone()
		innerVisitedConflicts.Add(conflictSet)

		cachedConflictMembers := o.branchDAG.ConflictMembers(conflictSet)
		for _, conflictMember := range cachedConflictMembers.Unwrap() {
			conflictBranchID := conflictMember.BranchID()
			// skip myself from the conflict set
			if conflictBranchID == branchID {
				continue
			}

			if o.doILike(conflictBranchID, innerVisitedConflicts) {
				if !o.weighsMore(branchID, conflictBranchID) {
					cachedConflictMembers.Release()
					return false
				}
			}
		}
		cachedConflictMembers.Release()
	}
	return true
}

// checks whether all parents of the given branchID are liked.
func (o *OnTangleVoting) areParentsLiked(branchID ledgerstate.BranchID, visitedConflicts ledgerstate.ConflictIDs) bool {
	parentsLiked := true
	o.branchDAG.Branch(branchID).Consume(func(branch ledgerstate.Branch) {
		for parent := range branch.Parents() {
			if parent == ledgerstate.MasterBranchID {
				continue
			}
			if !o.doILike(parent, visitedConflicts) {
				parentsLiked = false
				break
			}
		}
	})

	return parentsLiked
}

// checks whether branchA is heavier than branchB. If they have equal weight, the branch with lower lexical bytes is returned.
func (o *OnTangleVoting) weighsMore(branchA ledgerstate.BranchID, branchB ledgerstate.BranchID) bool {
	weight := o.weightFunc(branchA)
	weightConflict := o.weightFunc(branchB)
	// if the current highest weighted branch and the candidate branch share the same weight
	// we pick the branch with the lower lexical byte slice value to gain determinism
	if weight < weightConflict ||
		(weight == weightConflict && (bytes.Compare(branchA.Bytes(), branchB.Bytes()) > 0)) {
		return false
	}
	return true
}

// conflictsSets is a convenience wrapper to retrieve a copy of the conflictBranch's conflict sets.
func (o *OnTangleVoting) conflictsSets(conflictBranchID ledgerstate.BranchID) (conflicts ledgerstate.ConflictIDs) {
	o.branchDAG.Branch(conflictBranchID).Consume(func(branch ledgerstate.Branch) {
		conflicts = branch.(*ledgerstate.ConflictBranch).Conflicts()
	})
	return
}
