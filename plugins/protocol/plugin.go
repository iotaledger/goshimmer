package protocol

import (
	"github.com/iotaledger/hive.go/core/generics/event"
	"github.com/iotaledger/hive.go/core/node"
	"go.uber.org/dig"

	"github.com/iotaledger/goshimmer/packages/core/notarization"
	"github.com/iotaledger/goshimmer/packages/network"
	"github.com/iotaledger/goshimmer/packages/protocol"
	"github.com/iotaledger/goshimmer/packages/protocol/database"
	"github.com/iotaledger/goshimmer/packages/protocol/instance"
	"github.com/iotaledger/goshimmer/packages/protocol/instance/engine"
	"github.com/iotaledger/goshimmer/packages/protocol/instance/engine/congestioncontrol"
	"github.com/iotaledger/goshimmer/packages/protocol/instance/engine/congestioncontrol/icca/scheduler"
	"github.com/iotaledger/goshimmer/packages/protocol/instance/engine/tangle/blockdag"
	"github.com/iotaledger/goshimmer/packages/protocol/instance/engine/tangle/booker"
	"github.com/iotaledger/goshimmer/packages/protocol/instance/engine/tangle/virtualvoting"
	"github.com/iotaledger/goshimmer/packages/protocol/instance/engine/tsc"
	"github.com/iotaledger/goshimmer/packages/protocol/instance/tipmanager"
)

// PluginName is the name of the gossip plugin.
const PluginName = "Protocol"

var (
	Plugin *node.Plugin

	deps = new(dependencies)
)

type dependencies struct {
	dig.In

	Network  network.Interface
	Protocol *protocol.Protocol
}

func init() {
	Plugin = node.NewPlugin(PluginName, deps, node.Enabled, configureLogging)
	Plugin.Events.Init.Hook(event.NewClosure(func(event *node.InitEvent) {
		if err := event.Container.Provide(provide); err != nil {
			Plugin.Panic(err)
		}
	}))
}

func provide(n network.Interface) (p *protocol.Protocol) {

	// TODO:
	//		tangleold.GenesisTime(genesisTime), -> set global variable
	//		tangleold.SyncTimeWindow(Parameters.BootstrapWindow),
	//		tangleold.CacheTimeProvider(database.CacheTimeProvider()),

	var dbProvider database.DBProvider
	if DatabaseParameters.InMemory {
		dbProvider = database.NewMemDB
	} else {
		dbProvider = database.NewDB
	}
	p = protocol.New(n, Plugin.Logger(),
		protocol.WithInstanceOptions(
			instance.WithNotarizationManagerOptions(
				notarization.MinCommittableEpochAge(NotarizationParameters.MinEpochCommittableAge),
				notarization.BootstrapWindow(NotarizationParameters.BootstrapWindow),
				notarization.ManaEpochDelay(ManaParameters.EpochDelay),
				notarization.Log(Plugin.Logger()),
			),
			instance.WithEngineOptions(
				engine.WithCongestionControlOptions(
					congestioncontrol.WithSchedulerOptions(
						scheduler.WithMaxBufferSize(SchedulerParameters.MaxBufferSize),
						scheduler.WithAcceptedBlockScheduleThreshold(SchedulerParameters.ConfirmedBlockThreshold),
						scheduler.WithRate(SchedulerParameters.Rate),
					),
				),
				engine.WithTSCManagerOptions(
					tsc.WithTimeSinceConfirmationThreshold(Parameters.TimeSinceConfirmationThreshold),
				),
			),
			instance.WithTipManagerOptions(
				tipmanager.WithWidth(Parameters.TangleWidth),
				tipmanager.WithTimeSinceConfirmationThreshold(Parameters.TimeSinceConfirmationThreshold),
			),
			instance.WithDatabaseManagerOptions(
				database.WithDBProvider(dbProvider),
				database.WithMaxOpenDBs(DatabaseParameters.MaxOpenDBs),
				database.WithGranularity(DatabaseParameters.Granularity),
			),
		),
		protocol.WithBaseDirectory(DatabaseParameters.Directory),
		protocol.WithSnapshotFileName(Parameters.Snapshot.FileName),
		protocol.WithSettingsFileName(Parameters.Settings.FileName),
	)

	return p
}

func configureLogging(*node.Plugin) {
	deps.Protocol.Events.Instance.Engine.Tangle.BlockDAG.BlockAttached.Attach(event.NewClosure(func(block *blockdag.Block) {
		Plugin.LogInfof("Block %s attached", block.ID())
	}))

	deps.Protocol.Events.Instance.Engine.Tangle.Booker.BlockBooked.Attach(event.NewClosure(func(block *booker.Block) {
		Plugin.LogInfof("Block %s booked", block.ID())
	}))

	deps.Protocol.Events.Instance.Engine.Tangle.VirtualVoting.BlockTracked.Attach(event.NewClosure(func(block *virtualvoting.Block) {
		Plugin.LogInfof("Block %s tracked", block.ID())
	}))

	deps.Protocol.Events.Instance.Engine.CongestionControl.Scheduler.BlockScheduled.Attach(event.NewClosure(func(block *scheduler.Block) {
		Plugin.LogInfof("Block %s scheduled", block.ID())
	}))
}
