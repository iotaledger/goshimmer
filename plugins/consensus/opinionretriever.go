package consensus

import (
	"github.com/iotaledger/goshimmer/dapps/valuetransfers"
	"github.com/iotaledger/goshimmer/dapps/valuetransfers/packages/branchmanager"
	"github.com/iotaledger/goshimmer/packages/vote"
)

// OpinionRetriever returns the current opinion of the given id.
func OpinionRetriever(id string, objectType vote.ObjectType) vote.Opinion {
	switch objectType {
	case vote.TimestampType:
		// TODO: implement
		return vote.Like
	default: // conflict type
		branchID, err := branchmanager.BranchIDFromBase58(id)
		if err != nil {
			log.Errorf("received invalid vote request for branch '%s'", id)

			return vote.Unknown
		}

		cachedBranch := valuetransfers.Tangle().BranchManager().Branch(branchID)
		defer cachedBranch.Release()

		branch := cachedBranch.Unwrap()
		if branch == nil {
			return vote.Unknown
		}

		if !branch.Preferred() {
			return vote.Dislike
		}

		return vote.Like
	}
}
