package blocklayer

import (
	"context"
	"github.com/iotaledger/goshimmer/packages/core/epoch"

	"github.com/iotaledger/hive.go/core/daemon"
	"github.com/iotaledger/hive.go/core/generics/event"
	"github.com/iotaledger/hive.go/core/kvstore"
	"github.com/iotaledger/hive.go/core/node"
	"go.uber.org/dig"

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

	Tangle          *tangleold.Tangle
	Manager         *snapshot.Manager
	NotarizationMgr *notarization.Manager
}

type snapshotDependencies struct {
	dig.In

	NotarizationMgr *notarization.Manager
	Storage         kvstore.KVStore
}

var (
	SnapshotPlugin *node.Plugin
	snapshotDeps   = new(snapshotPluginDependencies)
)

func init() {
	SnapshotPlugin = node.NewPlugin(SnapshotPluginName, snapshotDeps, node.Enabled, configureSnapshotPlugin, runSnapshotPlugin)

	SnapshotPlugin.Events.Init.Hook(event.NewClosure(func(event *node.InitEvent) {
		if err := event.Container.Provide(newSnapshotManager); err != nil {
			SnapshotPlugin.Panic(err)
		}
	}))
}

func configureSnapshotPlugin(plugin *node.Plugin) {
	if Parameters.Snapshot.File != "" {
		emptyHeaderConsumer := func(*ledger.SnapshotHeader) {}
		emptyOutputsConsumer := func([]*ledger.OutputWithMetadata) {}
		emptyEpochDiffsConsumer := func(*ledger.EpochDiff) {}
		emptyActivityLogConsumer := func(activity epoch.SnapshotEpochActivity) {}

		err := snapshot.LoadSnapshot(Parameters.Snapshot.File,
			emptyHeaderConsumer,
			snapshotDeps.Manager.LoadSolidEntryPoints,
			emptyOutputsConsumer,
			emptyEpochDiffsConsumer,
			emptyActivityLogConsumer)
		if err != nil {
			plugin.Panic("could not load snapshot file:", err)
		}
	}

	snapshotDeps.Tangle.ConfirmationOracle.Events().BlockAccepted.Attach(event.NewClosure(func(e *tangleold.BlockAcceptedEvent) {
		e.Block.ForEachParentByType(tangleold.StrongParentType, func(parent tangleold.BlockID) bool {
			index := parent.EpochIndex
			if index < e.Block.ID().EpochIndex {
				snapshotDeps.Manager.InsertSolidEntryPoint(parent)
			}
			return true
		})
	}))

	snapshotDeps.Tangle.ConfirmationOracle.Events().BlockOrphaned.Attach(event.NewClosure(func(event *tangleold.BlockAcceptedEvent) {
		snapshotDeps.Manager.RemoveSolidEntryPoint(event.Block)
	}))

	snapshotDeps.NotarizationMgr.Events.EpochCommittable.Attach(event.NewClosure(func(e *notarization.EpochCommittableEvent) {
		snapshotDeps.Manager.AdvanceSolidEntryPoints(e.EI)
	}))
}

func runSnapshotPlugin(*node.Plugin) {
	if err := daemon.BackgroundWorker("Snapshot", func(ctx context.Context) {
		<-ctx.Done()
	}, shutdown.PriorityNotarization); err != nil {
		SnapshotPlugin.Panicf("Failed to start as daemon: %s", err)
	}
}

func newSnapshotManager(deps snapshotDependencies) *snapshot.Manager {
	return snapshot.NewManager(deps.NotarizationMgr, NotarizationParameters.SnapshotDepth)
}
