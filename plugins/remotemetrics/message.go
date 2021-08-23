package remotemetrics

import (
	"github.com/iotaledger/goshimmer/packages/ledgerstate"
	"github.com/iotaledger/goshimmer/packages/remotemetrics"
	"github.com/iotaledger/goshimmer/packages/tangle"
	"github.com/iotaledger/goshimmer/plugins/autopeering/local"
	"github.com/iotaledger/goshimmer/plugins/messagelayer"
	"github.com/iotaledger/goshimmer/plugins/remotelog"
)

func onTransactionConfirmed(transactionID ledgerstate.TransactionID) {
	if !messagelayer.Tangle().Synced() {
		return
	}

	messageIDs := messagelayer.Tangle().Storage.AttachmentMessageIDs(transactionID)
	if len(messageIDs) == 0 {
		return
	}

	onMessageFinalized(messageIDs[0])
}

func onMessageFinalized(messageID tangle.MessageID) {
	if !messagelayer.Tangle().Synced() {
		return
	}

	var nodeID string
	if local.GetInstance() != nil {
		nodeID = local.GetInstance().ID().String()
	}

	record := &remotemetrics.MessageFinalizedMetrics{
		Type:         "messageFinalized",
		NodeID:       nodeID,
		MetricsLevel: Parameters.MetricsLevel,
		MessageID:    messageID.Base58(),
	}

	messagelayer.Tangle().Storage.Message(messageID).Consume(func(message *tangle.Message) {
		record.IssuedTimestamp = message.IssuingTime()
		record.StrongParentCount = len(message.ParentsByType(tangle.StrongParentType))
		if weakParentsCount := len(message.ParentsByType(tangle.WeakParentType)); weakParentsCount > 0 {
			record.WeakParentsCount = weakParentsCount
		}
		if likeParentsCount := len(message.ParentsByType(tangle.LikeParentType)); likeParentsCount > 0 {
			record.LikeParentCount = len(message.ParentsByType(tangle.LikeParentType))
		}
	})
	messagelayer.Tangle().Storage.MessageMetadata(messageID).Consume(func(messageMetadata *tangle.MessageMetadata) {
		record.ScheduledTimestamp = messageMetadata.ScheduledTime()
		record.DeltaScheduled = messageMetadata.ScheduledTime().Sub(record.IssuedTimestamp).Nanoseconds()
		record.BookedTimestamp = messageMetadata.BookedTime()
		record.DeltaBooked = messageMetadata.BookedTime().Sub(record.IssuedTimestamp).Nanoseconds()
	})

	messagelayer.Tangle().Utils.ComputeIfTransaction(messageID, func(transactionID ledgerstate.TransactionID) {
		messagelayer.Tangle().LedgerState.TransactionMetadata(transactionID).Consume(func(transactionMetadata *ledgerstate.TransactionMetadata) {
			record.SolidTimestamp = transactionMetadata.SolidificationTime()
			record.TransactionID = transactionID.Base58()
			record.DeltaSolid = transactionMetadata.SolidificationTime().Sub(record.IssuedTimestamp).Nanoseconds()
		})
	})

	if err := remotelog.RemoteLogger().Send(record); err != nil {
		plugin.Logger().Errorw("Failed to send MessageFinalizedMetrics record", "err", err)
	}
}
