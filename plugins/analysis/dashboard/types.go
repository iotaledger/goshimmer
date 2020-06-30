package dashboard

const (
	// MsgTypePing defines a ping message type.
	MsgTypePing byte = iota
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
