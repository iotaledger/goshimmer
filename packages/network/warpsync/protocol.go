package warpsync

import (
	"github.com/pkg/errors"
	"google.golang.org/protobuf/proto"

	"github.com/iotaledger/goshimmer/packages/network"
	wp "github.com/iotaledger/goshimmer/packages/network/warpsync/proto"
	"github.com/iotaledger/hive.go/core/identity"
	"github.com/iotaledger/hive.go/core/logger"
	"github.com/iotaledger/hive.go/runtime/workerpool"
)

const (
	protocolID = "warpsync/0.0.1"
)

type Protocol struct {
	Events *Events

	workerPool      *workerpool.WorkerPool
	networkEndpoint network.Endpoint
	log             *logger.Logger
}

func New(workerPool *workerpool.WorkerPool, networkEndpoing network.Endpoint, log *logger.Logger) (protocol *Protocol) {
	protocol = &Protocol{
		Events:          NewEvents(),
		workerPool:      workerPool,
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
		submitTask(p.workerPool, p.processEpochBlocksRequestPacket, packetBody, id)
	case *wp.Packet_EpochBlocksStart:
		submitTask(p.workerPool, p.processEpochBlocksStartPacket, packetBody, id)
	case *wp.Packet_EpochBlocksBatch:
		submitTask(p.workerPool, p.processEpochBlocksBatchPacket, packetBody, id)
	case *wp.Packet_EpochBlocksEnd:
		submitTask(p.workerPool, p.processEpochBlocksEndPacket, packetBody, id)
	default:
		return errors.Errorf("unsupported packet; packet=%+v, packetBody=%T-%+v", wpPacket, packetBody, packetBody)
	}

	return nil
}

func warpSyncPacketFactory() proto.Message {
	return &wp.Packet{}
}

func submitTask[P any](wp *workerpool.WorkerPool, packetProcessor func(packet P, id identity.ID), packet P, id identity.ID) {
	wp.Submit(func() { packetProcessor(packet, id) })
}
