package client

import (
	"runtime"
	"time"

	"github.com/iotaledger/goshimmer/packages/metrics"
	"github.com/iotaledger/goshimmer/plugins/analysis/packet"
	"github.com/iotaledger/goshimmer/plugins/autopeering/local"
	"github.com/iotaledger/hive.go/network"
	"github.com/shirou/gopsutil/cpu"
)

func sendMetricHeartbeat(conn *network.ManagedConnection, hb *packet.MetricHeartbeat) {
	data, err := packet.NewMetricHeartbeatMessage(hb)
	if err != nil {
		log.Info(err, " - metric heartbeat message skipped")
		return
	}

	connLock.Lock()
	defer connLock.Unlock()
	if _, err = conn.Write(data); err != nil {
		log.Debugw("Error while writing to connection", "Description", err)
	}
	// trigger AnalysisOutboundBytes event
	metrics.Events().AnalysisOutboundBytes.Trigger(uint64(len(data)))
}

func createMetricHeartbeat() *packet.MetricHeartbeat {
	// get own ID
	var nodeID []byte
	if local.GetInstance() != nil {
		// doesn't copy the ID, take care not to modify underlying bytearray!
		nodeID = local.GetInstance().ID().Bytes()
	}

	return &packet.MetricHeartbeat{
		OwnID:  nodeID,
		OS:     runtime.GOOS,
		Arch:   runtime.GOARCH,
		NumCPU: runtime.NumCPU(),
		CPUUsage: func() (p float64) {
			percent, err := cpu.Percent(time.Second, false)
			if err == nil {
				p = percent[0]
			}
			return
		}(),
		MemoryUsage: func() uint64 {
			var m runtime.MemStats
			runtime.ReadMemStats(&m)
			return m.Alloc
		}(),
	}
}
