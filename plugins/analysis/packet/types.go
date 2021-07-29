package packet

import "github.com/iotaledger/hive.go/protocol/message"

const (
	// MessageTypeHeartbeat defines the Heartbeat msg type.
	MessageTypeHeartbeat message.Type = iota + 1
	// MessageTypeMetricHeartbeat defines the Metric Heartbeat msg type.
	MessageTypeMetricHeartbeat
)
