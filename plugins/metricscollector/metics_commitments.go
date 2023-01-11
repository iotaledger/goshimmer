package metricscollector

import (
	"github.com/iotaledger/goshimmer/packages/app/collector"
	"github.com/iotaledger/goshimmer/packages/core/commitment"
	"github.com/iotaledger/goshimmer/packages/protocol/chainmanager"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/notarization"
	"github.com/iotaledger/hive.go/core/generics/event"
)

const (
	commitmentsNamespace = "commitments"

	lastCommitment   = "latest"
	seenTotal        = "seen_total"
	missingRequested = "missing_requested_total"
	missingReceived  = "missing_received_total"
)

var CommitmentsMetrics = collector.NewCollection(commitmentsNamespace,
	collector.WithMetric(collector.NewMetric(lastCommitment,
		collector.WithType(collector.Gauge),
		collector.WithHelp("Last commitment of the node."),
		collector.WithLabels("commitment"),
		collector.WithInitFunc(func() {
			m := make(map[string]float64)
			deps.Protocol.Events.Engine.NotarizationManager.EpochCommitted.Attach(event.NewClosure(func(details *notarization.EpochCommittedDetails) {
				m[details.Commitment.ID().Base58()] = float64(details.Commitment.Index())
			}))
			deps.Collector.Update(commitmentsNamespace, lastCommitment, m)
			// todo reset
		}),
	)),
	collector.WithMetric(collector.NewMetric(seenTotal,
		collector.WithType(collector.Counter),
		collector.WithHelp("Number of commitments seen by the node."),
		collector.WithInitFunc(func() {
			deps.Protocol.ChainManager().Events.ForkDetected.Attach(event.NewClosure(func(chain *chainmanager.Chain) {
				deps.Collector.Increment(commitmentsNamespace, seenTotal)
			}))
		}),
	)),
	collector.WithMetric(collector.NewMetric(missingRequested,
		collector.WithType(collector.Counter),
		collector.WithHelp("Number of missing commitments requested by the node."),
		collector.WithInitFunc(func() {
			deps.Protocol.ChainManager().Events.CommitmentMissing.Attach(event.NewClosure(func(commitment commitment.ID) {
				deps.Collector.Increment(commitmentsNamespace, missingRequested)
			}))
		}),
	)),
	collector.WithMetric(collector.NewMetric(missingReceived,
		collector.WithType(collector.Counter),
		collector.WithHelp("Number of missing commitments received by the node."),
		collector.WithInitFunc(func() {
			deps.Protocol.ChainManager().Events.MissingCommitmentReceived.Attach(event.NewClosure(func(commitment commitment.ID) {
				deps.Collector.Increment(commitmentsNamespace, missingReceived)
			}))
		}),
	)),
)
