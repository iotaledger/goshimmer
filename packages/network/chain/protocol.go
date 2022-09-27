package chain

import (
	"github.com/cockroachdb/errors"
	"github.com/iotaledger/hive.go/core/generics/event"
	"google.golang.org/protobuf/proto"

	chainProtocol "github.com/iotaledger/goshimmer/packages/network/chain/proto"
	"github.com/iotaledger/goshimmer/packages/network/p2p"
)

const (
	protocolID = "chain/0.0.1"
)

type Protocol struct {
	Events *Events

	p2pManager *p2p.Manager
}

func New(p2pManager *p2p.Manager) (newProtocol *Protocol) {
	newProtocol = &Protocol{
		Events:     NewEvents(),
		p2pManager: p2pManager,
	}

	newProtocol.p2pManager.RegisterProtocol(protocolID, &p2p.ProtocolHandler{
		PacketFactory:      warpSyncPacketFactory,
		NegotiationSend:    sendNegotiationMessage,
		NegotiationReceive: receiveNegotiationMessage,
		PacketHandler:      newProtocol.handlePacket,
	})

	return
}

func (p *Protocol) Stop() {
	p.p2pManager.UnregisterProtocol(protocolID)
}

func (p *Protocol) handlePacket(nbr *p2p.Neighbor, packet proto.Message) error {
	wpPacket := packet.(*chainProtocol.Packet)
	switch packetBody := wpPacket.GetBody().(type) {
	case *chainProtocol.Packet_EpochBlocksRequest:
		submitTask(p.processEpochBlocksRequestPacket, packetBody, nbr)
	case *chainProtocol.Packet_EpochBlocksStart:
		submitTask(p.processEpochBlocksStartPacket, packetBody, nbr)
	case *chainProtocol.Packet_EpochBlocksBatch:
		submitTask(p.processEpochBlocksBatchPacket, packetBody, nbr)
	case *chainProtocol.Packet_EpochBlocksEnd:
		submitTask(p.processEpochBlocksEndPacket, packetBody, nbr)
	case *chainProtocol.Packet_EpochCommitmentRequest:
		submitTask(p.processEpochCommittmentRequestPacket, packetBody, nbr)
	case *chainProtocol.Packet_EpochCommitment:
		submitTask(p.processEpochCommitmentPacket, packetBody, nbr)
	default:
		return errors.Errorf("unsupported packet; packet=%+v, packetBody=%T-%+v", wpPacket, packetBody, packetBody)
	}

	return nil
}

func warpSyncPacketFactory() proto.Message {
	return &chainProtocol.Packet{}
}

func sendNegotiationMessage(ps *p2p.PacketsStream) error {
	packet := &chainProtocol.Packet{Body: &chainProtocol.Packet_Negotiation{Negotiation: &chainProtocol.Negotiation{}}}
	return errors.WithStack(ps.WritePacket(packet))
}

func receiveNegotiationMessage(ps *p2p.PacketsStream) (err error) {
	packet := &chainProtocol.Packet{}
	if err := ps.ReadPacket(packet); err != nil {
		return errors.WithStack(err)
	}
	packetBody := packet.GetBody()
	if _, ok := packetBody.(*chainProtocol.Packet_Negotiation); !ok {
		return errors.Newf(
			"received packet isn't the negotiation packet; packet=%+v, packetBody=%T-%+v",
			packet, packetBody, packetBody,
		)
	}
	return nil
}

func submitTask[P any](packetProcessor func(packet P, nbr *p2p.Neighbor), packet P, nbr *p2p.Neighbor) {
	event.Loop.Submit(func() { packetProcessor(packet, nbr) })
}
