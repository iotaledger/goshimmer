package remotelogmetrics

import (
	"github.com/iotaledger/goshimmer/packages/ledgerstate"
	"github.com/iotaledger/goshimmer/packages/remotelogmetrics"
	"github.com/iotaledger/goshimmer/packages/tangle"
	"github.com/iotaledger/goshimmer/plugins/autopeering/local"
	"github.com/iotaledger/goshimmer/plugins/messagelayer"
	"github.com/iotaledger/goshimmer/plugins/remotelog"
)

func onTransactionConfirmed(transactionID ledgerstate.TransactionID) {
	if !messagelayer.Tangle().Synced() {
		return
	}

	var nodeID string
	if local.GetInstance() != nil {
		nodeID = local.GetInstance().ID().String()
	}

	record := &remotelogmetrics.TransactionMetrics{
		Type:          "transaction",
		NodeID:        nodeID,
		TransactionID: transactionID.Base58(),
	}

	messageIDs := messagelayer.Tangle().Storage.AttachmentMessageIDs(transactionID)
	if len(messageIDs) == 0 {
		return
	}
	messageID := messageIDs[0]
	messagelayer.Tangle().Storage.Message(messageID).Consume(func(message *tangle.Message) {
		record.MessageID = messageID.Base58()
		record.IssuedTimestamp = message.IssuingTime()
	})
	messagelayer.Tangle().Storage.MessageMetadata(messageID).Consume(func(messageMetadata *tangle.MessageMetadata) {
		record.ScheduledTimestamp = messageMetadata.ScheduledTime()
		record.DeltaScheduled = messageMetadata.ScheduledTime().Sub(record.IssuedTimestamp).Nanoseconds()
		record.BookedTimestamp = messageMetadata.BookedTime()
		record.DeltaBooked = messageMetadata.BookedTime().Sub(record.IssuedTimestamp).Nanoseconds()
	})

	messagelayer.Tangle().LedgerState.TransactionMetadata(transactionID).Consume(func(transactionMetadata *ledgerstate.TransactionMetadata) {
		record.SolidTimestamp = transactionMetadata.SolidificationTime()
		record.DeltaSolid = transactionMetadata.SolidificationTime().Sub(record.IssuedTimestamp).Nanoseconds()
	})

	if err := remotelog.RemoteLogger().Send(record); err != nil {
		plugin.Logger().Errorw("Failed to send Transaction record", "err", err)
	}
}
