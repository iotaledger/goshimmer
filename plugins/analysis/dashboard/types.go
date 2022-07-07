package dashboard

const (
	// BlkTypePing defines a ping block type.
	BlkTypePing byte = 0
	// BlkTypeAddNode defines an addNode update block for autopeering visualizer.
	BlkTypeAddNode byte = iota + 2 // backwards compatibility due to FPC removal.
	// BlkTypeRemoveNode defines a removeNode update block for autopeering visualizer.
	BlkTypeRemoveNode
	// BlkTypeConnectNodes defines a connectNodes update block for autopeering visualizer.
	BlkTypeConnectNodes
	// BlkTypeDisconnectNodes defines a disconnectNodes update block for autopeering visualizer.
	BlkTypeDisconnectNodes
)

type wsblk struct {
	Type byte        `json:"type"`
	Data interface{} `json:"data"`
}
