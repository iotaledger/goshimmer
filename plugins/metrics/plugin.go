package metrics

import (
	"time"

	"github.com/iotaledger/goshimmer/packages/binary/messagelayer/message"
	"github.com/iotaledger/goshimmer/packages/binary/messagelayer/tangle"
	"github.com/iotaledger/goshimmer/packages/metrics"
	"github.com/iotaledger/goshimmer/packages/shutdown"
	"github.com/iotaledger/goshimmer/plugins/autopeering"
	"github.com/iotaledger/goshimmer/plugins/gossip"
	"github.com/iotaledger/goshimmer/plugins/messagelayer"
	"github.com/iotaledger/hive.go/daemon"
	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/logger"
	"github.com/iotaledger/hive.go/node"
	"github.com/iotaledger/hive.go/timeutil"
)

// PluginName is the name of the metrics plugin.
const PluginName = "Metrics"

// Plugin is the plugin instance of the metrics plugin.
var Plugin = node.NewPlugin(PluginName, node.Enabled, configure, run)

var log *logger.Logger

func configure(_ *node.Plugin) {
	log = logger.NewLogger(PluginName)
	// increase received MPS counter whenever we attached a message
	messagelayer.Tangle.Events.MessageAttached.Attach(events.NewClosure(func(cachedMessage *message.CachedMessage, cachedMessageMetadata *tangle.CachedMessageMetadata) {
		cachedMessage.Release()
		cachedMessageMetadata.Release()
		increaseReceivedMPSCounter()
	}))
	metrics.Events().FPCInboundBytes.Attach(events.NewClosure(func(amountBytes uint64) {
		_FPCInboundBytes.Add(amountBytes)
	}))
	metrics.Events().FPCOutboundBytes.Attach(events.NewClosure(func(amountBytes uint64) {
		_FPCOutboundBytes.Add(amountBytes)
	}))
	metrics.Events().AnalysisOutboundBytes.Attach(events.NewClosure(func(amountBytes uint64) {
		analysisOutboundBytes.Add(amountBytes)
	}))
	metrics.Events().CPUUsage.Attach(events.NewClosure(func(cpuPercent float64) {
		cpuUsage.Store(cpuPercent)
	}))
	metrics.Events().MemUsage.Attach(events.NewClosure(func(memAllocBytes uint64) {
		memUsageBytes.Store(memAllocBytes)
	}))
	metrics.Events().Synced.Attach(events.NewClosure(func(synced bool) {
		syncLock.Lock()
		defer syncLock.Unlock()
		isSynced = synced
	}))

	metrics.Events().DBSize.Attach(onDBSize)

	gossip.Manager().Events().NeighborRemoved.Attach(onNeighborRemoved)

	autopeering.Selection().Events().IncomingPeering.Attach(onAutopeeringSelection)
	autopeering.Selection().Events().OutgoingPeering.Attach(onAutopeeringSelection)
}

func run(_ *node.Plugin) {
	// create a background worker that "measures" the MPS value every second
	if err := daemon.BackgroundWorker("Metrics Updater", func(shutdownSignal <-chan struct{}) {
		timeutil.Ticker(func() {
			measureReceivedMPS()
			measureCPUUsage()
			measureMemUsage()
			measureSynced()
		}, 1*time.Second, shutdownSignal)
	}, shutdown.PriorityMetrics); err != nil {
		log.Panicf("Failed to start as daemon: %s", err)
	}
}
