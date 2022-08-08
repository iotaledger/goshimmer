package blocklayer

import (
	"context"

	"github.com/iotaledger/hive.go/core/daemon"
	"github.com/iotaledger/hive.go/core/generics/event"
	"github.com/iotaledger/hive.go/core/kvstore"
	"github.com/iotaledger/hive.go/core/node"
	"go.uber.org/dig"

	"github.com/iotaledger/goshimmer/packages/core/epoch"
	"github.com/iotaledger/goshimmer/packages/core/ledger"
	"github.com/iotaledger/goshimmer/packages/core/notarization"
	"github.com/iotaledger/goshimmer/packages/core/snapshot"
	"github.com/iotaledger/goshimmer/packages/node/shutdown"

	"github.com/iotaledger/goshimmer/packages/core/tangleold"
)

const (
	// SnapshotPluginName is the name of the snapshot plugin.
	SnapshotPluginName = "Snapshot"
)

type snapshotPluginDependencies struct {
	dig.In

	Tangle  *tangleold.Tangle
	Manager *snapshot.Manager
}

type snapshotDependencies struct {
	dig.In

	Tangle          *tangleold.Tangle
	NotarizationMgr *notarization.Manager
	Storage         kvstore.KVStore
}

var (
	SnapshotPlugin *node.Plugin
	snapshotnDeps  = new(snapshotPluginDependencies)
)

func init() {
	SnapshotPlugin = node.NewPlugin(SnapshotPluginName, snapshotnDeps, node.Enabled, configureSnapshotPlugin, runSnapshotPlugin)

	SnapshotPlugin.Events.Init.Hook(event.NewClosure(func(event *node.InitEvent) {
		if err := event.Container.Provide(newSnapshotManager); err != nil {
			SnapshotPlugin.Panic(err)
		}
	}))
}

func configureSnapshotPlugin(plugin *node.Plugin) {
	if Parameters.Snapshot.File != "" {
		headerConsumer := func(*ledger.SnapshotHeader) {}
		outputsConsumer := func([]*ledger.OutputWithMetadata) {}
		epochDiffsConsumer := func(*ledger.SnapshotHeader, map[epoch.Index]*ledger.EpochDiff) {}

		err := snapshot.LoadSnapshot(Parameters.Snapshot.File,
			headerConsumer,
			snapshotnDeps.Manager.LoadSolidEntryPoints,
			outputsConsumer,
			epochDiffsConsumer)
		if err != nil {
			plugin.Panic("could not load snapshot file:", err)
		}
	}
}

func runSnapshotPlugin(*node.Plugin) {
	if err := daemon.BackgroundWorker("Snapshot", func(ctx context.Context) {
		<-ctx.Done()
	}, shutdown.PriorityNotarization); err != nil {
		SnapshotPlugin.Panicf("Failed to start as daemon: %s", err)
	}
}

func newSnapshotManager(deps snapshotDependencies) *snapshot.Manager {
	return snapshot.NewManager(deps.Storage, deps.Tangle, deps.NotarizationMgr)
}
