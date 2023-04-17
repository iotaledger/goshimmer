package network

import (
	"google.golang.org/protobuf/proto"

	"github.com/iotaledger/hive.go/crypto/identity"
)

type Endpoint interface {
	RegisterProtocol(protocolID string, newMessage func() proto.Message, handler func(identity.ID, proto.Message) error)

	UnregisterProtocol(protocolID string)

	Send(packet proto.Message, protocolID string, to ...identity.ID)
}
