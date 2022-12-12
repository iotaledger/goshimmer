package metricscollector

import (
	"time"

	"github.com/iotaledger/goshimmer/packages/app/collector"
	"github.com/iotaledger/goshimmer/packages/protocol/ledger/utxo"
	"github.com/iotaledger/hive.go/core/generics/event"
)

const (
	conflictNamespace = "conflict"

	resolutionTimeMs      = "resolution_total_time_ms"
	resolvedConflictCount = "resolved_count"
	activeConflicts       = "active_conflicts"
	// todo finish: do we need conflicts in DB?
)

var ConflictMetrics = collector.NewCollection(conflictNamespace,
	collector.WithMetric(collector.NewMetric(resolutionTimeMs,
		collector.WithHelp("Time since transaction issuance to the conflict acceptance"),
		collector.WithInitFunc(func() {
			deps.Protocol.Events.Engine.Ledger.ConflictDAG.ConflictAccepted.Attach(event.NewClosure(func(conflictID utxo.TransactionID) {
				firstAttachment := deps.Protocol.Engine().Tangle.GetEarliestAttachment(conflictID)
				timeSinceIssuance := time.Since(firstAttachment.IssuingTime()).Milliseconds()
				deps.Collector.Update(conflictNamespace, resolutionTimeMs, collector.SingleValue(float64(timeSinceIssuance)))
			}))
		}),
	)),
	collector.WithMetric(collector.NewMetric(resolvedConflictCount,
		collector.WithHelp("Number of resolved (accepted) conflicts"),
		collector.WithInitValue(func() map[string]float64 {
			return collector.SingleValue(deps.Protocol.Engine().Ledger.ConflictDAG.Utils.ConflictCount())
		}),
		collector.WithInitFunc(func() {
			deps.Protocol.Events.Engine.Ledger.ConflictDAG.ConflictAccepted.Attach(event.NewClosure(func(conflictID utxo.TransactionID) {
				deps.Collector.Increment(conflictNamespace, resolvedConflictCount)
			}))
		}),
	)),
)
