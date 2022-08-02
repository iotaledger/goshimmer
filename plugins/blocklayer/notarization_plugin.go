package blocklayer

import (
	"context"

	"github.com/iotaledger/hive.go/daemon"
	"github.com/iotaledger/hive.go/generics/event"
	"github.com/iotaledger/hive.go/kvstore"
	"github.com/iotaledger/hive.go/node"
	"go.uber.org/dig"

	"github.com/iotaledger/goshimmer/packages/core/notarization"
	"github.com/iotaledger/goshimmer/packages/core/snapshot"
	"github.com/iotaledger/goshimmer/packages/node/shutdown"

	"github.com/iotaledger/goshimmer/packages/core/epoch"
	"github.com/iotaledger/goshimmer/packages/core/tangleold"
)

const (
	// NotarizationPluginName is the name of the notarization plugin.
	NotarizationPluginName = "Notarization"
)

type notarizationPluginDependencies struct {
	dig.In

	Tangle  *tangleold.Tangle
	Manager *notarization.Manager
}

type notarizationManagerDependencies struct {
	dig.In

	Tangle  *tangleold.Tangle
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
	if Parameters.Snapshot.File != "" {
		err := snapshot.LoadSnapshot(Parameters.Snapshot.File,
			notarizationDeps.Manager.LoadECandEIs,
			notarizationDeps.Manager.LoadOutputsWithMetadata,
			notarizationDeps.Manager.LoadEpochDiffs)
		if err != nil {
			plugin.Panic("could not load snapshot file:", err)
		}
	}
	// attach mana plugin event after notarization manager has been initialized
	notarizationDeps.Manager.Events.ManaVectorUpdate.Hook(onManaVectorToUpdateClosure)
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
		deps.Tangle,
		notarization.MinCommittableEpochAge(NotarizationParameters.MinEpochCommittableAge),
		notarization.BootstrapWindow(NotarizationParameters.BootstrapWindow),
		notarization.ManaDelay(ManaParameters.EpochDelay),
		notarization.Log(Plugin.Logger()))
}

// GetLatestEC returns the latest commitment that a new block should commit to.
func GetLatestEC() (ecRecord *epoch.ECRecord, latestConfirmedEpoch epoch.Index, err error) {
	ecRecord, err = notarizationDeps.Manager.GetLatestEC()
	if err != nil {
		return
	}
	latestConfirmedEpoch, err = notarizationDeps.Manager.LatestConfirmedEpochIndex()
	return
}
