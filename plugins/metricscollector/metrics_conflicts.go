package metricscollector

import (
	"time"

	"github.com/iotaledger/goshimmer/packages/app/collector"
	"github.com/iotaledger/goshimmer/packages/protocol/ledger/utxo"
	"github.com/iotaledger/hive.go/core/generics/event"
)

const (
	conflictNamespace = "conflict"

	resolutionTime        = "resolution_time_seconds"
	resolvedConflictCount = "resolved_total"
	activeConflicts       = "active_conflicts"
	// todo finish: do we need conflicts in DB?
)

var ConflictMetrics = collector.NewCollection(conflictNamespace,
	collector.WithMetric(collector.NewMetric(resolutionTime,
		collector.WithHelp("Time since transaction issuance to the conflict acceptance"),
		collector.WithInitFunc(func() {
			deps.Protocol.Events.Engine.Ledger.ConflictDAG.ConflictAccepted.Attach(event.NewClosure(func(conflictID utxo.TransactionID) {
				firstAttachment := deps.Protocol.Engine().Tangle.GetEarliestAttachment(conflictID)
				timeSinceIssuance := time.Since(firstAttachment.IssuingTime()).Milliseconds()
				timeIssuanceSeconds := float64(timeSinceIssuance) / 1000
				deps.Collector.Update(conflictNamespace, resolutionTime, collector.SingleValue(timeIssuanceSeconds))
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
