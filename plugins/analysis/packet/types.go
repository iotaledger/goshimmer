package packet

import "github.com/iotaledger/hive.go/protocol/message"

const (
	MessageTypeHeartbeat message.Type = iota + 1
	MessageTypeFPCHeartbeat
)
