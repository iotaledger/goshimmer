package remotemetrics

import (
	"github.com/iotaledger/goshimmer/packages/ledgerstate"
	"github.com/iotaledger/goshimmer/packages/remotemetrics"
	"github.com/iotaledger/goshimmer/packages/tangle"
)

func onTransactionConfirmed(transactionID ledgerstate.TransactionID) {
	if !deps.Tangle.Synced() {
		return
	}

	messageIDs := deps.Tangle.Storage.AttachmentMessageIDs(transactionID)
	if len(messageIDs) == 0 {
		return
	}

	onMessageFinalized(messageIDs[0])
}

func onMessageFinalized(messageID tangle.MessageID) {
	if !deps.Tangle.Synced() {
		return
	}

	var nodeID string
	if deps.Local != nil {
		nodeID = deps.Local.Identity.ID().String()
	}

	record := &remotemetrics.MessageFinalizedMetrics{
		Type:         "messageFinalized",
		NodeID:       nodeID,
		MetricsLevel: Parameters.MetricsLevel,
		MessageID:    messageID.Base58(),
	}

	deps.Tangle.Storage.Message(messageID).Consume(func(message *tangle.Message) {
		record.IssuedTimestamp = message.IssuingTime()
		record.StrongEdgeCount = len(message.ParentsByType(tangle.StrongParentType))
		if weakParentsCount := len(message.ParentsByType(tangle.WeakParentType)); weakParentsCount > 0 {
			record.StrongEdgeCount = weakParentsCount
		}
		if likeParentsCount := len(message.ParentsByType(tangle.LikeParentType)); likeParentsCount > 0 {
			record.StrongEdgeCount = len(message.ParentsByType(tangle.LikeParentType))
		}
	})
	deps.Tangle.Storage.MessageMetadata(messageID).Consume(func(messageMetadata *tangle.MessageMetadata) {
		record.ScheduledTimestamp = messageMetadata.ScheduledTime()
		record.DeltaScheduled = messageMetadata.ScheduledTime().Sub(record.IssuedTimestamp).Nanoseconds()
		record.BookedTimestamp = messageMetadata.BookedTime()
		record.DeltaBooked = messageMetadata.BookedTime().Sub(record.IssuedTimestamp).Nanoseconds()
	})

	deps.Tangle.Utils.ComputeIfTransaction(messageID, func(transactionID ledgerstate.TransactionID) {
		deps.Tangle.LedgerState.TransactionMetadata(transactionID).Consume(func(transactionMetadata *ledgerstate.TransactionMetadata) {
			record.SolidTimestamp = transactionMetadata.SolidificationTime()
			record.TransactionID = transactionID.Base58()
			record.DeltaSolid = transactionMetadata.SolidificationTime().Sub(record.IssuedTimestamp).Nanoseconds()
		})
	})

	if err := deps.RemoteLogger.Send(record); err != nil {
		Plugin.Logger().Errorw("Failed to send MessageFinalizedMetrics record", "err", err)
	}
}
