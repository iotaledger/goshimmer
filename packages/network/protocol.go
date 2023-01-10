package network

import (
	"sync"

	"github.com/cockroachdb/errors"
	"google.golang.org/protobuf/proto"

	"github.com/iotaledger/hive.go/core/bytesfilter"
	"github.com/iotaledger/hive.go/core/generics/event"
	"github.com/iotaledger/hive.go/core/generics/lo"
	"github.com/iotaledger/hive.go/core/generics/options"
	"github.com/iotaledger/hive.go/core/generics/shrinkingmap"
	"github.com/iotaledger/hive.go/core/identity"
	"github.com/iotaledger/hive.go/core/types"

	"github.com/iotaledger/goshimmer/packages/core/commitment"
	"github.com/iotaledger/goshimmer/packages/core/epoch"
	nwmodels "github.com/iotaledger/goshimmer/packages/network/models"
	"github.com/iotaledger/goshimmer/packages/protocol/models"
)

const (
	protocolID = "iota/0.0.1"
)

type Protocol struct {
	Events *Events

	network                   Endpoint
	duplicateBlockBytesFilter *bytesfilter.BytesFilter

	requestedBlockHashes      *shrinkingmap.ShrinkingMap[types.Identifier, types.Empty]
	requestedBlockHashesMutex sync.Mutex
}

func NewProtocol(network Endpoint, opts ...options.Option[Protocol]) (protocol *Protocol) {
	return options.Apply(&Protocol{
		Events: NewEvents(),

		network:                   network,
		duplicateBlockBytesFilter: bytesfilter.New(10000),
		requestedBlockHashes:      shrinkingmap.New[types.Identifier, types.Empty](shrinkingmap.WithShrinkingThresholdCount(1000)),
	}, opts, func(p *Protocol) {
		network.RegisterProtocol(protocolID, newPacket, p.handlePacket)
	})
}

func (p *Protocol) SendBlock(block *models.Block, to ...identity.ID) {
	p.network.Send(&nwmodels.Packet{Body: &nwmodels.Packet_Block{Block: &nwmodels.Block{
		Bytes: lo.PanicOnErr(block.Bytes()),
	}}}, protocolID, to...)
}

func (p *Protocol) RequestBlock(id models.BlockID, to ...identity.ID) {
	p.requestedBlockHashesMutex.Lock()
	p.requestedBlockHashes.Set(id.Identifier, types.Void)
	p.requestedBlockHashesMutex.Unlock()

	p.network.Send(&nwmodels.Packet{Body: &nwmodels.Packet_BlockRequest{BlockRequest: &nwmodels.BlockRequest{
		Bytes: lo.PanicOnErr(id.Bytes()),
	}}}, protocolID, to...)
}

func (p *Protocol) SendEpochCommitment(cm *commitment.Commitment, to ...identity.ID) {
	p.network.Send(&nwmodels.Packet{Body: &nwmodels.Packet_EpochCommitment{EpochCommitment: &nwmodels.EpochCommitment{
		Bytes: lo.PanicOnErr(cm.Bytes()),
	}}}, protocolID, to...)
}

func (p *Protocol) RequestCommitment(id commitment.ID, to ...identity.ID) {
	p.network.Send(&nwmodels.Packet{Body: &nwmodels.Packet_EpochCommitmentRequest{EpochCommitmentRequest: &nwmodels.EpochCommitmentRequest{
		Bytes: lo.PanicOnErr(id.Bytes()),
	}}}, protocolID, to...)
}

func (p *Protocol) RequestAttestations(index epoch.Index, to ...identity.ID) {
	p.network.Send(&nwmodels.Packet{Body: &nwmodels.Packet_AttestationsRequest{AttestationsRequest: &nwmodels.AttestationsRequest{
		Bytes: index.Bytes(),
	}}}, protocolID, to...)
}

func (p *Protocol) Unregister() {
	p.network.UnregisterProtocol(protocolID)
}

func (p *Protocol) handlePacket(nbr identity.ID, packet proto.Message) (err error) {
	switch packetBody := packet.(*nwmodels.Packet).GetBody().(type) {
	case *nwmodels.Packet_Block:
		event.Loop.Submit(func() { p.onBlock(packetBody.Block.GetBytes(), nbr) })
	case *nwmodels.Packet_BlockRequest:
		event.Loop.Submit(func() { p.onBlockRequest(packetBody.BlockRequest.GetBytes(), nbr) })
	case *nwmodels.Packet_EpochCommitment:
		event.Loop.Submit(func() { p.onEpochCommitment(packetBody.EpochCommitment.GetBytes(), nbr) })
	case *nwmodels.Packet_EpochCommitmentRequest:
		event.Loop.Submit(func() { p.onEpochCommitmentRequest(packetBody.EpochCommitmentRequest.GetBytes(), nbr) })
	case *nwmodels.Packet_Attestations:
		event.Loop.Submit(func() { p.onAttestations(packetBody.Attestations.GetBytes(), nbr) })
	case *nwmodels.Packet_AttestationsRequest:
		event.Loop.Submit(func() { p.onAttestationsRequest(packetBody.AttestationsRequest.GetBytes(), nbr) })
	default:
		return errors.Errorf("unsupported packet; packet=%+v, packetBody=%T-%+v", packet, packetBody, packetBody)
	}

	return
}

func (p *Protocol) onBlock(blockData []byte, id identity.ID) {
	blockIdentifier := models.DetermineID(blockData, 0).Identifier

	isNew := p.duplicateBlockBytesFilter.AddIdentifier(blockIdentifier)

	p.requestedBlockHashesMutex.Lock()
	requested := p.requestedBlockHashes.Delete(blockIdentifier)
	p.requestedBlockHashesMutex.Unlock()

	if !isNew && !requested {
		return
	}

	block := new(models.Block)
	if _, err := block.FromBytes(blockData); err != nil {
		p.Events.Error.Trigger(&ErrorEvent{
			Error:  errors.Errorf("failed to deserialize block: %w", err),
			Source: id,
		})

		return
	}
	err := block.DetermineID(blockIdentifier)
	if err != nil {
		p.Events.Error.Trigger(&ErrorEvent{
			Error:  errors.Wrap(err, "error while determining received block's ID"),
			Source: id,
		})

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
		Commitment: &receivedCommitment,
		Source:     id,
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

func (p *Protocol) onAttestations(attestationsBytes []byte, id identity.ID) {
	// TODO: PARSE BYTES

	p.Events.AttestationsReceived.Trigger(&AttestationsReceivedEvent{
		Source: id,
	})
}

func (p *Protocol) onAttestationsRequest(epochIndexBytes []byte, id identity.ID) {
	epochIndex, _, err := epoch.IndexFromBytes(epochIndexBytes)
	if err != nil {
		p.Events.Error.Trigger(&ErrorEvent{
			Error:  errors.Errorf("failed to deserialize epoch index: %w", err),
			Source: id,
		})

		return
	}

	p.Events.AttestationsRequestReceived.Trigger(&AttestationsRequestReceivedEvent{
		Index:  epochIndex,
		Source: id,
	})
}

func newPacket() proto.Message {
	return &nwmodels.Packet{}
}
