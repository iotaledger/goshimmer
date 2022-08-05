package packet

import "github.com/iotaledger/hive.go/core/protocol/message"

const (
	// MessageTypeHeartbeat defines the Heartbeat blk type.
	MessageTypeHeartbeat message.Type = iota + 1
	// MessageTypeMetricHeartbeat defines the Metric Heartbeat blk type.
	MessageTypeMetricHeartbeat
)
