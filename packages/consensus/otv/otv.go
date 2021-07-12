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
			liked[branchID] = types.Void
		} else {
			disliked[branchID] = types.Void
		}
	}
	return
}

func (o *OnTangleVoting) LikedFromConflictSet(branchID ledgerstate.BranchID) (liked ledgerstate.BranchID, err error) {
	resolvedConflictBranchIDs, err := o.branchDAG.ResolveConflictBranchIDs(ledgerstate.NewBranchIDs(branchID))
	if err != nil {
		return ledgerstate.BranchID{}, errors.Wrapf(err, "unable to resolve conflict branch IDs of %s", branchID)
	}

	liked = branchID
	for resolvedConflictBranchID := range resolvedConflictBranchIDs {
		o.branchDAG.ForEachConflictingBranchID(resolvedConflictBranchID, func(conflictingBranchID ledgerstate.BranchID) {
			if o.doILike(conflictingBranchID, ledgerstate.NewConflictIDs()) {
				liked = conflictingBranchID
			}

		})
	}

	return
}

// Opinion splits the given branch IDs by examining all the conflict sets for each branch and checking whether
// it is the branch with the highest approval weight across all its conflict sets of it is a member.
func (o *OnTangleVoting) doILike(branchID ledgerstate.BranchID, visitedConflicts ledgerstate.ConflictIDs) bool {
	conflictSets := o.conflictsSets(branchID)
	for conflictSet := range conflictSets {
		// Don't visit same conflict sets again
		if _, ok := visitedConflicts[conflictSet]; ok {
			continue
		}
		innervisitedConflicts := visitedConflicts.Clone()
		innervisitedConflicts[conflictSet] = types.Void
		innerConflictMembers := o.branchDAG.ConflictMembers(conflictSet).Unwrap()
		for _, innerConflictMember := range innerConflictMembers {
			conflictBranchID := innerConflictMember.BranchID()
			// I skip myself from the conflict set
			if conflictBranchID == branchID {
				continue
			}
			if o.doILike(conflictBranchID, innervisitedConflicts) {
				if !o.weighsMore(branchID, conflictBranchID) {
					return false
				}
			}
		}
	}
	return true
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

func (o *OnTangleVoting) forEveryConflictSet(conflictBranchID ledgerstate.BranchID, onConflictMember ConflictSetMemberFunc) bool {
	return o.branchDAG.Branch(conflictBranchID).Consume(func(branch ledgerstate.Branch) {
		// go through all conflict sets that the conflictBranchID is part of
		for conflictID := range branch.(*ledgerstate.ConflictBranch).Conflicts() {
			cachedMembers := o.branchDAG.ConflictMembers(conflictID)
			members := cachedMembers.Unwrap()
			/*
				sort.Slice(members, func(i, j int) bool {
					return bytes.Compare(members[i].BranchID().Bytes(), members[j].BranchID().Bytes()) == 1
				})
			*/
			for _, member := range members {
				onConflictMember(conflictID, member)
			}
			cachedMembers.Release()
		}
	})
}
