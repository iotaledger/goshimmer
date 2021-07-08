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

		allParentsHaveHighestWeight := true
		for conflictBranchID := range resolvedConflictBranchIDs {
			if _, highestWeightedBranchID := o.highestWeightedBranchFromConflictSets(conflictBranchID); highestWeightedBranchID != conflictBranchID {
				disliked[branchID] = types.Void
				allParentsHaveHighestWeight = false
				break
			}
		}

		if allParentsHaveHighestWeight {
			liked[branchID] = types.Void
		}
	}

	// TODO: given the liked set, examine whether they are conflicting with each other and then reduce the set
	// to the ones which have a higher approval weight within their conflict set.

	return
}

func (o *OnTangleVoting) LikedFromConflictSet(branchID ledgerstate.BranchID) (likedBranchIDs ledgerstate.BranchIDs, err error) {
	resolvedConflictBranchIDs, err := o.branchDAG.ResolveConflictBranchIDs(ledgerstate.NewBranchIDs(branchID))
	if err != nil {
		return nil, errors.Wrapf(err, "unable to resolve conflict branch IDs of %s", branchID)
	}

	branchWeights := make(map[ledgerstate.BranchID]float64)
	for conflictBranchID := range resolvedConflictBranchIDs {
		weight, branchID := o.highestWeightedBranchFromConflictSets(conflictBranchID)
		branchWeights[branchID] = weight
	}
	return
}

// returns the branch with the highest approval weight from all the conflict sets of which the given branch is a member of.
func (o *OnTangleVoting) highestWeightedBranchFromConflictSets(conflictBranchID ledgerstate.BranchID) (highestWeight float64, highestWeightedBranch ledgerstate.BranchID) {
	o.forEveryConflictSet(conflictBranchID,
		func(conflictMember *ledgerstate.ConflictMember) {
			weight := o.weightFunc(conflictMember.BranchID())
			// if the current highest weighted branch and the candidate branch share the same weight
			// we pick the branch with the lower lexical byte slice value to gain determinism
			if weight > highestWeight ||
				(weight == highestWeight && (bytes.Compare(highestWeightedBranch.Bytes(), conflictMember.Bytes()) == 1)) {
				highestWeight = weight
				highestWeightedBranch = conflictMember.BranchID()
			}
		})
	return
}

type ConflictSetMemberFunc func(conflictMember *ledgerstate.ConflictMember)

func (o *OnTangleVoting) forEveryConflictSet(conflictBranchID ledgerstate.BranchID, onConflictMember ConflictSetMemberFunc) bool {
	return o.branchDAG.Branch(conflictBranchID).Consume(func(branch ledgerstate.Branch) {
		// go through all conflict sets that the conflictBranchID is part of
		for conflictID := range branch.(*ledgerstate.ConflictBranch).Conflicts() {
			o.branchDAG.ConflictMembers(conflictID).Consume(onConflictMember)
		}
	})
}
