package statement

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

	statementPayload := statement.NewPayload(conflicts, timestamps)
	msg, err := issuer.IssuePayload(statementPayload)

	if err != nil {
		log.Warnf("error issuing statement: %w", err)
		return
	}

	log.Debugf("issued statement %s", msg.ID())
}

func readStatement(cachedMessageEvent *tangle.CachedMessageEvent) {
	defer cachedMessageEvent.Message.Release()
	defer cachedMessageEvent.MessageMetadata.Release()

	solidMessage := cachedMessageEvent.Message.Unwrap()
	if solidMessage == nil {
		log.Debug("failed to unpack solid message from message layer")

		return
	}

	messagePayload := solidMessage.Payload()
	if messagePayload.Type() != statement.PayloadType {
		return
	}

	statementPayload, ok := messagePayload.(*statement.Payload)
	if !ok {
		log.Debug("could not cast payload to statement payload")

		return
	}

	// TODO: check if the Mana threshold of the issuer is ok

	// TODO: check reduced version VS full
	issuerID := identity.NewID(solidMessage.IssuerPublicKey()).String()

	issuerRegistry := Registry().NodeRegistry(issuerID)

	for _, conflict := range statementPayload.Conflicts {
		issuerRegistry.AddConflict(conflict)
	}

	for _, timestamp := range statementPayload.Timestamps {
		issuerRegistry.AddTimestamp(timestamp)
	}
}
