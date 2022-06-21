package messagelayer

import (
	"context"

	"github.com/iotaledger/goshimmer/packages/conflictdag"
	"github.com/iotaledger/goshimmer/packages/epoch"
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

type notarizationPluginDependencies struct {
	dig.In

	Tangle  *tangle.Tangle
	Manager *notarization.Manager
}

type notarizationManagerDependencies struct {
	dig.In

	Tangle  *tangle.Tangle
	Storage kvstore.KVStore
}

var (
	NotarizationPlugin *node.Plugin
	notarizationDeps   = new(notarizationPluginDependencies)
)

func init() {
	NotarizationPlugin = node.NewPlugin(NotarizationPluginName, notarizationDeps, node.Enabled, configureNotarizationPlugin, runNotarizationPlugin)

	NotarizationPlugin.Events.Init.Hook(event.NewClosure(func(event *node.InitEvent) {
		if err := event.Container.Provide(newNotarizationManager); err != nil {
			NotarizationPlugin.Panic(err)
		}
	}))
}

func configureNotarizationPlugin(plugin *node.Plugin) {
	if nodeSnapshot != nil {
		notarizationDeps.Manager.LoadSnapshot(nodeSnapshot.LedgerSnapshot)
	}
	notarizationDeps.Tangle.ConfirmationOracle.Events().MessageConfirmed.Attach(event.NewClosure(func(event *tangle.MessageConfirmedEvent) {
		notarizationDeps.Tangle.Storage.Message(event.Message.ID()).Consume(func(m *tangle.Message) {
			notarizationDeps.Manager.OnMessageConfirmed(m)
		})
	}))
	notarizationDeps.Tangle.ConfirmationOracle.Events().MessageOrphaned.Attach(event.NewClosure(func(event *tangle.MessageConfirmedEvent) {
		notarizationDeps.Manager.OnMessageOrphaned(event.Message)
	}))
	notarizationDeps.Tangle.Ledger.Events.TransactionConfirmed.Attach(event.NewClosure(func(event *ledger.TransactionConfirmedEvent) {
		notarizationDeps.Tangle.Ledger.Storage.CachedTransaction(event.TransactionID).Consume(func(t utxo.Transaction) {
			notarizationDeps.Manager.OnTransactionConfirmed(t.(*devnetvm.Transaction))
		})
	}))
	notarizationDeps.Tangle.Ledger.Events.TransactionInclusionUpdated.Attach(event.NewClosure(func(event *ledger.TransactionInclusionUpdatedEvent) {
		notarizationDeps.Manager.OnTransactionInclusionUpdated(event)
	}))

	notarizationDeps.Tangle.Ledger.ConflictDAG.Events.BranchConfirmed.Attach(event.NewClosure(func(event *conflictdag.BranchConfirmedEvent[utxo.TransactionID]) {
		notarizationDeps.Manager.OnBranchConfirmed(event.ID)
	}))
	notarizationDeps.Tangle.Ledger.ConflictDAG.Events.ConflictCreated.Attach(event.NewClosure(func(event *conflictdag.ConflictCreatedEvent[utxo.TransactionID, utxo.OutputID]) {
		notarizationDeps.Manager.OnBranchCreated(event.ID)
	}))
	notarizationDeps.Tangle.Ledger.ConflictDAG.Events.BranchRejected.Attach(event.NewClosure(func(event *conflictdag.BranchRejectedEvent[utxo.TransactionID]) {
		notarizationDeps.Manager.OnBranchRejected(event.ID)
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

func newNotarizationManager(deps notarizationManagerDependencies) *notarization.Manager {
	return notarization.NewManager(
		notarization.NewEpochManager(),
		notarization.NewEpochCommitmentFactory(deps.Storage, deps.Tangle),
		notarizationDeps.Tangle,
		notarization.MinCommittableEpochAge(NotarizationParameters.MinEpochCommitableAge),
		notarization.Log(Plugin.Logger()))
}

// GetLatestEC returns the latest commitment that a new message should commit to.
func GetLatestEC() (ecRecord *epoch.ECRecord, latestConfirmedEpoch epoch.Index, err error) {
	ecRecord, err = notarizationDeps.Manager.GetLatestEC()
	if err != nil {
		return
	}
	latestConfirmedEpoch, err = notarizationDeps.Manager.LatestConfirmedEpochIndex()
	return
}
