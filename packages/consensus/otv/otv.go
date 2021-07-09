package otv

import (
	"bytes"
	"fmt"

	"github.com/cockroachdb/errors"
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
		_ = resolvedConflictBranchIDs

		o.forEveryConflictSet(branchID, func(conflictID ledgerstate.ConflictID, conflictMember *ledgerstate.ConflictMember) {
			if conflictMember.BranchID() == branchID {
				return
			}
			fmt.Println("outer conflict set member", conflictMember.BranchID())

			o.forEveryConflictSet(conflictMember.BranchID(), func(innerConflictID ledgerstate.ConflictID, innerConflictMember *ledgerstate.ConflictMember) {
				if innerConflictMember.BranchID() == conflictMember.BranchID() {
					return
				}
				if innerConflictID == conflictID {
					return
				}
				fmt.Println("inner conflict set", innerConflictMember.BranchID())
			})
		})

		/*
			allParentsHaveHighestWeight := true
			for conflictBranchID := range resolvedConflictBranchIDs {

				_, highestWeightedBranchID := o.highestWeightedBranchFromConflictSets(conflictBranchID)
				fmt.Println(conflictBranchID, highestWeightedBranchID)
				if highestWeightedBranchID != conflictBranchID {
					disliked[branchID] = types.Void
					allParentsHaveHighestWeight = false
					break
				}
			}

			if allParentsHaveHighestWeight {
				liked[branchID] = types.Void
			}
		*/
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
		func(_ ledgerstate.ConflictID, conflictMember *ledgerstate.ConflictMember) {
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

func (o *OnTangleVoting) resolve(conflictMembers []*ledgerstate.ConflictMember) []*ledgerstate.ConflictMember {
	var filteredMembers []*ledgerstate.ConflictMember
	for _, conflictMember := range conflictMembers {
		conflictSets := o.conflictsSets(conflictMember.BranchID())
		if len(conflictSets) == 1 {
			filteredMembers = append(filteredMembers, conflictMember)
			continue
		}

		isWinnerAcrossAllConflictSets := true
		for conflictSet := range conflictSets {
			if conflictSet == conflictMember.ConflictID() {
				continue
			}

			innerConflictMembers := o.branchDAG.ConflictMembers(conflictSet).Unwrap()
			for i, innerConflictMember := range innerConflictMembers {
				if innerConflictMember.BranchID() == conflictMember.BranchID() {
					innerConflictMembers = append(innerConflictMembers[:i], innerConflictMembers[i+1:]...)
					break
				}
			}
			innerConflictMembers = o.resolve(innerConflictMembers)
			if !o.winner(conflictMember.BranchID(), innerConflictMembers) {
				isWinnerAcrossAllConflictSets = false
				break
			}
		}
		if isWinnerAcrossAllConflictSets {
			filteredMembers = append(filteredMembers, conflictMember)
		}
	}
	return filteredMembers
}

func (o *OnTangleVoting) winner(candidate ledgerstate.BranchID, conflictMembers []*ledgerstate.ConflictMember) bool {
	var highestWeight float64
	var highestBranch ledgerstate.BranchID
	for _, conflictMember := range conflictMembers {
		if weight := o.weightFunc(conflictMember.BranchID()); weight > highestWeight {
			highestWeight = weight
			highestBranch = conflictMember.BranchID()
		}
	}
	return highestBranch == candidate
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
