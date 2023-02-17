package metrics

import (
	"time"

	"github.com/iotaledger/goshimmer/packages/app/collector"
	"github.com/iotaledger/goshimmer/packages/protocol/ledger/conflictdag"
	"github.com/iotaledger/goshimmer/packages/protocol/ledger/utxo"
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
			deps.Protocol.Events.Engine.Ledger.ConflictDAG.ConflictAccepted.Hook(func(conflictID utxo.TransactionID) {
				firstAttachment := deps.Protocol.Engine().Tangle.Booker.GetEarliestAttachment(conflictID)
				timeSinceIssuance := time.Since(firstAttachment.IssuingTime()).Milliseconds()
				timeIssuanceSeconds := float64(timeSinceIssuance) / 1000
				deps.Collector.Update(conflictNamespace, resolutionTime, collector.SingleValue(timeIssuanceSeconds))
			})
		}),
	)),
	collector.WithMetric(collector.NewMetric(resolvedConflictCount,
		collector.WithType(collector.Counter),
		collector.WithHelp("Number of resolved (accepted) conflicts"),
		collector.WithInitValue(func() map[string]float64 {
			return collector.SingleValue(deps.Protocol.Engine().Ledger.ConflictDAG.Utils.ConflictCount())
		}),
		collector.WithInitFunc(func() {
			deps.Protocol.Events.Engine.Ledger.ConflictDAG.ConflictAccepted.Hook(func(conflictID utxo.TransactionID) {
				deps.Collector.Increment(conflictNamespace, resolvedConflictCount)
			})
		}),
	)),
	collector.WithMetric(collector.NewMetric(allConflictCounts,
		collector.WithType(collector.Counter),
		collector.WithHelp("Number of created conflicts"),
		collector.WithInitFunc(func() {
			deps.Protocol.Events.Engine.Ledger.ConflictDAG.ConflictCreated.Hook(func(event *conflictdag.ConflictCreatedEvent[utxo.TransactionID, utxo.OutputID]) {
				deps.Collector.Increment(conflictNamespace, allConflictCounts)
			})
		}),
	)),
)
