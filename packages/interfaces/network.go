package interfaces

import (
	"github.com/iotaledger/hive.go/core/identity"
	"google.golang.org/protobuf/proto"
)

type Network interface {
	RegisterProtocol(protocolID string, newMessage func() proto.Message, handler func(identity.ID, proto.Message) error)

	UnregisterProtocol(protocolID string)
	
	Send(packet proto.Message, protocolID string, to ...identity.ID) []identity.ID
}
