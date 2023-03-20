package metrics

import (
	"strconv"

	"github.com/iotaledger/goshimmer/packages/app/collector"
	"github.com/iotaledger/goshimmer/packages/core/commitment"
	"github.com/iotaledger/goshimmer/packages/protocol/chainmanager"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/notarization"
	"github.com/iotaledger/hive.go/runtime/event"
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
		collector.WithLabels("slot", "commitment"),
		collector.WithLabelValuesCollection(),
		collector.WithInitFunc(func() {
			deps.Collector.ResetMetric(commitmentsNamespace, lastCommitment)
			deps.Protocol.Events.Engine.Notarization.SlotCommitted.Hook(func(details *notarization.SlotCommittedDetails) {
				deps.Collector.Update(commitmentsNamespace, lastCommitment, collector.MultiLabels(strconv.Itoa(int(details.Commitment.Index())), details.Commitment.ID().Base58()))
			}, event.WithWorkerPool(Plugin.WorkerPool))
		}),
	)),
	collector.WithMetric(collector.NewMetric(seenTotal,
		collector.WithType(collector.Counter),
		collector.WithHelp("Number of commitments seen by the node."),
		collector.WithInitFunc(func() {
			deps.Protocol.Events.ChainManager.ForkDetected.Hook(func(_ *chainmanager.Fork) {
				deps.Collector.Increment(commitmentsNamespace, seenTotal)
			}, event.WithWorkerPool(Plugin.WorkerPool))
		}),
	)),
	collector.WithMetric(collector.NewMetric(missingRequested,
		collector.WithType(collector.Counter),
		collector.WithHelp("Number of missing commitments requested by the node."),
		collector.WithInitFunc(func() {
			deps.Protocol.Events.ChainManager.CommitmentMissing.Hook(func(commitment commitment.ID) {
				deps.Collector.Increment(commitmentsNamespace, missingRequested)
			}, event.WithWorkerPool(Plugin.WorkerPool))
		}),
	)),
	collector.WithMetric(collector.NewMetric(missingReceived,
		collector.WithType(collector.Counter),
		collector.WithHelp("Number of missing commitments received by the node."),
		collector.WithInitFunc(func() {
			deps.Protocol.Events.ChainManager.MissingCommitmentReceived.Hook(func(commitment commitment.ID) {
				deps.Collector.Increment(commitmentsNamespace, missingReceived)
			}, event.WithWorkerPool(Plugin.WorkerPool))
		}),
	)),
	collector.WithMetric(collector.NewMetric(acceptedBlocks,
		collector.WithType(collector.GaugeVec),
		collector.WithHelp("Number of accepted blocks by the node per slot."),
		collector.WithLabels("slot"),
		collector.WithResetBeforeCollecting(true),
		collector.WithInitFunc(func() {
			deps.Protocol.Events.Engine.Notarization.SlotCommitted.Hook(func(details *notarization.SlotCommittedDetails) {
				deps.Collector.Update(commitmentsNamespace, acceptedBlocks, collector.MultiLabelsValues([]string{strconv.Itoa(int(details.Commitment.Index()))}, details.AcceptedBlocks.Size()))
			}, event.WithWorkerPool(Plugin.WorkerPool))
		}),
	)),
	collector.WithMetric(collector.NewMetric(transactions,
		collector.WithType(collector.GaugeVec),
		collector.WithHelp("Number of transactions by the node per slot."),
		collector.WithLabels("slot"),
		collector.WithResetBeforeCollecting(true),
		collector.WithInitFunc(func() {
			deps.Protocol.Events.Engine.Notarization.SlotCommitted.Hook(func(details *notarization.SlotCommittedDetails) {
				deps.Collector.Update(commitmentsNamespace, transactions, collector.MultiLabelsValues([]string{strconv.Itoa(int(details.Commitment.Index()))}, details.AcceptedTransactions.Size()))
			}, event.WithWorkerPool(Plugin.WorkerPool))
		}),
	)),
	collector.WithMetric(collector.NewMetric(validators,
		collector.WithType(collector.GaugeVec),
		collector.WithHelp("Number of active validators per slot."),
		collector.WithLabels("slot"),
		collector.WithResetBeforeCollecting(true),
		collector.WithInitFunc(func() {
			deps.Protocol.Events.Engine.Notarization.SlotCommitted.Hook(func(details *notarization.SlotCommittedDetails) {
				deps.Collector.Update(commitmentsNamespace, validators, collector.MultiLabelsValues([]string{strconv.Itoa(int(details.Commitment.Index()))}, details.ActiveValidatorsCount))
			}, event.WithWorkerPool(Plugin.WorkerPool))
		}),
	)),
)
