package chain

import (
	"github.com/cockroachdb/errors"
	"github.com/iotaledger/hive.go/core/generics/event"
	"github.com/iotaledger/hive.go/core/generics/lo"
	"github.com/iotaledger/hive.go/core/identity"
	"google.golang.org/protobuf/proto"

	"github.com/iotaledger/goshimmer/packages/core/commitment"
	protocol "github.com/iotaledger/goshimmer/packages/network/chain/proto"
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
	return (&Protocol{
		Events:     NewEvents(),
		p2pManager: p2pManager,
	}).init()
}

func (p *Protocol) SendEpochCommitment(commitment *commitment.Commitment, to ...identity.ID) {
	p.p2pManager.Send(&protocol.Packet{Body: &protocol.Packet_EpochCommitment{EpochCommitment: &protocol.EpochCommitment{
		Bytes: lo.PanicOnErr(commitment.Bytes()),
	}}}, protocolID, to...)
}

func (p *Protocol) RequestEpochCommitment(commitmentID commitment.ID, to ...identity.ID) {
	p.p2pManager.Send(&protocol.Packet{Body: &protocol.Packet_EpochCommitmentRequest{EpochCommitmentRequest: &protocol.EpochCommitmentRequest{
		Bytes: lo.PanicOnErr(commitmentID.Bytes()),
	}}}, protocolID, to...)
}

func (p *Protocol) Stop() {
	p.p2pManager.UnregisterProtocol(protocolID)
}

func (p *Protocol) init() (self *Protocol) {
	p.p2pManager.RegisterProtocol(protocolID, &p2p.ProtocolHandler{
		PacketFactory: func() proto.Message {
			return &protocol.Packet{}
		},

		NegotiationSend: func(ps *p2p.PacketsStream) error {
			return errors.WithStack(ps.WritePacket(&protocol.Packet{
				Body: &protocol.Packet_Negotiation{Negotiation: &protocol.Negotiation{}},
			}))
		},

		NegotiationReceive: func(ps *p2p.PacketsStream) (err error) {
			packet := &protocol.Packet{}
			if err := ps.ReadPacket(packet); err != nil {
				return errors.WithStack(err)
			}
			packetBody := packet.GetBody()
			if _, ok := packetBody.(*protocol.Packet_Negotiation); !ok {
				return errors.Newf("received packet isn't the negotiation packet; packet=%+v, packetBody=%T-%+v",
					packet, packetBody, packetBody,
				)
			}
			return nil
		},

		PacketHandler: p.handlePacket,
	})

	return p
}

func (p *Protocol) handlePacket(nbr identity.ID, packet proto.Message) error {
	switch packetBody := packet.(*protocol.Packet).GetBody().(type) {
	case *protocol.Packet_EpochCommitment:
		event.Loop.Submit(func() { p.receiveEpochCommitment(packetBody, nbr) })
	case *protocol.Packet_EpochCommitmentRequest:
		event.Loop.Submit(func() { p.receiveEpochCommitmentRequest(packetBody, nbr) })
	default:
		return errors.Errorf("unsupported packet; packet=%+v, packetBody=%T-%+v", packet, packetBody, packetBody)
	}

	return nil
}

func (p *Protocol) receiveEpochCommitment(packetEpochCommitment *protocol.Packet_EpochCommitment, id identity.ID) {
	var receivedCommitment commitment.Commitment
	if _, err := receivedCommitment.FromBytes(packetEpochCommitment.EpochCommitment.GetBytes()); err != nil {
		p.Events.Error.Trigger(&ErrorEvent{
			Error:    errors.Errorf("failed to deserialize epoch commitment packet: %w", err),
			Neighbor: id,
		})

		return
	}

	p.Events.EpochCommitmentReceived.Trigger(&EpochCommitmentReceivedEvent{
		Neighbor:   nbr,
		Commitment: &receivedCommitment,
	})
}

func (p *Protocol) receiveEpochCommitmentRequest(packetEpochRequest *protocol.Packet_EpochCommitmentRequest, nbr *p2p.Neighbor) {
	var receivedCommitmentID commitment.ID
	if _, err := receivedCommitmentID.FromBytes(packetEpochRequest.EpochCommitmentRequest.GetBytes()); err != nil {
		p.Events.Error.Trigger(&ErrorEvent{
			Error:    errors.Errorf("failed to deserialize epoch commitment request packet: %w", err),
			Neighbor: nbr,
		})

		return
	}

	p.Events.EpochCommitmentRequestReceived.Trigger(&EpochCommitmentRequestReceivedEvent{
		CommitmentID: receivedCommitmentID,
		Neighbor:     nbr,
	})
}
