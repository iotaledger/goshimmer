package messagelayer

import (
	"context"

	"github.com/iotaledger/goshimmer/packages/conflictdag"
	"github.com/iotaledger/goshimmer/packages/ledger/utxo"
	"github.com/iotaledger/goshimmer/packages/ledger/vm/devnetvm"

	"github.com/iotaledger/hive.go/daemon"
	"github.com/iotaledger/hive.go/generics/event"
	"github.com/iotaledger/hive.go/node"
	"go.uber.org/dig"

	"github.com/iotaledger/goshimmer/packages/ledger"
	"github.com/iotaledger/goshimmer/packages/notarization"
	"github.com/iotaledger/goshimmer/packages/shutdown"
	"github.com/iotaledger/goshimmer/packages/tangle"
)

const (
	// NotarizationPluginName is the name of the notarization plugin.
	NotarizationPluginName = "Notarization"
)

type notarizationDependencies struct {
	dig.In
	Tangle *tangle.Tangle
}

var (
	NotarizationPlugin *node.Plugin
	notarizationDeps   = new(notarizationDependencies)

	notarizationManager *notarization.Manager
)

func init() {
	NotarizationPlugin = node.NewPlugin(NotarizationPluginName, deps, node.Enabled, configureNotarizationPlugin, runNotarizationPlugin)
}

func configureNotarizationPlugin(_ *node.Plugin) {
	notarizationManager = newNotarizationManager()
	notarizationDeps.Tangle.ConfirmationOracle.Events().MessageConfirmed.Attach(event.NewClosure(func(event *tangle.MessageConfirmedEvent) {
		notarizationDeps.Tangle.Storage.Message(event.Message.ID()).Consume(func(m *tangle.Message) {
			notarizationManager.OnMessageConfirmed(m)
		})
	}))
	notarizationDeps.Tangle.Ledger.Events.TransactionConfirmed.Attach(event.NewClosure(func(event *ledger.TransactionConfirmedEvent) {
		notarizationDeps.Tangle.Ledger.Storage.CachedTransaction(event.TransactionID).Consume(func(t utxo.Transaction) {
			notarizationManager.OnTransactionConfirmed(t.(*devnetvm.Transaction))
		})
	}))
	// TODO: attach to TransactionInclusionUpdatedEvent
	notarizationDeps.Tangle.Ledger.ConflictDAG.Events.BranchConfirmed.Attach(event.NewClosure(func(event *conflictdag.BranchConfirmedEvent[utxo.TransactionID]) {
		notarizationManager.OnBranchConfirmed(event.BranchID)
	}))
	notarizationDeps.Tangle.Ledger.ConflictDAG.Events.ConflictCreated.Attach(event.NewClosure(func(event *conflictdag.ConflictCreatedEvent[utxo.TransactionID, utxo.OutputID]) {
		notarizationManager.OnBranchCreated(event.BranchID)
	}))
	notarizationDeps.Tangle.ConfirmationOracle.Events().BranchRejected.Attach(event.NewClosure(func(event *conflictdag.BranchRejectedEvent[utxo.TransactionID]) {
		notarizationManager.OnBranchRejected(event.BranchID)
	}))
}

func runNotarizationPlugin(*node.Plugin) {
	if err := daemon.BackgroundWorker("Notarization", func(ctx context.Context) {
		<-ctx.Done()
		notarizationDeps.Tangle.Shutdown()
	}, shutdown.PriorityTangle); err != nil {
		NotarizationPlugin.Panicf("Failed to start as daemon: %s", err)
	}
}

func newNotarizationManager() *notarization.Manager {
	return notarization.NewManager(
		notarization.NewEpochManager(),
		notarization.NewEpochCommitmentFactory(),
		notarizationDeps.Tangle,
		notarization.MinCommitableEpochAge(NotarizationParameters.MinEpochCommitableDuration),
		notarization.Log(Plugin.Logger()))
}

// GetLatestEC returns the latest commitment that a new message should commit to.
func GetLatestEC() *tangle.EpochCommitment {
	return notarizationManager.GetLatestEC()
}
