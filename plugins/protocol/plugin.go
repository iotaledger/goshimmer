package protocol

import (
	"github.com/iotaledger/hive.go/core/generics/event"
	"github.com/iotaledger/hive.go/core/node"
	"go.uber.org/dig"

	"github.com/iotaledger/goshimmer/packages/core/database"
	"github.com/iotaledger/goshimmer/packages/core/epoch"
	"github.com/iotaledger/goshimmer/packages/network/p2p"
	"github.com/iotaledger/goshimmer/packages/protocol"
	"github.com/iotaledger/goshimmer/packages/protocol/congestioncontrol"
	"github.com/iotaledger/goshimmer/packages/protocol/congestioncontrol/icca/scheduler"
	"github.com/iotaledger/goshimmer/packages/protocol/engine"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/notarization"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/sybilprotection"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/sybilprotection/activitytracker"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/tsc"
	"github.com/iotaledger/goshimmer/packages/protocol/ledger"
	"github.com/iotaledger/goshimmer/packages/protocol/ledger/vm/devnetvm"
	"github.com/iotaledger/goshimmer/packages/protocol/tipmanager"
)

// PluginName is the name of the gossip plugin.
const PluginName = "Protocol"

var (
	Plugin *node.Plugin

	deps = new(dependencies)
)

type dependencies struct {
	dig.In

	Protocol *protocol.Protocol
	Network  *p2p.Manager
}

func init() {
	Plugin = node.NewPlugin(PluginName, deps, node.Enabled, configureLogging, run)
	Plugin.Events.Init.Hook(event.NewClosure(func(event *node.InitEvent) {
		if err := event.Container.Provide(provide); err != nil {
			Plugin.Panic(err)
		}
	}))
}

func provide(n *p2p.Manager) (p *protocol.Protocol) {
	cacheTimeProvider := database.NewCacheTimeProvider(DatabaseParameters.ForceCacheTime)

	if Parameters.GenesisTime > 0 {
		epoch.GenesisTime = Parameters.GenesisTime
	}

	var dbProvider database.DBProvider
	if DatabaseParameters.InMemory {
		dbProvider = database.NewMemDB
	} else {
		dbProvider = database.NewDB
	}
	p = protocol.New(n,
		protocol.WithEngineOptions(
			engine.WithNotarizationManagerOptions(
				notarization.MinCommittableEpochAge(NotarizationParameters.MinEpochCommittableAge),
			),
			engine.WithBootstrapThreshold(Parameters.BootstrapWindow),
			engine.WithTSCManagerOptions(
				tsc.WithTimeSinceConfirmationThreshold(Parameters.TimeSinceConfirmationThreshold),
			),
			engine.WithDatabaseManagerOptions(
				database.WithDBProvider(dbProvider),
				database.WithMaxOpenDBs(DatabaseParameters.MaxOpenDBs),
				database.WithGranularity(DatabaseParameters.Granularity),
			),
			engine.WithLedgerOptions(
				ledger.WithVM(new(devnetvm.VM)),
				ledger.WithCacheTimeProvider(cacheTimeProvider),
			),
			engine.WithSnapshotDepth(Parameters.Snapshot.Depth),
			engine.WithSybilProtectionOptions(
				sybilprotection.WithActivityTrackerOptions(
					activitytracker.WithActivityWindow(Parameters.ValidatorActivityWindow),
				),
			),
		),
		protocol.WithTipManagerOptions(
			tipmanager.WithWidth(Parameters.TangleWidth),
			tipmanager.WithTimeSinceConfirmationThreshold(Parameters.TimeSinceConfirmationThreshold),
		),
		protocol.WithCongestionControlOptions(
			congestioncontrol.WithSchedulerOptions(
				scheduler.WithMaxBufferSize(SchedulerParameters.MaxBufferSize),
				scheduler.WithAcceptedBlockScheduleThreshold(SchedulerParameters.ConfirmedBlockThreshold),
				scheduler.WithRate(SchedulerParameters.Rate),
			),
		),
		protocol.WithBaseDirectory(DatabaseParameters.Directory),
		protocol.WithSnapshotPath(Parameters.Snapshot.Path),
	)

	return p
}

func configureLogging(*node.Plugin) {
	// deps.Protocol.Events.Engine.Tangle.BlockDAG.BlockAttached.Attach(event.NewClosure(func(block *blockdag.Block) {
	// 	Plugin.LogDebugf("Block %s attached", block.ID())
	// }))
	//
	// deps.Protocol.Events.Engine.Tangle.Booker.BlockBooked.Attach(event.NewClosure(func(block *booker.Block) {
	// 	Plugin.LogDebugf("Block %s booked", block.ID())
	// }))
	//
	// deps.Protocol.Events.Engine.Tangle.VirtualVoting.BlockTracked.Attach(event.NewClosure(func(block *virtualvoting.Block) {
	// 	Plugin.LogDebugf("Block %s tracked", block.ID())
	// }))
	//
	// deps.Protocol.Events.CongestionControl.Scheduler.BlockScheduled.Attach(event.NewClosure(func(block *scheduler.Block) {
	// 	Plugin.LogDebugf("Block %s scheduled", block.ID())
	// }))

	deps.Protocol.Events.Engine.Error.Attach(event.NewClosure(func(err error) {
		Plugin.LogErrorf("Error in Engine: %s", err)
	}))

	deps.Protocol.Events.CongestionControl.Scheduler.BlockDropped.Attach(event.NewClosure(func(block *scheduler.Block) {
		Plugin.LogDebugf("Block %s dropped", block.ID())
	}))

	// deps.Protocol.Events.Engine.NotarizationManager.EpochCommittable.Attach(event.NewClosure(func(e *notarization.EpochCommittableEvent) {
	// 	fmt.Println("EpochCommittableEvent", e.EI)
	// }))

	// deps.Protocol.Events.Engine.Tangle.BlockDAG.BlockMissing.Attach(event.NewClosure(func(block *blockdag.Block) {
	// 	fmt.Println(">>>>>>> BlockMissing", block.ID())
	// }))
	//
	// deps.Protocol.Events.Engine.Tangle.BlockDAG.MissingBlockAttached.Attach(event.NewClosure(func(block *blockdag.Block) {
	// 	fmt.Println(">>>>>>> MissingBlockAttached", block.ID())
	// }))
	// deps.Protocol.Events.Engine.BlockRequester.Tick.Attach(event.NewClosure(func(blockID models.BlockID) {
	// 	fmt.Println(">>>>>>> BlockRequesterTick", blockID)
	// }))

}

func run(*node.Plugin) {
	deps.Protocol.Run()
}
