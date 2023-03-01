package warpsync

import (
	"github.com/iotaledger/goshimmer/packages/core/commitment"
	wp "github.com/iotaledger/goshimmer/packages/network/warpsync/proto"
	"github.com/iotaledger/goshimmer/packages/protocol/models"
	"github.com/iotaledger/hive.go/core/slot"
	"github.com/iotaledger/hive.go/crypto/identity"
	"github.com/iotaledger/hive.go/lo"
)

func (p *Protocol) RequestSlotBlocks(index slot.Index, sc commitment.ID, to ...identity.ID) {
	slotBlocksReq := &wp.SlotBlocksRequest{
		SI: int64(index),
		SC: lo.PanicOnErr(sc.Bytes()),
	}
	packet := &wp.Packet{Body: &wp.Packet_SlotBlocksRequest{SlotBlocksRequest: slotBlocksReq}}
	p.networkEndpoint.Send(packet, protocolID, to...)

	p.log.Debugw("sent slot blocks request", "Index", index, "SC", sc.Base58())
}

func (p *Protocol) SendSlotStarter(index slot.Index, sc commitment.ID, blocksCount int, to ...identity.ID) {
	slotStartRes := &wp.SlotBlocksStart{
		SI:          int64(index),
		SC:          lo.PanicOnErr(sc.Bytes()),
		BlocksCount: int64(blocksCount),
	}
	packet := &wp.Packet{Body: &wp.Packet_SlotBlocksStart{SlotBlocksStart: slotStartRes}}

	p.networkEndpoint.Send(packet, protocolID, to...)
}

func (p *Protocol) SendBlocksBatch(index slot.Index, sc commitment.ID, blocks []*models.Block, to ...identity.ID) {
	blocksBytes := make([][]byte, len(blocks))

	for i, block := range blocks {
		blockBytes, err := block.Bytes()
		if err != nil {
			p.log.Errorf("failed to serialize block %s: %s", block.ID(), err)
			return
		}
		blocksBytes[i] = blockBytes
	}

	blocksBatchRes := &wp.SlotBlocksBatch{
		SI:     int64(index),
		SC:     lo.PanicOnErr(sc.Bytes()),
		Blocks: blocksBytes,
	}
	packet := &wp.Packet{Body: &wp.Packet_SlotBlocksBatch{SlotBlocksBatch: blocksBatchRes}}

	p.networkEndpoint.Send(packet, protocolID, to...)
}

func (p *Protocol) SendSlotEnd(index slot.Index, sc commitment.ID, roots *commitment.Roots, to ...identity.ID) {
	slotBlocksEnd := &wp.SlotBlocksEnd{
		SI:    int64(index),
		SC:    lo.PanicOnErr(sc.Bytes()),
		Roots: lo.PanicOnErr(roots.Bytes()),
	}
	packet := &wp.Packet{Body: &wp.Packet_SlotBlocksEnd{SlotBlocksEnd: slotBlocksEnd}}

	p.networkEndpoint.Send(packet, protocolID, to...)
}

func (p *Protocol) processSlotBlocksRequestPacket(packetSlotRequest *wp.Packet_SlotBlocksRequest, id identity.ID) {
	ei := slot.Index(packetSlotRequest.SlotBlocksRequest.GetSI())
	sc := new(commitment.Commitment)
	if _, err := sc.FromBytes(packetSlotRequest.SlotBlocksRequest.GetSC()); err != nil {
		p.log.Errorw("received slot blocks request: unable to deserialize commitment", "peer", id, "Index", ei, "err", err)
		return
	}

	p.log.Debugw("received slot blocks request", "peer", id, "Index", ei, "SC", sc)

	p.Events.SlotBlocksRequestReceived.Trigger(&SlotBlocksRequestReceivedEvent{
		ID: id,
		SI: ei,
	})
}

func (p *Protocol) processSlotBlocksStartPacket(packetSlotBlocksStart *wp.Packet_SlotBlocksStart, id identity.ID) {
	slotBlocksStart := packetSlotBlocksStart.SlotBlocksStart
	ei := slot.Index(slotBlocksStart.GetSI())

	p.log.Debugw("received slot blocks start", "peer", id, "Index", ei, "blocksCount", slotBlocksStart.GetBlocksCount())

	p.Events.SlotBlocksStart.Trigger(&SlotBlocksStartEvent{
		ID: id,
		SI: ei,
	})
}

func (p *Protocol) processSlotBlocksBatchPacket(packetSlotBlocksBatch *wp.Packet_SlotBlocksBatch, id identity.ID) {
	slotBlocksBatch := packetSlotBlocksBatch.SlotBlocksBatch
	ei := slot.Index(slotBlocksBatch.GetSI())

	blocksBytes := slotBlocksBatch.GetBlocks()
	p.log.Debugw("received slot blocks", "peer", id, "Index", ei, "blocksLen", len(blocksBytes))

	// TODO: TRIGGER UNPARSED BLOCK INSTEAD
	// for _, blockBytes := range blocksBytes {
	// 	block, err := p.parser.ParseBlock(nbr, blockBytes)
	// 	if err != nil {
	// 		p.log.Errorw("failed to deserialize block", "peer", nbr.Peer.ID(), "err", err)
	// 		return
	// 	}
	//
	// 	p.Events.SlotBlock.Trigger(&SlotBlockEvent{
	// 		SlotIndex:    ei,
	// 		Block: block,
	// 	})
	// }
}

func (p *Protocol) processSlotBlocksEndPacket(packetSlotBlocksEnd *wp.Packet_SlotBlocksEnd, id identity.ID) {
	slotBlocksBatch := packetSlotBlocksEnd.SlotBlocksEnd
	ei := slot.Index(slotBlocksBatch.GetSI())

	sc := new(commitment.Commitment)
	if _, err := sc.FromBytes(packetSlotBlocksEnd.SlotBlocksEnd.GetSC()); err != nil {
		p.log.Errorw("received slot blocks end: unable to deserialize commitment", "peer", id, "Index", ei, "err", err)
		return
	}

	p.log.Debugw("received slot blocks end", "peer", id, "Index", ei)

	eventToTrigger := &SlotBlocksEndEvent{
		ID:    id,
		SI:    ei,
		SC:    sc.ID(),
		Roots: new(commitment.Roots),
	}
	if _, err := eventToTrigger.Roots.FromBytes(packetSlotBlocksEnd.SlotBlocksEnd.GetRoots()); err != nil {
		p.log.Errorw("received slot blocks end: unable to deserialize roots", "peer", id, "Index", ei, "err", err)
		return
	}

	p.Events.SlotBlocksEnd.Trigger(eventToTrigger)
}
