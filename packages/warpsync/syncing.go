package warpsync

import (
	"github.com/iotaledger/goshimmer/packages/epoch"
	"github.com/iotaledger/goshimmer/packages/p2p"
	wp "github.com/iotaledger/goshimmer/packages/warpsync/warpsyncproto"
)

func (m *Manager) SyncRange(start, end epoch.Index) {
	for i := start; i <= end; i++ {
		m.RequestEpoch(i)
	}
}

func (m *Manager) RequestEpoch(index epoch.Index) {
	epochBlocksReq := &wp.EpochBlocksRequest{Epoch: int64(index)}
	packet := &wp.Packet{Body: &wp.Packet_EpochBlocksRequest{EpochBlocksRequest: epochBlocksReq}}
	m.send(packet)
}

func (m *Manager) processEpochRequestPacket(packetEpochRequest *wp.Packet_EpochBlocksRequest, nbr *p2p.Neighbor) {
	// TOOD
	/*
		var blkID tangle.BlockID
		_, err := blkID.Decode(packetEpochBlocks.BlockRequest.GetId())
		if err != nil {
			m.log.Debugw("invalid block id:", "err", err)
			return
		}

		blkBytes, err := m.loadBlockFunc(blkID)
		if err != nil {
			m.log.Debugw("error loading block", "blk-id", blkID, "err", err)
			return
		}

		// send the loaded block directly to the neighbor
		packet := &gp.Packet{Body: &gp.Packet_Block{Block: &gp.Block{Data: blkBytes}}}
		if err := nbr.GetStream(protocolID).WritePacket(packet); err != nil {
			nbr.Log.Warnw("Failed to send requested block back to the neighbor", "err", err)
			nbr.Close()
		}
		m.Events.BlockReceived.Trigger(&BlockReceivedEvent{Data: packetEpochRequest.Block.GetData(), Peer: nbr.Peer})
	*/
}

func (m *Manager) processEpochBlocksPacket(packetEpochBlocks *wp.Packet_EpochBlocks, nbr *p2p.Neighbor) {
	blocks := packetEpochBlocks.EpochBlocks.GetBlocks()
	for _, blockBytes := range blocks {
		m.Events.BlockReceived.Trigger(&BlockReceivedEvent{Data: blockBytes, Peer: nbr.Peer})
	}
}
