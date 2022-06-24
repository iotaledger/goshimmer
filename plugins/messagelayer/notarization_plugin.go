package messagelayer

import (
	"context"

	"github.com/iotaledger/hive.go/daemon"
	"github.com/iotaledger/hive.go/generics/event"
	"github.com/iotaledger/hive.go/kvstore"
	"github.com/iotaledger/hive.go/node"
	"go.uber.org/dig"

	"github.com/iotaledger/goshimmer/packages/epoch"
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
}

func runNotarizationPlugin(*node.Plugin) {
	if err := daemon.BackgroundWorker("Notarization", func(ctx context.Context) {
		<-ctx.Done()
		notarizationDeps.Manager.Shutdown()
	}, shutdown.PriorityNotarization); err != nil {
		NotarizationPlugin.Panicf("Failed to start as daemon: %s", err)
	}
}

func newNotarizationManager(deps notarizationManagerDependencies) *notarization.Manager {
	return notarization.NewManager(
		notarization.NewEpochCommitmentFactory(deps.Storage, deps.Tangle, NotarizationParameters.SnapshotDepth),
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
