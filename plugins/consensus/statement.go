package consensus

import (
	"github.com/iotaledger/goshimmer/dapps/valuetransfers/packages/transaction"
	"github.com/iotaledger/goshimmer/packages/tangle"
	"github.com/iotaledger/goshimmer/packages/vote"
	"github.com/iotaledger/goshimmer/packages/vote/statement"
	"github.com/iotaledger/goshimmer/plugins/issuer"
	"github.com/iotaledger/hive.go/identity"
)

func makeStatement(roundStats *vote.RoundStats) {
	// TODO: add check for Mana threshold

	timestamps := statement.Timestamps{}
	conflicts := statement.Conflicts{}

	for id, v := range roundStats.ActiveVoteContexts {
		switch v.Type {
		case vote.TimestampType:
			ID, err := tangle.NewMessageID(id)
			if err != nil {
				// TODO
				break
			}
			timestamps = append(timestamps, statement.Timestamp{
				ID: ID,
				Opinion: statement.Opinion{
					Value: v.LastOpinion(),
					Round: uint8(v.Rounds)}},
			)
		case vote.ConflictType:
			ID, err := transaction.IDFromBase58(id)
			if err != nil {
				// TODO
				break
			}
			conflicts = append(conflicts, statement.Conflict{
				ID: ID,
				Opinion: statement.Opinion{
					Value: v.LastOpinion(),
					Round: uint8(v.Rounds)}},
			)
		default:
		}
	}

	broadcastStatement(conflicts, timestamps)
}

// broadcastStatement broadcasts a statement via communication layer.
func broadcastStatement(conflicts statement.Conflicts, timestamps statement.Timestamps) {
	msg, err := issuer.IssuePayload(statement.NewPayload(conflicts, timestamps))

	if err != nil {
		log.Warnf("error issuing statement: %s", err)
		return
	}

	log.Infof("issued statement %s", msg.ID())
}

func readStatement(cachedMsgEvent *tangle.CachedMessageEvent) {
	cachedMsgEvent.MessageMetadata.Release()
	cachedMsgEvent.Message.Consume(func(msg *tangle.Message) {
		messagePayload := msg.Payload()

		log.Info(messagePayload.Type())

		if messagePayload.Type() != statement.Type {
			return
		}

		statementPayload, ok := messagePayload.(*statement.Payload)
		if !ok {
			log.Debug("could not cast payload to statement object")
			return
		}

		log.Info(statementPayload)

		// TODO: check if the Mana threshold of the issuer is ok

		// TODO: check reduced version VS full
		issuerID := identity.NewID(msg.IssuerPublicKey()).String()

		issuerRegistry := Registry().NodeView(issuerID)

		for _, conflict := range statementPayload.Conflicts {
			issuerRegistry.AddConflict(conflict)
		}

		for _, timestamp := range statementPayload.Timestamps {
			issuerRegistry.AddTimestamp(timestamp)
		}
	})
}
