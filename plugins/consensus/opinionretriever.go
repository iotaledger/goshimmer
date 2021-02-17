package consensus

import (
	"github.com/iotaledger/goshimmer/dapps/valuetransfers"
	"github.com/iotaledger/goshimmer/dapps/valuetransfers/packages/branchmanager"
	"github.com/iotaledger/goshimmer/packages/vote"
	"github.com/iotaledger/goshimmer/packages/vote/opinion"
)

// OpinionRetriever returns the current opinion of the given id.
func OpinionRetriever(id string, objectType vote.ObjectType) opinion.Opinion {
	switch objectType {
	case vote.TimestampType:
		// TODO: implement
		return opinion.Like
	default: // conflict type
		branchID, err := branchmanager.BranchIDFromBase58(id)
		if err != nil {
			log.Errorf("received invalid vote request for branch '%s'", id)

			return opinion.Unknown
		}

		cachedBranch := valuetransfers.Tangle().BranchManager().Branch(branchID)
		defer cachedBranch.Release()

		branch := cachedBranch.Unwrap()
		if branch == nil {
			return opinion.Unknown
		}

		if !branch.Preferred() {
			return opinion.Dislike
		}

		return opinion.Like
	}
}
