package otv

import (
	"github.com/cockroachdb/errors"

	"github.com/iotaledger/goshimmer/packages/ledgerstate"
	"github.com/iotaledger/goshimmer/packages/tangle"
)

type OnTangleVoting struct {
	tangle *tangle.Tangle
}

func NewOnTangleVoting(t *tangle.Tangle) *OnTangleVoting {
	return &OnTangleVoting{
		tangle: t,
	}
}

func (o *OnTangleVoting) Opinion(branches ledgerstate.BranchIDs) (liked, disliked ledgerstate.BranchIDs, err error) {
	panic("implement me")
}

func (o *OnTangleVoting) LikedFromConflictSet(branchID ledgerstate.BranchID) (likedBranchIDs ledgerstate.BranchIDs, err error) {
	resolvedConflictBranchIDs, err := o.tangle.LedgerState.BranchDAG.ResolveConflictBranchIDs(ledgerstate.NewBranchIDs(branchID))
	if err != nil {
		return nil, errors.Wrapf(err, "unable to resolve conflict branch IDs of %s", branchID)
	}

	branchWeights := make(map[ledgerstate.BranchID]float64)
	for conflictBranchID := range resolvedConflictBranchIDs {
		o.tangle.LedgerState.BranchDAG.Branch(conflictBranchID).Consume(func(branch ledgerstate.Branch) {
			// go through all conflict sets that the conflictBranchID is part of
			for conflictID := range branch.(*ledgerstate.ConflictBranch).Conflicts() {
				var highestWeight float64
				var highestWeightedBranch ledgerstate.BranchID

				// find branch with the highest weight for conflict set
				o.tangle.LedgerState.BranchDAG.ConflictMembers(conflictID).Consume(func(conflictMember *ledgerstate.ConflictMember) {
					if weight := o.tangle.ApprovalWeightManager.WeightOfBranch(conflictMember.BranchID()); weight > highestWeight {
						highestWeight = weight
						highestWeightedBranch = conflictMember.BranchID()
					}
				})
				branchWeights[highestWeightedBranch] = highestWeight
			}
		})
	}
	return
}
