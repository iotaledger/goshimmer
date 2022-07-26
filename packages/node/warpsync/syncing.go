package warpsync

import (
	"context"
	"fmt"

	"github.com/celestiaorg/smt"
	"github.com/iotaledger/goshimmer/packages/core/epoch"
	"github.com/iotaledger/goshimmer/packages/core/tangle"
	"github.com/iotaledger/goshimmer/packages/node/database"
	"github.com/iotaledger/goshimmer/packages/node/p2p"
	wp "github.com/iotaledger/goshimmer/packages/node/warpsync/warpsyncproto"
	"github.com/iotaledger/goshimmer/plugins/epochstorage"
	"github.com/iotaledger/hive.go/autopeering/peer"
	"github.com/iotaledger/hive.go/generics/lo"
	"github.com/iotaledger/hive.go/serix"
	"golang.org/x/crypto/blake2b"
)

type epochSyncBlock struct {
	ei    epoch.Index
	ec    epoch.EC
	block *tangle.Block
	peer  *peer.Peer
}

type epochSyncEnd struct {
	ei                epoch.Index
	ec                epoch.EC
	stateMutationRoot epoch.MerkleRoot
	stateRoot         epoch.MerkleRoot
	manaRoot          epoch.MerkleRoot
}

type blockReceived struct {
	block *tangle.Block
	peer  *peer.Peer
}

func (m *Manager) SyncRange(ctx context.Context, start, end epoch.Index, ecChain map[epoch.Index]*epoch.ECRecord) error {
	if m.syncingInProgress {
		return fmt.Errorf("epoch syncing already in progress")
	}

	if m.isStopped {
		return fmt.Errorf("warpsync manager is stopped")
	}

	m.syncingInProgress = true
	defer func() { m.syncingInProgress = false }()

	epochBlocks := make(map[epoch.Index][]*blockReceived)
	tangleRoots := make(map[epoch.Index]*smt.SparseMerkleTree)
	epochVerified := make(map[epoch.Index]bool)
	for ei := start; ei <= end; ei++ {
		db, _ := database.NewMemDB()
		tangleRoots[ei] = smt.NewSparseMerkleTree(db.NewStore(), db.NewStore(), lo.PanicOnErr(blake2b.New256(nil)))
		m.requestEpoch(ei)
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
			if epochSyncBlock.ec != epoch.ComputeEC(ecChain[ei]) {
				m.log.Warnf("received block on wrong EC chain")
				continue
			}
			if epochVerified[ei] {
				m.log.Warnf("received block for already-verified epoch %d", ei)
				continue
			}
			if epochBlocks[ei] == nil {
				epochBlocks[ei] = make([]*blockReceived, 0)
			}

			block := epochSyncBlock.block
			tangleRoots[ei].Update(block.IDBytes(), block.IDBytes())
			epochBlocks[ei] = append(epochBlocks[ei], &blockReceived{
				block: block,
				peer:  epochSyncBlock.peer,
			})
		case epochSyncEnd := <-m.epochSyncEndChan:
			ei := epochSyncEnd.ei
			ec := epochSyncEnd.ec
			if ei < start || ei > end {
				m.log.Warnf("received epoch terminator for epoch %d while we are syncing from %d to %d", ei, start, end)
				continue
			}
			if ec != epoch.ComputeEC(ecChain[ei]) {
				m.log.Warnf("received epoch terminator on wrong EC chain")
				continue
			}
			if epochVerified[ei] {
				m.log.Warnf("received termination for already-verified epoch %d", ei)
				continue
			}

			tangleRoot := tangleRoots[ei].Root()
			ecRecord := epoch.NewECRecord(ei)
			ecRecord.SetECR(epoch.ComputeECR(
				epoch.NewMerkleRoot(tangleRoot),
				epochSyncEnd.stateMutationRoot,
				epochSyncEnd.stateRoot,
				epochSyncEnd.manaRoot,
			))
			ecRecord.SetPrevEC(epoch.ComputeEC(ecChain[ei-1]))

			if epoch.ComputeEC(ecChain[ei]) != epoch.ComputeEC(ecRecord) {
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

	// We verified the epochs contents for the range, we can now parse them.
	for ei := start; ei <= end; ei++ {
		for _, blockReceived := range epochBlocks[ei] {
			blockBytes, err := blockReceived.block.Bytes()
			if err != nil {
				m.log.Warnf("received block from %s failed to serialize: %s", blockReceived.peer.ID(), err)
			}
			m.tangle.ProcessGossipBlock(blockBytes, blockReceived.peer)
		}
	}

	return nil
}

func (m *Manager) requestEpoch(index epoch.Index) {
	epochBlocksReq := &wp.EpochBlocksRequest{EI: int64(index)}
	packet := &wp.Packet{Body: &wp.Packet_EpochBlocksRequest{EpochBlocksRequest: epochBlocksReq}}
	m.p2pManager.Send(packet, protocolID)
}

func (m *Manager) processEpochRequestPacket(packetEpochRequest *wp.Packet_EpochBlocksRequest, nbr *p2p.Neighbor) {
	ei := epoch.Index(packetEpochRequest.EpochBlocksRequest.GetEI())
	ec := epoch.NewMerkleRoot(packetEpochRequest.EpochBlocksRequest.GetEC())

	ecRecord, exists := epochstorage.GetEpochCommittment(ei)
	if !exists || ec != epoch.ComputeEC(ecRecord) {
		return
	}
	blockIDs := epochstorage.GetEpochBlockIDs(ei)

	for batchNum := 0; batchNum <= len(blockIDs)/m.blockBatchSize; batchNum++ {
		blocksBytes := make([][]byte, 0)
		for i := batchNum * m.blockBatchSize; i < len(blockIDs) && i < (batchNum+1)*m.blockBatchSize; i++ {
			m.tangle.Storage.Block(blockIDs[i]).Consume(func(block *tangle.Block) {
				blockBytes, err := block.Bytes()
				if err != nil {
					m.log.Errorf("failed to serialize block %s: %s", block.ID().String(), err)
					return
				}
				blocksBytes = append(blocksBytes, blockBytes)
			})
		}

		blocksRes := &wp.EpochBlocks{
			EI:     int64(ei),
			EC:     ec.Bytes(),
			Blocks: blocksBytes,
		}
		packet := &wp.Packet{Body: &wp.Packet_EpochBlocks{EpochBlocks: blocksRes}}

		m.p2pManager.Send(packet, protocolID, nbr.ID())
	}
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
			peer:  nbr.Peer,
			block: block,
		}
	}
}

func (m *Manager) processEpochBlocksEndPacket(packetEpochBlocksEnd *wp.Packet_EpochBlocksEnd, nbr *p2p.Neighbor) {
	if !m.syncingInProgress {
		return
	}
	m.epochSyncEndChan <- &epochSyncEnd{
		ei:                epoch.Index(packetEpochBlocksEnd.EpochBlocksEnd.GetEI()),
		ec:                epoch.NewMerkleRoot(packetEpochBlocksEnd.EpochBlocksEnd.GetEC()),
		stateMutationRoot: epoch.NewMerkleRoot(packetEpochBlocksEnd.EpochBlocksEnd.GetStateMutationRoot()),
		stateRoot:         epoch.NewMerkleRoot(packetEpochBlocksEnd.EpochBlocksEnd.GetStateRoot()),
		manaRoot:          epoch.NewMerkleRoot(packetEpochBlocksEnd.EpochBlocksEnd.GetManaRoot()),
	}
}
