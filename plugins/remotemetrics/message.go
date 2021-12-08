package remotemetrics

import (
	"time"

	"github.com/iotaledger/goshimmer/packages/ledgerstate"
	"github.com/iotaledger/goshimmer/packages/remotemetrics"
	"github.com/iotaledger/goshimmer/packages/tangle"
	"github.com/iotaledger/hive.go/identity"
)

func onMessageScheduled(messageID tangle.MessageID) {
	sendMessageSchedulerRecord(messageID, "messageScheduled")
}

func onMessageDiscarded(messageID tangle.MessageID) {
	sendMessageSchedulerRecord(messageID, "messageDiscarded")
}

func sendMessageSchedulerRecord(messageID tangle.MessageID, recordType string) {
	if !deps.Tangle.Synced() {
		return
	}

	var nodeID string
	if deps.Local != nil {
		nodeID = deps.Local.Identity.ID().String()
	}

	record := &remotemetrics.MessageScheduledMetrics{
		Type:         recordType,
		NodeID:       nodeID,
		MetricsLevel: Parameters.MetricsLevel,
		MessageID:    messageID.Base58(),
	}

	deps.Tangle.Storage.Message(messageID).Consume(func(message *tangle.Message) {
		issuerID := identity.NewID(message.IssuerPublicKey())
		record.IssuedTimestamp = message.IssuingTime()
		record.AccessMana = deps.Tangle.Scheduler.GetManaFromCache(issuerID)
		deps.Tangle.Storage.MessageMetadata(messageID).Consume(func(messageMetadata *tangle.MessageMetadata) {
			record.ScheduledTimestamp = messageMetadata.ScheduledTime()
			record.DroppedTimestamp = messageMetadata.DiscardedTime()
			record.BookedTimestamp = messageMetadata.BookedTime()
			record.SolidTimestamp = messageMetadata.SolidificationTime()
			record.QueuedTimestamp = messageMetadata.QueuedTime()

			var scheduleDoneTime time.Time
			// one of those conditions must be true
			if !record.ScheduledTimestamp.IsZero() {
				scheduleDoneTime = record.ScheduledTimestamp
			} else if !record.DroppedTimestamp.IsZero() {
				scheduleDoneTime = record.DroppedTimestamp
			}
			record.ProcessingTime = int(scheduleDoneTime.Unix() - messageMetadata.ReceivedTime().Unix())
			record.SchedulingTime = int(scheduleDoneTime.Unix() - messageMetadata.QueuedTime().Unix())
		})
	})

	if err := deps.RemoteLogger.Send(record); err != nil {
		Plugin.Logger().Errorw("Failed to send "+recordType+" record", "err", err)
	}
}

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

func onMissingMessageRequest(messageID tangle.MessageID) {
	sendMissingMessageRecord(messageID, "missingMessage")
}

func onMissingMessageStored(messageID tangle.MessageID) {
	sendMissingMessageRecord(messageID, "missingMessageStored")
}

func sendMissingMessageRecord(messageID tangle.MessageID, recordType string) {
	if !deps.Tangle.Synced() {
		return
	}

	var nodeID string
	if deps.Local != nil {
		nodeID = deps.Local.Identity.ID().String()
	}

	record := &remotemetrics.MissingMessageMetrics{
		Type:         recordType,
		NodeID:       nodeID,
		MetricsLevel: Parameters.MetricsLevel,
		MessageID:    messageID.Base58(),
	}

	if err := deps.RemoteLogger.Send(record); err != nil {
		Plugin.Logger().Errorw("Failed to send "+recordType+" record", "err", err)
	}
}
