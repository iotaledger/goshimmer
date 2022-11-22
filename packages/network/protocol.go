package network

import (
	"github.com/cockroachdb/errors"
	"google.golang.org/protobuf/proto"

	"github.com/iotaledger/hive.go/core/bytesfilter"
	"github.com/iotaledger/hive.go/core/generics/event"
	"github.com/iotaledger/hive.go/core/generics/lo"
	"github.com/iotaledger/hive.go/core/generics/options"
	"github.com/iotaledger/hive.go/core/generics/set"
	"github.com/iotaledger/hive.go/core/identity"

	"github.com/iotaledger/goshimmer/packages/core/commitment"
	. "github.com/iotaledger/goshimmer/packages/network/models"
	"github.com/iotaledger/goshimmer/packages/protocol/models"
)

const (
	protocolID = "iota/0.0.1"
)

type Protocol struct {
	Events *Events

	network                   Endpoint
	duplicateBlockBytesFilter *bytesfilter.BytesFilter

	requestedBlocks set.Set[models.BlockID]
}

func NewProtocol(network Endpoint, opts ...options.Option[Protocol]) (protocol *Protocol) {
	return options.Apply(&Protocol{
		Events: NewEvents(),

		network:                   network,
		duplicateBlockBytesFilter: bytesfilter.New(100000),
		requestedBlocks:           set.New[models.BlockID](true),
	}, opts, func(p *Protocol) {
		network.RegisterProtocol(protocolID, newPacket, p.handlePacket)
	})
}

func (p *Protocol) SendBlock(block *models.Block, to ...identity.ID) {
	p.network.Send(&Packet{Body: &Packet_Block{Block: &Block{
		Bytes: lo.PanicOnErr(block.Bytes()),
	}}}, protocolID, to...)
}

func (p *Protocol) RequestBlock(id models.BlockID, to ...identity.ID) {
	p.requestedBlocks.Add(id)
	p.network.Send(&Packet{Body: &Packet_BlockRequest{BlockRequest: &BlockRequest{
		Bytes: lo.PanicOnErr(id.Bytes()),
	}}}, protocolID, to...)
}

func (p *Protocol) SendEpochCommitment(commitment *commitment.Commitment, to ...identity.ID) {
	p.network.Send(&Packet{Body: &Packet_EpochCommitment{EpochCommitment: &EpochCommitment{
		Bytes: lo.PanicOnErr(commitment.Bytes()),
	}}}, protocolID, to...)
}

func (p *Protocol) RequestCommitment(id commitment.ID, to ...identity.ID) {
	p.network.Send(&Packet{Body: &Packet_EpochCommitmentRequest{EpochCommitmentRequest: &EpochCommitmentRequest{
		Bytes: lo.PanicOnErr(id.Bytes()),
	}}}, protocolID, to...)
}

func (p *Protocol) Unregister() {
	p.network.UnregisterProtocol(protocolID)
}

func (p *Protocol) handlePacket(nbr identity.ID, packet proto.Message) (err error) {
	switch packetBody := packet.(*Packet).GetBody().(type) {
	case *Packet_Block:
		event.Loop.Submit(func() { p.onBlock(packetBody.Block.GetBytes(), nbr) })
	case *Packet_BlockRequest:
		event.Loop.Submit(func() { p.onBlockRequest(packetBody.BlockRequest.GetBytes(), nbr) })
	case *Packet_EpochCommitment:
		event.Loop.Submit(func() { p.onEpochCommitment(packetBody.EpochCommitment.GetBytes(), nbr) })
	case *Packet_EpochCommitmentRequest:
		event.Loop.Submit(func() { p.onEpochCommitmentRequest(packetBody.EpochCommitmentRequest.GetBytes(), nbr) })
	default:
		return errors.Errorf("unsupported packet; packet=%+v, packetBody=%T-%+v", packet, packetBody, packetBody)
	}

	return
}

func (p *Protocol) onBlock(blockData []byte, id identity.ID) {
	block := new(models.Block)
	if _, err := block.FromBytes(blockData); err != nil {
		p.Events.Error.Trigger(&ErrorEvent{
			Error:  errors.Errorf("failed to deserialize block: %w", err),
			Source: id,
		})

		return
	}

	blockID := block.DetermineIDFromBytes(blockData)

	requested := p.requestedBlocks.Delete(blockID)

	if !p.duplicateBlockBytesFilter.Add(lo.PanicOnErr(blockID.Bytes())) && !requested {
		return
	}

	p.Events.BlockReceived.Trigger(&BlockReceivedEvent{
		Block:  block,
		Source: id,
	})
}

func (p *Protocol) onBlockRequest(idBytes []byte, id identity.ID) {
	var blockID models.BlockID
	if _, err := blockID.FromBytes(idBytes); err != nil {
		p.Events.Error.Trigger(&ErrorEvent{
			Error:  errors.Errorf("failed to deserialize block request: %w", err),
			Source: id,
		})

		return
	}

	p.Events.BlockRequestReceived.Trigger(&BlockRequestReceivedEvent{
		BlockID: blockID,
		Source:  id,
	})
}

func (p *Protocol) onEpochCommitment(commitmentBytes []byte, id identity.ID) {
	var receivedCommitment commitment.Commitment
	if _, err := receivedCommitment.FromBytes(commitmentBytes); err != nil {
		p.Events.Error.Trigger(&ErrorEvent{
			Error:  errors.Errorf("failed to deserialize epoch commitment: %w", err),
			Source: id,
		})

		return
	}

	p.Events.EpochCommitmentReceived.Trigger(&EpochCommitmentReceivedEvent{
		Neighbor:   id,
		Commitment: &receivedCommitment,
	})
}

func (p *Protocol) onEpochCommitmentRequest(idBytes []byte, id identity.ID) {
	var receivedCommitmentID commitment.ID
	if _, err := receivedCommitmentID.FromBytes(idBytes); err != nil {
		p.Events.Error.Trigger(&ErrorEvent{
			Error:  errors.Errorf("failed to deserialize epoch commitment request: %w", err),
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
	return &Packet{}
}
