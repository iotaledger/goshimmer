package warpsync

import (
	"github.com/iotaledger/hive.go/core/generics/event"
	"github.com/iotaledger/hive.go/core/identity"
	"github.com/iotaledger/hive.go/core/logger"
	"github.com/pkg/errors"
	"google.golang.org/protobuf/proto"

	"github.com/iotaledger/goshimmer/packages/network"
	wp "github.com/iotaledger/goshimmer/packages/network/warpsync/proto"
)

const (
	protocolID = "warpsync/0.0.1"
)

type Protocol struct {
	Events *Events

	networkEndpoint network.Endpoint
	log             *logger.Logger
}

func New(networkEndpoing network.Endpoint, log *logger.Logger) (protocol *Protocol) {
	protocol = &Protocol{
		Events:          NewEvents(),
		networkEndpoint: networkEndpoing,
		log:             log,
	}

	protocol.networkEndpoint.RegisterProtocol(protocolID, warpSyncPacketFactory, protocol.handlePacket)

	return
}

func (p *Protocol) Stop() {
	p.networkEndpoint.UnregisterProtocol(protocolID)
}

func (p *Protocol) handlePacket(id identity.ID, packet proto.Message) error {
	wpPacket := packet.(*wp.Packet)
	switch packetBody := wpPacket.GetBody().(type) {
	case *wp.Packet_EpochBlocksRequest:
		submitTask(p.processEpochBlocksRequestPacket, packetBody, id)
	case *wp.Packet_EpochBlocksStart:
		submitTask(p.processEpochBlocksStartPacket, packetBody, id)
	case *wp.Packet_EpochBlocksBatch:
		submitTask(p.processEpochBlocksBatchPacket, packetBody, id)
	case *wp.Packet_EpochBlocksEnd:
		submitTask(p.processEpochBlocksEndPacket, packetBody, id)
	default:
		return errors.Errorf("unsupported packet; packet=%+v, packetBody=%T-%+v", wpPacket, packetBody, packetBody)
	}

	return nil
}

func warpSyncPacketFactory() proto.Message {
	return &wp.Packet{}
}

func submitTask[P any](packetProcessor func(packet P, id identity.ID), packet P, id identity.ID) {
	event.Loop.Submit(func() { packetProcessor(packet, id) })
}
