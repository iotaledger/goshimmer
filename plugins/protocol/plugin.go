package protocol

import (
	"github.com/iotaledger/hive.go/core/generics/event"
	"github.com/iotaledger/hive.go/core/node"
	"go.uber.org/dig"

	"github.com/iotaledger/goshimmer/packages/core/notarization"
	"github.com/iotaledger/goshimmer/packages/network"
	"github.com/iotaledger/goshimmer/packages/protocol"
	"github.com/iotaledger/goshimmer/packages/protocol/instance"
	"github.com/iotaledger/goshimmer/packages/protocol/instance/engine"
	"github.com/iotaledger/goshimmer/packages/protocol/instance/engine/congestioncontrol"
	"github.com/iotaledger/goshimmer/packages/protocol/instance/engine/congestioncontrol/icca/scheduler"
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
	//		tangleold.GenesisTime(genesisTime),
	//		tangleold.SyncTimeWindow(Parameters.TangleTimeWindow),
	//		tangleold.StartSynced(Parameters.StartSynced),
	//		tangleold.CacheTimeProvider(database.CacheTimeProvider()),

	p = protocol.New(deps.Network, Plugin.Logger(),
		protocol.WithInstanceOptions(
			instance.WithNotarizationManagerOptions(
				notarization.MinCommittableEpochAge(NotarizationParameters.MinEpochCommittableAge),
				notarization.BootstrapWindow(NotarizationParameters.BootstrapWindow),
				// TODO: notarization.ManaEpochDelay(ManaParameters.EpochDelay),
				notarization.Log(Plugin.Logger()),
			),
			instance.WithEngineOptions(
				engine.WithCongestionControlOptions(
					congestioncontrol.WithSchedulerOptions(
						scheduler.WithMaxBufferSize(SchedulerParameters.MaxBufferSize),
						// TODO: TotalSupply:                     2779530283277761,
						scheduler.WithAcceptedBlockScheduleThreshold(SchedulerParameters.ConfirmedBlockThreshold),
						scheduler.WithRate(SchedulerParameters.Rate),
						// TODO:			AccessManaMapRetrieverFunc:      accessManaMapRetriever,
						//			TotalAccessManaRetrieveFunc:     totalAccessManaRetriever,
					),
				),
			),
			instance.WithTipManagerOptions(
				tipmanager.WithWidth(Parameters.TangleWidth),
				tipmanager.WithTimeSinceConfirmationThreshold(Parameters.TimeSinceConfirmationThreshold),
			),
		),
	)

	return p
}
