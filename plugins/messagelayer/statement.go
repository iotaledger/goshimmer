package messagelayer

import (
	"github.com/iotaledger/goshimmer/packages/ledgerstate"
	"github.com/iotaledger/goshimmer/packages/tangle"
	"github.com/iotaledger/goshimmer/packages/vote"
	"github.com/iotaledger/goshimmer/packages/vote/statement"
	"github.com/iotaledger/goshimmer/plugins/autopeering/local"
	"github.com/iotaledger/hive.go/identity"
)

func makeStatement(roundStats *vote.RoundStats) {
	// TODO: add check for Mana threshold

	timestamps := statement.Timestamps{}
	conflicts := statement.Conflicts{}

	for id, v := range roundStats.ActiveVoteContexts {
		switch v.Type {
		case vote.TimestampType:
			messageID, err := tangle.NewMessageID(id)
			if err != nil {
				// TODO
				break
			}
			timestamps = append(timestamps, statement.Timestamp{
				ID: messageID,
				Opinion: statement.Opinion{
					Value: v.LastOpinion(),
					Round: uint8(v.Rounds)}},
			)
		case vote.ConflictType:
			messageID, err := ledgerstate.TransactionIDFromBase58(id)
			if err != nil {
				// TODO
				break
			}
			conflicts = append(conflicts, statement.Conflict{
				ID: messageID,
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
	msg, err := Tangle().IssuePayload(statement.New(conflicts, timestamps))

	if err != nil {
		log.Warnf("error issuing statement: %s", err)
		return
	}

	log.Debugf("issued statement %s", msg.ID())
}

func readStatement(messageID tangle.MessageID) {
	Tangle().Storage.Message(messageID).Consume(func(msg *tangle.Message) {
		messagePayload := msg.Payload()
		if messagePayload.Type() != statement.StatementType {
			return
		}
		statementPayload, ok := messagePayload.(*statement.Statement)
		if !ok {
			log.Debug("could not cast payload to statement object")
			return
		}

		// TODO: check if the Mana threshold of the issuer is ok

		issuerID := identity.NewID(msg.IssuerPublicKey())
		// Skip ourselves
		if issuerID == local.GetInstance().ID() {
			return
		}

		issuerRegistry := Registry().NodeView(issuerID)

		issuerRegistry.AddConflicts(statementPayload.Conflicts)

		issuerRegistry.AddTimestamps(statementPayload.Timestamps)

		Tangle().Storage.MessageMetadata(messageID).Consume(func(messageMetadata *tangle.MessageMetadata) {
			sendToRemoteLog(
				msg.ID().String(),
				issuerID.String(),
				msg.IssuingTime().UnixNano(),
				messageMetadata.ReceivedTime().UnixNano(),
				messageMetadata.SolidificationTime().UnixNano(),
			)
		})
	})
}
