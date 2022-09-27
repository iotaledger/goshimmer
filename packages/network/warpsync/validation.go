package warpsync

import (
	"github.com/iotaledger/hive.go/core/generics/lo"
	"github.com/iotaledger/hive.go/core/identity"

	"github.com/iotaledger/goshimmer/packages/core/commitment"
	"github.com/iotaledger/goshimmer/packages/core/epoch"
	"github.com/iotaledger/goshimmer/packages/network/p2p"
	wp "github.com/iotaledger/goshimmer/packages/network/warpsync/proto"
)

func (p *Protocol) RequestEpochCommittment(ei epoch.Index, to ...identity.ID) {
	committmentReq := &wp.EpochCommittmentRequest{EI: int64(ei)}
	packet := &wp.Packet{Body: &wp.Packet_EpochCommitmentRequest{EpochCommitmentRequest: committmentReq}}
	p.p2pManager.Send(packet, protocolID, to...)
	p.log.Debugw("sent epoch committment request", "Index", ei)
}

func (p *Protocol) processEpochCommittmentRequestPacket(packetEpochRequest *wp.Packet_EpochCommitmentRequest, nbr *p2p.Neighbor) {
	ei := epoch.Index(packetEpochRequest.EpochCommitmentRequest.GetEI())
	p.log.Debugw("received epoch committment request", "peer", nbr.Peer.ID(), "Index", ei)

	// TODO: trigger event instead
	// ecRecord, exists := epochstorage.GetEpochCommittment(ei)
	// if !exists {
	// 	return
	//
	// }
	//
	// p.sendEpochCommitmentMessage(ei, ecRecord.ECR(), ecRecord.PrevEC(), nbr.ID())
	//
	// p.log.Debugw("sent epoch committment", "peer", nbr.Peer.ID(), "Index", ei, "ID", ecRecord.ComputeEC().Base58())
}

func (p *Protocol) processEpochCommitmentPacket(packetEpochCommitment *wp.Packet_EpochCommitment, nbr *p2p.Neighbor) {
	event := EpochCommitmentReceivedEvent{
		Neighbor: nbr,
	}

	if _, err := event.ECRecord.FromBytes(packetEpochCommitment.EpochCommitment.GetBytes()); err == nil {
		p.log.Debugw("received epoch commitment", "peer", nbr.Peer.ID(), "ID", event.ECRecord.ID())

		p.Events.EpochCommitmentReceived.Trigger(&event)
	}
}

func (p *Protocol) sendEpochCommitmentMessage(commitment *commitment.Commitment, to ...identity.ID) {
	p.p2pManager.Send(&wp.Packet{Body: &wp.Packet_EpochCommitment{EpochCommitment: &wp.EpochCommitment{
		Bytes: lo.PanicOnErr(commitment.Bytes()),
	}}}, protocolID, to...)
}
