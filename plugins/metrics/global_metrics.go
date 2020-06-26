package metrics

import (
	"sync"

	analysisdashboard "github.com/iotaledger/goshimmer/plugins/analysis/dashboard"
	"github.com/iotaledger/goshimmer/plugins/analysis/packet"
	"github.com/iotaledger/hive.go/events"
	"github.com/iotaledger/hive.go/identity"
	"go.uber.org/atomic"
)

// NodeInfo holds info of a node.
type NodeInfo struct {
	OS string
	// Arch defines the system architecture of the node.
	Arch string
	// NumCPU defines number of logical cores of the node.
	NumCPU int
	// CPUUsage defines the CPU usage of the node.
	CPUUsage float64
	// MemoryUsage defines the memory usage of the node.
	MemoryUsage uint64
}

var (
	nodesMetrics      = make(map[string]NodeInfo)
	nodesMetricsMutex sync.RWMutex
	networkDiameter   atomic.Int32
)

var onMetricHeartbeatReceived = events.NewClosure(func(hb *packet.MetricHeartbeat) {
	nodesMetricsMutex.Lock()
	defer nodesMetricsMutex.Unlock()
	nodesMetrics[shortNodeIDString(hb.OwnID)] = NodeInfo{
		OS:          hb.OS,
		Arch:        hb.Arch,
		NumCPU:      hb.NumCPU,
		CPUUsage:    hb.CPUUsage,
		MemoryUsage: hb.MemoryUsage,
	}
})

// NodesMetrics returns info about the OS, arch, number of cpu cores, cpu load and memory usage.
func NodesMetrics() map[string]NodeInfo {
	nodesMetricsMutex.RLock()
	defer nodesMetricsMutex.RUnlock()
	// create copy of the map
	var copy = make(map[string]NodeInfo)
	// manually copy content
	for node, clientInfo := range nodesMetrics {
		copy[node] = clientInfo
	}
	return copy
}

func calculateNetworkDiameter() {
	g := analysisdashboard.NetworkGraph()
	diameter := g.Diameter()
	networkDiameter.Store(int32(diameter))
}

// NetworkDiameter returns the current network diameter.
func NetworkDiameter() int32 {
	return networkDiameter.Load()
}

func shortNodeIDString(b []byte) string {
	var id identity.ID
	copy(id[:], b)
	return id.String()
}
