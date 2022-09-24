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
	"github.com/iotaledger/goshimmer/packages/protocol/instance/congestioncontrol"
	"github.com/iotaledger/goshimmer/packages/protocol/instance/congestioncontrol/icca/scheduler"
	"github.com/iotaledger/goshimmer/packages/protocol/instance/engine"
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

	Network network.Interface
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

	var dbProvider database.DBProvider
	if DatabaseParameters.InMemory {
		dbProvider = database.NewMemDB
	} else {
		dbProvider = database.NewDB
	}

	p = protocol.New(deps.Network, Plugin.Logger(),
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
