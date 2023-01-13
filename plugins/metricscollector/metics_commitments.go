package metricscollector

import (
	"strconv"

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
	acceptedBlocks   = "accepted_blocks"
	transactions     = "accepted_transactions"
	validators       = "active_validators"
)

var CommitmentsMetrics = collector.NewCollection(commitmentsNamespace,
	collector.WithMetric(collector.NewMetric(lastCommitment,
		collector.WithType(collector.GaugeVec),
		collector.WithHelp("Last commitment of the node."),
		collector.WithLabels("epoch", "commitment"),
		collector.WithLabelValuesCollection(),
		collector.WithInitFunc(func() {
			deps.Collector.ResetMetric(commitmentsNamespace, lastCommitment)
			deps.Protocol.Events.Engine.NotarizationManager.EpochCommitted.Attach(event.NewClosure(func(details *notarization.EpochCommittedDetails) {
				deps.Collector.Update(commitmentsNamespace, lastCommitment, collector.MultiLabels(strconv.Itoa(int(details.Commitment.Index())), details.Commitment.ID().Base58()))
			}))
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
	collector.WithMetric(collector.NewMetric(acceptedBlocks,
		collector.WithType(collector.GaugeVec),
		collector.WithHelp("Number of accepted blocks by the node per epoch."),
		collector.WithLabels("epoch"),
		collector.WithResetBeforeCollecting(true),
		collector.WithInitFunc(func() {
			deps.Protocol.Events.Engine.NotarizationManager.EpochCommitted.Attach(event.NewClosure(func(details *notarization.EpochCommittedDetails) {
				deps.Collector.Update(commitmentsNamespace, acceptedBlocks, collector.MultiLabelsValues([]string{strconv.Itoa(int(details.Commitment.Index()))}, details.AcceptedBlocksCount))
			}))
		}),
	)),
	collector.WithMetric(collector.NewMetric(transactions,
		collector.WithType(collector.GaugeVec),
		collector.WithHelp("Number of transactions by the node per epoch."),
		collector.WithLabels("epoch"),
		collector.WithResetBeforeCollecting(true),
		collector.WithInitFunc(func() {
			deps.Protocol.Events.Engine.NotarizationManager.EpochCommitted.Attach(event.NewClosure(func(details *notarization.EpochCommittedDetails) {
				deps.Collector.Update(commitmentsNamespace, transactions, collector.MultiLabelsValues([]string{strconv.Itoa(int(details.Commitment.Index()))}, details.AcceptedTransactionsCount))
			}))
		}),
	)),
	collector.WithMetric(collector.NewMetric(validators,
		collector.WithType(collector.GaugeVec),
		collector.WithHelp("Number of active validators per epoch."),
		collector.WithLabels("epoch"),
		collector.WithResetBeforeCollecting(true),
		collector.WithInitFunc(func() {
			deps.Protocol.Events.Engine.NotarizationManager.EpochCommitted.Attach(event.NewClosure(func(details *notarization.EpochCommittedDetails) {
				deps.Collector.Update(commitmentsNamespace, validators, collector.MultiLabelsValues([]string{strconv.Itoa(int(details.Commitment.Index()))}, details.ActiveValidatorsCount))
			}))
		}),
	)),
)
