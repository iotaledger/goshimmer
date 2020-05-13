package packet

import (
	"errors"

	"github.com/iotaledger/hive.go/protocol/message"
	"github.com/iotaledger/hive.go/protocol/tlv"
)

var (
	// ErrMalformedPacket is returned when malformed packets are tried to be parsed.
	ErrMalformedPacket = errors.New("malformed packet")
)

// AnalysisMsgRegistry holds all message definitions for analysis server related messages
var AnalysisMsgRegistry *message.Registry

func init() {
	AnalysisMsgRegistry = message.NewRegistry([]*message.Definition{
		tlv.HeaderMessageDefinition,
		HeartbeatMessageDefinition,
		FPCHeartbeatMessageDefinition,
	})
	// // register tlv header type
	// if err := AnalysisMsgRegistry.RegisterType(tlv.MessageTypeHeader, tlv.HeaderMessageDefinition); err != nil {
	// 	panic(err)
	// }

	// // analysis plugin specific types (msgType > 0)
	// if err := AnalysisMsgRegistry.RegisterType(MessageTypeHeartbeat, HeartbeatMessageDefinition); err != nil {
	// 	panic(err)
	// }

	// if err := AnalysisMsgRegistry.RegisterType(MessageTypeFPCHeartbeat, FPCHeartbeatMessageDefinition); err != nil {
	// 	panic(err)
	// }
}
