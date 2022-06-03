package messagelayer

import (
	"context"

	"github.com/iotaledger/goshimmer/packages/conflictdag"
	"github.com/iotaledger/goshimmer/packages/ledger/utxo"
	"github.com/iotaledger/goshimmer/packages/ledger/vm/devnetvm"

	"github.com/iotaledger/hive.go/daemon"
	"github.com/iotaledger/hive.go/generics/event"
	"github.com/iotaledger/hive.go/kvstore"
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
	Tangle  *tangle.Tangle
	Storage kvstore.KVStore
	VM      *devnetvm.VM
}

var (
	NotarizationPlugin *node.Plugin
	notarizationDeps   = new(notarizationDependencies)

	notarizationManager *notarization.Manager
)

func init() {
	NotarizationPlugin = node.NewPlugin(NotarizationPluginName, deps, node.Enabled, configureNotarizationPlugin, runNotarizationPlugin)

	NotarizationPlugin.Events.Init.Hook(event.NewClosure(func(event *node.InitEvent) {
		if err := event.Container.Provide(newNotarizationManager); err != nil {
			NotarizationPlugin.Panic(err)
		}
	}))
}

func configureNotarizationPlugin(plugin *node.Plugin) {
	if nodeSnapshot != nil {
		if err := notarizationManager.LoadSnapshot(nodeSnapshot.LedgerSnapshot); err != nil {
			plugin.Panic(err)
		}
	}
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
	notarizationDeps.Tangle.Ledger.Events.TransactionInclusionUpdated.Attach(event.NewClosure(func(event *ledger.TransactionInclusionUpdatedEvent) {
		notarizationManager.OnTransactionInclusionUpdated(event)
	}))

	notarizationDeps.Tangle.Ledger.ConflictDAG.Events.BranchConfirmed.Attach(event.NewClosure(func(event *conflictdag.BranchConfirmedEvent[utxo.TransactionID]) {
		notarizationManager.OnBranchConfirmed(event.ID)
	}))
	notarizationDeps.Tangle.Ledger.ConflictDAG.Events.ConflictCreated.Attach(event.NewClosure(func(event *conflictdag.ConflictCreatedEvent[utxo.TransactionID, utxo.OutputID]) {
		notarizationManager.OnBranchCreated(event.ID)
	}))
	notarizationDeps.Tangle.Ledger.ConflictDAG.Events.BranchRejected.Attach(event.NewClosure(func(event *conflictdag.BranchRejectedEvent[utxo.TransactionID]) {
		notarizationManager.OnBranchRejected(event.ID)
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

func newNotarizationManager(deps *notarizationDependencies) *notarization.Manager {
	return notarization.NewManager(
		notarization.NewEpochManager(),
		notarization.NewEpochCommitmentFactory(deps.Storage, deps.VM, deps.Tangle),
		notarizationDeps.Tangle,
		notarization.MinCommittableEpochAge(NotarizationParameters.MinEpochCommitableDuration),
		notarization.Log(Plugin.Logger()))
}

// GetLatestEC returns the latest commitment that a new message should commit to.
func GetLatestEC() *tangle.EpochCommitment {
	return notarizationManager.GetLatestEC()
}
