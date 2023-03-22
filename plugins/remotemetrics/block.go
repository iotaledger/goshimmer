package remotemetrics

import (
	"github.com/iotaledger/goshimmer/packages/app/remotemetrics"
	"github.com/iotaledger/goshimmer/packages/protocol/congestioncontrol/icca/scheduler"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/ledger/mempool"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/ledger/utxo"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/ledger/vm/devnetvm"
	"github.com/iotaledger/goshimmer/packages/protocol/models"
	"github.com/iotaledger/hive.go/crypto/identity"
)

func sendBlockSchedulerRecord(block *scheduler.Block, recordType string) {
	if !deps.Protocol.Engine().IsSynced() {
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
		BlockID:      block.ID().Base58(),
	}

	issuerID := identity.NewID(block.IssuerPublicKey())
	record.IssuedTimestamp = block.IssuingTime()
	record.IssuerID = issuerID.String()
	// TODO: implement when mana is refactored
	// record.AccessMana = deps.Protocol.Engine().CongestionControl.Scheduler.GetManaFromCache(issuerID)
	record.StrongEdgeCount = len(block.ParentsByType(models.StrongParentType))
	if weakParentsCount := len(block.ParentsByType(models.WeakParentType)); weakParentsCount > 0 {
		record.StrongEdgeCount = weakParentsCount
	}
	if likeParentsCount := len(block.ParentsByType(models.ShallowLikeParentType)); likeParentsCount > 0 {
		record.StrongEdgeCount = len(block.ParentsByType(models.ShallowLikeParentType))
	}

	// TODO: implement when retainer plugin is ready
	// deps.Tangle.Storage.BlockMetadata(blockID).Consume(func(blockMetadata *tangleold.BlockMetadata) {
	//	record.ReceivedTimestamp = blockMetadata.ReceivedTime()
	//	record.ScheduledTimestamp = blockMetadata.ScheduledTime()
	//	record.DroppedTimestamp = blockMetadata.DiscardedTime()
	//	record.BookedTimestamp = blockMetadata.BookedTime()
	//	// may be overridden by tx data
	//	record.SolidTimestamp = blockMetadata.SolidificationTime()
	//	record.DeltaSolid = blockMetadata.SolidificationTime().Sub(record.IssuedTimestamp).Nanoseconds()
	//	record.QueuedTimestamp = blockMetadata.QueuedTime()
	//	record.DeltaBooked = blockMetadata.BookedTime().Sub(record.IssuedTimestamp).Nanoseconds()
	//	record.ConfirmationState = uint8(blockMetadata.ConfirmationState())
	//	record.ConfirmationStateTimestamp = blockMetadata.ConfirmationStateTime()
	//	if !blockMetadata.ConfirmationStateTime().IsZero() {
	//		record.DeltaConfirmationStateTime = blockMetadata.ConfirmationStateTime().Sub(record.IssuedTimestamp).Nanoseconds()
	//	}
	//
	//	var scheduleDoneTime time.Time
	//	// one of those conditions must be true
	//	if !record.ScheduledTimestamp.IsZero() {
	//		scheduleDoneTime = record.ScheduledTimestamp
	//	} else if !record.DroppedTimestamp.IsZero() {
	//		scheduleDoneTime = record.DroppedTimestamp
	//	}
	//	record.DeltaScheduledIssued = scheduleDoneTime.Sub(record.IssuedTimestamp).Nanoseconds()
	//	record.DeltaScheduledReceived = scheduleDoneTime.Sub(blockMetadata.ReceivedTime()).Nanoseconds()
	//	record.DeltaReceivedIssued = blockMetadata.ReceivedTime().Sub(record.IssuedTimestamp).Nanoseconds()
	//	record.SchedulingTime = scheduleDoneTime.Sub(blockMetadata.QueuedTime()).Nanoseconds()
	// })

	// override block solidification data if block contains a transaction
	if block.Payload().Type() == devnetvm.TransactionType {
		transaction := block.Payload().(utxo.Transaction)
		deps.Protocol.Engine().Ledger.MemPool().Storage().CachedTransactionMetadata(transaction.ID()).Consume(func(transactionMetadata *mempool.TransactionMetadata) {
			record.SolidTimestamp = transactionMetadata.BookingTime()
			record.TransactionID = transaction.ID().Base58()
			record.DeltaSolid = transactionMetadata.BookingTime().Sub(record.IssuedTimestamp).Nanoseconds()
		})
	}

	_ = deps.RemoteLogger.Send(record)
}

func onTransactionAccepted(transactionEvent *mempool.TransactionEvent) {
	if !deps.Protocol.Engine().IsSynced() {
		return
	}

	earliestAttachment := deps.Protocol.Engine().Tangle.Booker().GetEarliestAttachment(transactionEvent.Metadata.ID())

	onBlockFinalized(earliestAttachment.ModelsBlock)
}

func onBlockFinalized(block *models.Block) {
	if !deps.Protocol.Engine().IsSynced() {
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
	record.StrongEdgeCount = len(block.ParentsByType(models.StrongParentType))
	if weakParentsCount := len(block.ParentsByType(models.WeakParentType)); weakParentsCount > 0 {
		record.WeakEdgeCount = weakParentsCount
	}
	if shallowLikeParentsCount := len(block.ParentsByType(models.ShallowLikeParentType)); shallowLikeParentsCount > 0 {
		record.ShallowLikeEdgeCount = shallowLikeParentsCount
	}

	// TODO: implement when retainer plugin is ready
	// deps.Tangle.Storage.BlockMetadata(blockID).Consume(func(blockMetadata *tangleold.BlockMetadata) {
	//	record.ScheduledTimestamp = blockMetadata.ScheduledTime()
	//	record.DeltaScheduled = blockMetadata.ScheduledTime().Sub(record.IssuedTimestamp).Nanoseconds()
	//	record.BookedTimestamp = blockMetadata.BookedTime()
	//	record.DeltaBooked = blockMetadata.BookedTime().Sub(record.IssuedTimestamp).Nanoseconds()
	// })

	if block.Payload().Type() == devnetvm.TransactionType {
		transaction := block.Payload().(utxo.Transaction)
		deps.Protocol.Engine().Ledger.MemPool().Storage().CachedTransactionMetadata(transaction.ID()).Consume(func(transactionMetadata *mempool.TransactionMetadata) {
			record.SolidTimestamp = transactionMetadata.BookingTime()
			record.TransactionID = transaction.ID().Base58()
			record.DeltaSolid = transactionMetadata.BookingTime().Sub(record.IssuedTimestamp).Nanoseconds()
		})
	}

	_ = deps.RemoteLogger.Send(record)
}

func sendMissingBlockRecord(block *models.Block, recordType string) {
	if !deps.Protocol.Engine().IsSynced() {
		return
	}

	var nodeID string
	if deps.Local != nil {
		nodeID = deps.Local.Identity.ID().String()
	}

	_ = deps.RemoteLogger.Send(&remotemetrics.MissingBlockMetrics{
		Type:         recordType,
		NodeID:       nodeID,
		MetricsLevel: Parameters.MetricsLevel,
		BlockID:      block.ID().Base58(),
		IssuerID:     identity.NewID(block.IssuerPublicKey()).String(),
	})
}
