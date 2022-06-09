package remotemetrics

import (
	"time"

	"github.com/iotaledger/hive.go/identity"

	"github.com/iotaledger/goshimmer/packages/ledger"
	"github.com/iotaledger/goshimmer/packages/ledger/utxo"

	"github.com/iotaledger/goshimmer/packages/remotemetrics"
	"github.com/iotaledger/goshimmer/packages/tangle"
)

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
		record.IssuerID = issuerID.String()
		record.AccessMana = deps.Tangle.Scheduler.GetManaFromCache(issuerID)
		record.StrongEdgeCount = len(message.ParentsByType(tangle.StrongParentType))
		if weakParentsCount := len(message.ParentsByType(tangle.WeakParentType)); weakParentsCount > 0 {
			record.StrongEdgeCount = weakParentsCount
		}
		if likeParentsCount := len(message.ParentsByType(tangle.ShallowLikeParentType)); likeParentsCount > 0 {
			record.StrongEdgeCount = len(message.ParentsByType(tangle.ShallowLikeParentType))
		}

		deps.Tangle.Storage.MessageMetadata(messageID).Consume(func(messageMetadata *tangle.MessageMetadata) {
			record.ReceivedTimestamp = messageMetadata.ReceivedTime()
			record.ScheduledTimestamp = messageMetadata.ScheduledTime()
			record.DroppedTimestamp = messageMetadata.DiscardedTime()
			record.BookedTimestamp = messageMetadata.BookedTime()
			// may be overridden by tx data
			record.SolidTimestamp = messageMetadata.SolidificationTime()
			record.DeltaSolid = messageMetadata.SolidificationTime().Sub(record.IssuedTimestamp).Nanoseconds()
			record.QueuedTimestamp = messageMetadata.QueuedTime()
			record.DeltaBooked = messageMetadata.BookedTime().Sub(record.IssuedTimestamp).Nanoseconds()
			record.GradeOfFinality = uint8(messageMetadata.GradeOfFinality())
			record.GradeOfFinalityTimestamp = messageMetadata.GradeOfFinalityTime()
			if !messageMetadata.GradeOfFinalityTime().IsZero() {
				record.DeltaGradeOfFinalityTime = messageMetadata.GradeOfFinalityTime().Sub(record.IssuedTimestamp).Nanoseconds()
			}

			var scheduleDoneTime time.Time
			// one of those conditions must be true
			if !record.ScheduledTimestamp.IsZero() {
				scheduleDoneTime = record.ScheduledTimestamp
			} else if !record.DroppedTimestamp.IsZero() {
				scheduleDoneTime = record.DroppedTimestamp
			}
			record.DeltaScheduledIssued = scheduleDoneTime.Sub(record.IssuedTimestamp).Nanoseconds()
			record.DeltaScheduledReceived = scheduleDoneTime.Sub(messageMetadata.ReceivedTime()).Nanoseconds()
			record.DeltaReceivedIssued = messageMetadata.ReceivedTime().Sub(record.IssuedTimestamp).Nanoseconds()
			record.SchedulingTime = scheduleDoneTime.Sub(messageMetadata.QueuedTime()).Nanoseconds()
		})
	})

	// override message solidification data if message contains a transaction
	deps.Tangle.Utils.ComputeIfTransaction(messageID, func(transactionID utxo.TransactionID) {
		deps.Tangle.Ledger.Storage.CachedTransactionMetadata(transactionID).Consume(func(transactionMetadata *ledger.TransactionMetadata) {
			record.SolidTimestamp = transactionMetadata.BookingTime()
			record.TransactionID = transactionID.Base58()
			record.DeltaSolid = transactionMetadata.BookingTime().Sub(record.IssuedTimestamp).Nanoseconds()
		})
	})

	_ = deps.RemoteLogger.Send(record)
}

func onTransactionConfirmed(transactionID utxo.TransactionID) {
	if !deps.Tangle.Synced() {
		return
	}

	messageIDs := deps.Tangle.Storage.AttachmentMessageIDs(transactionID)
	if len(messageIDs) == 0 {
		return
	}

	deps.Tangle.Storage.Message(messageIDs.First()).Consume(onMessageFinalized)
}

func onMessageFinalized(message *tangle.Message) {
	if !deps.Tangle.Synced() {
		return
	}

	messageID := message.ID()

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

	issuerID := identity.NewID(message.IssuerPublicKey())
	record.IssuedTimestamp = message.IssuingTime()
	record.IssuerID = issuerID.String()
	record.StrongEdgeCount = len(message.ParentsByType(tangle.StrongParentType))
	if weakParentsCount := len(message.ParentsByType(tangle.WeakParentType)); weakParentsCount > 0 {
		record.WeakEdgeCount = weakParentsCount
	}
	if shallowLikeParentsCount := len(message.ParentsByType(tangle.ShallowLikeParentType)); shallowLikeParentsCount > 0 {
		record.ShallowLikeEdgeCount = shallowLikeParentsCount
	}
	if shallowDislikeParentsCount := len(message.ParentsByType(tangle.DislikeParentType)); shallowDislikeParentsCount > 0 {
		record.ShallowDislikeEdgeCount = shallowDislikeParentsCount
	}
	deps.Tangle.Storage.MessageMetadata(messageID).Consume(func(messageMetadata *tangle.MessageMetadata) {
		record.ScheduledTimestamp = messageMetadata.ScheduledTime()
		record.DeltaScheduled = messageMetadata.ScheduledTime().Sub(record.IssuedTimestamp).Nanoseconds()
		record.BookedTimestamp = messageMetadata.BookedTime()
		record.DeltaBooked = messageMetadata.BookedTime().Sub(record.IssuedTimestamp).Nanoseconds()
	})

	deps.Tangle.Utils.ComputeIfTransaction(messageID, func(transactionID utxo.TransactionID) {
		deps.Tangle.Ledger.Storage.CachedTransactionMetadata(transactionID).Consume(func(transactionMetadata *ledger.TransactionMetadata) {
			record.SolidTimestamp = transactionMetadata.BookingTime()
			record.TransactionID = transactionID.Base58()
			record.DeltaSolid = transactionMetadata.BookingTime().Sub(record.IssuedTimestamp).Nanoseconds()
		})
	})

	_ = deps.RemoteLogger.Send(record)
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

	deps.Tangle.Storage.Message(messageID).Consume(func(message *tangle.Message) {
		record.IssuerID = identity.NewID(message.IssuerPublicKey()).String()
	})

	_ = deps.RemoteLogger.Send(record)
}
