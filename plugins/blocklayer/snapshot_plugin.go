package blocklayer

import (
	"context"

	"github.com/iotaledger/hive.go/daemon"
	"github.com/iotaledger/hive.go/generics/event"
	"github.com/iotaledger/hive.go/kvstore"
	"github.com/iotaledger/hive.go/node"
	"go.uber.org/dig"

	"github.com/iotaledger/goshimmer/packages/core/snapshot"
	"github.com/iotaledger/goshimmer/packages/node/shutdown"

	"github.com/iotaledger/goshimmer/packages/core/tangleold"
)

const (
	// SnapshotPluginName is the name of the notarization plugin.
	SnapshotPluginName = "Snapshot"
)

type snapshotPluginDependencies struct {
	dig.In

	Tangle  *tangleold.Tangle
	Manager *snapshot.Manager
}

type snapshotDependencies struct {
	dig.In

	Tangle  *tangleold.Tangle
	Storage kvstore.KVStore
}

var (
	SnapshotPlugin *node.Plugin
	snapshotnDeps  = new(snapshotPluginDependencies)
)

func init() {
	SnapshotPlugin = node.NewPlugin(SnapshotPluginName, snapshotnDeps, node.Enabled, configureSnapshotPlugin, runNotarizationPlugin)

	SnapshotPlugin.Events.Init.Hook(event.NewClosure(func(event *node.InitEvent) {
		if err := event.Container.Provide(newSnapshotManager); err != nil {
			SnapshotPlugin.Panic(err)
		}
	}))
}

func configureSnapshotPlugin(plugin *node.Plugin) {}

func runSnapshotPlugin(*node.Plugin) {
	if err := daemon.BackgroundWorker("Snapshot", func(ctx context.Context) {
		<-ctx.Done()
		snapshotnDeps.Manager.Shutdown()
	}, shutdown.PriorityNotarization); err != nil {
		SnapshotPlugin.Panicf("Failed to start as daemon: %s", err)
	}
}

func newSnapshotManager(deps snapshotDependencies) *snapshot.Manager {
	return snapshot.NewManager(deps.Storage, deps.Tangle)
}
