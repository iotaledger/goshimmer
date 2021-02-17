package consensus

import (
	"github.com/iotaledger/goshimmer/packages/ledgerstate"
	"github.com/iotaledger/goshimmer/packages/tangle"
	"github.com/iotaledger/goshimmer/packages/vote"
	"github.com/iotaledger/goshimmer/packages/vote/opinion"
	"github.com/iotaledger/goshimmer/plugins/messagelayer"
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

		opinionEssence := messagelayer.Tangle().PayloadOpinionProvider.TransactionOpinionEssence(transactionID)

		if opinionEssence.LevelOfKnowledge() == tangle.Pending {
			return opinion.Unknown
		}

		if !opinionEssence.Liked() {
			return opinion.Dislike
		}

		return opinion.Like
	}
}
