package metrics

import (
	"fmt"
	"sync"

	analysisdashboard "github.com/iotaledger/goshimmer/plugins/analysis/dashboard"
	"github.com/iotaledger/goshimmer/plugins/analysis/packet"
	"github.com/iotaledger/hive.go/events"
	"github.com/mr-tron/base58/base58"
	"go.uber.org/atomic"
)

type ClientInfo struct {
	OS          string
	Arch        string
	NumCPU      int
	CPUUsage    float64
	MemoryUsage uint64
}

var (
	clientsMetrics      = make(map[string]ClientInfo)
	clientsMetricsMutex sync.RWMutex
	networkDiameter     atomic.Int32
)

var onMetricHeartbeatReceived = events.NewClosure(func(hb *packet.MetricHeartbeat) {
	clientsMetricsMutex.Lock()
	defer clientsMetricsMutex.Unlock()
	clientsMetrics[base58.Encode(hb.OwnID)] = ClientInfo{
		OS:          hb.OS,
		Arch:        hb.Arch,
		NumCPU:      hb.NumCPU,
		CPUUsage:    hb.CPUUsage,
		MemoryUsage: hb.MemoryUsage,
	}
})

// ClientsMetrics returns info about the OS, arch, number of cpu cores, cpu load and memory usage.
func ClientsMetrics() map[string]ClientInfo {
	clientsMetricsMutex.RLock()
	defer clientsMetricsMutex.RUnlock()
	// create copy of the map
	var copy = make(map[string]ClientInfo)
	// manually copy content
	for node, clientInfo := range clientsMetrics {
		copy[node] = clientInfo
	}
	return copy
}

func calculateNetworkDiameter() {
	g := analysisdashboard.NetworkGraph()
	diameter := g.Diameter()
	networkDiameter.Store(int32(diameter))
	fmt.Println("Calculated network diameter: ", diameter)
}

// NetworkDiameter returns the current network diameter.
func NetworkDiameter() int32 {
	return networkDiameter.Load()
}
