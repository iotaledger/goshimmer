package metrics

import (
	"bytes"
	"encoding/gob"
	"sync"

	"github.com/iotaledger/goshimmer/plugins/analysis/packet"
	"github.com/iotaledger/hive.go/events"
	"github.com/mr-tron/base58/base58"
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

func ClientsMetrics() map[string]ClientInfo {
	clientsMetricsMutex.RLock()
	defer clientsMetricsMutex.RUnlock()

	var buf bytes.Buffer
	enc := gob.NewEncoder(&buf)
	err := enc.Encode(clientsMetrics)
	if err != nil {
		return nil
	}

	dec := gob.NewDecoder(&buf)
	var copy map[string]ClientInfo
	err = dec.Decode(&copy)
	if err != nil {
		return nil
	}

	return copy
}
