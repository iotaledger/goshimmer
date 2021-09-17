package remotelogmetrics

import (
	"github.com/iotaledger/goshimmer/packages/consensus/fcob"
	"github.com/iotaledger/goshimmer/packages/ledgerstate"
	"github.com/iotaledger/goshimmer/packages/remotelogmetrics"
	"github.com/iotaledger/goshimmer/packages/tangle"
)

// Evaluate evaluates the opinion of the given messageID.
func onTransactionOpinionFormed(messageID tangle.MessageID) {
	var nodeID string
	if deps.Local != nil {
		nodeID = deps.Local.ID().String()
	}

	deps.Tangle.Utils.ComputeIfTransaction(messageID, func(transactionID ledgerstate.TransactionID) {
		consensusMechanism := deps.ConsensusMechanism.(*fcob.ConsensusMechanism)
		consensusMechanism.Storage.Opinion(transactionID).Consume(func(opinion *fcob.Opinion) {
			record := &remotelogmetrics.TransactionOpinionMetrics{
				Type:             "transactionOpinion",
				NodeID:           nodeID,
				TransactionID:    transactionID.Base58(),
				Fcob1Time:        opinion.FCOBTime1(),
				Fcob2Time:        opinion.FCOBTime2(),
				Liked:            opinion.Liked(),
				LevelOfKnowledge: opinion.LevelOfKnowledge(),
				Timestamp:        opinion.Timestamp(),
				MessageID:        messageID.Base58(),
			}
			if err := deps.RemoteLogger.Send(record); err != nil {
				Plugin.Logger().Errorw("Failed to send Transaction record", "err", err)
			}
		})
	})
}

func onTransactionConfirmed(transactionID ledgerstate.TransactionID) {
	if !deps.Tangle.Synced() {
		return
	}

	var nodeID string
	if deps.Local != nil {
		nodeID = deps.Local.ID().String()
	}

	record := &remotelogmetrics.TransactionMetrics{
		Type:          "transaction",
		NodeID:        nodeID,
		TransactionID: transactionID.Base58(),
	}

	messageIDs := deps.Tangle.Storage.AttachmentMessageIDs(transactionID)
	if len(messageIDs) == 0 {
		return
	}
	messageID := messageIDs[0]
	deps.Tangle.Storage.Message(messageID).Consume(func(message *tangle.Message) {
		record.MessageID = messageID.Base58()
		record.IssuedTimestamp = message.IssuingTime()
	})
	deps.Tangle.Storage.MessageMetadata(messageID).Consume(func(messageMetadata *tangle.MessageMetadata) {
		record.ScheduledTimestamp = messageMetadata.ScheduledTime()
		record.DeltaScheduled = messageMetadata.ScheduledTime().Sub(record.IssuedTimestamp).Nanoseconds()
		record.BookedTimestamp = messageMetadata.BookedTime()
		record.DeltaBooked = messageMetadata.BookedTime().Sub(record.IssuedTimestamp).Nanoseconds()
		record.ConfirmedTimestamp = messageMetadata.FinalizedTime()
		record.DeltaConfirmed = messageMetadata.FinalizedTime().Sub(record.IssuedTimestamp).Nanoseconds()
	})

	deps.Tangle.LedgerState.TransactionMetadata(transactionID).Consume(func(transactionMetadata *ledgerstate.TransactionMetadata) {
		record.SolidTimestamp = transactionMetadata.SolidificationTime()
		record.DeltaSolid = transactionMetadata.SolidificationTime().Sub(record.IssuedTimestamp).Nanoseconds()
	})

	if err := deps.RemoteLogger.Send(record); err != nil {
		Plugin.Logger().Errorw("Failed to send Transaction record", "err", err)
	}
}
