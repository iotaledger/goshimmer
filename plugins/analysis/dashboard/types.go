package dashboard

const (
	// MsgTypeNodeStatus is the type of the NodeStatus message.
	MsgTypeNodeStatus byte = iota
	// MsgTypeMPSMetric is the type of the message per second (MPS) metric message.
	MsgTypeMPSMetric
	// MsgTypeMessage is the type of the message.
	MsgTypeMessage
	// MsgTypeNeighborMetric is the type of the NeighborMetric message.
	MsgTypeNeighborMetric
	// MsgTypeDrng is the type of the dRNG message.
	MsgTypeDrng
	// MsgTypeTipsMetric is the type of the TipsMetric message.
	MsgTypeTipsMetric
	// MsgTypeVertex defines a vertex message.
	MsgTypeVertex
	// MsgTypeTipInfo defines a tip info message.
	MsgTypeTipInfo
	// MsgTypeFPC defines a FPC update message.
	MsgTypeFPC
	// MsgTypeAddNode defines an addNode update message for autopeering visualizer.
	MsgTypeAddNode
	// MsgTypeRemoveNode defines a removeNode update message for autopeering visualizer.
	MsgTypeRemoveNode
	// MsgTypeConnectNodes defines a connectNodes update message for autopeering visualizer.
	MsgTypeConnectNodes
	// MsgTypeDisconnectNodes defines a disconnectNodes update message for autopeering visualizer.
	MsgTypeDisconnectNodes
)

type wsmsg struct {
	Type byte        `json:"type"`
	Data interface{} `json:"data"`
}

type msg struct {
	ID    string `json:"id"`
	Value int64  `json:"value"`
}

type nodestatus struct {
	ID      string      `json:"id"`
	Version string      `json:"version"`
	Uptime  int64       `json:"uptime"`
	Mem     *memmetrics `json:"mem"`
}

type memmetrics struct {
	Sys          uint64 `json:"sys"`
	HeapSys      uint64 `json:"heap_sys"`
	HeapInuse    uint64 `json:"heap_inuse"`
	HeapIdle     uint64 `json:"heap_idle"`
	HeapReleased uint64 `json:"heap_released"`
	HeapObjects  uint64 `json:"heap_objects"`
	MSpanInuse   uint64 `json:"m_span_inuse"`
	MCacheInuse  uint64 `json:"m_cache_inuse"`
	StackSys     uint64 `json:"stack_sys"`
	NumGC        uint32 `json:"num_gc"`
	LastPauseGC  uint64 `json:"last_pause_gc"`
}

type neighbormetric struct {
	ID               string `json:"id"`
	Address          string `json:"address"`
	ConnectionOrigin string `json:"connection_origin"`
	BytesRead        uint32 `json:"bytes_read"`
	BytesWritten     uint32 `json:"bytes_written"`
}
