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
	"golang.org/x/crypto/blake2b"
)

type epochSyncStart struct {
	ei          epoch.Index
	ec          epoch.EC
	blocksCount int64
}

type epochSyncBatchBlock struct {
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

func (m *Manager) SyncRange(ctx context.Context, startEpoch, endEpoch epoch.Index, startEC epoch.EC, ecChain map[epoch.Index]*epoch.ECRecord) error {
	if m.syncingInProgress {
		return fmt.Errorf("epoch syncing already in progress")
	}

	if m.isStopped {
		return fmt.Errorf("warpsync manager is stopped")
	}

	m.syncingInProgress = true
	defer func() { m.syncingInProgress = false }()

	tangleRoots := make(map[epoch.Index]*smt.SparseMerkleTree)
	epochSyncEnds := make(map[epoch.Index]*epochSyncEnd)

	// We do not request the start nor the ending epoch, as we know the beginning and the end of the chain.
	for ei := endEpoch - 1; ei > startEpoch; ei-- {
		db, _ := database.NewMemDB()
		tangleRoots[ei] = smt.NewSparseMerkleTree(db.NewStore(), db.NewStore(), lo.PanicOnErr(blake2b.New256(nil)))
		m.requestEpochBlocks(ei, ecChain[ei].ComputeEC())
	}

	// Counters to check termination conditions.
	epochsToReceive := endEpoch - startEpoch - 1
	epochBlocksLeft := make(map[epoch.Index]int64)

	m.log.Debug(">> START")

startReadLoop:
	for {
		select {
		case epochSyncStart := <-m.epochSyncStartChan:
			ei := epochSyncStart.ei
			ec := epochSyncStart.ec
			if ei <= startEpoch || ei > endEpoch-1 {
				m.log.Warnf("received epoch starter for epoch %d while we are syncing from %d to %d", ei, startEpoch, endEpoch)
				continue
			}
			if ec != ecChain[ei].ComputeEC() {
				m.log.Warnf("received epoch starter on wrong EC chain")
				continue
			}
			if _, exists := epochBlocksLeft[ei]; exists {
				m.log.Warnf("received starter for already-started epoch %d", ei)
				continue
			}

			epochBlocksLeft[ei] = epochSyncStart.blocksCount
			m.log.Debugw("read epoch block count", "EI", ei, "blockCount", epochBlocksLeft[ei])

			epochsToReceive--
			m.log.Debugf("epochs left %d", epochsToReceive)
			if epochsToReceive == 0 {
				break startReadLoop
			}
		case <-ctx.Done():
			return fmt.Errorf("cancelled while syncing epoch range %d to %d: %s", startEpoch, endEpoch, ctx.Err())
		}
	}

	epochsToReceive = 0
	for _, epochBlockCount := range epochBlocksLeft {
		if epochBlockCount > 0 {
			epochsToReceive++
		}
	}
	epochReceivedBlocks := make(map[epoch.Index]map[tangle.BlockID]*blockReceived)

	m.log.Debug(">> BLOCKS")

	for {
		if epochsToReceive == 0 {
			break
		}
		select {
		case epochSyncBlock := <-m.epochSyncBatchBlockChan:
			ei := epochSyncBlock.ei
			if ei <= startEpoch || ei > endEpoch-1 {
				m.log.Warnf("received block for epoch %d while we are syncing from %d to %d", ei, startEpoch, endEpoch)
				continue
			}
			if epochSyncBlock.ec != ecChain[ei].ComputeEC() {
				m.log.Warnf("received block on wrong EC chain")
				continue
			}
			if _, exists := epochSyncEnds[ei]; exists {
				m.log.Warnf("received block for already-ended epoch %d", ei)
				continue
			}
			if _, exists := epochReceivedBlocks[ei]; !exists {
				epochReceivedBlocks[ei] = make(map[tangle.BlockID]*blockReceived)
			}

			block := epochSyncBlock.block
			if _, exists := epochReceivedBlocks[ei][block.ID()]; exists {
				continue
			}

			m.log.Debugw("read block", "peer", epochSyncBlock.peer.ID(), "EI", ei, "blockID", block.ID())

			tangleRoots[ei].Update(block.IDBytes(), block.IDBytes())
			epochReceivedBlocks[ei][block.ID()] = &blockReceived{
				block: block,
				peer:  epochSyncBlock.peer,
			}

			epochBlocksLeft[ei]--
			m.log.Debugf("%d blocks left for epoch %d", epochBlocksLeft[ei], ei)
			if epochBlocksLeft[ei] == 0 {
				epochsToReceive--
				m.log.Debugf("%d epochs left", epochsToReceive)
			}
		case <-ctx.Done():
			return fmt.Errorf("cancelled while syncing epoch range %d to %d: %s", startEpoch, endEpoch, ctx.Err())
		}
	}

	// Counter to check termination conditions.
	epochsToReceive = endEpoch - startEpoch - 1

	m.log.Debug(">> END")

endReadLoop:
	for {
		select {
		case epochSyncEnd := <-m.epochSyncEndChan:
			ei := epochSyncEnd.ei
			ec := epochSyncEnd.ec
			if ei <= startEpoch || ei > endEpoch-1 {
				m.log.Warnf("received epoch terminator for epoch %d while we are syncing from %d to %d", ei, startEpoch, endEpoch)
				continue
			}
			if ec != ecChain[ei].ComputeEC() {
				m.log.Warnf("received epoch terminator on wrong EC chain")
				continue
			}
			if _, exists := epochSyncEnds[ei]; exists {
				m.log.Warnf("received termination for already-ended epoch %d", ei)
				continue
			}

			epochSyncEnds[ei] = epochSyncEnd
			m.log.Debugw("read epoch end", "EI", ei)

			epochsToReceive--
			m.log.Debugf("epochs left %d", epochsToReceive)
			if epochsToReceive == 0 {
				break endReadLoop
			}
		case <-ctx.Done():
			return fmt.Errorf("cancelled while syncing epoch range %d to %d: %s", startEpoch, endEpoch, ctx.Err())
		}
	}

	// Verify the epochs.
	for ei := endEpoch - 1; ei > startEpoch; ei-- {
		tangleRoot := tangleRoots[ei].Root()
		epochSyncEnd := epochSyncEnds[ei]
		ecRecord := epoch.NewECRecord(ei)
		ecRecord.SetECR(epoch.ComputeECR(
			epoch.NewMerkleRoot(tangleRoot),
			epochSyncEnd.stateMutationRoot,
			epochSyncEnd.stateRoot,
			epochSyncEnd.manaRoot,
		))

		if ei == startEpoch+1 {
			ecRecord.SetPrevEC(startEC)
		} else {
			ecRecord.SetPrevEC(ecChain[ei-1].ComputeEC())
		}

		if ecChain[ei].ComputeEC() != ecRecord.ComputeEC() {
			return fmt.Errorf("epoch %d EC record is not correct", ei)
		}
	}

	// We verified the epochs contents for the range, we can now parse them.
	for ei := startEpoch; ei <= endEpoch; ei++ {
		for _, blockReceived := range epochReceivedBlocks[ei] {
			blockBytes, err := blockReceived.block.Bytes()
			if err != nil {
				m.log.Warnf("received block from %s failed to serialize: %s", blockReceived.peer.ID(), err)
			}
			m.blockProcessorFunc(blockBytes, blockReceived.peer)
		}
	}

	m.log.Debugw("sync successful")

	return nil
}

func (m *Manager) requestEpochBlocks(ei epoch.Index, ec epoch.EC) {
	epochBlocksReq := &wp.EpochBlocksRequest{
		EI: int64(ei),
		EC: ec.Bytes(),
	}
	packet := &wp.Packet{Body: &wp.Packet_EpochBlocksRequest{EpochBlocksRequest: epochBlocksReq}}
	m.p2pManager.Send(packet, protocolID)

	m.log.Debugw("sent epoch blocks request", "EI", ei, "EC", ec.Base58())
}

func (m *Manager) processEpochBlocksRequestPacket(packetEpochRequest *wp.Packet_EpochBlocksRequest, nbr *p2p.Neighbor) {
	ei := epoch.Index(packetEpochRequest.EpochBlocksRequest.GetEI())
	ec := epoch.NewMerkleRoot(packetEpochRequest.EpochBlocksRequest.GetEC())

	m.log.Debugw("received epoch blocks request", "peer", nbr.Peer.ID(), "EI", ei, "EC", ec)

	ecRecord, exists := epochstorage.GetEpochCommittment(ei)
	if !exists || ec != ecRecord.ComputeEC() {
		m.log.Debugw("epoch blocks request rejected: unknown epoch or mismatching EC", "peer", nbr.Peer.ID(), "EI", ei, "EC", ec)
		return
	}
	blockIDs := epochstorage.GetEpochBlockIDs(ei)
	blocksCount := len(blockIDs)

	// Send epoch starter.
	{
		epochStartRes := &wp.EpochBlocksStart{
			EI:          int64(ei),
			EC:          ec.Bytes(),
			BlocksCount: int64(blocksCount),
		}
		packet := &wp.Packet{Body: &wp.Packet_EpochBlocksStart{EpochBlocksStart: epochStartRes}}

		m.p2pManager.Send(packet, protocolID, nbr.ID())

		m.log.Debugw("sent epoch blocks starter", "peer", nbr.Peer.ID(), "EI", ei, "blocksCount", blocksCount)
	}

	// Send epoch's blocks in batches.
	for batchNum := 0; batchNum <= len(blockIDs)/m.blockBatchSize; batchNum++ {
		blocksBytes := make([][]byte, 0)
		for i := batchNum * m.blockBatchSize; i < len(blockIDs) && i < (batchNum+1)*m.blockBatchSize; i++ {
			block, err := m.blockLoaderFunc(blockIDs[i])
			if err != nil {
				m.log.Errorf("failed to load block %s: %s", blockIDs[i], err)
				return
			}
			blockBytes, err := block.Bytes()
			if err != nil {
				m.log.Errorf("failed to serialize block %s: %s", block.ID().String(), err)
				return
			}
			blocksBytes = append(blocksBytes, blockBytes)
		}

		blocksBatchRes := &wp.EpochBlocksBatch{
			EI:     int64(ei),
			EC:     ec.Bytes(),
			Blocks: blocksBytes,
		}
		packet := &wp.Packet{Body: &wp.Packet_EpochBlocksBatch{EpochBlocksBatch: blocksBatchRes}}

		m.p2pManager.Send(packet, protocolID, nbr.ID())

		m.log.Debugw("sent epoch blocks batch", "peer", nbr.Peer.ID(), "EI", ei, "blocksLen", len(blocksBytes))
	}

	// Send epoch terminator.
	{
		roots := ecRecord.Roots()
		epochBlocksEnd := &wp.EpochBlocksEnd{
			EI:                int64(ei),
			EC:                ec.Bytes(),
			StateMutationRoot: roots.StateMutationRoot.Bytes(),
			StateRoot:         roots.StateRoot.Bytes(),
			ManaRoot:          roots.ManaRoot.Bytes(),
		}
		packet := &wp.Packet{Body: &wp.Packet_EpochBlocksEnd{EpochBlocksEnd: epochBlocksEnd}}
		m.p2pManager.Send(packet, protocolID, nbr.ID())

		m.log.Debugw("sent epoch blocks termination", "peer", nbr.Peer.ID(), "EI", ei, "EC", ec.Base58())
	}
}

func (m *Manager) processEpochBlocksStartPacket(packetEpochBlocksStart *wp.Packet_EpochBlocksStart, nbr *p2p.Neighbor) {
	if !m.syncingInProgress {
		return
	}

	epochBlocksStart := packetEpochBlocksStart.EpochBlocksStart
	ei := epoch.Index(epochBlocksStart.GetEI())
	m.log.Debugw("received epoch blocks start", "peer", nbr.Peer.ID(), "EI", ei)

	m.epochSyncStartChan <- &epochSyncStart{
		ei:          ei,
		ec:          epoch.NewMerkleRoot(epochBlocksStart.GetEC()),
		blocksCount: epochBlocksStart.GetBlocksCount(),
	}
}

func (m *Manager) processEpochBlocksBatchPacket(packetEpochBlocksBatch *wp.Packet_EpochBlocksBatch, nbr *p2p.Neighbor) {
	if !m.syncingInProgress {
		return
	}

	ei := epoch.Index(packetEpochBlocksBatch.EpochBlocksBatch.GetEI())
	blocksBytes := packetEpochBlocksBatch.EpochBlocksBatch.GetBlocks()
	m.log.Debugw("received epoch blocks", "peer", nbr.Peer.ID(), "EI", ei, "blocksLen", len(blocksBytes))

	for _, blockBytes := range blocksBytes {
		block := new(tangle.Block)
		if err := block.FromBytes(blockBytes); err != nil {
			m.log.Errorw("failed to deserialize block", "peer", nbr.Peer.ID(), "err", err)
			return
		}
		m.epochSyncBatchBlockChan <- &epochSyncBatchBlock{
			ei:    ei,
			ec:    epoch.NewMerkleRoot(packetEpochBlocksBatch.EpochBlocksBatch.GetEC()),
			peer:  nbr.Peer,
			block: block,
		}

		m.log.Debugw("write block", "blockID", block.ID())
	}
}

func (m *Manager) processEpochBlocksEndPacket(packetEpochBlocksEnd *wp.Packet_EpochBlocksEnd, nbr *p2p.Neighbor) {
	if !m.syncingInProgress {
		return
	}

	ei := epoch.Index(packetEpochBlocksEnd.EpochBlocksEnd.GetEI())
	m.log.Debugw("received epoch blocks end", "peer", nbr.Peer.ID(), "EI", ei)

	m.epochSyncEndChan <- &epochSyncEnd{
		ei:                ei,
		ec:                epoch.NewMerkleRoot(packetEpochBlocksEnd.EpochBlocksEnd.GetEC()),
		stateMutationRoot: epoch.NewMerkleRoot(packetEpochBlocksEnd.EpochBlocksEnd.GetStateMutationRoot()),
		stateRoot:         epoch.NewMerkleRoot(packetEpochBlocksEnd.EpochBlocksEnd.GetStateRoot()),
		manaRoot:          epoch.NewMerkleRoot(packetEpochBlocksEnd.EpochBlocksEnd.GetManaRoot()),
	}
}
