package metrics

import (
	"time"

	"github.com/iotaledger/goshimmer/packages/app/collector"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/ledger/mempool/conflictdag"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/ledger/utxo"
	"github.com/iotaledger/hive.go/runtime/event"
)

const (
	conflictNamespace = "conflict"

	resolutionTime        = "resolution_time_seconds_total"
	allConflictCounts     = "created_total"
	resolvedConflictCount = "resolved_total"
)

var ConflictMetrics = collector.NewCollection(conflictNamespace,
	collector.WithMetric(collector.NewMetric(resolutionTime,
		collector.WithType(collector.Counter),
		collector.WithHelp("Time since transaction issuance to the conflict acceptance"),
		collector.WithInitFunc(func() {
			deps.Protocol.Events.Engine.Ledger.MemPool.ConflictDAG.ConflictAccepted.Hook(func(conflict *conflictdag.Conflict[utxo.TransactionID, utxo.OutputID]) {
				firstAttachment := deps.Protocol.Engine().Tangle.Booker().GetEarliestAttachment(conflict.ID())
				timeSinceIssuance := time.Since(firstAttachment.IssuingTime()).Milliseconds()
				timeIssuanceSeconds := float64(timeSinceIssuance) / 1000
				deps.Collector.Update(conflictNamespace, resolutionTime, collector.SingleValue(timeIssuanceSeconds))
			}, event.WithWorkerPool(Plugin.WorkerPool))
		}),
	)),
	collector.WithMetric(collector.NewMetric(resolvedConflictCount,
		collector.WithType(collector.Counter),
		collector.WithHelp("Number of resolved (accepted) conflicts"),
		collector.WithInitFunc(func() {
			deps.Protocol.Events.Engine.Ledger.MemPool.ConflictDAG.ConflictAccepted.Hook(func(conflict *conflictdag.Conflict[utxo.TransactionID, utxo.OutputID]) {
				deps.Collector.Increment(conflictNamespace, resolvedConflictCount)
			}, event.WithWorkerPool(Plugin.WorkerPool))
		}),
	)),
	collector.WithMetric(collector.NewMetric(allConflictCounts,
		collector.WithType(collector.Counter),
		collector.WithHelp("Number of created conflicts"),
		collector.WithInitFunc(func() {
			deps.Protocol.Events.Engine.Ledger.MemPool.ConflictDAG.ConflictCreated.Hook(func(event *conflictdag.Conflict[utxo.TransactionID, utxo.OutputID]) {
				deps.Collector.Increment(conflictNamespace, allConflictCounts)
			}, event.WithWorkerPool(Plugin.WorkerPool))
		}),
	)),
)
