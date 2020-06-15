package metrics

import (
	"sync/atomic"

	"github.com/iotaledger/goshimmer/dapps/valuetransfers"
	"github.com/iotaledger/goshimmer/dapps/valuetransfers/packages/payload"
	valuetangle "github.com/iotaledger/goshimmer/dapps/valuetransfers/packages/tangle"
	"github.com/iotaledger/goshimmer/packages/binary/messagelayer/message"
	"github.com/iotaledger/goshimmer/packages/binary/messagelayer/tangle"
	"github.com/iotaledger/goshimmer/packages/metrics"
	"github.com/iotaledger/goshimmer/packages/shutdown"
	"github.com/iotaledger/goshimmer/packages/vote"
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

	//// Events declared in other packages which we want to listen to here ////

	// increase received MPS counter whenever we attached a message
	messagelayer.Tangle.Events.MessageAttached.Attach(events.NewClosure(func(cachedMessage *message.CachedMessage, cachedMessageMetadata *tangle.CachedMessageMetadata) {
		_payloadType := cachedMessage.Unwrap().Payload().Type()
		cachedMessage.Release()
		cachedMessageMetadata.Release()
		increaseReceivedMPSCounter()
		increasePerPayloadMPSCounter(_payloadType)
	}))

	// Value payload attached
	valuetransfers.Tangle.Events.PayloadAttached.Attach(events.NewClosure(func(cachedPayload *payload.CachedPayload, cachedPayloadMetadata *valuetangle.CachedPayloadMetadata) {
		cachedPayload.Release()
		cachedPayloadMetadata.Release()
		increaseReceivedTPSCounter()
	}))

	// FPC round executed
	valuetransfers.Voter().Events().RoundExecuted.Attach(events.NewClosure(func(roundStats vote.RoundStats) {
		processRoundStats(roundStats)
	}))

	//// Events coming from metrics package ////

	metrics.Events().FPCInboundBytes.Attach(events.NewClosure(func(amountBytes uint64) {
		atomic.AddUint64(&_FPCInboundBytes, amountBytes)
	}))
	metrics.Events().FPCOutboundBytes.Attach(events.NewClosure(func(amountBytes uint64) {
		atomic.AddUint64(&_FPCOutboundBytes, amountBytes)
	}))
	metrics.Events().CPUUsage.Attach(events.NewClosure(func(cpuPercent float64) {
		cpuLock.Lock()
		defer cpuLock.Unlock()
		_cpuUsage = cpuPercent
	}))
	metrics.Events().MemUsage.Attach(events.NewClosure(func(memAllocBytes uint64) {
		memUsageLock.Lock()
		defer memUsageLock.Unlock()
		_memUsageBytes = memAllocBytes
	}))
	metrics.Events().Synced.Attach(events.NewClosure(func(synced bool) {
		syncLock.Lock()
		defer syncLock.Unlock()
		isSynced = synced
	}))
	metrics.Events().MessageTips.Attach(events.NewClosure(func(tipsCount uint64) {
		atomic.StoreUint64(&messageTips, tipsCount)
	}))
	metrics.Events().ValueTips.Attach(events.NewClosure(func(tipsCount uint64) {
		atomic.StoreUint64(&valueTips, tipsCount)
	}))
}

func run(_ *node.Plugin) {
	// create a background worker that "measures" the MPS value every second
	if err := daemon.BackgroundWorker("Metrics Updater", func(shutdownSignal <-chan struct{}) {
		timeutil.Ticker(measureReceivedMPS, MPSMeasurementInterval, shutdownSignal)
		timeutil.Ticker(measureMPSPerPayload, MPSMeasurementInterval, shutdownSignal)
		timeutil.Ticker(measureMessageTips, MessageTipsMeasurementInterval, shutdownSignal)
		timeutil.Ticker(measureReceivedTPS, TPSMeasurementInterval, shutdownSignal)
		timeutil.Ticker(measureValueTips, ValueTipsMeasurementInterval, shutdownSignal)
		timeutil.Ticker(measureCPUUsage, CPUUsageMeasurementInterval, shutdownSignal)
		timeutil.Ticker(measureMemUsage, MemUsageMeasurementInterval, shutdownSignal)
		timeutil.Ticker(measureSynced, SyncedMeasurementInterval, shutdownSignal)
	}, shutdown.PriorityMetrics); err != nil {
		log.Panicf("Failed to start as daemon: %s", err)
	}
}
