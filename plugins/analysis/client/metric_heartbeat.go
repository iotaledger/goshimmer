package client

import (
	"io"
	"runtime"
	"time"

	"github.com/iotaledger/goshimmer/packages/metrics"
	"github.com/iotaledger/goshimmer/plugins/analysis/packet"
	"github.com/iotaledger/goshimmer/plugins/autopeering/local"
	"github.com/iotaledger/goshimmer/plugins/banner"
	"github.com/iotaledger/hive.go/identity"
	"github.com/shirou/gopsutil/cpu"
)

func sendMetricHeartbeat(w io.Writer, hb *packet.MetricHeartbeat) {
	data, err := packet.NewMetricHeartbeatMessage(hb)
	if err != nil {
		log.Debugw("metric heartbeat message skipped", "err", err)
		return
	}

	if _, err = w.Write(data); err != nil {
		log.Debugw("Error while writing to connection", "Description", err)
	}
	// trigger AnalysisOutboundBytes event
	metrics.Events().AnalysisOutboundBytes.Trigger(uint64(len(data)))
}

func createMetricHeartbeat() *packet.MetricHeartbeat {
	// get own ID
	nodeID := make([]byte, len(identity.ID{}))
	if local.GetInstance() != nil {
		copy(nodeID, local.GetInstance().ID().Bytes())
	}

	return &packet.MetricHeartbeat{
		Version: banner.AppVersion,
		OwnID:   nodeID,
		OS:      runtime.GOOS,
		Arch:    runtime.GOARCH,
		NumCPU:  runtime.GOMAXPROCS(0),
		// TODO: replace this with only the CPU usage of the GoShimmer process.
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
