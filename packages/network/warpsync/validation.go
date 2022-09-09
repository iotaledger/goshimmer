package warpsync

import (
	"github.com/iotaledger/hive.go/core/identity"

	"github.com/iotaledger/goshimmer/packages/core/epoch"
	"github.com/iotaledger/goshimmer/packages/network/p2p"
	wp "github.com/iotaledger/goshimmer/packages/network/warpsync/proto"
	"github.com/iotaledger/goshimmer/plugins/epochstorage"
)

func (p *Protocol) RequestEpochCommittment(ei epoch.Index, to ...identity.ID) {
	committmentReq := &wp.EpochCommittmentRequest{EI: int64(ei)}
	packet := &wp.Packet{Body: &wp.Packet_EpochCommitmentRequest{EpochCommitmentRequest: committmentReq}}
	p.p2pManager.Send(packet, protocolID, to...)
	p.log.Debugw("sent epoch committment request", "EI", ei)
}

func (p *Protocol) processEpochCommittmentRequestPacket(packetEpochRequest *wp.Packet_EpochCommitmentRequest, nbr *p2p.Neighbor) {
	ei := epoch.Index(packetEpochRequest.EpochCommitmentRequest.GetEI())
	p.log.Debugw("received epoch committment request", "peer", nbr.Peer.ID(), "EI", ei)

	ecRecord, exists := epochstorage.GetEpochCommittment(ei)
	if !exists {
		return

	}

	p.sendEpochCommittmentMessage(ei, ecRecord.ECR(), ecRecord.PrevEC(), nbr.ID())

	p.log.Debugw("sent epoch committment", "peer", nbr.Peer.ID(), "EI", ei, "EC", ecRecord.ComputeEC().Base58())
}

func (p *Protocol) processEpochCommittmentPacket(packetEpochCommittment *wp.Packet_EpochCommitment, nbr *p2p.Neighbor) {
	ei := epoch.Index(packetEpochCommittment.EpochCommitment.GetEI())
	ecr := epoch.NewMerkleRoot(packetEpochCommittment.EpochCommitment.GetECR())
	prevEC := epoch.NewMerkleRoot(packetEpochCommittment.EpochCommitment.GetPrevEC())

	ecRecord := epoch.NewECRecord(ei)
	ecRecord.SetECR(ecr)
	ecRecord.SetPrevEC(prevEC)

	p.log.Debugw("received epoch committment", "peer", nbr.Peer.ID(), "EI", ei, "EC", ecRecord.ComputeEC().Base58())

	p.Events.EpochCommitmentReceived.Trigger(&EpochCommitmentReceivedEvent{
		Neighbor: nbr,
		ECRecord: ecRecord,
	})
}

func (p *Protocol) sendEpochCommittmentMessage(ei epoch.Index, ecr epoch.ECR, prevEC epoch.EC, to ...identity.ID) {
	committmentRes := &wp.EpochCommittment{
		EI:     int64(ei),
		ECR:    ecr.Bytes(),
		PrevEC: prevEC.Bytes(),
	}
	packet := &wp.Packet{Body: &wp.Packet_EpochCommitment{EpochCommitment: committmentRes}}

	p.p2pManager.Send(packet, protocolID, to...)
}
