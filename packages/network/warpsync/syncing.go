package warpsync

import (
	"github.com/iotaledger/hive.go/core/identity"

	"github.com/iotaledger/goshimmer/packages/core/commitment"
	"github.com/iotaledger/goshimmer/packages/core/epoch"
	"github.com/iotaledger/goshimmer/packages/network/p2p"
	wp "github.com/iotaledger/goshimmer/packages/network/warpsync/proto"
	"github.com/iotaledger/goshimmer/packages/protocol/models"
)

func (p *Protocol) RequestEpochBlocks(ei epoch.Index, ec commitment.ID, to ...identity.ID) {
	epochBlocksReq := &wp.EpochBlocksRequest{
		EI: int64(ei),
		EC: ec.Bytes(),
	}
	packet := &wp.Packet{Body: &wp.Packet_EpochBlocksRequest{EpochBlocksRequest: epochBlocksReq}}
	p.p2pManager.Send(packet, protocolID, to...)

	p.log.Debugw("sent epoch blocks request", "Index", ei, "EC", ec.Base58())
}

func (p *Protocol) SendEpochStarter(ei epoch.Index, ec commitment.ID, blocksCount int, to ...identity.ID) {
	epochStartRes := &wp.EpochBlocksStart{
		EI:          int64(ei),
		EC:          ec.Bytes(),
		BlocksCount: int64(blocksCount),
	}
	packet := &wp.Packet{Body: &wp.Packet_EpochBlocksStart{EpochBlocksStart: epochStartRes}}

	p.p2pManager.Send(packet, protocolID, to...)
}

func (p *Protocol) SendBlocksBatch(ei epoch.Index, ec commitment.ID, blocks []*models.Block, to ...identity.ID) {
	blocksBytes := make([][]byte, len(blocks))

	for i, block := range blocks {
		blockBytes, err := block.Bytes()
		if err != nil {
			p.log.Errorf("failed to serialize block %s: %s", block.ID(), err)
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

	p.p2pManager.Send(packet, protocolID, to...)
}

func (p *Protocol) SendEpochEnd(ei epoch.Index, ec commitment.ID, roots *commitment.Roots, to ...identity.ID) {
	epochBlocksEnd := &wp.EpochBlocksEnd{
		EI:                int64(ei),
		EC:                ec.Bytes(),
		StateMutationRoot: roots.StateMutationRoot().Bytes(),
		StateRoot:         roots.StateRoot().Bytes(),
		ManaRoot:          roots.ManaRoot().Bytes(),
	}
	packet := &wp.Packet{Body: &wp.Packet_EpochBlocksEnd{EpochBlocksEnd: epochBlocksEnd}}

	p.p2pManager.Send(packet, protocolID, to...)
}

func (p *Protocol) processEpochBlocksRequestPacket(packetEpochRequest *wp.Packet_EpochBlocksRequest, nbr *p2p.Neighbor) {
	ei := epoch.Index(packetEpochRequest.EpochBlocksRequest.GetEI())
	ec := commitment.NewMerkleRoot(packetEpochRequest.EpochBlocksRequest.GetEC())

	p.log.Debugw("received epoch blocks request", "peer", nbr.Peer.ID(), "Index", ei, "EC", ec)

	p.Events.EpochBlocksRequestReceived.Trigger(&EpochBlocksRequestReceivedEvent{
		Neighbor: nbr,
		EI:       ei,
	})
}

func (p *Protocol) processEpochBlocksStartPacket(packetEpochBlocksStart *wp.Packet_EpochBlocksStart, nbr *p2p.Neighbor) {
	epochBlocksStart := packetEpochBlocksStart.EpochBlocksStart
	ei := epoch.Index(epochBlocksStart.GetEI())

	p.log.Debugw("received epoch blocks start", "peer", nbr.Peer.ID(), "Index", ei, "blocksCount", epochBlocksStart.GetBlocksCount())

	p.Events.EpochBlocksStart.Trigger(&EpochBlocksStartEvent{
		Neighbor: nbr,
		EI:       ei,
	})
}

func (p *Protocol) processEpochBlocksBatchPacket(packetEpochBlocksBatch *wp.Packet_EpochBlocksBatch, nbr *p2p.Neighbor) {
	epochBlocksBatch := packetEpochBlocksBatch.EpochBlocksBatch
	ei := epoch.Index(epochBlocksBatch.GetEI())

	blocksBytes := epochBlocksBatch.GetBlocks()
	p.log.Debugw("received epoch blocks", "peer", nbr.Peer.ID(), "Index", ei, "blocksLen", len(blocksBytes))

	// TODO: TRIGGER UNPARSED BLOCK INSTEAD
	// for _, blockBytes := range blocksBytes {
	// 	block, err := p.parser.ParseBlock(nbr, blockBytes)
	// 	if err != nil {
	// 		p.log.Errorw("failed to deserialize block", "peer", nbr.Peer.ID(), "err", err)
	// 		return
	// 	}
	//
	// 	p.Events.EpochBlock.Trigger(&EpochBlockEvent{
	// 		EI:    ei,
	// 		Block: block,
	// 	})
	// }
}

func (p *Protocol) processEpochBlocksEndPacket(packetEpochBlocksEnd *wp.Packet_EpochBlocksEnd, nbr *p2p.Neighbor) {
	epochBlocksBatch := packetEpochBlocksEnd.EpochBlocksEnd
	ei := epoch.Index(epochBlocksBatch.GetEI())

	p.log.Debugw("received epoch blocks end", "peer", nbr.Peer.ID(), "Index", ei)

	p.Events.EpochBlocksEnd.Trigger(&EpochBlocksEndEvent{
		Neighbor:          nbr,
		EI:                ei,
		EC:                commitment.NewMerkleRoot(packetEpochBlocksEnd.EpochBlocksEnd.GetEC()),
		StateMutationRoot: commitment.NewMerkleRoot(packetEpochBlocksEnd.EpochBlocksEnd.GetStateMutationRoot()),
		StateRoot:         commitment.NewMerkleRoot(packetEpochBlocksEnd.EpochBlocksEnd.GetStateRoot()),
		ManaRoot:          commitment.NewMerkleRoot(packetEpochBlocksEnd.EpochBlocksEnd.GetManaRoot()),
	})
}
