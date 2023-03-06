package dashboardmetrics

import (
	"context"
	"time"

	"go.uber.org/dig"

	"github.com/iotaledger/goshimmer/packages/app/blockissuer"
	"github.com/iotaledger/goshimmer/packages/core/shutdown"
	"github.com/iotaledger/goshimmer/packages/network/p2p"
	"github.com/iotaledger/goshimmer/packages/node"
	"github.com/iotaledger/goshimmer/packages/protocol"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/tangle/blockdag"
	"github.com/iotaledger/hive.go/app/daemon"
	"github.com/iotaledger/hive.go/autopeering/peer"
	"github.com/iotaledger/hive.go/autopeering/selection"
	"github.com/iotaledger/hive.go/logger"
	"github.com/iotaledger/hive.go/runtime/event"
	"github.com/iotaledger/hive.go/runtime/timeutil"
)

// PluginName is the name of the metrics plugin.
const PluginName = "DashboardMetrics"

var (
	// Plugin is the plugin instance of the metrics plugin.
	Plugin *node.Plugin
	deps   = new(dependencies)
	log    *logger.Logger
)

type dependencies struct {
	dig.In

	Protocol    *protocol.Protocol
	BlockIssuer *blockissuer.BlockIssuer

	P2Pmgr    *p2p.Manager        `optional:"true"`
	Selection *selection.Protocol `optional:"true"`
	Local     *peer.Local
}

func init() {
	Plugin = node.NewPlugin(PluginName, deps, node.Enabled, configure, run)
}

func configure(_ *node.Plugin) {
	log = logger.NewLogger(PluginName)
}

func run(plugin *node.Plugin) {
	log.Infof("Starting %s ...", PluginName)
	registerLocalMetrics(plugin)
	// create a background worker that update the metrics every second
	if err := daemon.BackgroundWorker("Metrics Updater", func(ctx context.Context) {
		// Do not block until the Ticker is shutdown because we might want to start multiple Tickers and we can
		// safely ignore the last execution when shutting down.
		timeutil.NewTicker(func() {
			measureAttachedBPS()
			measureRequestQueueSize()
			measurePerComponentCounter()
		}, 1*time.Second, ctx)

		// Wait before terminating so we get correct log blocks from the daemon regarding the shutdown order.
		<-ctx.Done()
	}, shutdown.PriorityMetrics); err != nil {
		log.Panicf("Failed to start as daemon: %s", err)
	}
}

func registerLocalMetrics(plugin *node.Plugin) {
	// increase received BPS counter whenever we attached a block
	deps.Protocol.Events.Engine.Tangle.BlockDAG.BlockAttached.Hook(func(block *blockdag.Block) {
		blockCountPerComponentMutex.Lock()
		defer blockCountPerComponentMutex.Unlock()
		increaseReceivedBPSCounter()
	}, event.WithWorkerPool(plugin.WorkerPool))
}
