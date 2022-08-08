package remotemetrics

import (
	"time"

	"github.com/iotaledger/hive.go/core/identity"

	"github.com/iotaledger/goshimmer/packages/core/ledger"
	"github.com/iotaledger/goshimmer/packages/core/ledger/utxo"

	"github.com/iotaledger/goshimmer/packages/core/tangleold"

	"github.com/iotaledger/goshimmer/packages/app/remotemetrics"
)

func sendBlockSchedulerRecord(blockID tangleold.BlockID, recordType string) {
	if !deps.Tangle.Synced() {
		return
	}
	var nodeID string
	if deps.Local != nil {
		nodeID = deps.Local.Identity.ID().String()
	}

	record := &remotemetrics.BlockScheduledMetrics{
		Type:         recordType,
		NodeID:       nodeID,
		MetricsLevel: Parameters.MetricsLevel,
		BlockID:      blockID.Base58(),
	}

	deps.Tangle.Storage.Block(blockID).Consume(func(block *tangleold.Block) {
		issuerID := identity.NewID(block.IssuerPublicKey())
		record.IssuedTimestamp = block.IssuingTime()
		record.IssuerID = issuerID.String()
		record.AccessMana = deps.Tangle.Scheduler.GetManaFromCache(issuerID)
		record.StrongEdgeCount = len(block.ParentsByType(tangleold.StrongParentType))
		if weakParentsCount := len(block.ParentsByType(tangleold.WeakParentType)); weakParentsCount > 0 {
			record.StrongEdgeCount = weakParentsCount
		}
		if likeParentsCount := len(block.ParentsByType(tangleold.ShallowLikeParentType)); likeParentsCount > 0 {
			record.StrongEdgeCount = len(block.ParentsByType(tangleold.ShallowLikeParentType))
		}

		deps.Tangle.Storage.BlockMetadata(blockID).Consume(func(blockMetadata *tangleold.BlockMetadata) {
			record.ReceivedTimestamp = blockMetadata.ReceivedTime()
			record.ScheduledTimestamp = blockMetadata.ScheduledTime()
			record.DroppedTimestamp = blockMetadata.DiscardedTime()
			record.BookedTimestamp = blockMetadata.BookedTime()
			// may be overridden by tx data
			record.SolidTimestamp = blockMetadata.SolidificationTime()
			record.DeltaSolid = blockMetadata.SolidificationTime().Sub(record.IssuedTimestamp).Nanoseconds()
			record.QueuedTimestamp = blockMetadata.QueuedTime()
			record.DeltaBooked = blockMetadata.BookedTime().Sub(record.IssuedTimestamp).Nanoseconds()
			record.ConfirmationState = uint8(blockMetadata.ConfirmationState())
			record.ConfirmationStateTimestamp = blockMetadata.ConfirmationStateTime()
			if !blockMetadata.ConfirmationStateTime().IsZero() {
				record.DeltaConfirmationStateTime = blockMetadata.ConfirmationStateTime().Sub(record.IssuedTimestamp).Nanoseconds()
			}

			var scheduleDoneTime time.Time
			// one of those conditions must be true
			if !record.ScheduledTimestamp.IsZero() {
				scheduleDoneTime = record.ScheduledTimestamp
			} else if !record.DroppedTimestamp.IsZero() {
				scheduleDoneTime = record.DroppedTimestamp
			}
			record.DeltaScheduledIssued = scheduleDoneTime.Sub(record.IssuedTimestamp).Nanoseconds()
			record.DeltaScheduledReceived = scheduleDoneTime.Sub(blockMetadata.ReceivedTime()).Nanoseconds()
			record.DeltaReceivedIssued = blockMetadata.ReceivedTime().Sub(record.IssuedTimestamp).Nanoseconds()
			record.SchedulingTime = scheduleDoneTime.Sub(blockMetadata.QueuedTime()).Nanoseconds()
		})
	})

	// override block solidification data if block contains a transaction
	deps.Tangle.Utils.ComputeIfTransaction(blockID, func(transactionID utxo.TransactionID) {
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

	blockIDs := deps.Tangle.Storage.AttachmentBlockIDs(transactionID)
	if len(blockIDs) == 0 {
		return
	}

	deps.Tangle.Storage.Block(blockIDs.First()).Consume(onBlockFinalized)
}

func onBlockFinalized(block *tangleold.Block) {
	if !deps.Tangle.Synced() {
		return
	}

	blockID := block.ID()

	var nodeID string
	if deps.Local != nil {
		nodeID = deps.Local.Identity.ID().String()
	}

	record := &remotemetrics.BlockFinalizedMetrics{
		Type:         "blockFinalized",
		NodeID:       nodeID,
		MetricsLevel: Parameters.MetricsLevel,
		BlockID:      blockID.Base58(),
	}

	issuerID := identity.NewID(block.IssuerPublicKey())
	record.IssuedTimestamp = block.IssuingTime()
	record.IssuerID = issuerID.String()
	record.StrongEdgeCount = len(block.ParentsByType(tangleold.StrongParentType))
	if weakParentsCount := len(block.ParentsByType(tangleold.WeakParentType)); weakParentsCount > 0 {
		record.WeakEdgeCount = weakParentsCount
	}
	if shallowLikeParentsCount := len(block.ParentsByType(tangleold.ShallowLikeParentType)); shallowLikeParentsCount > 0 {
		record.ShallowLikeEdgeCount = shallowLikeParentsCount
	}
	deps.Tangle.Storage.BlockMetadata(blockID).Consume(func(blockMetadata *tangleold.BlockMetadata) {
		record.ScheduledTimestamp = blockMetadata.ScheduledTime()
		record.DeltaScheduled = blockMetadata.ScheduledTime().Sub(record.IssuedTimestamp).Nanoseconds()
		record.BookedTimestamp = blockMetadata.BookedTime()
		record.DeltaBooked = blockMetadata.BookedTime().Sub(record.IssuedTimestamp).Nanoseconds()
	})

	deps.Tangle.Utils.ComputeIfTransaction(blockID, func(transactionID utxo.TransactionID) {
		deps.Tangle.Ledger.Storage.CachedTransactionMetadata(transactionID).Consume(func(transactionMetadata *ledger.TransactionMetadata) {
			record.SolidTimestamp = transactionMetadata.BookingTime()
			record.TransactionID = transactionID.Base58()
			record.DeltaSolid = transactionMetadata.BookingTime().Sub(record.IssuedTimestamp).Nanoseconds()
		})
	})

	_ = deps.RemoteLogger.Send(record)
}

func sendMissingBlockRecord(blockID tangleold.BlockID, recordType string) {
	if !deps.Tangle.Synced() {
		return
	}

	var nodeID string
	if deps.Local != nil {
		nodeID = deps.Local.Identity.ID().String()
	}

	record := &remotemetrics.MissingBlockMetrics{
		Type:         recordType,
		NodeID:       nodeID,
		MetricsLevel: Parameters.MetricsLevel,
		BlockID:      blockID.Base58(),
	}

	deps.Tangle.Storage.Block(blockID).Consume(func(block *tangleold.Block) {
		record.IssuerID = identity.NewID(block.IssuerPublicKey()).String()
	})

	_ = deps.RemoteLogger.Send(record)
}
