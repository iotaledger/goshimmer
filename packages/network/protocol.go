package network

import (
	"sync"

	"github.com/pkg/errors"
	"google.golang.org/protobuf/proto"

	"github.com/iotaledger/goshimmer/packages/core/commitment"
	"github.com/iotaledger/goshimmer/packages/core/epoch"
	nwmodels "github.com/iotaledger/goshimmer/packages/network/models"
	"github.com/iotaledger/goshimmer/packages/protocol/engine/notarization"
	"github.com/iotaledger/goshimmer/packages/protocol/models"
	"github.com/iotaledger/hive.go/core/bytesfilter"
	"github.com/iotaledger/hive.go/core/identity"
	"github.com/iotaledger/hive.go/core/types"
	"github.com/iotaledger/hive.go/ds/advancedset"
	"github.com/iotaledger/hive.go/ds/orderedmap"
	"github.com/iotaledger/hive.go/ds/shrinkingmap"
	dsTypes "github.com/iotaledger/hive.go/ds/types"
	"github.com/iotaledger/hive.go/lo"
	"github.com/iotaledger/hive.go/runtime/options"
	"github.com/iotaledger/hive.go/runtime/workerpool"
)

const (
	protocolID = "iota/0.0.1"
)

type Protocol struct {
	Events *Events

	epochTimeProvider *epoch.TimeProvider

	network                   Endpoint
	workerPool                *workerpool.WorkerPool
	duplicateBlockBytesFilter *bytesfilter.BytesFilter

	requestedBlockHashes      *shrinkingmap.ShrinkingMap[types.Identifier, dsTypes.Empty]
	requestedBlockHashesMutex sync.Mutex
}

func NewProtocol(network Endpoint, workerPool *workerpool.WorkerPool, epochTimeProvider *epoch.TimeProvider, opts ...options.Option[Protocol]) (protocol *Protocol) {
	return options.Apply(&Protocol{
		Events: NewEvents(),

		network:                   network,
		workerPool:                workerPool,
		epochTimeProvider:         epochTimeProvider,
		duplicateBlockBytesFilter: bytesfilter.New(10000),
		requestedBlockHashes:      shrinkingmap.New[types.Identifier, dsTypes.Empty](shrinkingmap.WithShrinkingThresholdCount(1000)),
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
	p.requestedBlockHashes.Set(id.Identifier, dsTypes.Void)
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

func (p *Protocol) SendAttestations(cm *commitment.Commitment, blockIDs models.BlockIDs, attestations *orderedmap.OrderedMap[epoch.Index, *advancedset.AdvancedSet[*notarization.Attestation]], to ...identity.ID) {
	p.network.Send(&nwmodels.Packet{Body: &nwmodels.Packet_Attestations{Attestations: &nwmodels.Attestations{
		Commitment:   lo.PanicOnErr(cm.Bytes()),
		Blocks:       lo.PanicOnErr(blockIDs.Bytes()),
		Attestations: lo.PanicOnErr(attestations.Encode()),
	}}}, protocolID, to...)
}

func (p *Protocol) RequestCommitment(id commitment.ID, to ...identity.ID) {
	p.network.Send(&nwmodels.Packet{Body: &nwmodels.Packet_EpochCommitmentRequest{EpochCommitmentRequest: &nwmodels.EpochCommitmentRequest{
		Bytes: lo.PanicOnErr(id.Bytes()),
	}}}, protocolID, to...)
}

func (p *Protocol) RequestAttestations(cm *commitment.Commitment, endIndex epoch.Index, to ...identity.ID) {
	p.network.Send(&nwmodels.Packet{Body: &nwmodels.Packet_AttestationsRequest{AttestationsRequest: &nwmodels.AttestationsRequest{
		Commitment: lo.PanicOnErr(cm.Bytes()),
		EndIndex:   endIndex.Bytes(),
	}}}, protocolID, to...)
}

func (p *Protocol) Unregister() {
	p.network.UnregisterProtocol(protocolID)
}

func (p *Protocol) handlePacket(nbr identity.ID, packet proto.Message) (err error) {
	switch packetBody := packet.(*nwmodels.Packet).GetBody().(type) {
	case *nwmodels.Packet_Block:
		p.workerPool.Submit(func() { p.onBlock(packetBody.Block.GetBytes(), nbr) })
	case *nwmodels.Packet_BlockRequest:
		p.workerPool.Submit(func() { p.onBlockRequest(packetBody.BlockRequest.GetBytes(), nbr) })
	case *nwmodels.Packet_EpochCommitment:
		p.workerPool.Submit(func() { p.onEpochCommitment(packetBody.EpochCommitment.GetBytes(), nbr) })
	case *nwmodels.Packet_EpochCommitmentRequest:
		p.workerPool.Submit(func() { p.onEpochCommitmentRequest(packetBody.EpochCommitmentRequest.GetBytes(), nbr) })
	case *nwmodels.Packet_Attestations:
		p.workerPool.Submit(func() {
			p.onAttestations(packetBody.Attestations.GetCommitment(), packetBody.Attestations.GetBlocks(), packetBody.Attestations.GetAttestations(), nbr)
		})
	case *nwmodels.Packet_AttestationsRequest:
		p.workerPool.Submit(func() {
			p.onAttestationsRequest(packetBody.AttestationsRequest.GetCommitment(), packetBody.AttestationsRequest.GetEndIndex(), nbr)
		})
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
			Error:  errors.Wrap(err, "failed to deserialize block"),
			Source: id,
		})

		return
	}
	err := block.DetermineID(p.epochTimeProvider, blockIdentifier)
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
			Error:  errors.Wrap(err, "failed to deserialize block request"),
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
			Error:  errors.Wrap(err, "failed to deserialize epoch commitment"),
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
			Error:  errors.Wrap(err, "failed to deserialize epoch commitment request"),
			Source: id,
		})

		return
	}

	p.Events.EpochCommitmentRequestReceived.Trigger(&EpochCommitmentRequestReceivedEvent{
		CommitmentID: receivedCommitmentID,
		Source:       id,
	})
}

