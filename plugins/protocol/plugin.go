package protocol

import (
	"github.com/iotaledger/hive.go/core/generics/event"
	"github.com/iotaledger/hive.go/core/node"
	"go.uber.org/dig"

	"github.com/iotaledger/goshimmer/packages/core/notarization"
	"github.com/iotaledger/goshimmer/packages/network/p2p"
	"github.com/iotaledger/goshimmer/packages/protocol"
	"github.com/iotaledger/goshimmer/packages/protocol/database"
	"github.com/iotaledger/goshimmer/packages/protocol/engine"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/congestioncontrol"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/congestioncontrol/icca/scheduler"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/tipmanager"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/tsc"
)

// PluginName is the name of the gossip plugin.
const PluginName = "Protocol"

var (
	Plugin *node.Plugin

	deps = new(dependencies)
)

type dependencies struct {
	dig.In

	Network *p2p.Manager
}

func init() {
	Plugin = node.NewPlugin(PluginName, deps, node.Enabled)
	Plugin.Events.Init.Hook(event.NewClosure(func(event *node.InitEvent) {
		if err := event.Container.Provide(provide); err != nil {
			Plugin.Panic(err)
		}
	}))
}

func provide() (p *protocol.Protocol) {

	// TODO:
	//		tangleold.GenesisTime(genesisTime), -> set global variable
	//		tangleold.SyncTimeWindow(Parameters.BootstrapWindow),
	//		tangleold.CacheTimeProvider(database.CacheTimeProvider()),

	deps.Network.

	var dbProvider database.DBProvider
	if DatabaseParameters.InMemory {
		dbProvider = database.NewMemDB
	} else {
		dbProvider = database.NewDB
	}

	p = protocol.New(deps.Network, Plugin.Logger(),
		protocol.WithInstanceOptions(
			engine.WithNotarizationManagerOptions(
				notarization.MinCommittableEpochAge(NotarizationParameters.MinEpochCommittableAge),
				notarization.BootstrapWindow(NotarizationParameters.BootstrapWindow),
				notarization.ManaEpochDelay(ManaParameters.EpochDelay),
				notarization.Log(Plugin.Logger()),
			),
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
			engine.WithTipManagerOptions(
				tipmanager.WithWidth(Parameters.TangleWidth),
				tipmanager.WithTimeSinceConfirmationThreshold(Parameters.TimeSinceConfirmationThreshold),
			),
			engine.WithDatabaseManagerOptions(
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
