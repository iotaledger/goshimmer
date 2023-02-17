package metrics

import (
	"github.com/iotaledger/hive.go/runtime/event"
	"strconv"

	"github.com/iotaledger/goshimmer/packages/app/collector"
	"github.com/iotaledger/goshimmer/packages/core/epoch"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/consensus/blockgadget"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/notarization"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/tangle/blockdag"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/tangle/virtualvoting"
	"github.com/iotaledger/goshimmer/packages/protocol/ledger"
	"github.com/iotaledger/goshimmer/packages/protocol/ledger/conflictdag"
	"github.com/iotaledger/goshimmer/packages/protocol/ledger/utxo"
)

const (
	epochNamespace            = "epochs"
	labelName                 = "epoch"
	metricEvictionOffset      = 6
	totalBlocks               = "total_blocks"
	acceptedBlocksInEpoch     = "accepted_blocks"
	orphanedBlocks            = "orphaned_blocks"
	invalidBlocks             = "invalid_blocks"
	subjectivelyInvalidBlocks = "subjectively_invalid_blocks"
	totalTransactions         = "total_transactions"
	invalidTransactions       = "invalid_transactions"
	acceptedTransactions      = "accepted_transactions"
	orphanedTransactions      = "orphaned_transactions"
	createdConflicts          = "created_conflicts"
	acceptedConflicts         = "accepted_conflicts"
	rejectedConflicts         = "rejected_conflicts"
	notConflictingConflicts   = "not_conflicting_conflicts"
)

