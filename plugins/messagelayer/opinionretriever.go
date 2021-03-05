package messagelayer

import (
	"github.com/iotaledger/goshimmer/packages/consensus/fcob"
	"github.com/iotaledger/goshimmer/packages/ledgerstate"
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
		transactionID, err := ledgerstate.TransactionIDFromBase58(id)
		if err != nil {
			log.Errorf("received invalid vote request for branch '%s'", id)

			return opinion.Unknown
		}

		opinionEssence := ConsensusMechanism().TransactionOpinionEssence(transactionID)

		if opinionEssence.LevelOfKnowledge() == fcob.Pending {
			return opinion.Unknown
		}

		if !opinionEssence.Liked() {
			return opinion.Dislike
		}

		return opinion.Like
	}
}
