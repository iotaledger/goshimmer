package otv

import (
	"bytes"

	"github.com/cockroachdb/errors"

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
			if !o.doILike(resolvedBranch, ledgerstate.NewConflictIDs()) {
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
		if o.doILike(resolvedConflictBranchID, ledgerstate.NewConflictIDs()) {
			continue
		}

		o.branchDAG.ForEachConflictingBranchID(resolvedConflictBranchID, func(conflictingBranchID ledgerstate.BranchID) {
			if o.doILike(conflictingBranchID, ledgerstate.NewConflictIDs()) {
				opinionTuple = append(opinionTuple, consensus.OpinionTuple{
					Liked:    conflictingBranchID,
					Disliked: resolvedConflictBranchID,
				})
			}
		})

		// here any direct conflicting branch is also disliked
		// which means that instead the liked branches have to be derived from branch's parents
		cachedBranch := o.branchDAG.Branch(resolvedConflictBranchID)
		for parent := range cachedBranch.Unwrap().Parents() {
			parentsOpinionTuple, err := o.LikedInstead(parent)
			if err != nil {
				cachedBranch.Release()
				return nil, errors.Wrapf(err, "unable to determine liked instead of parent %s of %s", parent, branchID)
			}
			// If I have multiple parents I have to add all of them to my tuple
			opinionTuple = append(opinionTuple, parentsOpinionTuple...)
		}
		cachedBranch.Release()
	}

	return opinionTuple, nil
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
func (o *OnTangleVoting) weighsMore(branchA, branchB ledgerstate.BranchID) bool {
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