var EpochMetrics = collector.NewCollection(epochNamespace,
	collector.WithMetric(collector.NewMetric(totalBlocks,
		collector.WithType(collector.CounterVec),
		collector.WithLabels(labelName),
		collector.WithHelp("Number of blocks seen by the node in an epoch."),
		collector.WithInitFunc(func() {
			deps.Protocol.Events.Engine.Tangle.BlockDAG.BlockAttached.Hook(func(block *blockdag.Block) {
				eventEpoch := int(block.ID().Index())
				deps.Collector.Increment(epochNamespace, totalBlocks, strconv.Itoa(eventEpoch))

				// need to initialize epoch metrics with 0 to have consistent data for each epoch
				for _, metricName := range []string{acceptedBlocksInEpoch, orphanedBlocks, invalidBlocks, subjectivelyInvalidBlocks, totalTransactions, acceptedTransactions, invalidTransactions, orphanedTransactions, createdConflicts, acceptedConflicts, rejectedConflicts, notConflictingConflicts} {
					deps.Collector.Update(epochNamespace, metricName, map[string]float64{
						strconv.Itoa(eventEpoch): 0,
					})
				}
			}, event.WithWorkerPool(Plugin.WorkerPool))

			// initialize it once and remove committed epoch from all metrics (as they will not change afterwards)
			// in a single attachment instead of multiple ones
			deps.Protocol.Events.Engine.NotarizationManager.EpochCommitted.Hook(func(details *notarization.EpochCommittedDetails) {
				epochToEvict := int(details.Commitment.Index()) - metricEvictionOffset

				// need to remove metrics for old epochs, otherwise they would be stored in memory and always exposed to Prometheus, forever
				for _, metricName := range []string{totalBlocks, acceptedBlocksInEpoch, orphanedBlocks, invalidBlocks, subjectivelyInvalidBlocks, totalTransactions, acceptedTransactions, invalidTransactions, orphanedTransactions, createdConflicts, acceptedConflicts, rejectedConflicts, notConflictingConflicts} {
					deps.Collector.ResetMetricLabels(epochNamespace, metricName, map[string]string{
						labelName: strconv.Itoa(epochToEvict),
					})
				}
			}, event.WithWorkerPool(Plugin.WorkerPool))
		}),
	)),

	collector.WithMetric(collector.NewMetric(acceptedBlocksInEpoch,
		collector.WithType(collector.CounterVec),
		collector.WithHelp("Number of accepted blocks in an epoch."),
		collector.WithLabels(labelName),
		collector.WithInitFunc(func() {
			deps.Protocol.Events.Engine.Consensus.BlockGadget.BlockAccepted.Hook(func(block *blockgadget.Block) {
				eventEpoch := int(block.ID().Index())
				deps.Collector.Increment(epochNamespace, acceptedBlocksInEpoch, strconv.Itoa(eventEpoch))
			}, event.WithWorkerPool(Plugin.WorkerPool))
		}),
	)),
	collector.WithMetric(collector.NewMetric(orphanedBlocks,
		collector.WithType(collector.CounterVec),
		collector.WithLabels(labelName),
		collector.WithHelp("Number of orphaned blocks in an epoch."),
		collector.WithInitFunc(func() {
			deps.Protocol.Events.Engine.Tangle.BlockDAG.BlockOrphaned.Hook(func(block *blockdag.Block) {
				eventEpoch := int(block.ID().Index())
				deps.Collector.Increment(epochNamespace, orphanedBlocks, strconv.Itoa(eventEpoch))
			}, event.WithWorkerPool(Plugin.WorkerPool))
		}),
	)),
	collector.WithMetric(collector.NewMetric(invalidBlocks,
		collector.WithType(collector.CounterVec),
		collector.WithLabels(labelName),
		collector.WithHelp("Number of invalid blocks in an epoch epoch."),
		collector.WithInitFunc(func() {
			deps.Protocol.Events.Engine.Tangle.BlockDAG.BlockInvalid.Hook(func(blockInvalidEvent *blockdag.BlockInvalidEvent) {
				eventEpoch := int(blockInvalidEvent.Block.ID().Index())
				deps.Collector.Increment(epochNamespace, invalidBlocks, strconv.Itoa(eventEpoch))
			}, event.WithWorkerPool(Plugin.WorkerPool))
		}),
	)),
	collector.WithMetric(collector.NewMetric(subjectivelyInvalidBlocks,
		collector.WithType(collector.CounterVec),
		collector.WithLabels(labelName),
		collector.WithHelp("Number of invalid blocks in an epoch epoch."),
		collector.WithInitFunc(func() {
			deps.Protocol.Events.Engine.Tangle.VirtualVoting.BlockTracked.Hook(func(block *virtualvoting.Block) {
				if block.IsSubjectivelyInvalid() {
					eventEpoch := int(block.ID().Index())
					deps.Collector.Increment(epochNamespace, subjectivelyInvalidBlocks, strconv.Itoa(eventEpoch))
				}
			}, event.WithWorkerPool(Plugin.WorkerPool))
		}),
	)),
	collector.WithMetric(collector.NewMetric(totalTransactions,
		collector.WithType(collector.CounterVec),
		collector.WithLabels(labelName),
		collector.WithHelp("Number of transactions by the node per epoch."),
		collector.WithInitFunc(func() {
			deps.Protocol.Events.Engine.Ledger.TransactionBooked.Hook(func(bookedEvent *ledger.TransactionBookedEvent) {
				eventEpoch := int(epoch.IndexFromTime(deps.Protocol.Engine().Tangle.Booker.GetEarliestAttachment(bookedEvent.TransactionID).IssuingTime()))
				deps.Collector.Increment(epochNamespace, totalTransactions, strconv.Itoa(eventEpoch))
			}, event.WithWorkerPool(Plugin.WorkerPool))
		}),
	)),
	collector.WithMetric(collector.NewMetric(invalidTransactions,
		collector.WithType(collector.CounterVec),
		collector.WithLabels(labelName),
		collector.WithHelp("Number of transactions by the node per epoch."),
		collector.WithInitFunc(func() {
			deps.Protocol.Events.Engine.Ledger.TransactionInvalid.Hook(func(invalidEvent *ledger.TransactionInvalidEvent) {
				eventEpoch := int(epoch.IndexFromTime(deps.Protocol.Engine().Tangle.Booker.GetEarliestAttachment(invalidEvent.TransactionID).IssuingTime()))
				deps.Collector.Increment(epochNamespace, invalidTransactions, strconv.Itoa(eventEpoch))
			}, event.WithWorkerPool(Plugin.WorkerPool))
		}),
	)),
	collector.WithMetric(collector.NewMetric(acceptedTransactions,
		collector.WithType(collector.CounterVec),
		collector.WithLabels(labelName),
		collector.WithHelp("Number of accepted transactions by the node per epoch."),
		collector.WithInitFunc(func() {
			deps.Protocol.Events.Engine.Ledger.TransactionAccepted.Hook(func(transaction *ledger.TransactionEvent) {
				eventEpoch := int(transaction.Metadata.InclusionEpoch())
				deps.Collector.Increment(epochNamespace, acceptedTransactions, strconv.Itoa(eventEpoch))
			}, event.WithWorkerPool(Plugin.WorkerPool))
		}),
	)),
	collector.WithMetric(collector.NewMetric(orphanedTransactions,
		collector.WithType(collector.CounterVec),
		collector.WithLabels(labelName),
		collector.WithHelp("Number of accepted transactions by the node per epoch."),
		collector.WithInitFunc(func() {
			deps.Protocol.Events.Engine.Ledger.TransactionOrphaned.Hook(func(transaction *ledger.TransactionEvent) {
				eventEpoch := int(transaction.Metadata.InclusionEpoch())
				deps.Collector.Increment(epochNamespace, orphanedTransactions, strconv.Itoa(eventEpoch))
			}, event.WithWorkerPool(Plugin.WorkerPool))
		}),
	)),
	collector.WithMetric(collector.NewMetric(createdConflicts,
		collector.WithType(collector.CounterVec),
		collector.WithLabels(labelName),
		collector.WithHelp("Number of conflicts created per epoch."),
		collector.WithInitFunc(func() {
			// TODO: iterate through all attachments
			deps.Protocol.Events.Engine.Ledger.ConflictDAG.ConflictCreated.Hook(func(conflictCreated *conflictdag.ConflictCreatedEvent[utxo.TransactionID, utxo.OutputID]) {
				eventEpoch := int(epoch.IndexFromTime(deps.Protocol.Engine().Tangle.Booker.GetEarliestAttachment(conflictCreated.ID).IssuingTime()))
				deps.Collector.Increment(epochNamespace, createdConflicts, strconv.Itoa(eventEpoch))
			}, event.WithWorkerPool(Plugin.WorkerPool))
		}),
	)),
	collector.WithMetric(collector.NewMetric(acceptedConflicts,
		collector.WithType(collector.CounterVec),
		collector.WithLabels(labelName),
		collector.WithHelp("Number of conflicts accepted per epoch."),
		collector.WithInitFunc(func() {
			// TODO: iterate through all attachments
			deps.Protocol.Events.Engine.Ledger.ConflictDAG.ConflictAccepted.Hook(func(conflictID utxo.TransactionID) {
				eventEpoch := int(epoch.IndexFromTime(deps.Protocol.Engine().Tangle.Booker.GetEarliestAttachment(conflictID).IssuingTime()))
				deps.Collector.Increment(epochNamespace, acceptedConflicts, strconv.Itoa(eventEpoch))
			}, event.WithWorkerPool(Plugin.WorkerPool))
		}),
	)),
	collector.WithMetric(collector.NewMetric(rejectedConflicts,
		collector.WithType(collector.CounterVec),
		collector.WithLabels(labelName),
		collector.WithHelp("Number of conflicts rejected per epoch."),
		collector.WithInitFunc(func() {
			// TODO: iterate through all attachments
			deps.Protocol.Events.Engine.Ledger.ConflictDAG.ConflictRejected.Hook(func(conflictID utxo.TransactionID) {
				eventEpoch := int(epoch.IndexFromTime(deps.Protocol.Engine().Tangle.Booker.GetEarliestAttachment(conflictID).IssuingTime()))
				deps.Collector.Increment(epochNamespace, rejectedConflicts, strconv.Itoa(eventEpoch))
			}, event.WithWorkerPool(Plugin.WorkerPool))
		}),
	)),
	collector.WithMetric(collector.NewMetric(notConflictingConflicts,
		collector.WithType(collector.CounterVec),
		collector.WithLabels(labelName),
		collector.WithHelp("Number of conflicts rejected per epoch."),
		collector.WithInitFunc(func() {
			// TODO: iterate through all attachments
			//deps.Protocol.Events.Engine.Ledger.ConflictDAG.ConflictNotConflicting.Hook(func(conflictID utxo.TransactionID) {
			//	deps.Collector.Increment(epochNamespace, notConflictingConflicts, strconv.Itoa(int(epoch.IndexFromTime(deps.Protocol.Engine().Tangle.GetEarliestAttachment(conflictID).IssuingTime()))))
			//}, event.WithWorkerPool(Plugin.WorkerPool))
		}),
	)),
)
