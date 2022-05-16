package notarization

import (
	"context"

	"github.com/iotaledger/hive.go/daemon"
	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/node"
	"go.uber.org/dig"

	"github.com/iotaledger/goshimmer/packages/ledgerstate"
	"github.com/iotaledger/goshimmer/packages/notarization"
	"github.com/iotaledger/goshimmer/packages/shutdown"
	"github.com/iotaledger/goshimmer/packages/tangle"
)

const (
	// PluginName is the name of the notarization plugin.
	PluginName = "Notarization"
)

type dependencies struct {
	dig.In

	NotarizationManager    *notarization.Manager
	EpochManager           *notarization.EpochManager
	EpochCommitmentFactory *notarization.EpochCommitmentFactory
	Tangle                 *tangle.Tangle
}

var (
	Plugin *node.Plugin
	deps   = new(dependencies)
)

func init() {
	Plugin = node.NewPlugin(PluginName, deps, node.Enabled, configure, run)

	Plugin.Events.Init.Attach(events.NewClosure(func(_ *node.Plugin, container *dig.Container) {
		if err := container.Provide(newEpochManager); err != nil {
			Plugin.Panic(err)
		}

		if err := container.Provide(newEpochCommitmentFactory); err != nil {
			Plugin.Panic(err)
		}

		if err := container.Provide(newNotarizationManager); err != nil {
			Plugin.Panic(err)
		}

		if err := container.Provide(func() *node.Plugin {
			return Plugin
		}, dig.Name("notarization")); err != nil {
			Plugin.Panic(err)
		}
	}))
}

func configure(plugin *node.Plugin) {
	deps.Tangle.ConfirmationOracle.Events().MessageConfirmed.Attach(events.NewClosure(func(messageID tangle.MessageID) {
		deps.Tangle.Storage.Message(messageID).Consume(func(m *tangle.Message) {
			deps.NotarizationManager.OnMessageConfirmed(m)
		})
	}))
	deps.Tangle.ConfirmationOracle.Events().TransactionConfirmed.Attach(events.NewClosure(func(transactionID ledgerstate.TransactionID) {
		deps.Tangle.LedgerState.Transaction(transactionID).Consume(func(t *ledgerstate.Transaction) {
			deps.NotarizationManager.OnTransactionConfirmed(t)
		})
	}))
	deps.Tangle.ConfirmationOracle.Events().BranchConfirmed.Attach(events.NewClosure(func(branchID ledgerstate.BranchID) {
		deps.NotarizationManager.OnBranchConfirmed(branchID)
	}))
	deps.Tangle.LedgerState.BranchDAG.Events.BranchCreated.Attach(events.NewClosure(func(branchID ledgerstate.BranchID) {
		deps.NotarizationManager.OnBranchCreated(branchID)
	}))
	// TODO Branch Rejected
}

func run(*node.Plugin) {
	if err := daemon.BackgroundWorker("Notarization", func(ctx context.Context) {
		<-ctx.Done()
		deps.Tangle.Shutdown()
	}, shutdown.PriorityTangle); err != nil {
		Plugin.Panicf("Failed to start as daemon: %s", err)
	}
}

func newNotarizationManager() *notarization.Manager {
	return notarization.NewManager(
		deps.EpochManager,
		deps.EpochCommitmentFactory,
		deps.Tangle,
		notarization.MinCommitableEpochAge(Parameters.MinEpochCommitableDuration))
}

func newEpochManager() *notarization.EpochManager {
	return notarization.NewEpochManager()
}

func newEpochCommitmentFactory() *notarization.EpochCommitmentFactory {
	return notarization.NewCommitmentFactory()
}
