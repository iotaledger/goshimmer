package otv

import (
	"bytes"

	"github.com/cockroachdb/errors"
	"github.com/iotaledger/hive.go/types"

	"github.com/iotaledger/goshimmer/packages/ledgerstate"
)

// WeightFunc returns the approval weight for the given branch.
type WeightFunc func(branchID ledgerstate.BranchID) (weight float64)

type OnTangleVoting struct {
	branchDAG  *ledgerstate.BranchDAG
	weightFunc WeightFunc
}

func NewOnTangleVoting(weightFunc WeightFunc, branchDAG *ledgerstate.BranchDAG) *OnTangleVoting {
	return &OnTangleVoting{
		weightFunc: weightFunc,
		branchDAG:  branchDAG,
	}
}

// Opinion splits the given branch IDs by examining all the conflict sets for each branch and checking whether
// it is the branch with the highest approval weight across all its conflict sets of it is a member.
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
			liked[branchID] = types.Void
		} else {
			disliked[branchID] = types.Void
			innerLikedInstead, err := o.LikedFromConflictSet(branchID)
			if err != nil {
				return nil, nil, nil, err
			}
			likedInstead[branchID] = innerLikedInstead
		}
	}
	return
}

func (o *OnTangleVoting) LikedFromConflictSet(branchID ledgerstate.BranchID) (likedInstead ledgerstate.BranchIDs, err error) {
	resolvedConflictBranchIDs, err := o.branchDAG.ResolveConflictBranchIDs(ledgerstate.NewBranchIDs(branchID))
	likedInstead = ledgerstate.NewBranchIDs()
	if err != nil {
		return likedInstead, errors.Wrapf(err, "unable to resolve conflict branch IDs of %s", branchID)
	}

	for resolvedConflictBranchID := range resolvedConflictBranchIDs {
		o.branchDAG.ForEachConflictingBranchID(resolvedConflictBranchID, func(conflictingBranchID ledgerstate.BranchID) {
			if o.doILike(conflictingBranchID, ledgerstate.NewConflictIDs()) {
				likedInstead[conflictingBranchID] = types.Void
			}

		})
	}

	return
}

// Opinion splits the given branch IDs by examining all the conflict sets for each branch and checking whether
// it is the branch with the highest approval weight across all its conflict sets of it is a member.
func (o *OnTangleVoting) doILike(branchID ledgerstate.BranchID, visitedConflicts ledgerstate.ConflictIDs) bool {
	if parentsLiked := o.areParentsLiked(branchID, visitedConflicts); !parentsLiked {
		return false
	}

	conflictSets := o.conflictsSets(branchID)
	for conflictSet := range conflictSets {
		// Don't visit same conflict sets again
		if _, ok := visitedConflicts[conflictSet]; ok {
			continue
		}
		innerVisitedConflicts := visitedConflicts.Clone()
		innerVisitedConflicts[conflictSet] = types.Void
		cachedInnerConflictMembers := o.branchDAG.ConflictMembers(conflictSet)
		for _, innerConflictMember := range cachedInnerConflictMembers.Unwrap() {
			conflictBranchID := innerConflictMember.BranchID()
			// I skip myself from the conflict set
			if conflictBranchID == branchID {
				continue
			}
			if o.doILike(conflictBranchID, innerVisitedConflicts) {
				if !o.weighsMore(branchID, conflictBranchID) {
					cachedInnerConflictMembers.Release()
					return false
				}
			}
		}
		cachedInnerConflictMembers.Release()
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

func (o *OnTangleVoting) conflictsSets(conflictBranchID ledgerstate.BranchID) (conflicts ledgerstate.ConflictIDs) {
	o.branchDAG.Branch(conflictBranchID).Consume(func(branch ledgerstate.Branch) {
		conflicts = branch.(*ledgerstate.ConflictBranch).Conflicts()
	})
	return
}

type ConflictSetMemberFunc func(conflictID ledgerstate.ConflictID, conflictMember *ledgerstate.ConflictMember)
