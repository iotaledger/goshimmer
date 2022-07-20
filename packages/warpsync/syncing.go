package warpsync

import (
	"context"
	"fmt"

	"github.com/celestiaorg/smt"
	"github.com/iotaledger/goshimmer/packages/core/epoch"
	"github.com/iotaledger/goshimmer/packages/core/notarization"
	"github.com/iotaledger/goshimmer/packages/core/tangle"
	"github.com/iotaledger/goshimmer/packages/node/database"
	"github.com/iotaledger/goshimmer/packages/node/p2p"
	wp "github.com/iotaledger/goshimmer/packages/warpsync/warpsyncproto"
	"github.com/iotaledger/hive.go/generics/lo"
	"github.com/iotaledger/hive.go/serix"
	"golang.org/x/crypto/blake2b"
)

type epochSyncBlock struct {
	ei    epoch.Index
	ec    epoch.EC
	block *tangle.Block
}

type epochSyncEnd struct {
	ei                epoch.Index
	ec                epoch.EC
	stateMutationRoot []byte
	stateRoot         []byte
	manaRoot          []byte
}

func (m *Manager) SyncRange(ctx context.Context, start, end epoch.Index, ecChain map[epoch.Index]*epoch.ECRecord) error {
	m.syncingInProgress = true
	defer func() { m.syncingInProgress = false }()

	epochBlocks := make(map[epoch.Index][]*tangle.Block)
	tangleRoots := make(map[epoch.Index]*smt.SparseMerkleTree)
	epochVerified := make(map[epoch.Index]bool)
	for ei := start; ei <= end; ei++ {
		db, _ := database.NewMemDB()
		tangleRoots[ei] = smt.NewSparseMerkleTree(db.NewStore(), db.NewStore(), lo.PanicOnErr(blake2b.New256(nil)))
		m.RequestEpoch(ei)
	}

	toReceive := end - start

readLoop:
	for {
		select {
		case epochSyncBlock := <-m.epochSyncBlockChan:
			ei := epochSyncBlock.ei
			if ei < start || ei > end {
				m.log.Warnf("received block for epoch %d while we are syncing from %d to %d", ei, start, end)
				continue
			}
			if epochSyncBlock.ec != notarization.EC(ecChain[ei]) {
				m.log.Warnf("received block on wrong EC chain")
				continue
			}
			if epochVerified[ei] {
				m.log.Warnf("received block for already-verified epoch %d", ei)
				continue
			}

			block := epochSyncBlock.block
			tangleRoots[ei].Update(block.IDBytes(), block.IDBytes())
			epochBlocks[ei] = append(epochBlocks[ei], block)
		case epochSyncEnd := <-m.epochSyncEndChan:
			ei := epochSyncEnd.ei
			ec := epochSyncEnd.ec
			if ei < start || ei > end {
				m.log.Warnf("received epoch terminator for epoch %d while we are syncing from %d to %d", ei, start, end)
				continue
			}
			if ec != notarization.EC(ecChain[ei]) {
				m.log.Warnf("received epoch terminator on wrong EC chain")
				continue
			}
			if epochVerified[ei] {
				m.log.Warnf("received termination for already-verified epoch %d", ei)
				continue
			}

			tangleRoot := tangleRoots[ei].Root()
			ecRecord := epoch.NewECRecord(ei)
			ecRecord.SetECR(notarization.ECR(
				epoch.NewMerkleRoot(tangleRoot),
				epoch.NewMerkleRoot(epochSyncEnd.stateMutationRoot),
				epoch.NewMerkleRoot(epochSyncEnd.stateRoot),
				epoch.NewMerkleRoot(epochSyncEnd.manaRoot),
			))
			ecRecord.SetPrevEC(notarization.EC(ecChain[ei-1]))

			if notarization.EC(ecChain[ei]) != notarization.EC(ecRecord) {
				return fmt.Errorf("epoch %d EC record is not correct", ei)
			}

			epochVerified[ei] = true

			toReceive--
			if toReceive == 0 {
				break readLoop
			}
		case <-ctx.Done():
			return fmt.Errorf("cancelled while syncing epoch range %d to %d", start, end)
		}
	}

	// We verified the epochs contents for the range, we can now store them.
	for ei := start; ei <= end; ei++ {
		for _, block := range epochBlocks[ei] {
			m.tangle.Storage.StoreBlock(block)
		}
	}

	return nil
}

func (m *Manager) RequestEpoch(index epoch.Index) {
	epochBlocksReq := &wp.EpochBlocksRequest{EI: int64(index)}
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
	if !m.syncingInProgress {
		return
	}
	for _, blockBytes := range packetEpochBlocks.EpochBlocks.GetBlocks() {
		block := new(tangle.Block)
		if _, err := serix.DefaultAPI.Decode(context.Background(), blockBytes, block); err != nil {
			return
		}
		m.epochSyncBlockChan <- &epochSyncBlock{
			ei:    epoch.Index(packetEpochBlocks.EpochBlocks.GetEI()),
			ec:    epoch.NewMerkleRoot(packetEpochBlocks.EpochBlocks.GetEC()),
			block: block,
		}
	}
}
