package warpsync

import (
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
	// p.sendEpochCommittmentMessage(ei, ecRecord.ECR(), ecRecord.PrevEC(), nbr.ID())
	//
	// p.log.Debugw("sent epoch committment", "peer", nbr.Peer.ID(), "Index", ei, "ID", ecRecord.ComputeEC().Base58())
}

func (p *Protocol) processEpochCommitmentPacket(packetEpochCommitment *wp.Packet_EpochCommitment, nbr *p2p.Neighbor) {
	ei := epoch.Index(packetEpochCommitment.EpochCommitment.GetEI())
	ecr := commitment.NewMerkleRoot(packetEpochCommitment.EpochCommitment.GetECR())
	prevEC := commitment.NewMerkleRoot(packetEpochCommitment.EpochCommitment.GetPrevEC())

	ecRecord := commitment.New(commitment.NewID(ei, ecr, prevEC))
	ecRecord.PublishData(ei, ecr, prevEC)

	p.log.Debugw("received epoch committment", "peer", nbr.Peer.ID(), "Index", ei, "ID", ecRecord.ID.Base58())

	p.Events.EpochCommitmentReceived.Trigger(&EpochCommitmentReceivedEvent{
		Neighbor: nbr,
		ECRecord: ecRecord,
	})
}

func (p *Protocol) sendEpochCommittmentMessage(ei epoch.Index, ecr commitment.RootsID, prevEC commitment.ID, to ...identity.ID) {
	committmentRes := &wp.EpochCommittment{
		EI:     int64(ei),
		ECR:    ecr.Bytes(),
		PrevEC: prevEC.Bytes(),
	}
	packet := &wp.Packet{Body: &wp.Packet_EpochCommitment{EpochCommitment: committmentRes}}

	p.p2pManager.Send(packet, protocolID, to...)
}
