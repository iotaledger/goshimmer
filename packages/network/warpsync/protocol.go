package warpsync

import (
	"github.com/cockroachdb/errors"
	"github.com/iotaledger/hive.go/core/generics/event"
	"github.com/iotaledger/hive.go/core/identity"
	"github.com/iotaledger/hive.go/core/logger"
	"google.golang.org/protobuf/proto"

	"github.com/iotaledger/goshimmer/packages/core/parser"
	"github.com/iotaledger/goshimmer/packages/network/p2p"
	wp "github.com/iotaledger/goshimmer/packages/network/warpsync/proto"
)

const (
	protocolID = "warpsync/0.0.1"
)

type Protocol struct {
	Events *Events

	p2pManager *p2p.Manager
	parser     *parser.Parser
	log        *logger.Logger
}

func New(p2pManager *p2p.Manager, parser *parser.Parser, log *logger.Logger) (new *Protocol) {
	new = &Protocol{
		Events:     NewEvents(),
		p2pManager: p2pManager,
		parser:     parser,
		log:        log,
	}

	new.p2pManager.RegisterProtocol(protocolID,
		warpSyncPacketFactory,
		new.handlePacket,
	)

	return
}

func (p *Protocol) Stop() {
	p.p2pManager.UnregisterProtocol(protocolID)
}

func (p *Protocol) handlePacket(id identity.ID, packet proto.Message) error {
	nbr, err := p.p2pManager.GetNeighbor(id)
	if err != nil {
		return err
	}

	wpPacket := packet.(*wp.Packet)
	switch packetBody := wpPacket.GetBody().(type) {
	case *wp.Packet_EpochBlocksRequest:
		submitTask(p.processEpochBlocksRequestPacket, packetBody, nbr)
	case *wp.Packet_EpochBlocksStart:
		submitTask(p.processEpochBlocksStartPacket, packetBody, nbr)
	case *wp.Packet_EpochBlocksBatch:
		submitTask(p.processEpochBlocksBatchPacket, packetBody, nbr)
	case *wp.Packet_EpochBlocksEnd:
		submitTask(p.processEpochBlocksEndPacket, packetBody, nbr)
	default:
		return errors.Errorf("unsupported packet; packet=%+v, packetBody=%T-%+v", wpPacket, packetBody, packetBody)
	}

	return nil
}

func warpSyncPacketFactory() proto.Message {
	return &wp.Packet{}
}

func submitTask[P any](packetProcessor func(packet P, nbr *p2p.Neighbor), packet P, nbr *p2p.Neighbor) {
	event.Loop.Submit(func() { packetProcessor(packet, nbr) })
}
