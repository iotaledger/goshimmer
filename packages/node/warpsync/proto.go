package warpsync

import (
	"github.com/cockroachdb/errors"
	"github.com/iotaledger/goshimmer/packages/core/epoch"
	"github.com/iotaledger/goshimmer/packages/core/tangleold"
	"github.com/iotaledger/goshimmer/packages/node/p2p"
	wp "github.com/iotaledger/goshimmer/packages/node/warpsync/warpsyncproto"
	"github.com/iotaledger/hive.go/core/identity"
	"google.golang.org/protobuf/proto"
)

func (m *Manager) handlePacket(nbr *p2p.Neighbor, packet proto.Message) error {
	wpPacket := packet.(*wp.Packet)
	switch packetBody := wpPacket.GetBody().(type) {
	case *wp.Packet_EpochBlocksRequest:
		return submitTask(m.processEpochBlocksRequestPacket, packetBody, nbr)
	case *wp.Packet_EpochBlocksStart:
		return submitTask(m.processEpochBlocksStartPacket, packetBody, nbr)
	case *wp.Packet_EpochBlocksBatch:
		return submitTask(m.processEpochBlocksBatchPacket, packetBody, nbr)
	case *wp.Packet_EpochBlocksEnd:
		return submitTask(m.processEpochBlocksEndPacket, packetBody, nbr)
	case *wp.Packet_EpochCommitmentRequest:
		return submitTask(m.processEpochCommittmentRequestPacket, packetBody, nbr)
	case *wp.Packet_EpochCommitment:
		return submitTask(m.processEpochCommittmentPacket, packetBody, nbr)
	default:
		return errors.Errorf("unsupported packet; packet=%+v, packetBody=%T-%+v", wpPacket, packetBody, packetBody)
	}
}

func (m *Manager) requestEpochCommittment(ei epoch.Index, to ...identity.ID) {
	committmentReq := &wp.EpochCommittmentRequest{EI: int64(ei)}
	packet := &wp.Packet{Body: &wp.Packet_EpochCommitmentRequest{EpochCommitmentRequest: committmentReq}}
	m.p2pManager.Send(packet, protocolID, to...)
	m.log.Debugw("sent epoch committment request", "EI", ei)
}

func (m *Manager) sendEpochCommittmentMessage(ei epoch.Index, ecr epoch.ECR, prevEC epoch.EC, to ...identity.ID) {
	committmentRes := &wp.EpochCommittment{
		EI:     int64(ei),
		ECR:    ecr.Bytes(),
		PrevEC: prevEC.Bytes(),
	}
	packet := &wp.Packet{Body: &wp.Packet_EpochCommitment{EpochCommitment: committmentRes}}

	m.p2pManager.Send(packet, protocolID, to...)
}

func (m *Manager) requestEpochBlocks(ei epoch.Index, ec epoch.EC, to ...identity.ID) {
	epochBlocksReq := &wp.EpochBlocksRequest{
		EI: int64(ei),
		EC: ec.Bytes(),
	}
	packet := &wp.Packet{Body: &wp.Packet_EpochBlocksRequest{EpochBlocksRequest: epochBlocksReq}}
	m.p2pManager.Send(packet, protocolID, to...)

	m.log.Debugw("sent epoch blocks request", "EI", ei, "EC", ec.Base58())
}

func (m *Manager) sendEpochStarter(ei epoch.Index, ec epoch.EC, blocksCount int, to ...identity.ID) {
	epochStartRes := &wp.EpochBlocksStart{
		EI:          int64(ei),
		EC:          ec.Bytes(),
		BlocksCount: int64(blocksCount),
	}
	packet := &wp.Packet{Body: &wp.Packet_EpochBlocksStart{EpochBlocksStart: epochStartRes}}

	m.p2pManager.Send(packet, protocolID, to...)
}

func (m *Manager) sendBlocksBatch(ei epoch.Index, ec epoch.EC, blocks []*tangleold.Block, to ...identity.ID) {
	blocksBytes := make([][]byte, len(blocks))

	for i, block := range blocks {
		blockBytes, err := block.Bytes()
		if err != nil {
			m.log.Errorf("failed to serialize block %s: %s", block.ID(), err)
			return
		}
		blocksBytes[i] = blockBytes
	}

	blocksBatchRes := &wp.EpochBlocksBatch{
		EI:     int64(ei),
		EC:     ec.Bytes(),
		Blocks: blocksBytes,
	}
	packet := &wp.Packet{Body: &wp.Packet_EpochBlocksBatch{EpochBlocksBatch: blocksBatchRes}}

	m.p2pManager.Send(packet, protocolID, to...)
}

func (m *Manager) sendEpochEnd(ei epoch.Index, ec epoch.EC, roots *epoch.CommitmentRoots, to ...identity.ID) {
	epochBlocksEnd := &wp.EpochBlocksEnd{
		EI:                int64(ei),
		EC:                ec.Bytes(),
		StateMutationRoot: roots.StateMutationRoot.Bytes(),
		StateRoot:         roots.StateRoot.Bytes(),
		ManaRoot:          roots.ManaRoot.Bytes(),
	}
	packet := &wp.Packet{Body: &wp.Packet_EpochBlocksEnd{EpochBlocksEnd: epochBlocksEnd}}

	m.p2pManager.Send(packet, protocolID, to...)
}

func warpsyncPacketFactory() proto.Message {
	return &wp.Packet{}
}

func sendNegotiationMessage(ps *p2p.PacketsStream) error {
	packet := &wp.Packet{Body: &wp.Packet_Negotiation{Negotiation: &wp.Negotiation{}}}
	return errors.WithStack(ps.WritePacket(packet))
}

func receiveNegotiationMessage(ps *p2p.PacketsStream) (err error) {
	packet := &wp.Packet{}
	if err := ps.ReadPacket(packet); err != nil {
		return errors.WithStack(err)
	}
	packetBody := packet.GetBody()
	if _, ok := packetBody.(*wp.Packet_Negotiation); !ok {
		return errors.Newf(
			"received packet isn't the negotiation packet; packet=%+v, packetBody=%T-%+v",
			packet, packetBody, packetBody,
		)
	}
	return nil
}
