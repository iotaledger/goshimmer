package dashboard

import (
	"net"
	"runtime"
	"strconv"
	"time"

	"github.com/iotaledger/goshimmer/packages/shutdown"
	"github.com/iotaledger/goshimmer/plugins/autopeering"
	"github.com/iotaledger/goshimmer/plugins/autopeering/local"
	"github.com/iotaledger/goshimmer/plugins/banner"
	"github.com/iotaledger/goshimmer/plugins/gossip"
	"github.com/iotaledger/goshimmer/plugins/messagelayer"
	"github.com/iotaledger/goshimmer/plugins/metrics"
	"github.com/iotaledger/hive.go/autopeering/peer/service"
	"github.com/iotaledger/hive.go/daemon"
	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/workerpool"
)

var (
	// settings
	metricsWorkerCount     = 1
	metricsWorkerQueueSize = 250
	metricsWorkerPool      *workerpool.WorkerPool
)

func configureMetricsWorkerPool() {
	metricsWorkerPool = workerpool.New(func(task workerpool.Task) {
		broadcastWsMessage(&wsmsg{MsgTypeMPSMetric, task.Param(0).(uint64)})
		broadcastWsMessage(&wsmsg{MsgTypeNodeStatus, currentNodeStatus()})
		broadcastWsMessage(&wsmsg{MsgTypeNeighborMetric, neighborMetrics()})
		broadcastWsMessage(&wsmsg{MsgTypeTipsMetric, messagelayer.TipSelector.TipCount()})
		task.Return(nil)
	}, workerpool.WorkerCount(metricsWorkerCount), workerpool.QueueSize(metricsWorkerQueueSize))
}

// runs metric feeds to report status updates (MPS, NodeStatus, neighbors, tips) to frontend
func runMetricsFeed() {
	updateStatus := events.NewClosure(func(mps uint64) {
		metricsWorkerPool.TrySubmit(mps)
	})

	daemon.BackgroundWorker("Analysis-Dashboard[StatusUpdate]", func(shutdownSignal <-chan struct{}) {
		metrics.Events.ReceivedMPSUpdated.Attach(updateStatus)
		metricsWorkerPool.Start()
		<-shutdownSignal
		log.Info("Stopping Dashboard[StatusUpdate] ...")
		metrics.Events.ReceivedMPSUpdated.Detach(updateStatus)
		metricsWorkerPool.Stop()
		log.Info("Stopping Dashboard[StatusUpdate] ... done")
	}, shutdown.PriorityDashboard)
}

func neighborMetrics() []neighbormetric {
	var stats []neighbormetric

	// gossip plugin might be disabled
	neighbors := gossip.Manager().AllNeighbors()
	if neighbors == nil {
		return stats
	}

	for _, neighbor := range neighbors {
		// unfortunately the neighbor manager doesn't keep track of the origin of the connection
		origin := "Inbound"
		for _, peer := range autopeering.Selection().GetOutgoingNeighbors() {
			if neighbor.Peer == peer {
				origin = "Outbound"
				break
			}
		}

		host := neighbor.Peer.IP().String()
		port := neighbor.Peer.Services().Get(service.GossipKey).Port()
		stats = append(stats, neighbormetric{
			ID:               neighbor.Peer.ID().String(),
			Address:          net.JoinHostPort(host, strconv.Itoa(port)),
			BytesRead:        neighbor.BytesRead(),
			BytesWritten:     neighbor.BytesWritten(),
			ConnectionOrigin: origin,
		})
	}
	return stats
}

func currentNodeStatus() *nodestatus {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	status := &nodestatus{}
	status.ID = local.GetInstance().ID().String()

	// node status
	status.Version = banner.AppVersion
	status.Uptime = time.Since(nodeStartAt).Milliseconds()

	// memory metrics
	status.Mem = &memmetrics{
		Sys:          m.Sys,
		HeapSys:      m.HeapSys,
		HeapInuse:    m.HeapInuse,
		HeapIdle:     m.HeapIdle,
		HeapReleased: m.HeapReleased,
		HeapObjects:  m.HeapObjects,
		MSpanInuse:   m.MSpanInuse,
		MCacheInuse:  m.MCacheInuse,
		StackSys:     m.StackSys,
		NumGC:        m.NumGC,
		LastPauseGC:  m.PauseNs[(m.NumGC+255)%256],
	}
	return status
}