func (p *Protocol) onAttestations(commitmentBytes []byte, blockIDBytes []byte, attestationsBytes []byte, id identity.ID) {
	cm := &commitment.Commitment{}
	if _, err := cm.FromBytes(commitmentBytes); err != nil {
		p.Events.Error.Trigger(&ErrorEvent{
			Error:  errors.Wrap(err, "failed to deserialize commitment"),
			Source: id,
		})

		return
	}

	blockIDs := models.NewBlockIDs()
	if _, err := blockIDs.FromBytes(blockIDBytes); err != nil {
		p.Events.Error.Trigger(&ErrorEvent{
			Error:  errors.Wrap(err, "failed to deserialize blockIDs"),
			Source: id,
		})

		return
	}

	attestations := orderedmap.New[epoch.Index, *advancedset.AdvancedSet[*notarization.Attestation]]()
	if _, err := attestations.Decode(attestationsBytes); err != nil {
		p.Events.Error.Trigger(&ErrorEvent{
			Error:  errors.Wrap(err, "failed to deserialize attestations"),
			Source: id,
		})

		return
	}

	p.Events.AttestationsReceived.Trigger(&AttestationsReceivedEvent{
		Commitment:   cm,
		BlockIDs:     blockIDs,
		Attestations: attestations,
		Source:       id,
	})
}

func (p *Protocol) onAttestationsRequest(commitmentBytes []byte, epochIndexBytes []byte, id identity.ID) {
	cm := &commitment.Commitment{}
	if _, err := cm.FromBytes(commitmentBytes); err != nil {
		p.Events.Error.Trigger(&ErrorEvent{
			Error:  errors.Wrap(err, "failed to deserialize commitment"),
			Source: id,
		})

		return
	}

	endEpochIndex, _, err := epoch.IndexFromBytes(epochIndexBytes)
	if err != nil {
		p.Events.Error.Trigger(&ErrorEvent{
			Error:  errors.Wrap(err, "failed to deserialize end epoch index"),
			Source: id,
		})

		return
	}

	p.Events.AttestationsRequestReceived.Trigger(&AttestationsRequestReceivedEvent{
		Commitment: cm,
		EndIndex:   endEpochIndex,
		Source:     id,
	})
}

func newPacket() proto.Message {
	return &nwmodels.Packet{}
}
