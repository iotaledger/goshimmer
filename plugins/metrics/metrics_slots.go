package metrics

import (
	"fmt"
	"strconv"

	"github.com/iotaledger/goshimmer/packages/app/collector"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/consensus/blockgadget"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/ledger/mempool"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/ledger/mempool/conflictdag"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/ledger/utxo"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/notarization"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/tangle/blockdag"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/tangle/booker"
	"github.com/iotaledger/hive.go/runtime/event"
)

const (
	slotNamespace             = "slots"
	labelName                 = "slot"
	metricEvictionOffset      = 6
	totalBlocks               = "total_blocks"
	acceptedBlocksInSlot      = "accepted_blocks"
	orphanedBlocks            = "orphaned_blocks"
	invalidBlocks             = "invalid_blocks"
	subjectivelyInvalidBlocks = "subjectively_invalid_blocks"
	totalAttachments          = "total_attachments"
	rejectedAttachments       = "rejected_attachments"
	acceptedAttachments       = "accepted_attachments"
	orphanedAttachments       = "orphaned_attachments"
	createdConflicts          = "created_conflicts"
	acceptedConflicts         = "accepted_conflicts"
	rejectedConflicts         = "rejected_conflicts"
	notConflictingConflicts   = "not_conflicting_conflicts"
)

