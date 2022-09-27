package chain

import (
	"github.com/cockroachdb/errors"
	"github.com/iotaledger/hive.go/core/generics/event"
	"github.com/iotaledger/hive.go/core/generics/lo"
	"github.com/iotaledger/hive.go/core/identity"
	"google.golang.org/protobuf/proto"

	"github.com/iotaledger/goshimmer/packages/core/commitment"
	"github.com/iotaledger/goshimmer/packages/interfaces"
	protocol "github.com/iotaledger/goshimmer/packages/network/chain/proto"
)

const (
	protocolID = "chain/0.0.1"
)

type Protocol struct {
	Events *Events

	network interfaces.Network
}

func New(network interfaces.Network) (newProtocol *Protocol) {
	newProtocol = &Protocol{
		Events:  NewEvents(),
		network: network,
	}
	network.RegisterProtocol(protocolID, newPacket, newProtocol.handlePacket)

	return
}

func (p *Protocol) SendEpochCommitment(commitment *commitment.Commitment, to ...identity.ID) {
	p.network.Send(&protocol.Packet{Body: &protocol.Packet_EpochCommitment{EpochCommitment: &protocol.EpochCommitment{
		Bytes: lo.PanicOnErr(commitment.Bytes()),
	}}}, protocolID, to...)
}

func (p *Protocol) RequestEpochCommitment(commitmentID commitment.ID, to ...identity.ID) {
	p.network.Send(&protocol.Packet{Body: &protocol.Packet_EpochCommitmentRequest{EpochCommitmentRequest: &protocol.EpochCommitmentRequest{
		Bytes: lo.PanicOnErr(commitmentID.Bytes()),
	}}}, protocolID, to...)
}

func (p *Protocol) Stop() {
	p.network.UnregisterProtocol(protocolID)
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
			Error:  errors.Errorf("failed to deserialize epoch commitment packet: %w", err),
			Source: id,
		})

		return
	}

	p.Events.EpochCommitmentReceived.Trigger(&EpochCommitmentReceivedEvent{
		Neighbor:   id,
		Commitment: &receivedCommitment,
	})
}

func (p *Protocol) receiveEpochCommitmentRequest(packetEpochRequest *protocol.Packet_EpochCommitmentRequest, id identity.ID) {
	var receivedCommitmentID commitment.ID
	if _, err := receivedCommitmentID.FromBytes(packetEpochRequest.EpochCommitmentRequest.GetBytes()); err != nil {
		p.Events.Error.Trigger(&ErrorEvent{
			Error:  errors.Errorf("failed to deserialize epoch commitment request packet: %w", err),
			Source: id,
		})

		return
	}

	p.Events.EpochCommitmentRequestReceived.Trigger(&EpochCommitmentRequestReceivedEvent{
		CommitmentID: receivedCommitmentID,
		Source:       id,
	})
}

func newPacket() proto.Message {
	return &protocol.Packet{}
}
