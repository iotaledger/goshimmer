package dashboardmetrics

import (
	"context"
	"time"

	"github.com/iotaledger/hive.go/core/autopeering/peer"
	"github.com/iotaledger/hive.go/core/autopeering/selection"
	"github.com/iotaledger/hive.go/core/daemon"
	"github.com/iotaledger/hive.go/core/generics/event"
	"github.com/iotaledger/hive.go/core/logger"
	"github.com/iotaledger/hive.go/core/node"
	"github.com/iotaledger/hive.go/core/timeutil"
	"go.uber.org/dig"

	"github.com/iotaledger/goshimmer/packages/app/blockissuer"
	"github.com/iotaledger/goshimmer/packages/core/shutdown"
	"github.com/iotaledger/goshimmer/packages/network"
	"github.com/iotaledger/goshimmer/packages/network/p2p"
	"github.com/iotaledger/goshimmer/packages/protocol"
	"github.com/iotaledger/goshimmer/packages/protocol/congestioncontrol/icca/scheduler"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/tangle/blockdag"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/tangle/booker"
	"github.com/iotaledger/goshimmer/packages/protocol/models"
)

// PluginName is the name of the metrics plugin.
const PluginName = "Metrics"

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

func run(_ *node.Plugin) {
	log.Infof("Starting %s ...", PluginName)
	registerLocalMetrics()

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

func registerLocalMetrics() {
	// // Events declared in other packages which we want to listen to here ////

	// increase received BPS counter whenever we receive a block
	deps.Protocol.Network().Events.BlockReceived.Attach(event.NewClosure(func(_ *network.BlockReceivedEvent) {
		sumTimeMutex.Lock()
		defer sumTimeMutex.Unlock()

		increasePerComponentCounter(Received)
	}))

	// increase received BPS counter whenever a block passes filter checks
	deps.Protocol.Events.Engine.Filter.BlockAllowed.Attach(event.NewClosure(func(_ *models.Block) {
		sumTimeMutex.Lock()
		defer sumTimeMutex.Unlock()

		increasePerComponentCounter(Allowed)
	}))

	// increase received BPS counter whenever rate setter issues a block
	deps.BlockIssuer.RateSetter.Events.BlockIssued.Attach(event.NewClosure(func(_ *models.Block) {
		sumTimeMutex.Lock()
		defer sumTimeMutex.Unlock()

		increasePerComponentCounter(Issued)
	}))

	// increase received BPS counter whenever we attached a block
	deps.Protocol.Events.Engine.Tangle.BlockDAG.BlockAttached.Attach(event.NewClosure(func(block *blockdag.Block) {
		sumTimeMutex.Lock()
		defer sumTimeMutex.Unlock()

		sumTimesSinceIssued[Attached] += time.Since(block.IssuingTime())
		increasePerComponentCounter(Attached)
	}))

	// blocks can only become solid once, then they stay like that, hence no .Dec() part
	deps.Protocol.Events.Engine.Tangle.BlockDAG.BlockSolid.Attach(event.NewClosure(func(block *blockdag.Block) {
		increasePerComponentCounter(Solidified)
		sumTimeMutex.Lock()
		defer sumTimeMutex.Unlock()

		// Consume should release cachedBlockMetadata
		if block.IsSolid() {
			sumTimesSinceIssued[Solidified] += time.Since(block.IssuingTime())
		}
	}))

	// fired when a missing block was received and removed from missing block storage
	deps.Protocol.Events.Engine.Tangle.BlockDAG.MissingBlockAttached.Attach(event.NewClosure(func(_ *blockdag.Block) {
		missingBlockCountDB.Dec()
	}))

	deps.Protocol.Events.CongestionControl.Scheduler.BlockScheduled.Attach(event.NewClosure(func(block *scheduler.Block) {
		increasePerComponentCounter(Scheduled)
		sumTimeMutex.Lock()
		defer sumTimeMutex.Unlock()

		if block.IsScheduled() {
			sumTimesSinceIssued[Scheduled] += time.Since(block.IssuingTime())
		}
	}))

	deps.Protocol.Events.Engine.Tangle.Booker.BlockBooked.Attach(event.NewClosure(func(block *booker.Block) {
		increasePerComponentCounter(Booked)
		sumTimeMutex.Lock()
		defer sumTimeMutex.Unlock()

		if block.IsBooked() {
			sumTimesSinceIssued[Booked] += time.Since(block.IssuingTime())
		}
	}))

	deps.Protocol.Events.CongestionControl.Scheduler.BlockDropped.Attach(event.NewClosure(func(block *scheduler.Block) {
		increasePerComponentCounter(SchedulerDropped)
		sumTimeMutex.Lock()
		defer sumTimeMutex.Unlock()

		sumTimesSinceIssued[SchedulerDropped] += time.Since(block.IssuingTime())
	}))

	deps.Protocol.Events.CongestionControl.Scheduler.BlockSkipped.Attach(event.NewClosure(func(block *scheduler.Block) {
		increasePerComponentCounter(SchedulerSkipped)
		sumTimeMutex.Lock()
		defer sumTimeMutex.Unlock()

		sumTimesSinceIssued[SchedulerSkipped] += time.Since(block.IssuingTime())
	}))
}