var SlotMetrics = collector.NewCollection(slotNamespace,
	collector.WithMetric(collector.NewMetric(totalBlocks,
		collector.WithType(collector.CounterVec),
		collector.WithLabels(labelName),
		collector.WithHelp("Number of blocks seen by the node in a slot."),
		collector.WithInitFunc(func() {
			deps.Protocol.Events.Engine.Tangle.BlockDAG.BlockAttached.Hook(func(block *blockdag.Block) {
				eventSlot := int(block.ID().Index())
				deps.Collector.Increment(slotNamespace, totalBlocks, strconv.Itoa(eventSlot))

				// need to initialize slot metrics with 0 to have consistent data for each slot
				for _, metricName := range []string{acceptedBlocksInSlot, orphanedBlocks, invalidBlocks, subjectivelyInvalidBlocks, totalAttachments, orphanedAttachments, rejectedAttachments, acceptedAttachments, createdConflicts, acceptedConflicts, rejectedConflicts, notConflictingConflicts} {
					deps.Collector.Update(slotNamespace, metricName, map[string]float64{
						strconv.Itoa(eventSlot): 0,
					})
				}
			}, event.WithWorkerPool(Plugin.WorkerPool))

			// initialize it once and remove committed slot from all metrics (as they will not change afterwards)
			// in a single attachment instead of multiple ones
			deps.Protocol.Events.Engine.Notarization.SlotCommitted.Hook(func(details *notarization.SlotCommittedDetails) {
				slotToEvict := int(details.Commitment.Index()) - metricEvictionOffset

				// need to remove metrics for old slots, otherwise they would be stored in memory and always exposed to Prometheus, forever
				for _, metricName := range []string{totalBlocks, acceptedBlocksInSlot, orphanedBlocks, invalidBlocks, subjectivelyInvalidBlocks, totalAttachments, orphanedAttachments, rejectedAttachments, acceptedAttachments, createdConflicts, acceptedConflicts, rejectedConflicts, notConflictingConflicts} {
					deps.Collector.ResetMetricLabels(slotNamespace, metricName, map[string]string{
						labelName: strconv.Itoa(slotToEvict),
					})
				}
			}, event.WithWorkerPool(Plugin.WorkerPool))
		}),
	)),

	collector.WithMetric(collector.NewMetric(acceptedBlocksInSlot,
		collector.WithType(collector.CounterVec),
		collector.WithHelp("Number of accepted blocks in a slot."),
		collector.WithLabels(labelName),
		collector.WithInitFunc(func() {
			deps.Protocol.Events.Engine.Consensus.BlockGadget.BlockAccepted.Hook(func(block *blockgadget.Block) {
				eventSlot := int(block.ID().Index())
				deps.Collector.Increment(slotNamespace, acceptedBlocksInSlot, strconv.Itoa(eventSlot))
			}, event.WithWorkerPool(Plugin.WorkerPool))
		}),
	)),
	collector.WithMetric(collector.NewMetric(orphanedBlocks,
		collector.WithType(collector.CounterVec),
		collector.WithLabels(labelName),
		collector.WithHelp("Number of orphaned blocks in a slot."),
		collector.WithInitFunc(func() {
			deps.Protocol.Events.Engine.Tangle.BlockDAG.BlockOrphaned.Hook(func(block *blockdag.Block) {
				eventSlot := int(block.ID().Index())
				deps.Collector.Increment(slotNamespace, orphanedBlocks, strconv.Itoa(eventSlot))
			}, event.WithWorkerPool(Plugin.WorkerPool))
		}),
	)),
	collector.WithMetric(collector.NewMetric(invalidBlocks,
		collector.WithType(collector.CounterVec),
		collector.WithLabels(labelName),
		collector.WithHelp("Number of invalid blocks in a slot slot."),
		collector.WithInitFunc(func() {
			deps.Protocol.Events.Engine.Tangle.BlockDAG.BlockInvalid.Hook(func(blockInvalidEvent *blockdag.BlockInvalidEvent) {
				fmt.Println("block invalid", blockInvalidEvent.Block.ID(), blockInvalidEvent.Reason)
				eventSlot := int(blockInvalidEvent.Block.ID().Index())
				deps.Collector.Increment(slotNamespace, invalidBlocks, strconv.Itoa(eventSlot))
			}, event.WithWorkerPool(Plugin.WorkerPool))
		}),
	)),
	collector.WithMetric(collector.NewMetric(subjectivelyInvalidBlocks,
		collector.WithType(collector.CounterVec),
		collector.WithLabels(labelName),
		collector.WithHelp("Number of invalid blocks in a slot slot."),
		collector.WithInitFunc(func() {
			deps.Protocol.Events.Engine.Tangle.Booker.BlockTracked.Hook(func(block *booker.Block) {
				if block.IsSubjectivelyInvalid() {
					eventSlot := int(block.ID().Index())
					deps.Collector.Increment(slotNamespace, subjectivelyInvalidBlocks, strconv.Itoa(eventSlot))
				}
			}, event.WithWorkerPool(Plugin.WorkerPool))
		}),
	)),

	collector.WithMetric(collector.NewMetric(totalAttachments,
		collector.WithType(collector.CounterVec),
		collector.WithLabels(labelName),
		collector.WithHelp("Number of transaction attachments by the node per slot."),
		collector.WithInitFunc(func() {
			deps.Protocol.Events.Engine.Tangle.Booker.AttachmentCreated.Hook(func(block *booker.Block) {
				eventSlot := int(block.ID().Index())
				deps.Collector.Increment(slotNamespace, totalAttachments, strconv.Itoa(eventSlot))
			}, event.WithWorkerPool(Plugin.WorkerPool))
		}),
	)),
	collector.WithMetric(collector.NewMetric(orphanedAttachments,
		collector.WithType(collector.CounterVec),
		collector.WithLabels(labelName),
		collector.WithHelp("Number of orphaned attachments by the node per slot."),
		collector.WithInitFunc(func() {
			deps.Protocol.Events.Engine.Tangle.Booker.AttachmentOrphaned.Hook(func(block *booker.Block) {
				eventSlot := int(block.ID().Index())
				deps.Collector.Increment(slotNamespace, orphanedAttachments, strconv.Itoa(eventSlot))
			}, event.WithWorkerPool(Plugin.WorkerPool))
		}),
	)),
	collector.WithMetric(collector.NewMetric(rejectedAttachments,
		collector.WithType(collector.CounterVec),
		collector.WithLabels(labelName),
		collector.WithHelp("Number of rejected attachments by the node per slot."),
		collector.WithInitFunc(func() {
			deps.Protocol.Events.Engine.Ledger.MemPool.TransactionRejected.Hook(func(transactionMetadata *mempool.TransactionMetadata) {
				for it := deps.Protocol.Engine().Tangle.Booker().GetAllAttachments(transactionMetadata.ID()).Iterator(); it.HasNext(); {
					attachmentBlock := it.Next()
					if !attachmentBlock.IsOrphaned() {
						deps.Collector.Increment(slotNamespace, rejectedAttachments, strconv.Itoa(int(attachmentBlock.ID().Index())))
					}
				}
			}, event.WithWorkerPool(Plugin.WorkerPool))
		}),
	)),
	collector.WithMetric(collector.NewMetric(acceptedAttachments,
		collector.WithType(collector.CounterVec),
		collector.WithLabels(labelName),
		collector.WithHelp("Number of accepted attachments by the node per slot."),
		collector.WithInitFunc(func() {
			deps.Protocol.Events.Engine.Ledger.MemPool.TransactionAccepted.Hook(func(transactionEvent *mempool.TransactionEvent) {
				for it := deps.Protocol.Engine().Tangle.Booker().GetAllAttachments(transactionEvent.Metadata.ID()).Iterator(); it.HasNext(); {
					attachmentBlock := it.Next()
					if !attachmentBlock.IsOrphaned() {
						deps.Collector.Increment(slotNamespace, acceptedAttachments, strconv.Itoa(int(attachmentBlock.ID().Index())))
					}
				}
			}, event.WithWorkerPool(Plugin.WorkerPool))
		}),
	)),
	collector.WithMetric(collector.NewMetric(createdConflicts,
		collector.WithType(collector.CounterVec),
		collector.WithLabels(labelName),
		collector.WithHelp("Number of conflicts created per slot."),
		collector.WithInitFunc(func() {
			deps.Protocol.Events.Engine.Ledger.MemPool.ConflictDAG.ConflictCreated.Hook(func(conflictCreated *conflictdag.Conflict[utxo.TransactionID, utxo.OutputID]) {
				for it := deps.Protocol.Engine().Tangle.Booker().GetAllAttachments(conflictCreated.ID()).Iterator(); it.HasNext(); {
					deps.Collector.Increment(slotNamespace, createdConflicts, strconv.Itoa(int(it.Next().ID().Index())))
				}
			}, event.WithWorkerPool(Plugin.WorkerPool))
		}),
	)),
	collector.WithMetric(collector.NewMetric(acceptedConflicts,
		collector.WithType(collector.CounterVec),
		collector.WithLabels(labelName),
		collector.WithHelp("Number of conflicts accepted per slot."),
		collector.WithInitFunc(func() {
			deps.Protocol.Events.Engine.Ledger.MemPool.ConflictDAG.ConflictAccepted.Hook(func(conflict *conflictdag.Conflict[utxo.TransactionID, utxo.OutputID]) {
				for it := deps.Protocol.Engine().Tangle.Booker().GetAllAttachments(conflict.ID()).Iterator(); it.HasNext(); {
					deps.Collector.Increment(slotNamespace, acceptedConflicts, strconv.Itoa(int(it.Next().ID().Index())))
				}
			}, event.WithWorkerPool(Plugin.WorkerPool))
		}),
	)),
	collector.WithMetric(collector.NewMetric(rejectedConflicts,
		collector.WithType(collector.CounterVec),
		collector.WithLabels(labelName),
		collector.WithHelp("Number of conflicts rejected per slot."),
		collector.WithInitFunc(func() {
			deps.Protocol.Events.Engine.Ledger.MemPool.ConflictDAG.ConflictRejected.Hook(func(conflict *conflictdag.Conflict[utxo.TransactionID, utxo.OutputID]) {
				for it := deps.Protocol.Engine().Tangle.Booker().GetAllAttachments(conflict.ID()).Iterator(); it.HasNext(); {
					deps.Collector.Increment(slotNamespace, rejectedConflicts, strconv.Itoa(int(it.Next().ID().Index())))
				}
			}, event.WithWorkerPool(Plugin.WorkerPool))
		}),
	)),
	collector.WithMetric(collector.NewMetric(notConflictingConflicts,
		collector.WithType(collector.CounterVec),
		collector.WithLabels(labelName),
		collector.WithHelp("Number of conflicts rejected per slot."),
		collector.WithInitFunc(func() {
			deps.Protocol.Events.Engine.Ledger.MemPool.ConflictDAG.ConflictNotConflicting.Hook(func(conflict *conflictdag.Conflict[utxo.TransactionID, utxo.OutputID]) {
				for it := deps.Protocol.Engine().Tangle.Booker().GetAllAttachments(conflict.ID()).Iterator(); it.HasNext(); {
					deps.Collector.Increment(slotNamespace, notConflictingConflicts, strconv.Itoa(int(it.Next().ID().Index())))
				}
			}, event.WithWorkerPool(Plugin.WorkerPool))
		}),
	)),
)
